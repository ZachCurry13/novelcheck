--[[
NovelCheck plugin: "Set up everything". One tap points KOReader's Progress sync
and Reading statistics sync at NovelCheck, using the server, username and sync
code this plugin already has. It also runs the statistics sync on demand.
It adds its functions to the NovelCheck plugin class (see main.lua).

What it writes is what KOReader itself would store if you set these up by hand:
  - Progress sync: settings/kosync.lua (custom server, username, key)
  - Reading statistics sync: the statistics settings (a WebDAV server entry)
]]

local ConfirmBox = require("ui/widget/confirmbox")
local DataStorage = require("datastorage")
local Event = require("ui/event")
local InfoMessage = require("ui/widget/infomessage")
local LuaSettings = require("luasettings")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local md5 = require("ffi/sha2").md5

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

return function(NovelCheck, ctx)
    local _unused = { ctx }

    -- setProgressSync points KOReader's Progress sync at NovelCheck.
    function NovelCheck:setProgressSync()
        local server = trim(self.settings:readSetting("server"))
        local user = trim(self.settings:readSetting("user"))
        local key = md5(trim(self.settings:readSetting("code")))
        local function fill(s)
            s.custom_server = server .. "/kosync"
            s.username = user
            s.userkey = key
            s.auto_sync = true
        end
        local live = self.ui and self.ui.kosync
        if live and type(live.settings) == "table" then
            -- Progress sync is loaded (a book is open): change it where it lives and let it save.
            fill(live.settings)
            live.updated = true
            pcall(live.onFlushSettings, live)
            return true
        end
        local obj = LuaSettings:open(DataStorage:getSettingsDir() .. "/kosync.lua")
        local s = obj:readSetting("settings", {})
        fill(s)
        obj:saveSetting("settings", s)
        obj:flush()
        return true
    end

    -- setStatsSync points Reading statistics -> Cloud sync at the NovelCheck
    -- WebDAV folder. It returns true, or false and what the person should do.
    function NovelCheck:setStatsSync()
        local stats = self.ui.statistics
        local settings = stats and stats.settings
        if not settings then
            settings = G_reader_settings:readSetting("statistics")
        end
        if type(settings) ~= "table" then
            return false, _("Open any book once so KOReader creates its statistics settings, then run this again.")
        end
        local old = settings.sync_server
        settings.sync_server = {
            name = "NovelCheck",
            type = "webdav",
            address = trim(self.settings:readSetting("server")) .. "/dav",
            username = trim(self.settings:readSetting("user")),
            password = trim(self.settings:readSetting("code")),
            url = "/",
        }
        if not old or old.address ~= settings.sync_server.address then
            os.remove(DataStorage:getSettingsDir() .. "/statistics.sqlite3.sync") -- merge afresh with the new folder
        end
        G_reader_settings:flush()
        return true, (not self.ui.cloudstorage) and _("KOReader's Cloud storage is not loaded in this screen; if Synchronize now does nothing, open a book and try from there.") or nil
    end

    -- syncStats presses "Reading statistics > Synchronize now".
    function NovelCheck:syncStats()
        return pcall(function()
            self.ui:handleEvent(Event:new("SyncBookStats"))
        end)
    end

    function NovelCheck:easySetup()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        UIManager:show(ConfirmBox:new{
            text = _("Set up NovelCheck sync?") .. "\n\n"
                .. _("Progress sync: your place in each book follows you between devices.") .. "\n"
                .. _("Reading statistics: your reading history goes to NovelCheck.") .. "\n\n"
                .. _("No restart should be needed."),
            ok_text = _("Set up"),
            ok_callback = function() self:runSetup() end,
        })
    end

    function NovelCheck:runSetup()
        NetworkMgr:runWhenOnline(function()
            local code = self:request("/kosync/users/auth")
            if code == 401 then
                UIManager:show(InfoMessage:new{ text = _("NovelCheck did not accept the username or sync code. Check them under Connect to NovelCheck.") })
                return
            elseif code ~= 200 then
                UIManager:show(InfoMessage:new{ text = _("Could not reach NovelCheck.") .. "\n" .. tostring(code) })
                return
            end
            local lines = {}
            if pcall(self.setProgressSync, self) then
                table.insert(lines, "[x] " .. _("Progress sync"))
            else
                table.insert(lines, "[ ] " .. _("Progress sync: could not write its settings."))
            end
            local ok, why = self:setStatsSync()
            if ok then
                table.insert(lines, "[x] " .. _("Reading statistics sync"))
                if why then
                    table.insert(lines, why)
                end
                self.settings:saveSetting("auto_stats", true)
                self.settings:flush()
            else
                table.insert(lines, "[ ] " .. _("Reading statistics sync:") .. " " .. tostring(why))
            end
            local text = table.concat(lines, "\n") .. "\n\n"
                .. _("Next: NovelCheck > Sync reading statistics now.") .. "\n"
                .. _("If progress sync does not pick this up, restart KOReader (on a Kindle, reopen it from KUAL).")
            UIManager:show(InfoMessage:new{ text = text })
        end)
    end

    -- syncStatsNow is the menu action: sync the reading statistics now.
    function NovelCheck:syncStatsNow()
        NetworkMgr:runWhenOnline(function()
            if not self:syncStats() then
                UIManager:show(InfoMessage:new{ text = _("Could not start the statistics sync. Run NovelCheck > Set up everything first.") })
            end
        end)
    end
end
