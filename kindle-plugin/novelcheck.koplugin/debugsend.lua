--[[
NovelCheck plugin: sending the developer log. An error in the plugin sends the log to NovelCheck by
itself (at most once every ten minutes, only when connected and online, switch in Developer tools),
and "Send log with a note..." lets you say what you were doing.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local InfoMessage = require("ui/widget/infomessage")
local InputDialog = require("ui/widget/inputdialog")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")

local EVERY = 600 -- seconds between automatic sends

return function(NovelCheck, ctx)
    local _unused = { ctx }

    function NovelCheck:autoSendOn()
        return self.settings:readSetting("auto_send") ~= false
    end

    -- autoSend runs after an error was logged. It never raises and never asks anything.
    function NovelCheck:autoSend(why)
        if self._auto_busy or not self:autoSendOn() or not self:isConnected() or not NetworkMgr:isConnected() then
            return
        end
        if tostring(why):find("sending the log failed", 1, true) then
            return -- do not try again because sending failed
        end
        local now = os.time()
        if self._auto_last and now - self._auto_last < EVERY then
            return
        end
        self._auto_last = now
        self._auto_busy = true
        UIManager:scheduleIn(5, function()
            pcall(function()
                self:uploadLog("AUTOMATIC after an error: " .. tostring(why):sub(1, 150))
            end)
            self._auto_busy = false
        end)
    end

    -- sendWithNote asks what you were doing, then sends the log with that note.
    function NovelCheck:sendWithNote()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        local dialog
        dialog = InputDialog:new{
            title = _("What were you doing?"),
            input = "",
            input_hint = _("for example: pressed Report books, got an error"),
            buttons = { {
                { text = _("Cancel"), callback = function() UIManager:close(dialog) end },
                { text = _("Send"), is_enter_default = true, callback = function()
                    local note = dialog:getInputText()
                    UIManager:close(dialog)
                    NetworkMgr:runWhenOnline(function()
                        local ok, err = self:uploadLog(note)
                        if ok then
                            UIManager:show(InfoMessage:new{ text = _("Sent. It is in NovelCheck under Settings > Developer > Bug reports.") })
                        else
                            UIManager:show(InfoMessage:new{ text = _("Could not send the log.") .. "\n" .. tostring(err) })
                        end
                    end)
                end },
            } },
        }
        UIManager:show(dialog)
        dialog:onShowKeyboard()
    end
end
