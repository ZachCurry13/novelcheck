--[[
NovelCheck plugin: the launcher card. Tapping NovelCheck's icon in KOReader's top bar opens one card with big buttons, like an
app's home screen, instead of a menu list: what it is connected as and when it last synced, then a Check a book bar, the book that is next with one big Read it / Download button, Nightstand, New and My Kindle,
Sync now and More (More is the whole list, with the rarely used things: Back up and tidy, Settings).
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local Blitbuffer = require("ffi/blitbuffer")
local ButtonDialog = require("ui/widget/buttondialog")
local Device = require("device")
local Menu = require("ui/widget/menu")
local UIManager = require("ui/uimanager")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")
local _ = require("gettext")

return function(NovelCheck)
    local function trim(s)
        return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
    end

    local function plural(n, one, many)
        return string.format("%d %s", n, n == 1 and one or many)
    end

    -- ago says how long since a time, in words ("1 minute ago", "4 days ago").
    local function ago(ts)
        ts = tonumber(ts)
        if not ts or ts <= 0 then
            return nil
        end
        local d = os.time() - ts
        if d < 90 then
            return _("just now")
        elseif d < 3600 then
            return plural(math.floor(d / 60), _("minute"), _("minutes")) .. " " .. _("ago")
        elseif d < 86400 then
            return plural(math.floor(d / 3600), _("hour"), _("hours")) .. " " .. _("ago")
        end
        return plural(math.floor(d / 86400), _("day"), _("days")) .. " " .. _("ago")
    end

    -- rememberNext keeps what is next on the Nightstand (the book being read, else the first), so the launcher card can say so at
    -- once, without asking the network.
    function NovelCheck:rememberNext(books)
        local pick
        for _i, b in ipairs(books or {}) do
            if b.status == "reading" then
                pick = b
                break
            end
        end
        pick = pick or (books or {})[1]
        self.settings:saveSetting("next_title", pick and pick.title or "")
        self.settings:saveSetting("next_reading", pick and pick.status == "reading" or false)
        self.settings:saveSetting("next_book", pick and { id = pick.id, url = pick.url, format = pick.format, title = pick.title, author = pick.author } or false)
        self.settings:saveSetting("next_count", #(books or {}))
        local then_ = {}
        for _i, b in ipairs(books or {}) do
            if b ~= pick and #then_ < 2 then table.insert(then_, b.title) end
        end
        self.settings:saveSetting("next_then", table.concat(then_, ", "))
        self.settings:flush()
    end

    -- refreshNext asks for the Nightstand quietly and keeps what is next (used after a report, when the network is up).
    function NovelCheck:refreshNext()
        local code, body = self:request("/kosync/plugin/up-next")
        if code == 200 then
            local ok, data = pcall(json.decode, body or "")
            if ok and type(data) == "table" and type(data.books) == "table" then
                self:rememberNext(data.books)
            end
        end
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

    -- showHub opens the launcher card: one big button for the Nightstand with what is next under it, a small grid of the
    -- other everyday things, and More for the rest.
    function NovelCheck:showHub()
        local dialog
        local function go(fn)
            return function()
                UIManager:close(dialog)
                fn()
            end
        end
        local function note(text) -- a line of plain gray text (a button that does nothing)
            return { { text = text, enabled = false, font_size = 18 } }
        end
        local rows = {}
        if self:isConnected() then
            local synced = ago(self.settings:readSetting("last_auto"))
            table.insert(rows, note(trim(self.settings:readSetting("user")) .. " · " .. (synced and (_("synced") .. " " .. synced) or _("not synced yet"))))
        else
            table.insert(rows, note(_("Not connected yet: More > Settings > Connect")))
        end
        table.insert(rows, { { text = "🔍  " .. _("Check a book…"), font_size = 22, height = Device.screen:scaleBySize(52),
            callback = go(function() self:searchLibrary() end) } })
        local nb = self.settings:readSetting("next_book")
        if type(nb) == "table" and nb.title then
            local path = nb.url and self:bookPath(nb)
            local have = path and lfs.attributes(path, "mode") == "file"
            local by = trim(nb.author) ~= "" and (" - " .. nb.author) or ""
            table.insert(rows, note((self.settings:readSetting("next_reading") and _("Reading") or _("Up next")) .. ": " .. nb.title .. by))
            local then_ = trim(self.settings:readSetting("next_then"))
            if then_ ~= "" then table.insert(rows, note(_("Then") .. ": " .. then_)) end
            table.insert(rows, { {
                text = have and _("Read it") or _("Download"), font_bold = true, font_size = 26, height = Device.screen:scaleBySize(64),
                background = Blitbuffer.COLOR_LIGHT_GRAY,
                callback = go(function()
                    if have then
                        require("apps/reader/readerui"):showReader(path)
                    else
                        self:download(nb)
                    end
                end),
            } })
        end
        local count = tonumber(self.settings:readSetting("next_count"))
        table.insert(rows, {
            { text = _("Nightstand") .. (count and (" (" .. count .. ")") or ""), height = Device.screen:scaleBySize(56), callback = go(function() self:showUpNext() end) },
            { text = _("New"), height = Device.screen:scaleBySize(56), callback = go(function() self:newInLibrary() end) },
            { text = _("My Kindle"), height = Device.screen:scaleBySize(56), callback = go(function() self.DeviceBooks.show(self) end) },
        })
        local backed = ago(self.settings:readSetting("last_backup"))
        if backed then
            table.insert(rows, note(_("Last backup") .. " " .. backed))
        end
        table.insert(rows, {
            { text = _("Sync now"), callback = go(function() self:syncNow() end) },
            { text = _("Back up"), callback = go(function() if self.backupKindle then self:backupKindle(false) end end) },
        })
        table.insert(rows, {
            { text = _("More…"), callback = go(function() self:showList(_("NovelCheck"), self:menuItems()) end) },
            { text = _("Close"), callback = function() UIManager:close(dialog) end },
        })
        dialog = ButtonDialog:new{
            title = _("NovelCheck"),
            title_align = "center",
            use_info_style = false,
            buttons = rows,
        }
        UIManager:show(dialog)
    end
end
