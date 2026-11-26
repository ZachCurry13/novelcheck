--[[
NovelCheck plugin: putting the NovelCheck Kindle app (the Amazon-reader app, no KOReader needed) on this Kindle.
NovelCheck icon > More > Settings > Install the Kindle app downloads the app's files and this account's sign-in from NovelCheck into
extensions/novelcheck-app, and the launcher "book" into documents. It needs the Kindle jailbroken with KindleForge.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ConfirmBox = require("ui/widget/confirmbox")
local InfoMessage = require("ui/widget/infomessage")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")

return function(NovelCheck)
    local ROOT = "/mnt/us"

    local function enc(s)
        return (tostring(s):gsub("[^%w%-%._~]", function(c) return string.format("%%%02X", c:byte()) end))
    end

    local function say(text)
        UIManager:show(InfoMessage:new{ text = text })
    end

    function NovelCheck:installKindleApp()
        if lfs.attributes(ROOT, "mode") ~= "directory" then
            return say(_("This is not a Kindle, so the Kindle app can't go here."))
        end
        if not self:isConnected() then
            return self:connectDialog()
        end
        UIManager:show(ConfirmBox:new{
            text = _("Put the NovelCheck Kindle app on this Kindle, signed in as you? It opens from the Kindle's own library (a book called NovelCheck), and needs KindleForge."),
            ok_text = _("Install"),
            ok_callback = function() NetworkMgr:runWhenOnline(function() self:doInstallKindleApp() end) end,
        })
    end

    function NovelCheck:doInstallKindleApp()
        local code, body = self:request("/kosync/plugin/kindle-app")
        local ok, info = pcall(json.decode, code == 200 and body or "")
        if not (ok and type(info) == "table" and type(info.files) == "table" and #info.files > 0) then
            return say(_("Could not reach NovelCheck. Nothing was changed."))
        end
        local dir = ROOT .. "/extensions/novelcheck-app"
        lfs.mkdir(ROOT .. "/extensions")
        lfs.mkdir(dir)
        local q = "?server=" .. enc((self.settings:readSetting("server") or ""):gsub("%s+", "")) .. "&name=" .. enc(self:deviceName())
        local jobs = { { "config.js", dir .. "/config.js", q }, { "NovelCheck.sh", ROOT .. "/documents/NovelCheck.sh", "" } }
        for _i, name in ipairs(info.files) do
            if name:match("^[%w_%.]+$") then
                table.insert(jobs, { name, dir .. "/" .. name, "" })
            end
        end
        local staged = {}
        for _i, j in ipairs(jobs) do
            local tmp = j[2] .. ".new"
            table.insert(staged, { tmp = tmp, dest = j[2] })
            local c = self:request("/kosync/plugin/kindle-app/" .. j[1] .. j[3], tmp)
            local size = lfs.attributes(tmp, "size")
            if c ~= 200 or not size or size == 0 then
                for _k, s in ipairs(staged) do os.remove(s.tmp) end
                return say(_("The download failed. Nothing was changed."))
            end
        end
        for _i, s in ipairs(staged) do
            os.remove(s.dest)
            os.rename(s.tmp, s.dest)
        end
        os.execute("chmod +x " .. ROOT .. "/documents/NovelCheck.sh 2>/dev/null")
        say(_("The NovelCheck Kindle app is installed (version") .. " " .. tostring(info.version) .. "). " .. _("Close KOReader, then open the book called NovelCheck from the Kindle's library."))
    end
end
