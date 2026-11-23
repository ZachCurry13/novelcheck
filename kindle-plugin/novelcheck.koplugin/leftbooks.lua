--[[
NovelCheck plugin: books that left this device. When a report finds that library books are gone from the e-reader
(deleted here or elsewhere), NovelCheck sends them back in its answer, and the plugin asks for each (five at most;
the rest wait in NovelCheck under Settings > Kindle & KOReader): keep it in NovelCheck, remove it from my library
(only this person stops seeing it), or remove it and ask for it to be deleted for good (the main admin decides).
The book's rating and cover always stay in NovelCheck. It adds its functions to the NovelCheck plugin class.
]]

local ButtonDialog = require("ui/widget/buttondialog")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")

return function(NovelCheck)
    -- askLeft takes the report's answer and asks about each book in its "left" list, one at a time.
    function NovelCheck:askLeft(answer)
        local good, data = pcall(json.decode, answer or "")
        if not good or type(data) ~= "table" or type(data.left) ~= "table" then
            return
        end
        local list = {}
        for i, b in ipairs(data.left) do
            if i <= 5 then
                table.insert(list, b)
            end
        end
        local function ask(i)
            local b = list[i]
            if not b then
                return
            end
            local dialog
            local function choose(choice)
                UIManager:close(dialog)
                pcall(function() self:request("/kosync/plugin/left", nil, json.encode{ book_id = b.id, choice = choice }) end)
                UIManager:scheduleIn(0.5, function() ask(i + 1) end)
            end
            dialog = ButtonDialog:new{
                title = _("Left this device:") .. "\n" .. tostring(b.title) .. "\n\n" .. _("Keep it in NovelCheck, or remove it? Its rating stays either way."),
                buttons = {
                    { { text = _("Keep"), callback = function() choose("keep") end } },
                    { { text = _("Remove from my library"), callback = function() choose("hide") end } },
                    { { text = _("Remove, ask to delete"), callback = function() choose("delete") end } },
                },
            }
            UIManager:show(dialog)
        end
        ask(1)
    end
end
