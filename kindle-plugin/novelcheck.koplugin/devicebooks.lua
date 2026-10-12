--[[
NovelCheck plugin: "On this device". Lists the books on this e-reader with how
far you are in each, and lets you delete one (asking first) or clear all the
finished ones.
]]

local ConfirmBox = require("ui/widget/confirmbox")
local Device = require("device")
local InfoMessage = require("ui/widget/infomessage")
local Menu = require("ui/widget/menu")
local UIManager = require("ui/uimanager")
local _ = require("gettext")

local DeviceBooks = {}

local FINISHED = 0.97

-- progress says how far a book is: its percent (0 to 1) and whether KOReader
-- marked it finished. nil when the book has never been opened.
local function progress(path)
    local ok, pct, done = pcall(function()
        local DocSettings = require("docsettings")
        local ds = DocSettings:open(path)
        local summary = ds:readSetting("summary")
        return ds:readSetting("percent_finished"), summary and summary.status == "complete"
    end)
    if not ok then
        return nil, false
    end
    return pct, done and true or false
end

local function isFinished(pct, done)
    return done or (pct ~= nil and pct >= FINISHED)
end

local function sizeText(bytes)
    if bytes >= 1048576 then
        return string.format("%.1f MB", bytes / 1048576)
    end
    return string.format("%d KB", math.max(1, math.floor(bytes / 1024)))
end

-- list reads the books under the download folder and their progress.
function DeviceBooks.list(plugin)
    local books = {}
    plugin:scanBooks(plugin:downloadDir(), books, 0, true)
    for _i, b in ipairs(books) do
        b.pct, b.done = progress(b.path)
        b.finished = isFinished(b.pct, b.done)
    end
    table.sort(books, function(a, b)
        return a.title:lower() < b.title:lower()
    end)
    return books
end

-- remove deletes a book file and what KOReader keeps about it.
function DeviceBooks.remove(book)
    pcall(function()
        local DocSettings = require("docsettings")
        DocSettings:open(book.path):purge()
    end)
    pcall(function()
        require("readhistory"):removeItemByPath(book.path)
    end)
    return os.remove(book.path)
end

local function status(b)
    if b.finished then
        return _("finished")
    elseif b.pct then
        return string.format("%d%%", math.floor(b.pct * 100 + 0.5))
    end
    return _("new")
end

-- show opens the list; tapping a book offers to delete it.
function DeviceBooks.show(plugin)
    local books = DeviceBooks.list(plugin)
    if #books == 0 then
        UIManager:show(InfoMessage:new{ text = _("No books found in the download folder.") })
        return
    end
    local items = {}
    for _i, b in ipairs(books) do
        table.insert(items, {
            text = b.title,
            mandatory = status(b) .. "  " .. sizeText(b.size),
            callback = function()
                UIManager:show(ConfirmBox:new{
                    text = _("Delete this book from the device?") .. "\n\n" .. b.title,
                    ok_text = _("Delete"),
                    ok_callback = function()
                        if DeviceBooks.remove(b) then
                            plugin:reportQuietly()
                            DeviceBooks.reopen(plugin)
                        else
                            UIManager:show(InfoMessage:new{ text = _("Could not delete the file.") })
                        end
                    end,
                })
            end,
        })
    end
    plugin.devicemenu = Menu:new{
        title = _("On this device") .. " (" .. tostring(#books) .. ")",
        item_table = items,
        is_borderless = true,
        is_popout = false,
        covers_fullscreen = true,
        width = Device.screen:getWidth(),
        height = Device.screen:getHeight(),
        close_callback = function() plugin.devicemenu = nil end,
    }
    UIManager:show(plugin.devicemenu)
end

-- reopen redraws the list after a change.
function DeviceBooks.reopen(plugin)
    if plugin.devicemenu then
        UIManager:close(plugin.devicemenu)
        plugin.devicemenu = nil
    end
    UIManager:nextTick(function() DeviceBooks.show(plugin) end)
end

-- removeFinished deletes every finished book, after listing them.
function DeviceBooks.removeFinished(plugin)
    local finished = {}
    for _i, b in ipairs(DeviceBooks.list(plugin)) do
        if b.finished then
            table.insert(finished, b)
        end
    end
    if #finished == 0 then
        UIManager:show(InfoMessage:new{ text = _("No finished books on this device.") })
        return
    end
    local lines = {}
    for i, b in ipairs(finished) do
        if i > 8 then
            table.insert(lines, "...")
            break
        end
        table.insert(lines, b.title)
    end
    UIManager:show(ConfirmBox:new{
        text = _("Delete these finished books from the device?") .. " (" .. tostring(#finished) .. ")\n\n" .. table.concat(lines, "\n"),
        ok_text = _("Delete"),
        ok_callback = function()
            local gone = 0
            for _i, b in ipairs(finished) do
                if DeviceBooks.remove(b) then
                    gone = gone + 1
                end
            end
            plugin:reportQuietly()
            UIManager:show(InfoMessage:new{ text = tostring(gone) .. " " .. _("books deleted.") })
        end,
    })
end

return DeviceBooks
