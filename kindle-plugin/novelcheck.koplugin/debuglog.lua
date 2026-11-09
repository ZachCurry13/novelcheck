--[[
NovelCheck plugin: developer tools. Keeps a small log of what the plugin does (menu taps, requests to
NovelCheck, errors), can run a self-test, show the log on the device, and send it to NovelCheck
(Admin > Device logs) so a problem can be looked at. The log never holds your sync code.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ConfirmBox = require("ui/widget/confirmbox")
local DataStorage = require("datastorage")
local Device = require("device")
local InfoMessage = require("ui/widget/infomessage")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")

local MAX_BYTES = 120 * 1024
local SHOW_CHARS = 6000

return function(NovelCheck, ctx)
    local VERSION, PLUGIN_DIR = ctx.VERSION, ctx.PLUGIN_DIR
    local LOG = DataStorage:getSettingsDir() .. "/novelcheck-debug.log"

    local function stamp()
        return os.date("%Y-%m-%d %H:%M:%S")
    end

    -- dlog adds one line to the log. It never raises: logging must not break the plugin.
    function NovelCheck:dlog(kind, msg)
        pcall(function()
            local f = io.open(LOG, "a")
            if not f then
                return
            end
            f:write(stamp(), " [", tostring(kind), "] ", (tostring(msg):gsub("[\r\n]+", " | ")), "\n")
            f:close()
            local size = lfs.attributes(LOG, "size") or 0
            if size > MAX_BYTES then
                local r = io.open(LOG, "rb")
                if r then
                    r:seek("set", size - MAX_BYTES / 2)
                    local rest = r:read("*a") or ""
                    r:close()
                    local w = io.open(LOG, "wb")
                    if w then
                        w:write("-- older lines trimmed --\n", (rest:gsub("^[^\n]*\n", "", 1)))
                        w:close()
                    end
                end
            end
        end)
    end

    function NovelCheck:readLog()
        local f = io.open(LOG, "rb")
        if not f then
            return ""
        end
        local s = f:read("*a") or ""
        f:close()
        return s
    end

    function NovelCheck:verbose()
        return self.settings:readSetting("debug_verbose") and true or false
    end

    -- trackMenu wraps every menu action so each tap is logged and an error is
    -- shown (and logged) instead of silently failing.
    function NovelCheck:trackMenu(items)
        for _i, it in ipairs(items or {}) do
            local cb = it.callback
            if cb then
                local name = type(it.text) == "string" and it.text or "menu item"
                it.callback = function(...)
                    self:dlog("tap", name)
                    local ok, err = pcall(cb, ...)
                    if not ok then
                        self:dlog("error", name .. ": " .. tostring(err))
                        UIManager:show(InfoMessage:new{ text = _("Something went wrong:") .. " " .. name .. "\n" .. tostring(err):sub(1, 300)
                            .. "\n" .. _("NovelCheck > Developer tools > Send log to NovelCheck.") })
                    end
                end
            end
            if it.sub_item_table then
                self:trackMenu(it.sub_item_table)
            end
        end
    end

    local function listFiles(dir)
        local out = {}
        if not dir then
            return "(plugin folder unknown)"
        end
        local ok = pcall(function()
            for name in lfs.dir(dir) do
                if name ~= "." and name ~= ".." then
                    table.insert(out, name .. " " .. tostring(lfs.attributes(dir .. "/" .. name, "size") or "?"))
                end
            end
        end)
        table.sort(out)
        return ok and table.concat(out, ", ") or "(could not read the folder)"
    end

    -- selfTest checks the connection and the plugin's own files and writes the result to the log.
    function NovelCheck:selfTest()
        local function put(msg)
            self:dlog("test", msg)
        end
        put("self-test, plugin " .. VERSION .. ", KOReader " .. tostring((pcall(require, "version") and require("version"):getCurrentRevision()) or "?")
            .. ", device " .. tostring(Device.model))
        put("plugin folder " .. tostring(PLUGIN_DIR) .. ": " .. listFiles(PLUGIN_DIR))
        if self.load_errors and #self.load_errors > 0 then
            put("LOAD ERRORS: " .. table.concat(self.load_errors, " ; "))
        end
        put("connected settings: " .. tostring(self:isConnected()) .. ", server " .. tostring(self.settings:readSetting("server"))
            .. ", user " .. tostring(self.settings:readSetting("user")) .. ", backup " .. tostring(self.settings:readSetting("server_alt")))
        put("wifi: " .. tostring(NetworkMgr:isConnected()))
        for _i, p in ipairs{ "/kosync/plugin/version", "/kosync/plugin/up-next", "/kosync/plugin/names" } do
            local code, body = self:request(p)
            put(p .. " -> " .. tostring(code) .. " " .. tostring(code == 200 and #body .. " bytes" or body))
        end
    end

    function NovelCheck:showLog()
        local TextViewer = require("ui/widget/textviewer")
        local text = self:readLog()
        if text == "" then
            text = _("The log is empty. Use the plugin, then look again.")
        elseif #text > SHOW_CHARS then
            text = "...\n" .. text:sub(-SHOW_CHARS)
        end
        UIManager:show(TextViewer:new{ title = _("NovelCheck log (newest last)"), text = text })
    end

    function NovelCheck:sendDebugLog()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        NetworkMgr:runWhenOnline(function()
            self:selfTest()
            local text = self:readLog()
            if #text > 100 * 1024 then
                text = text:sub(-100 * 1024)
            end
            local body = json.encode{ device = self.settings:readSetting("device_name") or Device.model or "device", version = VERSION, text = text }
            local code, resp = self:request("/kosync/plugin/debug", nil, body)
            if code == 200 then
                UIManager:show(InfoMessage:new{ text = _("Sent. It is under Admin > Device logs in NovelCheck.") })
            else
                self:dlog("error", "sending the log failed: " .. tostring(code) .. " " .. tostring(resp))
                UIManager:show(InfoMessage:new{ text = _("Could not send the log.") .. "\n" .. tostring(code or resp) })
            end
        end)
    end

    -- developerMenu is the "Developer tools" submenu.
    function NovelCheck:developerMenu()
        return { text = _("Developer tools"), sub_item_table = {
            { text = _("Run self-test"), callback = function()
                NetworkMgr:runWhenOnline(function()
                    self:selfTest()
                    self:showLog()
                end)
            end },
            { text = _("View log"), callback = function() self:showLog() end },
            { text = _("Send log to NovelCheck"), callback = function() self:sendDebugLog() end },
            { text = _("Log more detail"), keep_menu_open = true,
              checked_func = function() return self:verbose() end,
              callback = function()
                  self.settings:saveSetting("debug_verbose", not self:verbose())
                  self.settings:flush()
              end },
            { text = _("Clear log"), callback = function()
                UIManager:show(ConfirmBox:new{ text = _("Clear the log?"), ok_callback = function() os.remove(LOG) end })
            end },
        } }
    end
end
