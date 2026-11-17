--[[
NovelCheck plugin: "Tidy my Kindle's books". Books that lie around in other folders (the Kindle's own documents
folder, loose in Books, anywhere on the user storage) are moved into the Books folder, one folder per author
(Books/Author Name/Book.epub; "Unknown author" when the book doesn't say). Rules that keep it safe:
  - only a book NovelCheck already has a backup copy of (same path and size) is moved: back up first;
  - nothing is overwritten and nothing is deleted;
  - the book's reading data (its .sdr folder) moves with it;
  - it asks first, showing what it will do, and "Undo the last tidy" puts everything back;
  - NovelCheck's own download folder, KOReader, system and hidden folders are left alone, and so is a book already
    inside a folder of Books.
NovelCheck is told about each move so its backup copy moves too (the next backup doesn't send the books again).
Kindle-store books moved out of documents no longer show in Amazon's own Kindle reader: use KOReader.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ConfirmBox = require("ui/widget/confirmbox")
local InfoMessage = require("ui/widget/infomessage")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")
local util = require("util")

local EXTS = { epub = true, pdf = true, mobi = true, azw = true, azw3 = true, azw1 = true, kfx = true, kepub = true, fb2 = true, cbz = true, prc = true }

local function safeName(s)
    s = tostring(s or ""):gsub('[/\\:*?"<>|%c]', " "):gsub("%s+", " "):gsub("^%s+", ""):gsub("%s+$", ""):gsub("%.+$", "")
    if s == "" then
        return "Unknown author"
    end
    return s:sub(1, 80)
end

local function exists(p)
    return lfs.attributes(p, "mode") ~= nil
end

return function(NovelCheck)
    -- tidyCandidates lists the book files that are not yet where they belong: {path, size, name}.
    function NovelCheck:tidyCandidates(root, booksDir)
        local out = {}
        local function walk(dir, depth)
            if depth > 12 or #out >= 2000 then
                return
            end
            local ok, iter, state = pcall(lfs.dir, dir)
            if not ok then
                return
            end
            for name in iter, state do
                local l = name:lower()
                if name ~= "." and name ~= ".." and name:sub(1, 1) ~= "." and l ~= "koreader" and l ~= "system" and l ~= "novelcheck"
                    and l ~= "extensions" and l ~= "trash" and not l:match("%.sdr$") then
                    local path = dir .. "/" .. name
                    local attr = lfs.attributes(path)
                    if attr and attr.mode == "directory" then
                        -- a folder inside Books is already tidy (an author folder); look at it only outside Books
                        if path ~= booksDir then
                            walk(path, depth + 1)
                        end
                    elseif attr and attr.mode == "file" and EXTS[(name:match("%.([^.]+)$") or ""):lower()] and attr.size >= 2048 then
                        table.insert(out, { path = path, size = attr.size, name = name })
                    end
                end
            end
        end
        walk(root, 0)
        -- loose files directly in Books count (the walk skipped Books itself, so look at its top level)
        local ok, iter, state = pcall(lfs.dir, booksDir)
        if ok then
            for name in iter, state do
                local path = booksDir .. "/" .. name
                local attr = lfs.attributes(path)
                if name:sub(1, 1) ~= "." and attr and attr.mode == "file" and EXTS[(name:match("%.([^.]+)$") or ""):lower()] and attr.size >= 2048 then
                    table.insert(out, { path = path, size = attr.size, name = name })
                end
            end
        end
        return out
    end

    function NovelCheck:tidyBooks()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        local root, booksDir = self:kindleRoot(), self:libraryDir()
        local found = self:tidyCandidates(root, booksDir)
        if #found == 0 then
            UIManager:show(InfoMessage:new{ text = _("Every book is already in its place.") })
            return
        end
        -- which of them does NovelCheck already hold a copy of?
        local list = {}
        for _i, f in ipairs(found) do
            list[#list + 1] = { path = f.path, size = f.size, mtime = lfs.attributes(f.path, "modification") or 0 }
        end
        local missing = {}
        local i = 1
        while i <= #list do
            local batch = {}
            for j = i, math.min(i + 1999, #list) do
                batch[#batch + 1] = list[j]
            end
            i = i + 2000
            local code, answer = self:request("/kosync/plugin/kindle/check", nil, json.encode{ device = self:deviceName(), mode = "backup", files = batch })
            local good, data = pcall(json.decode, answer or "")
            if code ~= 200 or not good or type(data) ~= "table" then
                UIManager:show(InfoMessage:new{ text = _("Could not reach NovelCheck.") .. "\n" .. tostring(code or answer) })
                return
            end
            for _k, p in ipairs(type(data.send) == "table" and data.send or {}) do
                missing[p] = true
            end
        end
        local plan, skipped = {}, 0
        local used = {}
        for _i, f in ipairs(found) do
            if missing[f.path] then
                skipped = skipped + 1
            else
                local author = ""
                local ok, sum = pcall(util.partialMD5, f.path)
                if ok and sum and self.bookProps then
                    local okp, _t, a = pcall(self.bookProps, self, f.path, sum)
                    author = (okp and a) or ""
                end
                local target = booksDir .. "/" .. safeName(author) .. "/" .. f.name
                if target ~= f.path and not exists(target) and not used[target] then
                    used[target] = true
                    plan[#plan + 1] = { from = f.path, to = target }
                else
                    skipped = skipped + 1
                end
            end
        end
        if #plan == 0 then
            UIManager:show(InfoMessage:new{ text = skipped > 0 and (tostring(skipped) .. " " .. _("books were left where they are (not backed up yet, or the name is taken). Run Back up my Kindle now first, then tidy again.")) or _("Every book is already in its place.") })
            return
        end
        UIManager:show(ConfirmBox:new{
            text = _("Move") .. " " .. #plan .. " " .. _("books into Books/<author> folders?") .. "\n\n"
                .. _("Example:") .. "\n" .. plan[1].from:gsub("^.*/", "") .. "\n> " .. plan[1].to:gsub("^/mnt/us/", "") .. "\n\n"
                .. (skipped > 0 and (tostring(skipped) .. " " .. _("are left where they are (not backed up yet, or the name is taken).") .. "\n") or "")
                .. _("Nothing is deleted. Undo the last tidy any time."),
            ok_text = _("Move them"),
            ok_callback = function() self:doTidy(plan) end,
        })
    end

    local function moveOne(from, to)
        local dir = to:match("^(.*)/[^/]*$")
        if dir and not exists(dir) then
            local acc = ""
            for part in dir:gmatch("[^/]+") do
                acc = acc .. "/" .. part
                if not exists(acc) then
                    lfs.mkdir(acc)
                end
            end
        end
        return os.rename(from, to)
    end

    -- doTidy moves each book and its .sdr folder, tells NovelCheck, and remembers it for the undo.
    function NovelCheck:doTidy(plan)
        local done, moves = 0, {}
        for _i, m in ipairs(plan) do
            if not exists(m.to) and moveOne(m.from, m.to) then
                done = done + 1
                moves[#moves + 1] = { from = m.from, to = m.to }
                local sf, st = m.from:gsub("%.[^./]+$", "") .. ".sdr", m.to:gsub("%.[^./]+$", "") .. ".sdr"
                if lfs.attributes(sf, "mode") == "directory" and not exists(st) and moveOne(sf, st) then
                    moves[#moves + 1] = { from = sf, to = st }
                end
            end
        end
        self.settings:saveSetting("tidy_last", moves)
        self.settings:flush()
        self:tellMoved(moves)
        UIManager:show(InfoMessage:new{ text = _("Moved") .. " " .. done .. " " .. _("books.") .. " " .. _("NovelCheck > Undo the last tidy puts them back.") })
    end

    function NovelCheck:tellMoved(moves)
        local i = 1
        while i <= #moves do
            local batch = {}
            for j = i, math.min(i + 499, #moves) do
                batch[#batch + 1] = moves[j]
            end
            i = i + 500
            self:request("/kosync/plugin/kindle/moved", nil, json.encode{ device = self:deviceName(), moves = batch })
        end
    end

    function NovelCheck:undoTidy()
        local moves = self.settings:readSetting("tidy_last")
        if type(moves) ~= "table" or #moves == 0 then
            UIManager:show(InfoMessage:new{ text = _("There is nothing to undo.") })
            return
        end
        UIManager:show(ConfirmBox:new{
            text = _("Put the last") .. " " .. #moves .. " " .. _("moved items back where they were?"),
            ok_text = _("Put them back"),
            ok_callback = function()
                local back, undone = {}, 0
                for k = #moves, 1, -1 do
                    local m = moves[k]
                    if exists(m.to) and not exists(m.from) and moveOne(m.to, m.from) then
                        undone = undone + 1
                        back[#back + 1] = { from = m.to, to = m.from }
                    end
                end
                self.settings:saveSetting("tidy_last", {})
                self.settings:flush()
                self:tellMoved(back)
                UIManager:show(InfoMessage:new{ text = _("Put back:") .. " " .. undone })
            end,
        })
    end
end
