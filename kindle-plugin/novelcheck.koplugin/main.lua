--[[
NovelCheck plugin for KOReader: connect to your NovelCheck, see your Up Next
list, tap a book to download it, and report which books are on this device.

It signs in the way KOReader's progress sync does: your NovelCheck username
and the sync code from Profile -> KOReader setup.
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

local NovelCheck = WidgetContainer:extend{
    name = "novelcheck",
    is_doc_only = false,
}

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function NovelCheck:init()
    self.settings = LuaSettings:open(DataStorage:getSettingsDir() .. "/novelcheck.lua")
    self.ui.menu:registerToMainMenu(self)
end

function NovelCheck:addToMainMenu(menu_items)
    menu_items.novelcheck = {
        text = _("NovelCheck"),
        sorting_hint = "tools",
        sub_item_table = {
            { text = _("Up Next"), callback = function() self:showUpNext() end },
            { text = _("Report books on this device"), callback = function() self:reportDevice() end },
            { text = _("Device name..."), keep_menu_open = true, callback = function() self:deviceNameDialog() end },
            { text = _("Connect to NovelCheck..."), keep_menu_open = true, callback = function() self:connectDialog() end },
        },
    }
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
            { text = self.settings:readSetting("code") or "", hint = _("Sync code (Profile > KOReader setup)") },
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
                UIManager:show(InfoMessage:new{ text = _("Saved. Open NovelCheck > Up Next to try it.") })
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
            UIManager:show(InfoMessage:new{ text = _("Nothing to download: your Up Next has no books with a file.") })
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
            title = _("NovelCheck: Up Next"),
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

-- deviceName is what NovelCheck calls this e-reader (the model, unless set).
function NovelCheck:deviceName()
    local name = trim(self.settings:readSetting("device_name"))
    if name == "" then
        name = (Device.model or "KOReader device")
    end
    return name
end

function NovelCheck:deviceNameDialog()
    local InputDialog = require("ui/widget/inputdialog")
    local dialog
    dialog = InputDialog:new{
        title = _("Name of this device"),
        input = self:deviceName(),
        buttons = { {
            { text = _("Cancel"), callback = function() UIManager:close(dialog) end },
            { text = _("Save"), is_enter_default = true, callback = function()
                self.settings:saveSetting("device_name", trim(dialog:getInputText()))
                self.settings:flush()
                UIManager:close(dialog)
            end },
        } },
    }
    UIManager:show(dialog)
    dialog:onShowKeyboard()
end

local BOOK_EXT = { epub = true, pdf = true, mobi = true, azw3 = true, azw = true, fb2 = true, cbz = true, txt = true, djvu = true }
local MAX_BOOKS = 500

-- scanBooks lists the books under dir (without folder names): file name,
-- format, size and KOReader's fingerprint of the file.
function NovelCheck:scanBooks(dir, books, depth)
    if #books >= MAX_BOOKS or depth > 4 then return end
    for entry in lfs.dir(dir) do
        if entry:sub(1, 1) ~= "." and not entry:match("%.sdr$") then
            local path = dir .. "/" .. entry
            local attr = lfs.attributes(path)
            if attr and attr.mode == "directory" then
                self:scanBooks(path, books, depth + 1)
            elseif attr and attr.mode == "file" then
                local ext = entry:match("%.([^.]+)$")
                if ext and BOOK_EXT[ext:lower()] then
                    local ok, sum = pcall(util.partialMD5, path)
                    table.insert(books, {
                        filename = entry,
                        title = (entry:gsub("%.[^.]+$", ""):gsub("[-_]", " ")),
                        format = ext:lower(),
                        size = attr.size,
                        md5 = ok and sum or "",
                    })
                end
            end
            if #books >= MAX_BOOKS then return end
        end
    end
end

function NovelCheck:reportDevice()
    if not self:isConnected() then
        self:connectDialog()
        return
    end
    NetworkMgr:runWhenOnline(function()
        UIManager:show(InfoMessage:new{ text = _("Listing the books on this device..."), timeout = 1 })
        UIManager:nextTick(function()
            local books = {}
            local ok = pcall(self.scanBooks, self, self:downloadDir(), books, 0)
            if not ok then
                UIManager:show(InfoMessage:new{ text = _("Could not read the books folder.") })
                return
            end
            local body = json.encode{ device = self:deviceName(), books = books }
            local code, answer = self:request("/kosync/plugin/device", nil, body)
            if code == 200 then
                local good, data = pcall(json.decode, answer)
                local text = _("Reported to NovelCheck:") .. " " .. tostring(#books) .. " " .. _("books")
                if good and type(data) == "table" and data.matched then
                    text = text .. "\n" .. tostring(data.matched) .. " " .. _("are in your NovelCheck library.")
                end
                if #books >= MAX_BOOKS then
                    text = text .. "\n" .. _("(Only the first 500 books are listed.)")
                end
                UIManager:show(InfoMessage:new{ text = text })
            elseif code == 401 then
                UIManager:show(InfoMessage:new{ text = _("NovelCheck did not accept the username or sync code.") })
            else
                UIManager:show(InfoMessage:new{ text = _("Could not report to NovelCheck.") .. "\n" .. tostring(code or answer) })
            end
        end)
    end)
end

function NovelCheck:downloadDir()
    local dir = G_reader_settings:readSetting("download_dir") or G_reader_settings:readSetting("home_dir")
    if dir and lfs.attributes(dir, "mode") == "directory" then
        return dir
    end
    return DataStorage:getDataDir()
end

function NovelCheck:download(book)
    local name = book.url:match("([^/]+)$") or ("book-" .. tostring(book.id) .. "." .. tostring(book.format))
    local path = self:downloadDir() .. "/" .. name
    if lfs.attributes(path, "mode") == "file" then
        UIManager:show(InfoMessage:new{ text = _("Already on this device:") .. "\n" .. path })
        return
    end
    UIManager:show(InfoMessage:new{ text = _("Downloading..."), timeout = 1 })
    UIManager:nextTick(function()
        local code = self:request(book.url, path)
        if code == 200 then
            UIManager:show(InfoMessage:new{ text = _("Downloaded to:") .. "\n" .. path })
        else
            os.remove(path)
            UIManager:show(InfoMessage:new{ text = _("The download failed.") .. "\n" .. tostring(code) })
        end
    end)
end

return NovelCheck
