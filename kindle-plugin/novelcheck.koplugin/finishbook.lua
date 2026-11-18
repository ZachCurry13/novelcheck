--[[
NovelCheck plugin: finishing a book. At the end of a book (or from NovelCheck > Finish this book...) a small
window asks for stars (they go to your NovelCheck, and from there to Hardcover), marks the book finished in
NovelCheck and in KOReader, and offers the next book of the series to download.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ButtonDialog = require("ui/widget/buttondialog")
local ConfirmBox = require("ui/widget/confirmbox")
local InfoMessage = require("ui/widget/infomessage")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local util = require("util")

return function(NovelCheck, ctx)
    local _unused = { ctx }

    -- openBook is the book that is open now: its file, fingerprint, title and author (nil when none is open).
    function NovelCheck:openBook()
        local doc = self.ui and self.ui.document
        local path = doc and doc.file
        if type(path) ~= "string" or path == "" then
            return nil
        end
        local okm, md5 = pcall(util.partialMD5, path)
        md5 = okm and md5 or ""
        local title, author = self:bookProps(path, md5)
        local filename = path:match("([^/]+)$") or path
        return { path = path, md5 = md5, filename = filename, title = title or (filename:gsub("%.[^.]+$", "")), author = author or "" }
    end

    -- markFinishedHere sets KOReader's own "finished" mark, like its book status screen does.
    function NovelCheck:markFinishedHere()
        pcall(function()
            local settings = self.ui and self.ui.doc_settings
            if not settings then
                return
            end
            local summary = settings:readSetting("summary") or {}
            summary.status = "complete"
            summary.modified = os.date("%Y-%m-%d")
            settings:saveSetting("summary", summary)
            settings:flush()
        end)
    end

    -- finishBookDialog asks for stars, then tells NovelCheck. From the menu the book must be open.
    function NovelCheck:finishBookDialog()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        local book = self:openBook()
        if not book then
            UIManager:show(InfoMessage:new{ text = _("Open the book you finished first.") })
            return
        end
        local dialog
        local function pick(stars)
            return function()
                UIManager:close(dialog)
                self:finishBook(book, stars)
            end
        end
        local row = {}
        for n = 1, 5 do
            table.insert(row, { text = tostring(n) .. " ★", callback = pick(n) })
        end
        dialog = ButtonDialog:new{
            title = _("Finished") .. " “" .. book.title .. "”\n" .. _("How many stars?"),
            title_align = "center",
            buttons = {
                row,
                { { text = _("Finished, no stars"), callback = pick(0) },
                  { text = _("Not now"), callback = function() UIManager:close(dialog) end } },
            },
        }
        UIManager:show(dialog)
    end

    -- finishBook sends the end of the book to NovelCheck (stars 0 = none) and shows what came back.
    function NovelCheck:finishBook(book, stars)
        self:markFinishedHere()
        NetworkMgr:runWhenOnline(function()
            local body = json.encode{
                device = self:deviceName(), md5 = book.md5, filename = book.filename, title = book.title, author = book.author,
                stars = stars, day = os.date("%Y-%m-%d"),
            }
            local code, answer = self:request("/kosync/plugin/finish", nil, body)
            local ok, data = pcall(json.decode, answer or "")
            if code == 404 then
                UIManager:show(InfoMessage:new{ text = _("This book isn't in your NovelCheck library, so nothing was sent.") })
                return
            elseif code ~= 200 or not ok or type(data) ~= "table" then
                self:dlog("error", "finish: " .. tostring(code))
                UIManager:show(InfoMessage:new{ text = (code == 401) and _("Wrong username or sync code.")
                    or (_("Could not reach NovelCheck.") .. "\n" .. tostring(code or answer)) })
                return
            end
            local text = _("Marked finished in NovelCheck.")
            if (data.stars or 0) > 0 then
                text = text .. "\n" .. tostring(data.stars) .. " ★"
            end
            local nxt = type(data.next) == "table" and data.next or nil
            if nxt and nxt.have and nxt.url and nxt.url ~= "" then
                UIManager:show(ConfirmBox:new{
                    text = text .. "\n\n" .. _("Next in the series:") .. "\n" .. tostring(nxt.title) .. "\n" .. _("Download it now?"),
                    ok_text = _("Download"),
                    cancel_text = _("Not now"),
                    ok_callback = function() self:download(nxt) end,
                })
                return
            end
            if nxt and nxt.title then
                text = text .. "\n\n" .. _("Next in the series:") .. " " .. tostring(nxt.title) .. "\n" .. _("(NovelCheck has no file of it yet.)")
            end
            UIManager:show(InfoMessage:new{ text = text })
        end)
    end

    -- hookEndOfBook shows the stars window when KOReader reaches the end of a book (switch it off under
    -- NovelCheck > Automatic). KOReader's own end-of-book action still runs after it. A book is asked about once
    -- a day at most, so paging back and forth near the end doesn't ask again.
    function NovelCheck:hookEndOfBook()
        local status = self.ui and self.ui.status
        if not status or status.novelcheck_hooked or type(status.onEndOfBook) ~= "function" then
            return
        end
        status.novelcheck_hooked = true
        local original = status.onEndOfBook
        local plugin = self
        status.onEndOfBook = function(this, ...)
            pcall(function()
                if plugin:isConnected() and plugin:autoEnabled("auto_finish", true) then
                    local book = plugin:openBook()
                    local asked = plugin.settings:readSetting("finish_asked") or {}
                    local key = book and (book.md5 ~= "" and book.md5 or book.filename)
                    if key and os.time() - (tonumber(asked[key]) or 0) > 24 * 3600 then
                        asked[key] = os.time()
                        plugin.settings:saveSetting("finish_asked", asked)
                        plugin.settings:flush()
                        UIManager:scheduleIn(1, function() plugin:finishBookDialog() end)
                    end
                end
            end)
            return original(this, ...)
        end
    end
end
