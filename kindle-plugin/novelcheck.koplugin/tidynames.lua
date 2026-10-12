--[[
NovelCheck plugin: "Tidy book names". For the books on this device that
NovelCheck matched to your library, it sets KOReader's title and author to the
library's, so the file list and home screen show proper names instead of file
names. It changes the displayed name only (KOReader's own custom metadata),
never the files, so progress sync is not affected. Undo any of them in
KOReader's book info (Book information > the title > Reset).
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ConfirmBox = require("ui/widget/confirmbox")
local Event = require("ui/event")
local InfoMessage = require("ui/widget/infomessage")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local util = require("util")

return function(NovelCheck, ctx)
    local _unused = { ctx }

    -- currentProps is what KOReader shows for a book now (custom values included).
    -- With open set it may open the book to read its original title and author.
    local function currentProps(info, path, open)
        local ok, props = pcall(info.getDocProps, info, path, nil, not open)
        if ok and type(props) == "table" then
            return props
        end
        return {}
    end

    -- shallowCopy keeps the original props intact while KOReader edits its copy.
    local function shallowCopy(t)
        local out = {}
        for k, v in pairs(t) do
            out[k] = v
        end
        return out
    end

    function NovelCheck:tidyNames()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        local info = self.ui and self.ui.bookinfo
        if not info or not info.setCustomMetadata then
            UIManager:show(InfoMessage:new{ text = _("This KOReader version cannot change book names from a plugin.") })
            return
        end
        NetworkMgr:runWhenOnline(function()
            local code, body = self:request("/kosync/plugin/names?device=" .. util.urlEncode(self:deviceName()))
            local ok, data = pcall(json.decode, body or "")
            if code ~= 200 or not ok or type(data) ~= "table" or type(data.names) ~= "table" then
                UIManager:show(InfoMessage:new{ text = _("Could not get the names from NovelCheck.") .. "\n" .. tostring(code) })
                return
            end
            local byMd5 = {}
            for _i, n in ipairs(data.names) do
                if n.md5 and n.md5 ~= "" then
                    byMd5[n.md5] = n
                end
            end
            local books = {}
            self:scanBooks(self:downloadDir(), books, 0, false)
            local todo = {}
            for _i, b in ipairs(books) do
                local want = byMd5[b.md5]
                if want then
                    local props = currentProps(info, b.path)
                    if props.title ~= want.title or (want.author ~= "" and props.authors ~= want.author) then
                        table.insert(todo, { path = b.path, want = want })
                    end
                end
            end
            if #todo == 0 then
                UIManager:show(InfoMessage:new{ text = _("The names on this device already match your library.") })
                return
            end
            UIManager:show(ConfirmBox:new{
                text = _("Show your library's title and author for") .. " " .. tostring(#todo) .. " " .. _("books?") .. "\n\n" .. _("Only the displayed name changes, not the files."),
                ok_text = _("Tidy"),
                ok_callback = function()
                    local changed = 0
                    for _i, t in ipairs(todo) do
                        local props = shallowCopy(currentProps(info, t.path, true))
                        local done = pcall(info.setCustomMetadata, info, t.path, props, "title", t.want.title)
                        if done and t.want.author ~= "" then
                            pcall(info.setCustomMetadata, info, t.path, shallowCopy(currentProps(info, t.path, true)), "authors", t.want.author)
                        end
                        if done then
                            changed = changed + 1
                        end
                    end
                    pcall(function() UIManager:broadcastEvent(Event:new("BookMetadataChanged")) end)
                    UIManager:show(InfoMessage:new{ text = tostring(changed) .. " " .. _("book names tidied.") })
                end,
            })
        end)
    end
end
