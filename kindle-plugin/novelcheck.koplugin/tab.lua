--[[
NovelCheck plugin: its own icon in KOReader's top bar. The file manager's and the reader's top menus are a row of tabs, each
an icon; a tab exists when its id is in the "KOMenu:menu_buttons" list of the menu's order file and the menu has an entry of
that id with an icon. This adds the tab "novelcheck_tab" before the hamburger and puts the plugin's items in it (each item
says sorting_hint = "novelcheck_tab"; KOReader orders them by their ids). The icon file is written into KOReader's icon
folders (a plugin has no other way to ship one).
If anything about this fails, or the person has their own menu order that leaves the tab out, addTab says so (false) and the
plugin keeps its old place, a NovelCheck entry in the Tools tab, so the menu is never lost.
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local DataStorage = require("datastorage")
local lfs = require("libs/libkoreader-lfs")

return function(NovelCheck)
    -- A book with a check mark, like NovelCheck's own icon; black on clear, as KOReader's icons are.
    local ICON = [[<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><path d="M10 7h24a4 4 0 0 1 4 4v30H14a4 4 0 0 1-4-4z" fill="none" stroke="#000" stroke-width="3" stroke-linejoin="round"/><path d="M14 37h24" stroke="#000" stroke-width="3"/><path d="M17 21l5.5 5.5L32 15" fill="none" stroke="#000" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"/></svg>]]

    local function writeIcon(dir)
        if lfs.attributes(dir, "mode") ~= "directory" then
            lfs.mkdir(dir)
        end
        local path = dir .. "/novelcheck.svg"
        local f = io.open(path, "rb")
        local have = f and f:read("*a")
        if f then
            f:close()
        end
        if have ~= ICON then
            local out = io.open(path, "wb")
            if out then
                out:write(ICON)
                out:close()
            end
        end
    end

    -- inOrder says where a tab id sits in a menu's top-level list (nil when absent).
    local function inOrder(buttons, id)
        for i, v in ipairs(buttons) do
            if v == id then
                return i
            end
        end
    end

    -- userKeepsTab: the person's own menu order file (Settings, or a menu customizer) replaces KOReader's list when it has
    -- one; if it does not mention the tab, the tab would not show, and neither would its items.
    local function userLeavesTabOut(prefix)
        local path = DataStorage:getSettingsDir() .. "/" .. prefix .. "_menu_order.lua"
        if lfs.attributes(path, "mode") ~= "file" then
            return false
        end
        local ok, user = pcall(dofile, path)
        if not ok or type(user) ~= "table" then
            return true
        end
        local buttons = user["KOMenu:menu_buttons"]
        return type(buttons) == "table" and not inOrder(buttons, "novelcheck_tab")
    end

    -- closeMenu closes KOReader's top menu (the file manager's or the reader's), as the "last document" tab does.
    local function closeMenu(ui)
        local menu = ui and ui.menu
        if not menu then
            return
        end
        if menu.onCloseFileManagerMenu then
            menu:onCloseFileManagerMenu()
        elseif menu.onCloseReaderMenu then
            menu:onCloseReaderMenu()
        end
    end

    -- addTab puts items in the tab; true when it did. Tapping the icon opens the launcher card (hub.lua); if that fails the
    -- menu stays open, and the tab shows the list of items.
    function NovelCheck:addTab(menu_items, items)
        local ok, done = pcall(function()
            local prefix, order
            if menu_items.filemanager_settings ~= nil then
                prefix, order = "filemanager", require("ui/elements/filemanager_menu_order")
            elseif menu_items.navi ~= nil then
                prefix, order = "reader", require("ui/elements/reader_menu_order")
            else
                return false -- a menu this does not know
            end
            local buttons = order["KOMenu:menu_buttons"]
            if type(buttons) ~= "table" then
                return false
            end
            if not inOrder(buttons, "novelcheck_tab") then
                if userLeavesTabOut(prefix) then
                    return false
                end
                table.insert(buttons, inOrder(buttons, "main") or (#buttons + 1), "novelcheck_tab")
            end
            order.novelcheck_tab = order.novelcheck_tab or {}
            writeIcon(DataStorage:getDataDir() .. "/icons") -- survives a KOReader update; read at the next start
            writeIcon("resources/icons/mdlight") -- read at once
            menu_items.novelcheck_tab = {
                icon = "novelcheck",
                remember = false,
                callback = function()
                    if self.showHub and pcall(self.showHub, self) then
                        pcall(closeMenu, self.ui)
                    end
                end,
            }
            for i, it in ipairs(items) do
                it.sorting_hint = "novelcheck_tab"
                menu_items[string.format("novelcheck_%02d", i)] = it
            end
            return true
        end)
        if not ok then
            table.insert(self.load_errors, "tab: " .. tostring(done))
            return false
        end
        return done
    end
end
