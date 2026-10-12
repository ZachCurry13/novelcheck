--[[
NovelCheck plugin for KOReader, step 1: connect to your NovelCheck, see your
Up Next list, and tap a book to download it.

It signs in the way KOReader's progress sync does: your NovelCheck username
and the sync code from Profile -> KOReader setup.
]]

local DataStorage = require("datastorage")
local Device = require("device")
local InfoMessage = require("ui/widget/infomessage")
local LuaSettings = require("luasettings")
local Menu = require("ui/widget/menu")
local MultiInputDialog = require("ui/widget/multiinputdialog")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local WidgetContainer = require("ui/widget/container/widgetcontainer")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")
local socket = require("socket")
local socketutil = require("socketutil")
local http = require("socket.http")
local https = require("ssl.https")
local md5 = require("ffi/sha2").md5

local NovelCheck = WidgetContainer:extend{
    name = "novelcheck",
    is_doc_only = false,
}

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function NovelCheck:init()
    self.settings = LuaSettings:open(DataStorage:getSettingsDir() .. "/novelcheck.lua")
    self.ui.menu:registerToMainMenu(self)
end

function NovelCheck:addToMainMenu(menu_items)
    menu_items.novelcheck = {
        text = _("NovelCheck"),
        sorting_hint = "tools",
        sub_item_table = {
            { text = _("Up Next"), callback = function() self:showUpNext() end },
            { text = _("Connect to NovelCheck..."), keep_menu_open = true, callback = function() self:connectDialog() end },
        },
    }
end

function NovelCheck:isConnected()
    return trim(self.settings:readSetting("server")) ~= ""
        and trim(self.settings:readSetting("user")) ~= ""
        and trim(self.settings:readSetting("code")) ~= ""
end

function NovelCheck:connectDialog()
    local dialog
    dialog = MultiInputDialog:new{
        title = _("Connect to NovelCheck"),
        fields = {
            { text = self.settings:readSetting("server") or "", hint = _("Address, like https://novelcheck.example.com") },
            { text = self.settings:readSetting("user") or "", hint = _("Your NovelCheck username") },
            { text = self.settings:readSetting("code") or "", hint = _("Sync code (Profile > KOReader setup)") },
        },
        buttons = { {
            { text = _("Cancel"), callback = function() UIManager:close(dialog) end },
            { text = _("Save"), is_enter_default = true, callback = function()
                local f = dialog:getFields()
                self.settings:saveSetting("server", (trim(f[1]):gsub("/+$", "")))
                self.settings:saveSetting("user", trim(f[2]))
                self.settings:saveSetting("code", trim(f[3]))
                self.settings:flush()
                UIManager:close(dialog)
                UIManager:show(InfoMessage:new{ text = _("Saved. Open NovelCheck > Up Next to try it.") })
            end },
        } },
    }
    UIManager:show(dialog)
    dialog:onShowKeyboard()
end

-- request fetches path from NovelCheck. With file, the body is saved there;
-- otherwise it is returned as text. It returns the HTTP code and the body.
function NovelCheck:request(path, file)
    local url = self.settings:readSetting("server") .. path
    local fetch = url:match("^https") and https.request or http.request
    local chunks = {}
    local sink
    if file then
        sink = socketutil.file_sink(io.open(file, "wb"))
    else
        sink = socketutil.table_sink(chunks)
    end
    socketutil:set_timeout(15, file and 300 or 30)
    local ok, code = pcall(function()
        return socket.skip(1, fetch{
            url = url,
            method = "GET",
            headers = {
                ["x-auth-user"] = trim(self.settings:readSetting("user")),
                ["x-auth-key"] = md5(trim(self.settings:readSetting("code"))),
                ["accept"] = "application/json",
            },
            sink = sink,
        })
    end)
    socketutil:reset_timeout()
    if not ok then
        return nil, tostring(code)
    end
    return code, table.concat(chunks)
end

function NovelCheck:showUpNext()
    if not self:isConnected() then
        self:connectDialog()
        return
    end
    NetworkMgr:runWhenOnline(function()
        local code, body = self:request("/kosync/plugin/up-next")
        if code == 401 then
            UIManager:show(InfoMessage:new{ text = _("NovelCheck did not accept the username or sync code. Check them under Connect to NovelCheck.") })
            return
        elseif code ~= 200 then
            UIManager:show(InfoMessage:new{ text = _("Could not reach NovelCheck.") .. "\n" .. tostring(code or body) })
            return
        end
        local ok, data = pcall(json.decode, body)
        if not ok or type(data) ~= "table" or type(data.books) ~= "table" then
            UIManager:show(InfoMessage:new{ text = _("NovelCheck answered something unexpected.") })
            return
        end
        if #data.books == 0 then
            UIManager:show(InfoMessage:new{ text = _("Nothing to download: your Up Next has no books with a file.") })
            return
        end
        local items = {}
        for _i, b in ipairs(data.books) do
            local line = b.title
            if b.author and b.author ~= "" then
                line = line .. " - " .. b.author
            end
            table.insert(items, {
                text = line,
                mandatory = (b.status == "reading") and _("reading") or nil,
                callback = function() self:download(b) end,
            })
        end
        self.menu = Menu:new{
            title = _("NovelCheck: Up Next"),
            item_table = items,
            is_borderless = true,
            is_popout = false,
            covers_fullscreen = true,
            width = Device.screen:getWidth(),
            height = Device.screen:getHeight(),
            close_callback = function() self.menu = nil end,
        }
        UIManager:show(self.menu)
    end)
end

function NovelCheck:downloadDir()
    local dir = G_reader_settings:readSetting("download_dir") or G_reader_settings:readSetting("home_dir")
    if dir and lfs.attributes(dir, "mode") == "directory" then
        return dir
    end
    return DataStorage:getDataDir()
end

function NovelCheck:download(book)
    local name = book.url:match("([^/]+)$") or ("book-" .. tostring(book.id) .. "." .. tostring(book.format))
    local path = self:downloadDir() .. "/" .. name
    if lfs.attributes(path, "mode") == "file" then
        UIManager:show(InfoMessage:new{ text = _("Already on this device:") .. "\n" .. path })
        return
    end
    UIManager:show(InfoMessage:new{ text = _("Downloading..."), timeout = 1 })
    UIManager:nextTick(function()
        local code = self:request(book.url, path)
        if code == 200 then
            UIManager:show(InfoMessage:new{ text = _("Downloaded to:") .. "\n" .. path })
        else
            os.remove(path)
            UIManager:show(InfoMessage:new{ text = _("The download failed.") .. "\n" .. tostring(code) })
        end
    end)
end

return NovelCheck
