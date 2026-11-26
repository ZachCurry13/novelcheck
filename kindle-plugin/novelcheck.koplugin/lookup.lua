--[[
NovelCheck plugin: look up a book the library does not have (the same as Check a book), see its rating, and ask for it as an
ebook or audiobook. Also the book's page (showBookInfo) for every search result: Download and Nightstand when the library has
it, a request button for each format it lacks.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ButtonDialog = require("ui/widget/buttondialog")
local InfoMessage = require("ui/widget/infomessage")
local Menu = require("ui/widget/menu")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local util = require("util")
local Device = require("device")

return function(NovelCheck)
    local function sizeText(bytes)
        bytes = tonumber(bytes) or 0
        if bytes >= 1048576 then
            return string.format("%.1f MB", bytes / 1048576)
        end
        return string.format("%d KB", math.max(1, math.floor(bytes / 1024)))
    end

    -- postJSON sends a change; the message NovelCheck answered with when it fails.
    function NovelCheck:postJSON(path, tbl)
        local code, body = self:request(path, nil, json.encode(tbl))
        local ok, data = pcall(json.decode, body or "")
        if not ok or type(data) ~= "table" then
            data = {}
        end
        return code, data
    end

    local FORMAT = { ebook = _("ebook"), audio = _("audiobook") }

    -- lookupOnline: the books a typed name could mean; one tap picks it (a sure match is picked for you).
    function NovelCheck:lookupOnline(q)
        UIManager:show(InfoMessage:new{ text = _("Looking it up..."), timeout = 1 })
        UIManager:nextTick(function()
            local code, body = self:request("/kosync/plugin/lookup?q=" .. util.urlEncode(q))
            local ok, data = pcall(json.decode, body or "")
            if not ok or type(data) ~= "table" then
                data = {}
            end
            if code ~= 200 then
                UIManager:show(InfoMessage:new{ text = data.message or (_("Could not reach NovelCheck.") .. "\n" .. tostring(code)) })
                return
            end
            local choices = data.choices or {}
            if #choices == 0 then
                UIManager:show(InfoMessage:new{ text = _("No books found.") })
            elseif data.sure then
                self:checkPick(choices[1])
            else
                local items = {}
                for _i, c in ipairs(choices) do
                    local line = c.title .. ((c.author or "") ~= "" and (" - " .. c.author) or "")
                    table.insert(items, {
                        text = line,
                        mandatory = (c.year or 0) > 0 and tostring(c.year) or nil,
                        callback = function()
                            UIManager:close(self.pickmenu)
                            self:checkPick(c)
                        end,
                    })
                end
                self.pickmenu = Menu:new{
                    title = _("Which book did you mean?"),
                    item_table = items,
                    is_borderless = true,
                    is_popout = false,
                    covers_fullscreen = true,
                    width = Device.screen:getWidth(),
                    height = Device.screen:getHeight(),
                    close_callback = function() self.pickmenu = nil end,
                }
                if self.addBack then
                    self:addBack(self.pickmenu)
                end
                UIManager:show(self.pickmenu)
            end
        end)
    end

    -- checkPick saves the picked book and starts rating it, then shows its page once the rating is ready.
    function NovelCheck:checkPick(c)
        UIManager:show(InfoMessage:new{ text = _("Checking this book. It can take a minute."), timeout = 2 })
        UIManager:nextTick(function()
            local code, data = self:postJSON("/kosync/plugin/check", { title = c.title, author = c.author, isbn = c.isbn })
            if code ~= 200 or not data.id then
                UIManager:show(InfoMessage:new{ text = data.message or (_("Could not check it.") .. "\n" .. tostring(code)) })
                return
            end
            local tries = 0
            local function poll()
                local info = self:getJSON("/kosync/plugin/info/" .. tostring(data.id))
                tries = tries + 1
                if info and info.status ~= "analyzed" and tries < 45 then
                    UIManager:scheduleIn(4, poll)
                    return
                end
                self:showBookInfo({ id = data.id, title = c.title })
            end
            poll()
        end)
    end

    -- showBookInfo: what the book is, with Download and Nightstand when the library has it, and a request button for each format it lacks.
    function NovelCheck:showBookInfo(hit)
        local info, err = self:getJSON("/kosync/plugin/info/" .. tostring(hit.id))
        if not info then
            UIManager:show(InfoMessage:new{ text = err })
            return
        end
        if info.pending then
            UIManager:show(InfoMessage:new{ text = _("Still being checked. Try again in a minute.") })
            return
        end
        if info.blocked then -- outside this account's rules: nothing about the book, and a way to ask a parent
            local ask
            ask = ButtonDialog:new{
                title = info.message,
                buttons = {
                    { { text = _("Ask a parent for access"), callback = function() UIManager:close(ask); self:askAccess(hit) end } },
                    { { text = _("Close"), callback = function() UIManager:close(ask) end } },
                },
            }
            UIManager:show(ask)
            return
        end
        local lines = { info.title or hit.title }
        if info.author and info.author ~= "" then
            table.insert(lines, _("by") .. " " .. info.author)
        end
        if info.series and info.series ~= "" then
            local n = tonumber(info.series_index)
            table.insert(lines, info.series .. ((n and n > 0) and (" #" .. tostring(n)) or ""))
        end
        if type(info.level) == "number" then
            local why = (info.reason and info.reason ~= "") and (": " .. info.reason) or ""
            table.insert(lines, _("Spice level") .. " " .. tostring(info.level) .. why)
        elseif info.status and info.status ~= "analyzed" then
            table.insert(lines, _("Still being rated. Open it again in a minute."))
        end
        if info.rating_only then
            table.insert(lines, info.message)
        elseif info.url then
            table.insert(lines, _("In your library") .. (info.format and (" (" .. info.format .. ", " .. sizeText(info.size) .. ")") or ""))
        elseif info.in_library then
            table.insert(lines, _("In the library, but not as an ebook."))
        else
            table.insert(lines, _("Not available in the library yet."))
        end
        -- everything else is behind Details, so the buttons are always on the screen
        local more = {}
        if info.verdict and info.verdict ~= "" then
            table.insert(more, info.verdict)
        end
        if info.summary and info.summary ~= "" then
            table.insert(more, info.summary)
        end
        for _i, g in ipairs(info.content or {}) do
            table.insert(more, g.group .. ": " .. table.concat(g.items or {}, ", "))
        end
        if info.genres and #info.genres > 0 then
            table.insert(more, table.concat(info.genres, ", "))
        end
        local dialog
        local buttons = {}
        if info.url then
            local on_device = hit.on_device
            table.insert(buttons, { { text = on_device and _("Already on this device") or _("Download to this device"),
                enabled = not on_device,
                callback = function() UIManager:close(dialog); self:downloadHit(hit, info) end } })
        end
        if info.in_library then
            local queued = info.queue and info.queue ~= ""
            table.insert(buttons, { { text = queued and _("On your Nightstand") or _("Add to Nightstand"), enabled = not queued,
                callback = function() UIManager:close(dialog); self:addToNightstand(hit) end } })
        end
        if info.free and not info.url then
            table.insert(buttons, { { text = _("Add the free copy to the library"),
                callback = function() UIManager:close(dialog); self:addFreeCopy(info.title or hit.title, info.author) end } })
        end
        for _i, f in ipairs(info.request or {}) do
            table.insert(buttons, { { text = _("Request the") .. " " .. FORMAT[f],
                callback = function() UIManager:close(dialog); self:requestFormat(hit, f) end } })
        end
        for _i, f in ipairs(info.wished or {}) do
            table.insert(buttons, { { text = _("Asked for the") .. " " .. FORMAT[f], enabled = false } })
        end
        if #more > 0 then
            table.insert(buttons, { { text = _("Details: why this rating, summary"),
                callback = function()
                    local TextViewer = require("ui/widget/textviewer")
                    UIManager:show(TextViewer:new{ title = info.title or hit.title, text = table.concat(more, "\n\n") })
                end } })
        end
        table.insert(buttons, { { text = _("Close"), callback = function() UIManager:close(dialog) end } })
        dialog = ButtonDialog:new{ title = table.concat(lines, "\n"), buttons = buttons }
        UIManager:show(dialog)
    end

    -- addFreeCopy puts the public-domain copy of a book in the family library (Project Gutenberg); it shows in the library a minute later.
    function NovelCheck:addFreeCopy(title, author)
        UIManager:show(InfoMessage:new{ text = _("Adding the free copy..."), timeout = 1 })
        UIManager:nextTick(function()
            local code, data = self:postJSON("/kosync/plugin/free-add", { title = title, author = author })
            if code == 200 then
                UIManager:show(InfoMessage:new{ text = _("Added to the library. Open this book again in a minute to download it."), timeout = 4 })
            else
                UIManager:show(InfoMessage:new{ text = data.message or (_("Could not add it.") .. "\n" .. tostring(code)) })
            end
        end)
    end

    -- showWant lists the books on your Hardcover Want to read shelf against the library: tap one to download it, add its free copy,
    -- or request it.
    function NovelCheck:showWant()
        NetworkMgr:runWhenOnline(function()
            local code, body = self:request("/kosync/plugin/want")
            local ok, data = pcall(json.decode, body or "")
            if not ok or type(data) ~= "table" then
                data = {}
            end
            if code ~= 200 then
                UIManager:show(InfoMessage:new{ text = data.message or (_("Could not reach NovelCheck.") .. "\n" .. tostring(code)) })
                return
            end
            if not data.connected then
                UIManager:show(InfoMessage:new{ text = _("Connect Hardcover in NovelCheck (Settings, Connections) to see your Want to read list here.") })
                return
            end
            local items = {}
            for _i, b in ipairs(data.books or {}) do
                local mark = b.in_library and (b.downloadable and _("ready") or _("in library")) or (b.free and _("free") or "")
                table.insert(items, {
                    text = b.title .. ((b.author or "") ~= "" and (" - " .. b.author) or ""),
                    mandatory = mark ~= "" and mark or nil,
                    callback = function()
                        UIManager:close(self.wantmenu)
                        if b.in_library and b.id then
                            self:showBookInfo({ id = b.id, title = b.title })
                        else
                            self:checkPick({ title = b.title, author = b.author })
                        end
                    end,
                })
            end
            if #items == 0 then
                UIManager:show(InfoMessage:new{ text = _("Nothing on your Want to read shelf.") })
                return
            end
            self.wantmenu = Menu:new{
                title = _("Want to read"),
                item_table = items,
                is_borderless = true,
                is_popout = false,
                covers_fullscreen = true,
                width = Device.screen:getWidth(),
                height = Device.screen:getHeight(),
                close_callback = function() self.wantmenu = nil end,
            }
            if self.addBack then
                self:addBack(self.wantmenu)
            end
            UIManager:show(self.wantmenu)
        end)
    end

    function NovelCheck:askAccess(hit)
        local code, data = self:postJSON("/kosync/plugin/access-request", { book_id = hit.id })
        if code == 200 then
            UIManager:show(InfoMessage:new{ text = _("Asked. A parent will look at it."), timeout = 3 })
        else
            UIManager:show(InfoMessage:new{ text = data.message or (_("Could not ask.") .. "\n" .. tostring(code)) })
        end
    end

    function NovelCheck:requestFormat(hit, format)
        local code, data = self:postJSON("/kosync/plugin/wish", { book_id = hit.id, format = format })
        if code == 200 then
            UIManager:show(InfoMessage:new{ text = _("Asked for the") .. " " .. FORMAT[format] .. ".\n" .. _("A parent will see it on the wishlist."), timeout = 3 })
        else
            UIManager:show(InfoMessage:new{ text = data.message or (_("Could not ask for it.") .. "\n" .. tostring(code)) })
        end
    end
end
