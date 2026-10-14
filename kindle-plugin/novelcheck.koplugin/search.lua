--[[
NovelCheck plugin: search your NovelCheck library from the e-reader. Type any
words of a title or author, pick a book, read what it is about (with its
content rating), then download it to this device and/or put it on your
Nightstand. "New in library" lists the newest books the same way.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ButtonDialog = require("ui/widget/buttondialog")
local Device = require("device")
local InfoMessage = require("ui/widget/infomessage")
local InputDialog = require("ui/widget/inputdialog")
local Menu = require("ui/widget/menu")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")
local util = require("util")

return function(NovelCheck, ctx)
    local _unused = { ctx }

    local function sizeText(bytes)
        bytes = tonumber(bytes) or 0
        if bytes >= 1048576 then
            return string.format("%.1f MB", bytes / 1048576)
        end
        return string.format("%d KB", math.max(1, math.floor(bytes / 1024)))
    end

    -- getJSON asks NovelCheck and decodes the answer; nil and a message when it fails.
    function NovelCheck:getJSON(path)
        local code, body = self:request(path)
        if code == 401 then
            return nil, _("NovelCheck did not accept the username or sync code.")
        elseif code ~= 200 then
            return nil, _("Could not reach NovelCheck.") .. "\n" .. tostring(code or body)
        end
        local ok, data = pcall(json.decode, body)
        if not ok or type(data) ~= "table" then
            return nil, _("NovelCheck answered something unexpected.")
        end
        return data
    end

    -- Search library: ask for words, then list the matching books.
    function NovelCheck:searchLibrary()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        local dialog
        dialog = InputDialog:new{
            title = _("Search your NovelCheck library"),
            input = "",
            input_hint = _("Any words of the title or author"),
            buttons = { {
                { text = _("Cancel"), callback = function() UIManager:close(dialog) end },
                { text = _("Search"), is_enter_default = true, callback = function()
                    local q = (dialog:getInputText() or ""):gsub("^%s+", ""):gsub("%s+$", "")
                    UIManager:close(dialog)
                    if #q >= 2 then
                        self:runSearch("/kosync/plugin/search?q=" .. util.urlEncode(q), _("Search") .. ": " .. q)
                    end
                end },
            } },
        }
        UIManager:show(dialog)
        dialog:onShowKeyboard()
    end

    function NovelCheck:newInLibrary()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        self:runSearch("/kosync/plugin/new?", _("New in library"))
    end

    function NovelCheck:runSearch(path, title)
        NetworkMgr:runWhenOnline(function()
            local data, err = self:getJSON(path .. "&device=" .. util.urlEncode(self:deviceName()))
            if not data then
                UIManager:show(InfoMessage:new{ text = err })
                return
            end
            if type(data.books) ~= "table" or #data.books == 0 then
                UIManager:show(InfoMessage:new{ text = _("No books found.") })
                return
            end
            self:showHits(title, data.books)
        end)
    end

    function NovelCheck:showHits(title, hits)
        local items = {}
        for _i, h in ipairs(hits) do
            local line = h.title
            if h.author and h.author ~= "" then
                line = line .. " - " .. h.author
            end
            local marks = {}
            if type(h.level) == "number" then
                table.insert(marks, "L" .. tostring(h.level))
            end
            if h.on_device then
                table.insert(marks, _("on device"))
            end
            if h.queue and h.queue ~= "" then
                table.insert(marks, _("Nightstand"))
            end
            table.insert(items, {
                text = line,
                mandatory = table.concat(marks, "  "),
                callback = function() self:showBookInfo(h) end,
            })
        end
        if self.searchmenu then
            UIManager:close(self.searchmenu)
        end
        self.searchmenu = Menu:new{
            title = title .. " (" .. tostring(#hits) .. ")",
            item_table = items,
            is_borderless = true,
            is_popout = false,
            covers_fullscreen = true,
            width = Device.screen:getWidth(),
            height = Device.screen:getHeight(),
            close_callback = function() self.searchmenu = nil end,
        }
        UIManager:show(self.searchmenu)
    end

    -- showBookInfo: what the book is, with Download and Nightstand buttons.
    function NovelCheck:showBookInfo(hit)
        local info, err = self:getJSON("/kosync/plugin/info/" .. tostring(hit.id))
        if not info then
            UIManager:show(InfoMessage:new{ text = err })
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
            table.insert(lines, _("Content rating") .. ": " .. tostring(info.level) .. " / 5" .. why)
        end
        if info.verdict and info.verdict ~= "" then
            table.insert(lines, info.verdict)
        end
        if info.summary and info.summary ~= "" then
            table.insert(lines, "")
            table.insert(lines, info.summary)
        end
        if info.format then
            table.insert(lines, "")
            table.insert(lines, info.format .. ", " .. sizeText(info.size))
        end
        local dialog
        local on_device = hit.on_device
        local queued = info.queue and info.queue ~= ""
        dialog = ButtonDialog:new{
            title = table.concat(lines, "\n"),
            buttons = {
                {
                    { text = on_device and _("Already on this device") or _("Download to this device"), enabled = info.url ~= nil and not on_device,
                      callback = function()
                          UIManager:close(dialog)
                          self:downloadHit(hit, info)
                      end },
                },
                {
                    { text = queued and _("On your Nightstand") or _("Add to Nightstand"), enabled = not queued,
                      callback = function()
                          UIManager:close(dialog)
                          self:addToNightstand(hit)
                      end },
                },
                { { text = _("Close"), callback = function() UIManager:close(dialog) end } },
            },
        }
        UIManager:show(dialog)
    end

    function NovelCheck:downloadHit(hit, info)
        UIManager:show(InfoMessage:new{ text = _("Downloading..."), timeout = 1 })
        UIManager:nextTick(function()
            local ok, code = self:fetchBook({ id = hit.id, url = info.url, format = info.format })
            if ok then
                hit.on_device = true
                self:reportQuietly()
                UIManager:show(InfoMessage:new{ text = _("Downloaded:") .. "\n" .. (hit.title or "") })
            else
                UIManager:show(InfoMessage:new{ text = _("The download failed.") .. "\n" .. tostring(code) })
            end
        end)
    end

    function NovelCheck:addToNightstand(hit)
        local code = self:request("/kosync/plugin/nightstand", nil, json.encode{ book_id = hit.id })
        if code == 200 then
            hit.queue = "queued"
            UIManager:show(InfoMessage:new{ text = _("Added to your Nightstand."), timeout = 2 })
        else
            UIManager:show(InfoMessage:new{ text = _("Could not add it.") .. "\n" .. tostring(code) })
        end
    end
end
