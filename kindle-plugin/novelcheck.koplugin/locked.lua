--[[
NovelCheck plugin: Kindle-store books stay out of KOReader's library folder. A Kindle-store book (KFX, AZW, or a MOBI
or AZW3 whose header says it is encrypted) only opens in Amazon's own reader; KOReader cannot read it, and opening it
for a cover or a title can make KOReader stop. Tidy never moves one into Books, and at start the plugin moves any it
finds in the library folder back to the Kindle's documents folder (where Amazon's reader shows them), with its .sdr
folder, without overwriting anything, and tells NovelCheck so its backup copy moves too.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local lfs = require("libs/libkoreader-lfs")

local function u32(s, i)
    local a, b, c, d = s:byte(i, i + 3)
    return ((a or 0) * 16777216) + ((b or 0) * 65536) + ((c or 0) * 256) + (d or 0)
end

return function(NovelCheck)
    -- isLockedBook says whether a book file is a Kindle-store (DRM) book KOReader cannot open.
    function NovelCheck:isLockedBook(path)
        local ext = (path:match("%.([^./]+)$") or ""):lower()
        if ext == "kfx" or ext == "azw" or ext == "azw1" or ext == "kfx-zip" then
            return true
        end
        if ext ~= "mobi" and ext ~= "azw3" and ext ~= "prc" then
            return false
        end
        local f = io.open(path, "rb")
        if not f then
            return false
        end
        local head = f:read(96) or ""
        local locked = false
        if #head >= 82 then
            local rec0 = u32(head, 79) -- offset 78, 1-based 79
            if f:seek("set", rec0 + 12) then
                local enc = f:read(2) or ""
                locked = #enc == 2 and (enc:byte(1) * 256 + enc:byte(2)) ~= 0
            end
        end
        f:close()
        return locked
    end

    -- rescueLockedBooks moves Kindle-store books out of the library folder back to documents. It says how many.
    function NovelCheck:rescueLockedBooks()
        local lib = self:libraryDir()
        local docs = self:kindleRoot() .. "/documents"
        if lfs.attributes(docs, "mode") ~= "directory" or lib == docs then
            return 0
        end
        local found = {}
        local function walk(dir, depth)
            if depth > 4 or #found > 2000 then return end
            local ok, iter, state = pcall(lfs.dir, dir)
            if not ok then return end
            for name in iter, state do
                if name ~= "." and name ~= ".." and name:sub(1, 1) ~= "." and not name:match("%.sdr$") then
                    local p = dir .. "/" .. name
                    local mode = lfs.attributes(p, "mode")
                    if mode == "directory" then
                        walk(p, depth + 1)
                    elseif mode == "file" and self:isLockedBook(p) then
                        found[#found + 1] = p
                    end
                end
            end
        end
        walk(lib, 0)
        local moves = {}
        for _i, p in ipairs(found) do
            local name = p:match("[^/]+$")
            local to = docs .. "/" .. name
            if lfs.attributes(to, "mode") == nil and os.rename(p, to) then
                moves[#moves + 1] = { from = p, to = to }
                local sf, st = p:gsub("%.[^./]+$", "") .. ".sdr", to:gsub("%.[^./]+$", "") .. ".sdr"
                if lfs.attributes(sf, "mode") == "directory" and lfs.attributes(st, "mode") == nil and os.rename(sf, st) then
                    moves[#moves + 1] = { from = sf, to = st }
                end
            end
        end
        if #found > 0 then
            self:dlog("info", "moved " .. #moves .. " of " .. #found .. " Kindle-store books out of the library folder back to documents")
        end
        if #moves > 0 and self.tellMoved then
            pcall(self.tellMoved, self, moves)
        end
        return #found
    end
end
