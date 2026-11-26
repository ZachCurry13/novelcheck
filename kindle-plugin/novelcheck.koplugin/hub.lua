--[[
NovelCheck plugin: the launcher card. Tapping NovelCheck's icon in KOReader's top bar opens one card with big buttons, like an
app's home screen, instead of a menu list: what it is connected as and when it last synced, then Nightstand, New, Search,
This Kindle, Sync now and More (More is the whole list, with the rarely used things: Back up and tidy, Settings).
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ButtonDialog = require("ui/widget/buttondialog")
local Device = require("device")
local Menu = require("ui/widget/menu")
local UIManager = require("ui/uimanager")
local _ = require("gettext")

return function(NovelCheck)
    local function trim(s)
        return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
    end

    -- ago says how long since a time, in words.
    local function ago(ts)
        ts = tonumber(ts)
        if not ts or ts <= 0 then
            return nil
        end
        local d = os.time() - ts
        if d < 90 then
            return _("just now")
        elseif d < 3600 then
            return string.format(_("%d minutes ago"), math.floor(d / 60))
        elseif d < 86400 then
            return string.format(_("%d hours ago"), math.floor(d / 3600))
        end
        return string.format(_("%d days ago"), math.floor(d / 86400))
    end

    -- showList shows a list of menu items (the plugin's own list) full screen; an item with a submenu opens it, a switch
    -- shows its state and the list is drawn again after it is pressed.
    function NovelCheck:showList(title, items)
        local menu
        local rows = {}
        for _i, it in ipairs(items) do
            if type(it) == "table" and it.text then
                local row = { text = it.text }
                if it.sub_item_table then
                    row.mandatory = "▸"
                    row.callback = function()
                        UIManager:close(menu)
                        self:showList(it.text, it.sub_item_table)
                    end
                else
                    local cb = it.callback
                    if it.checked_func then
                        row.mandatory = it.checked_func() and "✓" or ""
                    end
                    row.callback = function()
                        UIManager:close(menu)
                        if cb then
                            cb()
                        end
                        if it.checked_func then
                            self:showList(title, items) -- the switch's new state
                        end
                    end
                end
                table.insert(rows, row)
            end
        end
        menu = Menu:new{
            title = title,
            item_table = rows,
            is_borderless = true,
            is_popout = false,
            covers_fullscreen = true,
            width = Device.screen:getWidth(),
            height = Device.screen:getHeight(),
        }
        UIManager:show(menu)
    end

    -- showHub opens the launcher card.
    function NovelCheck:showHub()
        local dialog
        local function go(fn)
            return function()
                UIManager:close(dialog)
                fn()
            end
        end
        local lines = { _("NovelCheck") }
        if self:isConnected() then
            table.insert(lines, string.format(_("Signed in as %s"), trim(self.settings:readSetting("user"))))
            local synced = ago(self.settings:readSetting("last_auto"))
            table.insert(lines, synced and (_("Last synced") .. " " .. synced) or _("Not synced yet"))
            local backed = ago(self.settings:readSetting("last_backup"))
            if backed then
                table.insert(lines, _("Last backup") .. " " .. backed)
            end
        else
            table.insert(lines, _("Not connected yet: More > Settings > Connect"))
        end
        local function row(text, fn)
            return { { text = text, callback = go(fn) } }
        end
        dialog = ButtonDialog:new{
            title = table.concat(lines, "\n"),
            title_align = "center",
            buttons = {
                row(_("Nightstand"), function() self:showUpNext() end),
                row(_("New in the library"), function() self:newInLibrary() end),
                row(_("Search"), function() self:searchLibrary() end),
                row(_("On this e-reader"), function() self.DeviceBooks.show(self) end),
                row(_("Sync now"), function() self:syncNow() end),
                row(_("More…"), function() self:showList(_("NovelCheck"), self:menuItems()) end),
                { { text = _("Close"), callback = function() UIManager:close(dialog) end } },
            },
        }
        UIManager:show(dialog)
    end
end
