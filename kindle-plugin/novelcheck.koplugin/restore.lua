--[[
NovelCheck plugin: Restore from NovelCheck (Kindle care menu). Puts back, from this e-reader's backup in NovelCheck,
every file that is missing here: books, KOReader's settings, reading progress and highlights, other plugins. It
never replaces a file that is here. It downloads one file at a time (scheduled, so the screen keeps answering), can
be stopped with Settings > Stop current job, and says what it did. Back up first: a restore is only as good as the
backup. It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ConfirmBox = require("ui/widget/confirmbox")
local InfoMessage = require("ui/widget/infomessage")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")

local function encode(s)
    return (tostring(s):gsub("[^%w%-_%.~/]", function(c) return string.format("%%%02X", c:byte()) end))
end

-- makeDirs creates every folder above path.
local function makeDirs(path)
    local acc = ""
    for part in path:gmatch("[^/]+/") do
        acc = acc .. part
        if lfs.attributes(acc, "mode") == nil then
            lfs.mkdir(acc)
        end
    end
end

return function(NovelCheck)
    function NovelCheck:restoreKindle()
        if not self:isConnected() then
            return self:connectDialog()
        end
        if self.kindle_copy_running then
            return UIManager:show(InfoMessage:new{ text = _("A backup or restore is already running.") })
        end
        UIManager:show(ConfirmBox:new{
            text = _("Restore from NovelCheck?") .. "\n\n" .. _("Puts back every file from this e-reader's backup that is missing here (books, settings, reading progress, highlights). Files that are here are never replaced."),
            ok_text = _("Restore"),
            ok_callback = function() NetworkMgr:runWhenOnline(function() self:runRestore() end) end,
        })
    end

    function NovelCheck:runRestore()
        local device = self:deviceName()
        local code, answer = self:request("/kosync/plugin/restore?device=" .. encode(device))
        local good, data = pcall(json.decode, answer or "")
        if code ~= 200 or not good or type(data) ~= "table" or type(data.files) ~= "table" then
            return UIManager:show(InfoMessage:new{ text = _("Could not get the backup from NovelCheck.") .. "\n" .. tostring(code or answer) })
        end
        local root = self:kindleRoot()
        local todo = {}
        for _i, f in ipairs(data.files) do
            if type(f.path) == "string" and not f.path:find("%.%.") and lfs.attributes(root .. "/" .. f.path, "mode") == nil then
                table.insert(todo, f.path)
            end
        end
        if #todo == 0 then
            return UIManager:show(InfoMessage:new{ text = _("Nothing to restore: every file of the backup is already here.") .. "\n(" .. tostring(#data.files) .. " " .. _("files in the backup") .. ")" })
        end
        self.kindle_copy_running, self.kindle_stop = true, false
        local done, failed = 0, 0
        local function finish(stopped)
            self.kindle_copy_running = false
            UIManager:show(InfoMessage:new{ text = _("Restore") .. ": " .. tostring(done) .. " " .. _("files put back") ..
                (failed > 0 and (", " .. tostring(failed) .. " " .. _("failed")) or "") .. (stopped and ("\n" .. _("Stopped early.")) or "") ..
                (done > 0 and ("\n" .. _("Restart KOReader so it sees the restored settings.")) or "") })
        end
        local function step(i)
            if self.kindle_stop then
                return finish(true)
            end
            local rel = todo[i]
            if not rel then
                return finish(false)
            end
            local dest = root .. "/" .. rel
            makeDirs(dest)
            local got = self:request("/kosync/plugin/restore/file?device=" .. encode(device) .. "&path=" .. encode(rel), dest .. ".nc-part")
            if got == 200 and os.rename(dest .. ".nc-part", dest) then
                done = done + 1
            else
                os.remove(dest .. ".nc-part")
                failed = failed + 1
                self:dlog("error", "restore " .. rel .. ": " .. tostring(got))
            end
            if i % 20 == 0 then
                UIManager:show(InfoMessage:new{ text = _("Restoring") .. " " .. tostring(i) .. " / " .. tostring(#todo) .. "\n" .. _("Stop: NovelCheck icon > More > Settings > Stop current job"), timeout = 3 })
            end
            UIManager:scheduleIn(0.05, function() step(i + 1) end)
        end
        UIManager:show(InfoMessage:new{ text = _("Restoring") .. " " .. tostring(#todo) .. " " .. _("files…"), timeout = 3 })
        UIManager:scheduleIn(0.5, function() step(1) end)
    end
end
