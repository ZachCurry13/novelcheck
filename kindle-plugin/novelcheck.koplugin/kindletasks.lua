--[[
NovelCheck plugin: tasks from the app. In NovelCheck (Library cleanup → Kindle tasks) the admin can ask this Kindle
to back itself up, tidy its books, delete its crash logs, delete books (only ones NovelCheck holds a copy of, and only
while the file is still the same size as the copy), move books into a folder under Books, or download library books.
At each check-in (and NovelCheck > Do the tasks from NovelCheck now) the plugin asks for the waiting tasks, does them
and reports each one back. Only these kinds exist; anything else is reported as skipped. A deleted book's reading data
goes with it; nothing is overwritten.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local DocSettings = require("docsettings")
local InfoMessage = require("ui/widget/infomessage")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")

return function(NovelCheck)
    -- findDeviceFile finds the book file with this name and size anywhere on the user storage: its path, or nil.
    function NovelCheck:findDeviceFile(filename, size)
        local files = {}
        self:findBooks(self:kindleRoot(), files, 0)
        for _i, f in ipairs(files) do
            if f.path:match("[^/]+$") == filename and f.size == size then
                return f.path
            end
        end
        return nil
    end

    local function result(self, id, status, note)
        self:request("/kosync/plugin/tasks/result", nil, json.encode{ id = id, status = status, note = note })
    end

    local DO = {}

    DO.backup = function(self)
        if self.backupKindle then
            self:backupKindle(true)
        end
        return "done", "the backup started (it carries on while the Kindle is online)"
    end

    DO.tidy = function(self)
        if not self.tidyBooks then
            return "skipped", "this plugin can't tidy"
        end
        local n, note = self:tidyBooks(true)
        if n == nil then
            return "failed", note or "tidy failed"
        end
        return "done", "moved " .. tostring(n) .. " books; " .. tostring(note or "")
    end

    DO.crashlogs = function(self)
        if not self.deleteCrashLogsQuietly then
            return "skipped", "this plugin can't delete crash logs"
        end
        return "done", "deleted " .. tostring(self:deleteCrashLogsQuietly()) .. " crash logs"
    end

    DO.delete_books = function(self, args)
        local gone, missing = 0, 0
        for _i, it in ipairs(type(args.items) == "table" and args.items or {}) do
            local path = self:findDeviceFile(tostring(it.filename), tonumber(it.size) or -1)
            if not path then
                missing = missing + 1 -- not there, or not the same file any more: left alone
            else
                pcall(function() DocSettings:open(path):purge() end) -- its reading data
                if os.remove(path) then
                    gone = gone + 1
                end
            end
        end
        self:reportQuietly()
        return "done", "deleted " .. gone .. ", left alone " .. missing .. " (gone or changed since the backup)"
    end

    DO.move_books = function(self, args)
        local folder = tostring(args.folder or ""):gsub("[/\\:%c]", " ")
        if folder == "" then
            return "failed", "no folder"
        end
        local moves, n = {}, 0
        for _i, it in ipairs(type(args.items) == "table" and args.items or {}) do
            local from = self:findDeviceFile(tostring(it.filename), tonumber(it.size) or -1)
            if from then
                local to = self:libraryDir() .. "/" .. folder .. "/" .. tostring(it.filename)
                if from ~= to and lfs.attributes(to, "mode") == nil and self:moveOneFile(from, to) then
                    n = n + 1
                    moves[#moves + 1] = { from = from, to = to }
                    local sf, st = from:gsub("%.[^./]+$", "") .. ".sdr", to:gsub("%.[^./]+$", "") .. ".sdr"
                    if lfs.attributes(sf, "mode") == "directory" and lfs.attributes(st, "mode") == nil and self:moveOneFile(sf, st) then
                        moves[#moves + 1] = { from = sf, to = st }
                    end
                end
            end
        end
        if #moves > 0 and self.tellMoved then
            self:tellMoved(moves)
        end
        self:reportQuietly()
        return "done", "moved " .. n .. " books into Books/" .. folder
    end

    -- a download goes one book per turn, so the Kindle keeps answering; it reports through report() when it ends
    DO.download_books = function(self, args, report)
        local got, failed = 0, 0
        self:runSteps(_("Downloading"), type(args.books) == "table" and args.books or {}, function(b)
            if b.url and lfs.attributes(self:bookPath(b), "mode") ~= "file" then
                if self:fetchBook(b) then
                    got = got + 1
                else
                    failed = failed + 1
                end
            end
        end, { batch = 1, quiet = true }, function(info)
            self:reportQuietly()
            report((failed > 0 and got == 0) and "failed" or "done", "downloaded " .. got .. ", failed " .. failed .. (info.stopped and ", stopped early" or ""))
        end)
        return nil -- the answer comes later
    end

    -- runKindleTasks asks for the waiting tasks and does them one after another.
    function NovelCheck:runKindleTasks(quiet)
        if not self:isConnected() then
            return
        end
        local code, body = self:request("/kosync/plugin/tasks?device=" .. tostring(self:deviceName()):gsub("[^%w%-%._~]", function(c) return string.format("%%%02X", c:byte()) end))
        local ok, data = pcall(json.decode, body or "")
        if code ~= 200 or not ok or type(data) ~= "table" or type(data.tasks) ~= "table" then
            return
        end
        if #data.tasks == 0 then
            if not quiet then
                UIManager:show(InfoMessage:new{ text = _("No tasks are waiting from NovelCheck."), timeout = 3 })
            end
            return
        end
        local count = #data.tasks
        -- one task per turn; a long one (a download) reports itself when it ends
        local idx = 0
        local function nextTask()
            idx = idx + 1
            local t = data.tasks[idx]
            if not t then
                return
            end
            local fn = DO[tostring(t.kind)]
            local function report(status, note)
                self:dlog("info", "task " .. tostring(t.kind) .. ": " .. tostring(status) .. " " .. tostring(note))
                result(self, t.id, status, note)
            end
            if not fn then
                report("skipped", "this plugin does not know that task")
            else
                local good, s2, n2 = pcall(fn, self, type(t.args) == "table" and t.args or {}, report)
                if not good then
                    report("failed", tostring(s2))
                elseif s2 ~= nil then
                    report(s2, n2)
                end
            end
            UIManager:scheduleIn(0.2, nextTask)
        end
        nextTask()
        local done = count
        if not quiet then
            UIManager:show(InfoMessage:new{ text = _("Did") .. " " .. done .. " " .. _("tasks from NovelCheck."), timeout = 4 })
        end
    end
end
