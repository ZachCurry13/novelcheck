--[[
NovelCheck plugin for KOReader: connect to your NovelCheck, see your Nightstand
list, tap a book to download it, and report which books are on this device.

It signs in the way KOReader's progress sync does: your NovelCheck username
and the sync code from Settings -> KOReader setup.
]]

local DataStorage = require("datastorage")
local Device = require("device")
local InfoMessage = require("ui/widget/infomessage")
local LuaSettings = require("luasettings")
local Menu = require("ui/widget/menu")
local MultiInputDialog = require("ui/widget/multiinputdialog")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local WidgetContainer = require("ui/widget/container/widgetcontainer")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")
local ltn12 = require("ltn12")
local socket = require("socket")
local socketutil = require("socketutil")
local http = require("socket.http")
local https = require("ssl.https")
local md5 = require("ffi/sha2").md5
local util = require("util")
local ConfirmBox = require("ui/widget/confirmbox")
local DeviceBooks = require("devicebooks")

local VERSION = "1.41.1"
-- The folder this plugin runs from (config.lua from NovelCheck's download sits there).
local PLUGIN_DIR = (debug.getinfo(1, "S").source or ""):match("^@(.*)/[^/]*$")

local NovelCheck = WidgetContainer:extend{
    name = "novelcheck",
    is_doc_only = false,
}

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function NovelCheck:init()
    self.settings = LuaSettings:open(DataStorage:getSettingsDir() .. "/novelcheck.lua")
    self:importConfig()
    self.ui.menu:registerToMainMenu(self)
    if self.settings:readSetting("offer_setup") and self:isConnected() then
        -- First start after installing the downloaded plugin: offer the one-tap setup once.
        self.settings:saveSetting("offer_setup", nil)
        self.settings:flush()
        UIManager:scheduleIn(4, function() self:easySetup() end)
    end
    if self:isConnected() and self.settings:readSetting("reported_version") ~= VERSION then
        -- A new plugin version: tell NovelCheck once Wi-Fi is up (it shows the version in Profile).
        UIManager:scheduleIn(10, function()
            if NetworkMgr:isConnected() and self:autoEnabled("auto_report", true) then
                self:reportQuietly()
            end
        end)
    end
end

-- importConfig connects the plugin from the config.lua that NovelCheck puts in
-- its download (server, username and sync code), so nothing is typed here.
function NovelCheck:importConfig()
    if self:isConnected() or not PLUGIN_DIR then
        return
    end
    local ok, cfg = pcall(dofile, PLUGIN_DIR .. "/config.lua")
    if ok and type(cfg) == "table" and cfg.server and cfg.user and cfg.code then
        self.settings:saveSetting("server", (trim(cfg.server):gsub("/+$", "")))
        self.settings:saveSetting("user", trim(cfg.user))
        self.settings:saveSetting("code", trim(cfg.code))
        self.settings:saveSetting("offer_setup", true)
        self.settings:flush()
    end
end

function NovelCheck:addToMainMenu(menu_items)
    menu_items.novelcheck = {
        text = _("NovelCheck"),
        sorting_hint = "tools",
        sub_item_table = {
            { text = _("Set up everything"), callback = function() self:easySetup() end },
            { text = _("Nightstand"), callback = function() self:showUpNext() end },
            { text = _("Download all Nightstand"), callback = function() self:downloadAll() end },
            { text = _("Sync reading statistics now"), callback = function() self:syncStatsNow() end },
            { text = _("On this device"), callback = function() DeviceBooks.show(self) end },
            { text = _("Delete finished books..."), callback = function() DeviceBooks.removeFinished(self) end },
            { text = _("Tidy book names"), callback = function() self:tidyNames() end },
            { text = _("Report books on this device"), callback = function() self:reportDevice() end },
            { text = _("Automatic"), sub_item_table = {
                { text = _("Report books when online"), keep_menu_open = true,
                  checked_func = function() return self:autoEnabled("auto_report", true) end,
                  callback = function() self:toggleAuto("auto_report", true) end },
                { text = _("Download Nightstand when online"), keep_menu_open = true,
                  checked_func = function() return self:autoEnabled("auto_download", false) end,
                  callback = function() self:toggleAuto("auto_download", false) end },
                { text = _("Sync reading statistics when online"), keep_menu_open = true,
                  checked_func = function() return self:autoEnabled("auto_stats", false) end,
                  callback = function() self:toggleAuto("auto_stats", false) end },
            } },
            { text = _("Device name..."), keep_menu_open = true, callback = function() self:deviceNameDialog() end },
            { text = _("Connect to NovelCheck..."), keep_menu_open = true, callback = function() self:connectDialog() end },
            { text = _("Update plugin"), callback = function() self:checkUpdate(false) end },
            { text = _("About"), keep_menu_open = true, callback = function() self:showAbout() end },
        },
    }
end

-- About: the version, which NovelCheck this is connected to, and the project
-- on GitHub (KOReader has no browser, so the link is shown as a QR code).
local GITHUB_URL = "https://github.com/ZachCurry13/novelcheck/tree/main/kindle-plugin"

function NovelCheck:showAbout()
    local ButtonDialog = require("ui/widget/buttondialog")
    local dialog
    dialog = ButtonDialog:new{
        title = "NovelCheck plugin " .. VERSION .. "\n" .. tostring(self.settings:readSetting("server") or _("Not connected"))
            .. "\n\n" .. GITHUB_URL,
        buttons = { {
            { text = _("GitHub (QR code)"), callback = function()
                UIManager:close(dialog)
                local ok, QRMessage = pcall(require, "ui/widget/qrmessage")
                if ok then
                    local shown = pcall(function()
                        UIManager:show(QRMessage:new{
                            text = GITHUB_URL,
                            width = Device.screen:getWidth() * 0.8,
                            height = Device.screen:getHeight() * 0.8,
                        })
                    end)
                    if shown then
                        return
                    end
                end
                UIManager:show(InfoMessage:new{ text = GITHUB_URL })
            end },
            { text = _("Close"), callback = function() UIManager:close(dialog) end },
        } },
    }
    UIManager:show(dialog)
end

-- Automatic work: KOReader tells plugins when the device goes online. At most
-- every few hours the plugin then reports what is on the device (NovelCheck adds
-- those books to your Nightstand) and, if switched on, downloads new Nightstand books.
local AUTO_INTERVAL = 6 * 3600

function NovelCheck:autoEnabled(key, default)
    local v = self.settings:readSetting(key)
    if v == nil then
        return default
    end
    return v and true or false
end

function NovelCheck:toggleAuto(key, default)
    self.settings:saveSetting(key, not self:autoEnabled(key, default))
    self.settings:flush()
end

function NovelCheck:onNetworkConnected()
    if not self:isConnected() then
        return
    end
    if not (self:autoEnabled("auto_report", true) or self:autoEnabled("auto_download", false) or self:autoEnabled("auto_stats", false)) then
        return
    end
    local now = os.time()
    local updated = self:autoEnabled("auto_report", true) and self.settings:readSetting("reported_version") ~= VERSION
    if not updated and now - (tonumber(self.settings:readSetting("last_auto")) or 0) < AUTO_INTERVAL then
        return
    end
    self.settings:saveSetting("last_auto", now)
    self.settings:flush()
    UIManager:scheduleIn(5, function() self:autoRun() end)
end

function NovelCheck:autoRun()
    if self:autoEnabled("auto_download", false) then
        self:downloadAll(true)
    end
    if self:autoEnabled("auto_report", true) then
        self:reportQuietly()
    end
    if self:autoEnabled("auto_stats", false) then
        self:syncStats()
    end
    self:checkUpdate(true)
end

function NovelCheck:isConnected()
    return trim(self.settings:readSetting("server")) ~= ""
        and trim(self.settings:readSetting("user")) ~= ""
        and trim(self.settings:readSetting("code")) ~= ""
end

function NovelCheck:connectDialog()
    local dialog
    dialog = MultiInputDialog:new{
        title = _("Connect to NovelCheck"),
        fields = {
            { text = self.settings:readSetting("server") or "", hint = _("Address, like https://novelcheck.example.com") },
            { text = self.settings:readSetting("user") or "", hint = _("Your NovelCheck username") },
            { text = self.settings:readSetting("code") or "", hint = _("Sync code (Settings > KOReader setup)") },
        },
        buttons = { {
            { text = _("Cancel"), callback = function() UIManager:close(dialog) end },
            { text = _("Save"), is_enter_default = true, callback = function()
                local f = dialog:getFields()
                self.settings:saveSetting("server", (trim(f[1]):gsub("/+$", "")))
                self.settings:saveSetting("user", trim(f[2]))
                self.settings:saveSetting("code", trim(f[3]))
                self.settings:flush()
                UIManager:close(dialog)
                UIManager:show(InfoMessage:new{ text = _("Saved. Open NovelCheck > Nightstand to try it.") })
            end },
        } },
    }
    UIManager:show(dialog)
    dialog:onShowKeyboard()
end

-- request fetches path from NovelCheck. With file, the body is saved there;
-- otherwise it is returned as text. It returns the HTTP code and the body.
function NovelCheck:request(path, file, post_body)
    local url = self.settings:readSetting("server") .. path
    local fetch = url:match("^https") and https.request or http.request
    local chunks = {}
    local sink
    if file then
        sink = socketutil.file_sink(io.open(file, "wb"))
    else
        sink = socketutil.table_sink(chunks)
    end
    socketutil:set_timeout(15, file and 300 or 30)
    local ok, code = pcall(function()
        local headers = {
            ["x-auth-user"] = trim(self.settings:readSetting("user")),
            ["x-auth-key"] = md5(trim(self.settings:readSetting("code"))),
            ["accept"] = "application/json",
        }
        local source
        if post_body then
            headers["content-type"] = "application/json"
            headers["content-length"] = tostring(#post_body)
            source = ltn12.source.string(post_body)
        end
        return socket.skip(1, fetch{
            url = url,
            method = post_body and "POST" or "GET",
            headers = headers,
            source = source,
            sink = sink,
        })
    end)
    socketutil:reset_timeout()
    if not ok then
        return nil, tostring(code)
    end
    return code, table.concat(chunks)
end

function NovelCheck:showUpNext()
    if not self:isConnected() then
        self:connectDialog()
        return
    end
    NetworkMgr:runWhenOnline(function()
        local code, body = self:request("/kosync/plugin/up-next")
        if code == 401 then
            UIManager:show(InfoMessage:new{ text = _("NovelCheck did not accept the username or sync code. Check them under Connect to NovelCheck.") })
            return
        elseif code ~= 200 then
            UIManager:show(InfoMessage:new{ text = _("Could not reach NovelCheck.") .. "\n" .. tostring(code or body) })
            return
        end
        local ok, data = pcall(json.decode, body)
        if not ok or type(data) ~= "table" or type(data.books) ~= "table" then
            UIManager:show(InfoMessage:new{ text = _("NovelCheck answered something unexpected.") })
            return
        end
        if #data.books == 0 then
            UIManager:show(InfoMessage:new{ text = _("Nothing to download: your Nightstand has no books with a file.") })
            return
        end
        local items = {}
        for _i, b in ipairs(data.books) do
            local line = b.title
            if b.author and b.author ~= "" then
                line = line .. " - " .. b.author
            end
            table.insert(items, {
                text = line,
                mandatory = (b.status == "reading") and _("reading") or nil,
                callback = function() self:download(b) end,
            })
        end
        self.menu = Menu:new{
            title = _("NovelCheck: Nightstand"),
            item_table = items,
            is_borderless = true,
            is_popout = false,
            covers_fullscreen = true,
            width = Device.screen:getWidth(),
            height = Device.screen:getHeight(),
            close_callback = function() self.menu = nil end,
        }
        UIManager:show(self.menu)
    end)
end

-- bookPath is where a book from your Nightstand is saved on this device.
function NovelCheck:bookPath(book)
    local name = book.url:match("([^/]+)$") or ("book-" .. tostring(book.id) .. "." .. tostring(book.format))
    return self:downloadDir() .. "/" .. name
end

-- fetchBook downloads one book; it returns true, or false and the HTTP code.
function NovelCheck:fetchBook(book)
    local path = self:bookPath(book)
    local code = self:request(book.url, path)
    if code == 200 then
        return true
    end
    os.remove(path)
    return false, code
end

function NovelCheck:download(book)
    local path = self:bookPath(book)
    if lfs.attributes(path, "mode") == "file" then
        UIManager:show(InfoMessage:new{ text = _("Already on this device:") .. "\n" .. path })
        return
    end
    UIManager:show(InfoMessage:new{ text = _("Downloading..."), timeout = 1 })
    UIManager:nextTick(function()
        local ok, code = self:fetchBook(book)
        if ok then
            UIManager:show(InfoMessage:new{ text = _("Downloaded to:") .. "\n" .. path })
        else
            UIManager:show(InfoMessage:new{ text = _("The download failed.") .. "\n" .. tostring(code) })
        end
    end)
end

-- downloadAll gets every book on your Nightstand that is not on this device yet.
function NovelCheck:downloadAll(quiet)
    if not self:isConnected() then
        if not quiet then
            self:connectDialog()
        end
        return
    end
    NetworkMgr:runWhenOnline(function()
        local code, body = self:request("/kosync/plugin/up-next")
        local ok, data = pcall(json.decode, body or "")
        if code ~= 200 or not ok or type(data) ~= "table" or type(data.books) ~= "table" then
            if not quiet then
                UIManager:show(InfoMessage:new{ text = _("Could not read your Nightstand list.") .. "\n" .. tostring(code) })
            end
            return
        end
        local todo = {}
        for _i, b in ipairs(data.books) do
            if lfs.attributes(self:bookPath(b), "mode") ~= "file" then
                table.insert(todo, b)
            end
        end
        if #todo == 0 then
            if not quiet then
                UIManager:show(InfoMessage:new{ text = _("Everything on your Nightstand is already on this device.") })
            end
            return
        end
        if not quiet then
            UIManager:show(InfoMessage:new{ text = _("Downloading") .. " " .. tostring(#todo) .. " " .. _("books..."), timeout = 2 })
        end
        UIManager:nextTick(function()
            local got, failed = 0, 0
            for _i, b in ipairs(todo) do
                if self:fetchBook(b) then
                    got = got + 1
                else
                    failed = failed + 1
                end
            end
            self:reportQuietly()
            local text = tostring(got) .. " " .. _("books downloaded.")
            if failed > 0 then
                text = text .. "\n" .. tostring(failed) .. " " .. _("failed.")
            end
            if not quiet or got > 0 then
                UIManager:show(InfoMessage:new{ text = text })
            end
        end)
    end)
end

require("setup")(NovelCheck, { VERSION = VERSION, PLUGIN_DIR = PLUGIN_DIR })
require("tidynames")(NovelCheck, { VERSION = VERSION, PLUGIN_DIR = PLUGIN_DIR })
require("updater")(NovelCheck, { VERSION = VERSION, PLUGIN_DIR = PLUGIN_DIR })
require("devicereport")(NovelCheck, { VERSION = VERSION, PLUGIN_DIR = PLUGIN_DIR })

return NovelCheck
