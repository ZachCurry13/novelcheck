--[[
NovelCheck plugin: what the self-test says about KOReader's sync set-up, so a problem with progress
sync or reading-statistics sync can be seen from the log. Passwords and keys are never written.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local DataStorage = require("datastorage")
local LuaSettings = require("luasettings")

return function(NovelCheck, ctx)
    local _unused = { ctx }

    -- syncCheck writes lines (through put) about progress sync, statistics sync and cloud storage.
    function NovelCheck:syncCheck(put)
        local ok, err = pcall(function()
            local ks = self.ui and self.ui.kosync and self.ui.kosync.settings
            if type(ks) ~= "table" then
                ks = LuaSettings:open(DataStorage:getSettingsDir() .. "/kosync.lua"):readSetting("settings", {})
            end
            put("progress sync: server " .. tostring(ks.custom_server) .. ", user " .. tostring(ks.username)
                .. ", key " .. (ks.userkey and "set" or "NOT set") .. ", auto sync " .. tostring(ks.auto_sync)
                .. ", send metadata " .. tostring(ks.send_metadata) .. ", loaded " .. tostring(self.ui and self.ui.kosync ~= nil))
            local stats = G_reader_settings:readSetting("statistics")
            local srv = type(stats) == "table" and stats.sync_server
            if type(srv) == "table" then
                put("statistics sync: " .. tostring(srv.name) .. " (" .. tostring(srv.type) .. ") " .. tostring(srv.address)
                    .. " folder " .. tostring(srv.url) .. ", user " .. tostring(srv.username) .. ", password " .. (srv.password and "set" or "NOT set"))
            else
                put("statistics sync: no server set")
            end
            put("cloud storage plugin loaded: " .. tostring(self.ui and self.ui.cloudstorage ~= nil))
        end)
        if not ok then
            put("sync check failed: " .. tostring(err))
        end
    end
end
