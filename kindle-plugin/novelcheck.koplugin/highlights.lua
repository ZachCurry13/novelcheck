--[[
NovelCheck plugin: highlights and notes. Everything you highlight or write a note on in KOReader is sent to
NovelCheck, which shows it on the book's page (My highlights). Each book is sent again only when its highlights
changed. The device stays the source: delete a highlight in KOReader, not in NovelCheck.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local DataStorage = require("datastorage")
local InfoMessage = require("ui/widget/infomessage")
local LuaSettings = require("luasettings")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")

local MAX_BOOKS = 500
local MAX_PER_BOOK = 1000

local function str(v)
    if v == nil then
        return ""
    end
    return tostring(v)
end

-- parseHighlights turns what KOReader keeps for a book into a plain list of {text, note, chapter, page, datetime}.
-- KOReader 2024.04 and newer keep one "annotations" list (a highlight has a "drawer"; a plain bookmark does not);
-- older versions keep "highlight" by page, with the note in the entry or in "bookmarks".
local function parseHighlights(annotations, legacy, bookmarks)
    local out = {}
    local function add(text, note, chapter, page, datetime)
        text, note = str(text), str(note)
        if (text ~= "" or note ~= "") and #out < MAX_PER_BOOK then
            table.insert(out, { text = text, note = note, chapter = str(chapter), page = str(page), datetime = str(datetime) })
        end
    end
    if type(annotations) == "table" then
        for _i, a in ipairs(annotations) do
            if type(a) == "table" and a.drawer then
                add(a.text, a.note, a.chapter, a.pageno or a.page, a.datetime)
            end
        end
        return out
    end
    local notes = {}
    if type(bookmarks) == "table" then
        for _i, b in ipairs(bookmarks) do
            if type(b) == "table" and b.datetime and b.notes and b.highlighted then
                notes[str(b.datetime)] = b.notes
            end
        end
    end
    if type(legacy) == "table" then
        for page, items in pairs(legacy) do
            if type(items) == "table" then
                for _i, h in ipairs(items) do
                    if type(h) == "table" then
                        add(h.text, h.note or notes[str(h.datetime)], h.chapter, page, h.datetime)
                    end
                end
            end
        end
        table.sort(out, function(a, b) return a.datetime < b.datetime end)
    end
    return out
end

-- signature changes whenever a highlight or note is added, removed or edited.
local function signature(list)
    local n, len = #list, 0
    for _i, h in ipairs(list) do
        len = len + #h.text + 3 * #h.note
    end
    return tostring(n) .. ":" .. tostring(len)
end

return function(NovelCheck, ctx)
    local _unused = { ctx }

    NovelCheck.parseHighlights = parseHighlights -- also used by the tests of this file

    -- highlightsFor reads one book's highlights from KOReader's sidecar file for it.
    function NovelCheck:highlightsFor(path)
        local ok, list = pcall(function()
            local DocSettings = require("docsettings")
            if not DocSettings:hasSidecarFile(path) then
                return {}
            end
            local ds = DocSettings:open(path)
            return parseHighlights(ds:readSetting("annotations"), ds:readSetting("highlight"), ds:readSetting("bookmarks"))
        end)
        if ok and type(list) == "table" then
            return list
        end
        return {}
    end

    -- sendHighlights sends the highlights of every book on this device that changed since the last time.
    -- quiet: no messages (the automatic run).
    function NovelCheck:sendHighlights(quiet)
        if not self:isConnected() then
            if not quiet then
                self:connectDialog()
            end
            return
        end
        NetworkMgr:runWhenOnline(function()
            if not quiet then
                UIManager:show(InfoMessage:new{ text = _("Looking for highlights..."), timeout = 1 })
            end
            UIManager:nextTick(function()
                local ok, err = pcall(function() self:sendHighlightsNow(quiet) end)
                if not ok then
                    self:dlog("error", "highlights: " .. tostring(err))
                    if not quiet then
                        UIManager:show(InfoMessage:new{ text = _("Could not send the highlights.") .. "\n" .. tostring(err) })
                    end
                end
            end)
        end)
    end

    function NovelCheck:sendHighlightsNow(quiet)
        -- the scan and the reading of each book's notes go a few books at a time, so the Kindle keeps answering
        self:scanBooksAsync(function(files, stopped)
            if stopped then
                return
            end
            local sigs = LuaSettings:open(DataStorage:getSettingsDir() .. "/novelcheck_highlights.lua")
            local books, keys, total = {}, {}, 0
            self:runSteps(_("Reading highlights"), files, function(f)
                if #books < MAX_BOOKS then
                    local list = self:highlightsFor(f.path)
                    local key = (f.md5 ~= "" and f.md5) or f.filename
                    if #list > 0 and sigs:readSetting(key) ~= signature(list) then
                        table.insert(books, { md5 = f.md5, filename = f.filename, title = f.title, author = f.author, highlights = list })
                        keys[#books] = { key = key, sig = signature(list) }
                        total = total + #list
                    end
                end
            end, { batch = 4, quiet = quiet }, function(info)
                if info.stopped then
                    return
                end
                self:sendHighlightBatch(quiet, sigs, books, keys, total)
            end)
        end, { quiet = true })
    end

    function NovelCheck:sendHighlightBatch(quiet, sigs, books, keys, total)
        if #books == 0 then
            if not quiet then
                UIManager:show(InfoMessage:new{ text = _("No new highlights to send.") })
            end
            return
        end
        local code, answer = self:request("/kosync/plugin/highlights", nil, json.encode{ device = self:deviceName(), books = books })
        if code ~= 200 then
            self:dlog("error", "highlights: NovelCheck answered " .. tostring(code))
            if not quiet then
                UIManager:show(InfoMessage:new{ text = (code == 401) and _("Wrong username or sync code.")
                    or (_("Could not send the highlights.") .. "\n" .. tostring(code or answer)) })
            end
            return
        end
        local good, data = pcall(json.decode, answer)
        for i, k in ipairs(keys) do
            -- a book NovelCheck doesn't have yet is sent again next time (it may be in the library by then)
            if not (good and type(data) == "table" and type(data.ok) == "table") or data.ok[i] then
                sigs:saveSetting(k.key, k.sig)
            end
        end
        sigs:flush()
        if not quiet then
            local text = _("Sent highlights from") .. " " .. tostring(#books) .. " " .. _("books") .. " (" .. tostring(total) .. ")."
            if good and type(data) == "table" and (data.unmatched or 0) > 0 then
                text = text .. "\n" .. tostring(data.unmatched) .. " " .. _("are not in your NovelCheck library, so they were skipped.")
            end
            UIManager:show(InfoMessage:new{ text = text })
        end
    end
end
