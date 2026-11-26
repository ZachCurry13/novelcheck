--[[
NovelCheck plugin: long jobs that keep the Kindle answering. KOReader draws the screen and reads the buttons from one
loop; a job that works through hundreds of books without handing control back freezes the Kindle (even the power
button). runSteps goes through a list a few items at a time and returns to the loop between them, says how far it
is now and then, and can be stopped (NovelCheck icon > Settings > Stop current job). It adds its functions to the
NovelCheck plugin class (see main.lua).
]]

local InfoMessage = require("ui/widget/infomessage")
local UIManager = require("ui/uimanager")
local _ = require("gettext")

return function(NovelCheck)
    -- runSteps(label, items, handler, opts, done): handler(item, index) is called for each item (errors are caught
    -- and counted); done(info) is called at the end with {stopped = bool, handled = n, errors = n, total = n}.
    -- opts: batch (items per turn, default 1), show (false hides the progress), quiet.
    function NovelCheck:runSteps(label, items, handler, opts, done)
        opts = opts or {}
        local batch = opts.batch or 1
        local total = #items
        local i, errors, lastShown = 0, 0, os.time()
        self.job_stop = false
        self.job_label = label
        pcall(function() self:dlog("step", label .. ": " .. total .. " items") end)
        local function finish(stopped)
            self.job_label = nil
            self.job_stop = false
            if done then
                done({ stopped = stopped, handled = i, errors = errors, total = total })
            end
        end
        local function step()
            if self.job_stop then
                return finish(true)
            end
            local n = 0
            while n < batch and i < total do
                i = i + 1
                n = n + 1
                local it = items[i]
                if type(it) == "table" and it.path and i % 10 == 1 then -- which book it is at, now and then
                    pcall(function() self:dlog("step", label .. " " .. i .. ": " .. tostring(it.path)) end)
                end
                local ok, err = pcall(handler, it, i)
                if not ok then
                    errors = errors + 1
                    pcall(function() self:dlog("error", label .. ": " .. tostring(err)) end)
                end
            end
            if i >= total then
                return finish(false)
            end
            if opts.show ~= false and not opts.quiet and os.time() - lastShown >= 8 then
                lastShown = os.time()
                UIManager:show(InfoMessage:new{ text = label .. " " .. i .. " / " .. total .. "\n" .. _("Stop: NovelCheck icon > Settings > Stop current job"), timeout = 4 })
            end
            UIManager:scheduleIn(0.02, step)
        end
        UIManager:scheduleIn(0.02, step)
    end

    -- stopJob stops the long job in progress (a backup, a download, a scan).
    function NovelCheck:stopJob()
        local any = false
        if self.kindle_copy_running then
            self.kindle_stop = true
            any = true
        end
        if self.job_label then
            self.job_stop = true
            any = true
        end
        UIManager:show(InfoMessage:new{ text = any and _("Stopping...") or _("Nothing is running."), timeout = 3 })
    end
end
