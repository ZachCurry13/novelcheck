--[[
NovelCheck plugin: "Delete the Kindle's crash logs". The Kindle's own software (pillowd, mesquite and others) leaves
a crash report in the top folder every time it falls over: files named like pillowd_2264_001-juno_..._crash_....
They are error reports for Amazon, not books, and they pile up by the hundred. This lists them (top folder only,
only names with "_crash_" in them), says how many and how big, asks, and deletes them. Nothing else is touched.
The backup leaves them out too. It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ConfirmBox = require("ui/widget/confirmbox")
local InfoMessage = require("ui/widget/infomessage")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local lfs = require("libs/libkoreader-lfs")

return function(NovelCheck)
    -- findCrashLogs lists the crash reports in the top folder of the user storage: {path, size}.
    function NovelCheck:findCrashLogs()
        local out = {}
        local root = self:kindleRoot()
        local ok, iter, state = pcall(lfs.dir, root)
        if not ok then
            return out
        end
        for name in iter, state do
            if name:find("_crash_", 1, true) then
                local path = root .. "/" .. name
                local attr = lfs.attributes(path)
                if attr and attr.mode == "file" then
                    out[#out + 1] = { path = path, size = attr.size }
                end
            end
        end
        return out
    end

    -- deleteCrashLogsQuietly deletes them without asking (the automatic cleanup) and says how many in the log.
    function NovelCheck:deleteCrashLogsQuietly()
        local gone = 0
        for _i, f in ipairs(self:findCrashLogs()) do
            if os.remove(f.path) then
                gone = gone + 1
            end
        end
        if gone > 0 then
            self:dlog("info", "deleted " .. gone .. " crash logs")
        end
        return gone
    end

    function NovelCheck:cleanCrashLogs(quiet)
        local list = self:findCrashLogs()
        if #list == 0 then
            if not quiet then
                UIManager:show(InfoMessage:new{ text = _("There are no crash logs on this Kindle.") })
            end
            return
        end
        local bytes = 0
        for _i, f in ipairs(list) do
            bytes = bytes + f.size
        end
        UIManager:show(ConfirmBox:new{
            text = _("Delete") .. " " .. #list .. " " .. _("crash logs") .. " (" .. string.format("%.1f", bytes / 1048576) .. " MB)?\n\n"
                .. _("They are error reports the Kindle's own software wrote, not books. Nothing else is touched."),
            ok_text = _("Delete them"),
            ok_callback = function()
                local gone = 0
                for _i, f in ipairs(list) do
                    if os.remove(f.path) then
                        gone = gone + 1
                    end
                end
                UIManager:show(InfoMessage:new{ text = _("Deleted") .. " " .. gone .. " " .. _("crash logs.") })
            end,
        })
    end
end
