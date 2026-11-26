--[[
NovelCheck plugin: updating itself. Asks NovelCheck for the newest plugin version and, when you agree, downloads
the new Lua files, checks they are valid and swaps them in.
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
local util = require("util")

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

return function(NovelCheck, ctx)
    local VERSION, PLUGIN_DIR = ctx.VERSION, ctx.PLUGIN_DIR
    local _unused = { Device, ConfirmBox, NetworkMgr, lfs, util, json, trim, VERSION, PLUGIN_DIR }

    -- versionLess says whether version a (like "1.38.0") is older than b.
    local function versionLess(a, b)
        local function parts(v)
            local t = {}
            for n in tostring(v or ""):gmatch("%d+") do
                table.insert(t, tonumber(n))
            end
            return t
        end
        local x, y = parts(a), parts(b)
        for i = 1, 3 do
            local p, q = x[i] or 0, y[i] or 0
            if p ~= q then
                return p < q
            end
        end
        return false
    end

    -- The newest release on GitHub carries the plugin's files too, so the plugin can be newer than the person's NovelCheck
    -- server. It is read with the Kindle's own curl (no sign-in), and taken only when the server is at least the version the
    -- release says it needs.
    local GH = "https://github.com/ZachCurry13/novelcheck/releases/"

    local function shq(s)
        return "'" .. (tostring(s):gsub("'", "'\\''")) .. "'"
    end

    local function curl(url, dest, seconds)
        local ok = os.execute("curl -sfL -m " .. seconds .. " -o " .. shq(dest) .. " " .. shq(url) .. " 2>/dev/null")
        return ok == true or ok == 0
    end

    function NovelCheck:githubInfo()
        local tmp = DataStorage:getDataDir() .. "/novelcheck-gh.json"
        os.remove(tmp)
        if not curl(GH .. "latest/download/kindle-plugin.json", tmp, 8) then
            os.remove(tmp)
            return nil
        end
        local f = io.open(tmp, "rb")
        local body = f and f:read("*a")
        if f then
            f:close()
        end
        os.remove(tmp)
        local ok, g = pcall(json.decode, body or "")
        if ok and type(g) == "table" and type(g.files) == "table" and #g.files > 0
            and tostring(g.version):match("^[%d%.]+$") and tostring(g.min_server):match("^[%d%.]+$")
            and g.prefix == "kindle-plugin-" and type(g.base) == "string"
            and g.base:sub(1, #GH + 9) == GH .. "download/" and g.base:match("^[%w%._/:%-]+$") then
            return g
        end
        return nil
    end

    -- latestVersion asks NovelCheck which plugin version it has, and its files, and GitHub for its own (the newer wins).
    function NovelCheck:latestVersion()
        local code, body = self:request("/kosync/plugin/version")
        if code ~= 200 then
            return nil
        end
        local ok, data = pcall(json.decode, body)
        if not (ok and type(data) == "table" and data.version and type(data.files) == "table") then
            return nil
        end
        local g = self:githubInfo()
        if g and versionLess(data.version, g.version) and data.server
            and (data.server == "dev" or not versionLess(data.server, g.min_server)) then
            return { version = g.version, files = g.files, from = "github", base = g.base, prefix = g.prefix }
        end
        return data
    end

    -- checkUpdate offers to update when NovelCheck has a newer plugin. Quiet, it
    -- only mentions a new version once, without installing anything.
    function NovelCheck:checkUpdate(quiet)
        if not self:isConnected() then
            if not quiet then
                self:connectDialog()
            end
            return
        end
        NetworkMgr:runWhenOnline(function()
            local info = self:latestVersion()
            if not info then
                if not quiet then
                    UIManager:show(InfoMessage:new{ text = _("Could not check for a plugin update.") })
                end
                return
            end
            if not versionLess(VERSION, info.version) then
                if not quiet then
                    UIManager:show(InfoMessage:new{ text = _("The plugin is up to date.") .. " (" .. VERSION .. ")" })
                end
            elseif quiet then
                if self.settings:readSetting("update_notified") ~= info.version then
                    self.settings:saveSetting("update_notified", info.version)
                    self.settings:flush()
                    UIManager:show(InfoMessage:new{ text = _("A NovelCheck plugin update is available:") .. " " .. info.version .. "\n" .. _("Open NovelCheck icon > Settings > Update.") })
                end
            else
                UIManager:show(ConfirmBox:new{
                    text = _("Update the NovelCheck plugin?") .. "\n" .. VERSION .. " > " .. info.version,
                    ok_text = _("Update"),
                    ok_callback = function() self:installUpdate(info) end,
                })
            end
        end)
    end

    -- installUpdate downloads the new files next to the old ones, checks that they
    -- are valid Lua, and only then swaps them in (keeping the old as .bak).
    function NovelCheck:installUpdate(info)
        if not PLUGIN_DIR then
            UIManager:show(InfoMessage:new{ text = _("Could not find the plugin folder.") })
            return
        end
        local staged = {}
        local function giveUp()
            for _i, s in ipairs(staged) do
                os.remove(s.tmp)
            end
            UIManager:show(InfoMessage:new{ text = _("The update failed. Nothing was changed.") })
        end
        for _i, name in ipairs(info.files) do
            if not tostring(name):match("^[%w_]+%.lua$") then
                return giveUp()
            end
            local path = PLUGIN_DIR .. "/" .. name
            local tmp = path .. ".new"
            table.insert(staged, { path = path, tmp = tmp })
            local fetched
            if info.from == "github" then
                fetched = curl(info.base .. info.prefix .. name, tmp, 120)
            else
                fetched = self:request("/kosync/plugin/file/" .. name, tmp) == 200
            end
            local size = lfs.attributes(tmp, "size")
            if not fetched or not size or size == 0 or not loadfile(tmp) then
                return giveUp()
            end
        end
        for _i, s in ipairs(staged) do
            os.remove(s.path .. ".bak")
            os.rename(s.path, s.path .. ".bak")
            os.rename(s.tmp, s.path)
        end
        local text = _("The plugin was updated to") .. " " .. info.version .. ". " .. _("Restart KOReader to use it (on a Kindle it closes: reopen it from KUAL).")
        if UIManager.askForRestart then
            UIManager:askForRestart(text)
        else
            UIManager:show(InfoMessage:new{ text = text })
        end
    end
end
