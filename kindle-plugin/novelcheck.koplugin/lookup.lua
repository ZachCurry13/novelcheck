--[[
NovelCheck plugin: look up a book the library does not have (the same as Check a book), see its rating, and ask for it as an
ebook or audiobook. Also the book's page (showBookInfo) for every search result: Download and Nightstand when the library has
it, a request button for each format it lacks.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ButtonDialog = require("ui/widget/buttondialog")
local InfoMessage = require("ui/widget/infomessage")
local Menu = require("ui/widget/menu")
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
        if info.blocked or info.pending then
            UIManager:show(InfoMessage:new{ text = info.message or _("Still being checked. Try again in a minute.") })
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
        if info.url then
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

    function NovelCheck:requestFormat(hit, format)
        local code, data = self:postJSON("/kosync/plugin/wish", { book_id = hit.id, format = format })
        if code == 200 then
            UIManager:show(InfoMessage:new{ text = _("Asked for the") .. " " .. FORMAT[format] .. ".\n" .. _("A parent will see it on the wishlist."), timeout = 3 })
        else
            UIManager:show(InfoMessage:new{ text = data.message or (_("Could not ask for it.") .. "\n" .. tostring(code)) })
        end
    end
end
