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
-- A module that fails to load must not take the whole plugin with it: the error is kept
-- (shown by Developer tools, and written to LOAD_ERROR.txt in the plugin folder).
local load_errors = {}
-- The plugin's own files are loaded by path: KOReader has modules of its own with plain names
-- like "search" or "setup", and require() could hand back one of those instead.
local PLUGIN_DIR = (debug.getinfo(1, "S").source or ""):match("^@(.*)/[^/]*$")
local function loadOwn(name)
    if PLUGIN_DIR then
        local chunk, err = loadfile(PLUGIN_DIR .. "/" .. name .. ".lua")
        if not chunk then
            return false, err
        end
        return pcall(chunk)
    end
    return pcall(require, name)
end
local okDB, DeviceBooks = loadOwn("devicebooks")
if not okDB then
    table.insert(load_errors, "devicebooks: " .. tostring(DeviceBooks))
    DeviceBooks = { show = function() end, removeFinished = function() end }
end

local VERSION = "1.63.0"
-- PLUGIN_DIR, the folder this plugin runs from (config.lua from NovelCheck's download sits there), is set above.

local NovelCheck = WidgetContainer:extend{
    name = "novelcheck",
    is_doc_only = false,
    load_errors = load_errors,
}

-- Until the developer tools load (debuglog.lua), logging does nothing.
function NovelCheck:dlog() end
function NovelCheck:trackMenu() end
function NovelCheck:developerMenu() return { text = _("Developer tools"), callback = function() end } end

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function NovelCheck:init()
    self.settings = LuaSettings:open(DataStorage:getSettingsDir() .. "/novelcheck.lua")
    self:importConfig()
    self.ui.menu:registerToMainMenu(self)
    -- Kindle-store books KOReader cannot open are moved out of its library folder first (locked.lua): opening one
    -- for a cover or a title can make KOReader stop. Once per start, quick (file names and a few header bytes).
    if not NovelCheck._rescued and self.rescueLockedBooks then
        NovelCheck._rescued = true
        pcall(self.rescueLockedBooks, self)
    end
    -- A shelf for the Bookshelf plugin, if it is installed (bookshelfsrc.lua); never allowed to stop the plugin.
    pcall(function() if self.offerBookshelf then self:offerBookshelf() end end)
    -- The stars window at the end of a book (finishbook.lua); never allowed to stop the plugin either.
    pcall(function() if self.hookEndOfBook then self:hookEndOfBook() end end)
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
    local entry = {
        text = _("NovelCheck"),
        sorting_hint = "tools",
        sub_item_table = {
            { text = _("Search"), callback = function() self:searchLibrary() end },
            { text = _("Nightstand"), callback = function() self:showUpNext() end },
            { text = _("Download Nightstand"), callback = function() self:downloadAll() end },
            { text = _("New"), callback = function() self:newInLibrary() end },
            { text = _("On this device"), callback = function() DeviceBooks.show(self) end },
            { text = _("Finish this book…"), callback = function() if self.finishBookDialog then self:finishBookDialog() end end },
            { text = _("Sync now"), callback = function() self:syncNow() end },
            { text = _("Kindle care"), sub_item_table = {
                { text = _("Back up now"), callback = function() if self.backupKindle then self:backupKindle(false) end end },
                { text = _("Tidy books…"), callback = function() if self.tidyBooks then self:tidyBooks() end end },
                { text = _("Undo tidy"), callback = function() if self.undoTidy then self:undoTidy() end end },
                { text = _("Tidy names"), callback = function() self:tidyNames() end },
                { text = _("Delete finished books…"), callback = function() DeviceBooks.removeFinished(self) end },
                { text = _("Delete crash logs…"), callback = function() if self.cleanCrashLogs then self:cleanCrashLogs() end end },
            } },
            { text = _("Automatic"), sub_item_table = {
                self:autoItem(_("Report books"), "auto_report", true),
                self:autoItem(_("Download Nightstand"), "auto_download", false),
                self:autoItem(_("Reading stats"), "auto_stats", false),
                self:autoItem(_("Highlights"), "auto_highlights", true),
                self:autoItem(_("Daily backup (on the charger)"), "auto_backup", true),
                self:autoItem(_("Delete crash logs"), "auto_crashlogs", true),
                self:autoItem(_("Ask for stars at the end"), "auto_finish", true),
            } },
            { text = _("Settings"), sub_item_table = {
                { text = _("Set up"), callback = function() self:easySetup() end },
                { text = _("Connect…"), keep_menu_open = true, callback = function() self:connectDialog() end },
                { text = _("Device name…"), keep_menu_open = true, callback = function() self:deviceNameDialog() end },
                { text = _("Update"), callback = function() self:checkUpdate(false) end },
                { text = _("Stop current job"), callback = function() if self.stopJob then self:stopJob() end end },
                { text = _("Resume automatic jobs"), callback = function() self:resumeAutomatic() end },
                self:developerMenu(),
                { text = _("About"), keep_menu_open = true, callback = function() self:showAbout() end },
            } },
        },
    }
    self:trackMenu(entry.sub_item_table)
    menu_items.novelcheck = entry
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
    -- tasks asked for in NovelCheck are looked at on every connection (at most every 15 minutes): it is one small request
    local t = os.time()
    if self.runKindleTasks and t - (self._tasks_last or 0) > 900 then
        self._tasks_last = t
        UIManager:scheduleIn(8, function() pcall(self.runKindleTasks, self, true) end)
    end
    if not (self:autoEnabled("auto_report", true) or self:autoEnabled("auto_download", false) or self:autoEnabled("auto_stats", false)
        or self:autoEnabled("auto_highlights", true) or self:autoEnabled("auto_backup", true) or self:autoEnabled("auto_crashlogs", true)) then
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
    -- A run that never finished (the Kindle froze or was restarted) pauses the automatic jobs until you resume them,
    -- so a job that freezes the Kindle can never freeze it again by itself at the next start.
    if self.settings:readSetting("auto_paused") then
        return
    end
    if (tonumber(self.settings:readSetting("auto_inflight")) or 0) > 0 then
        self.settings:saveSetting("auto_paused", true)
        self.settings:saveSetting("auto_inflight", 0)
        self.settings:flush()
        UIManager:show(InfoMessage:new{ text = _("The automatic jobs stopped early last time, so they're paused.") .. "\n"
            .. _("Turn them back on with NovelCheck > Settings > Resume automatic jobs."), timeout = 15 })
        return
    end
    self.settings:saveSetting("auto_inflight", os.time())
    self.settings:flush()
    local day = 22 * 3600
    -- the jobs start a little apart (each one works a few items at a time), the quick ones first
    local jobs = {
        { 5, function() if self.runKindleTasks then self:runKindleTasks(true) end end }, -- what was asked for in NovelCheck
        { 10, function() if self.deleteCrashLogsQuietly and self:autoEnabled("auto_crashlogs", true) then self:deleteCrashLogsQuietly() end end },
        { 20, function() if self:autoEnabled("auto_report", true) then self:reportQuietly() end end },
        { 70, function() if self.sendHighlights and self:autoEnabled("auto_highlights", true) then self:sendHighlights(true) end end },
        { 100, function() if self:autoEnabled("auto_stats", false) then self:syncStats() end end },
        { 130, function() if self:autoEnabled("auto_download", false) then self:downloadAll(true) end end },
        { 160, function()
            local charging = true -- the automatic backup only runs on the charger (a long job)
            pcall(function()
                if Device.powerd and Device.powerd.isCharging then
                    charging = Device.powerd:isCharging() and true or false
                end
            end)
            if self.backupKindle and self:autoEnabled("auto_backup", true) and charging and os.time() - (tonumber(self.settings:readSetting("last_backup")) or 0) > day then
                self:backupKindle(true)
            end
        end },
        { 190, function() self:checkUpdate(true) end },
    }
    for n, j in ipairs(jobs) do
        UIManager:scheduleIn(j[1], function()
            self:dlog("step", "automatic job " .. n .. " starts") -- the last such line before a freeze names the job
            local ok, err = pcall(j[2])
            if not ok then
                self:dlog("error", "automatic job: " .. tostring(err))
            end
        end)
    end
    -- the Kindle answered for five minutes after the jobs began: the run did not freeze it
    UIManager:scheduleIn(300, function()
        self.settings:saveSetting("auto_inflight", 0)
        self.settings:flush()
    end)
end

-- autoItem is one switch of the Automatic menu (done when the Kindle is online).
function NovelCheck:autoItem(text, key, default)
    return { text = text, keep_menu_open = true,
        checked_func = function() return self:autoEnabled(key, default) end,
        callback = function() self:toggleAuto(key, default) end }
end

-- syncNow does at once what the automatic jobs do when online, except the backup: the tasks asked for in NovelCheck,
-- the book report, highlights and reading statistics. Each works quietly and logs its own trouble.
function NovelCheck:syncNow()
    if not self:isConnected() then
        UIManager:show(InfoMessage:new{ text = _("Not connected. Open NovelCheck > Settings > Connect.") })
        return
    end
    UIManager:show(InfoMessage:new{ text = _("Syncing with NovelCheck…"), timeout = 3 })
    local jobs = {
        function() if self.runKindleTasks then self:runKindleTasks(true) end end,
        function() self:reportQuietly() end,
        function() if self.sendHighlights then self:sendHighlights(true) end end,
        function() self:syncStats() end,
    }
    for n, job in ipairs(jobs) do
        UIManager:scheduleIn(n * 4, function()
            local ok, err = pcall(job)
            if not ok then
                self:dlog("error", "sync now: " .. tostring(err))
            end
        end)
    end
end

function NovelCheck:resumeAutomatic()
    self.settings:saveSetting("auto_paused", false)
    self.settings:saveSetting("auto_inflight", 0)
    self.settings:flush()
    UIManager:show(InfoMessage:new{ text = _("The automatic jobs are on again."), timeout = 3 })
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
            { text = self.settings:readSetting("server_alt") or "", hint = _("Backup address (optional): tried when the first can't be reached") },
            { text = self.settings:readSetting("server_home") or "", hint = _("Home address (optional), like http://192.168.1.20:8080: used first when the Kindle is home") },
        },
        buttons = { {
            { text = _("Cancel"), callback = function() UIManager:close(dialog) end },
            { text = _("Save"), is_enter_default = true, callback = function()
                local f = dialog:getFields()
                self.settings:saveSetting("server", (trim(f[1]):gsub("/+$", "")))
                self.settings:saveSetting("user", trim(f[2]))
                self.settings:saveSetting("code", trim(f[3]))
                self.settings:saveSetting("server_alt", (trim(f[4]):gsub("/+$", "")))
                self.settings:saveSetting("server_home", (trim(f[5] or ""):gsub("/+$", "")))
                self.home_seen = nil
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
    -- The home address first when it answers (homeaddr.lua), then the main address, then the backup address.
    local list = self.addresses and self:addresses() or { self.settings:readSetting("server"), trim(self.settings:readSetting("server_alt")) }
    local code, body
    for i, base in ipairs(list) do
        if base and base ~= "" then
            code, body = self:requestAt(base, path, file, post_body)
            -- "cannot assign requested address" and "network is unreachable" mean Wi-Fi had no address yet (the Kindle
            -- just woke or is still joining): wait a moment and try once more before giving up.
            if type(code) ~= "number" and (tostring(body):find("assign requested address", 1, true) or tostring(body):find("unreachable", 1, true)) then
                socket.sleep(4)
                code, body = self:requestAt(base, path, file, post_body)
            end
            if type(code) == "number" then
                return code, body
            end
            if i == 1 and self.homeFailed and base == trim(self.settings:readSetting("server_home")) then
                self:homeFailed()
            end
        end
    end
    return code, body
end

function NovelCheck:requestAt(base, path, file, post_body)
    local url = base .. path
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
        self:dlog("net", (post_body and "POST " or "GET ") .. path .. " failed: " .. tostring(code))
        return nil, tostring(code)
    end
    local text = table.concat(chunks)
    self:dlog("net", (post_body and "POST " or "GET ") .. path .. " -> " .. tostring(code)
        .. (((type(code) == "number" and code >= 400) or (self.verbose and self:verbose() and code ~= 200)) and (" " .. text:sub(1, 200)) or ""))
    return code, text
end

function NovelCheck:showUpNext()
    if not self:isConnected() then
        self:connectDialog()
        return
    end
    NetworkMgr:runWhenOnline(function()
        local code, body = self:request("/kosync/plugin/up-next")
        if code == 401 then
            UIManager:show(InfoMessage:new{ text = _("Wrong username or sync code. Check Connect to NovelCheck.") })
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
        local got, failed = 0, 0
        self:runSteps(_("Downloading"), todo, function(b)
            if self:fetchBook(b) then
                got = got + 1
            else
                failed = failed + 1
            end
        end, { batch = 1, quiet = quiet }, function(info)
            self:reportQuietly()
            local text = tostring(got) .. " " .. _("books downloaded.") .. "\n" .. _("Saved in:") .. " " .. self:downloadDir()
            if failed > 0 then
                text = text .. "\n" .. tostring(failed) .. " " .. _("failed.")
            end
            if info.stopped then
                text = text .. "\n" .. _("Stopped early.")
            end
            if not quiet or got > 0 then
                UIManager:show(InfoMessage:new{ text = text })
            end
        end)
    end)
end

for _i, name in ipairs{ "debuglog", "debugsend", "setup", "synccheck", "tidynames", "search", "updater", "devicereport", "bookshelfsrc", "highlights", "finishbook", "kindlecopy", "tidybooks", "crashlogs", "kindletasks", "steps", "locked", "homeaddr", "leftbooks" } do
    local ok, mod = loadOwn(name)
    if ok and type(mod) ~= "function" then
        ok, mod = false, "returned " .. type(mod) .. " instead of a function"
    end
    if ok then
        ok, mod = pcall(mod, NovelCheck, { VERSION = VERSION, PLUGIN_DIR = PLUGIN_DIR })
    end
    if not ok then
        table.insert(load_errors, name .. ": " .. tostring(mod))
    end
end
if #load_errors > 0 then
    pcall(function()
        local f = io.open((PLUGIN_DIR or ".") .. "/LOAD_ERROR.txt", "w")
        if f then
            f:write(table.concat(load_errors, "\n"), "\n")
            f:close()
        end
    end)
    NovelCheck:dlog("error", "load errors: " .. table.concat(load_errors, " ; "))
end

return NovelCheck
