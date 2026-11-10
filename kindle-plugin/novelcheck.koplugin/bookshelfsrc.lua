--[[
NovelCheck plugin: a shelf for the Bookshelf plugin (github.com/AndyHazz/bookshelf.koplugin), if it is installed.
In Bookshelf, add a shelf and pick "NovelCheck books" under Source / grouping: it shows the books NovelCheck
downloaded to this device (the NovelCheck folder), newest first. Nothing here uses the network, and everything
is wrapped so that a missing or different Bookshelf can never stop the NovelCheck plugin from loading.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local lfs = require("libs/libkoreader-lfs")
local UIManager = require("ui/uimanager")
local _ = require("gettext")

local BOOK = { epub = true, pdf = true, mobi = true, azw3 = true, azw = true, kepub = true, fb2 = true, cbz = true, txt = true }

return function(NovelCheck, ctx)
    local _unused = { ctx }

    -- deviceShelf lists the book files in NovelCheck's download folder as Bookshelf records.
    function NovelCheck:deviceShelf()
        local out = {}
        local dir = self.downloadDir and self:downloadDir()
        if not dir or lfs.attributes(dir, "mode") ~= "directory" then
            return out
        end
        for name in lfs.dir(dir) do
            local ext = name:match("%.([%w]+)$")
            if ext and BOOK[ext:lower()] then
                local path = dir .. "/" .. name
                local title = name:gsub("%.[%w]+$", ""):gsub("_", " ")
                local author
                local t, a = title:match("^(.-) %- (.+)$") -- "Title - Author" file names
                if t and a then
                    title, author = t, a
                end
                table.insert(out, {
                    filepath = path,
                    title = title,
                    author = author,
                    format = ext:upper(),
                    added_time = lfs.attributes(path, "modification") or 0,
                })
            end
        end
        table.sort(out, function(x, y) return (x.added_time or 0) > (y.added_time or 0) end)
        return out
    end

    -- registerBookshelf offers the shelf to Bookshelf; false when Bookshelf isn't there (or is too old).
    function NovelCheck:registerBookshelf()
        local ok, res = pcall(function()
            local bs = self.ui and self.ui.bookshelf
            if not (bs and bs.registerSource and (bs.SOURCE_API or 0) >= 1) then
                return false
            end
            local plugin = self
            local done, why = bs:registerSource("novelcheck", {
                api = 1,
                label = function() return _("NovelCheck books") end,
                available = function() return true end,
                list = function()
                    local okList, list = pcall(plugin.deviceShelf, plugin)
                    return okList and list or {}
                end,
            })
            if not done then
                self:dlog("bookshelf: shelf refused: " .. tostring(why))
            end
            return done and true or false
        end)
        return ok and res or false
    end

    -- offerBookshelf registers now, and once more a moment later (plugins load in name order, so Bookshelf
    -- may not be there yet).
    function NovelCheck:offerBookshelf()
        if not self:registerBookshelf() then
            UIManager:nextTick(function() pcall(function() self:registerBookshelf() end) end)
        end
    end
end
