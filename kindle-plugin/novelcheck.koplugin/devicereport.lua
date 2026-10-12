--[[
NovelCheck plugin: reporting the books on this device (names, formats, sizes and fingerprints; never folder names).
It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local ConfirmBox = require("ui/widget/confirmbox")
local DataStorage = require("datastorage")
local LuaSettings = require("luasettings")
local Device = require("device")
local InfoMessage = require("ui/widget/infomessage")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")
local util = require("util")

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

return function(NovelCheck, ctx)
    local VERSION, PLUGIN_DIR = ctx.VERSION, ctx.PLUGIN_DIR
    local _unused = { Device, ConfirmBox, NetworkMgr, lfs, util, json, trim, VERSION, PLUGIN_DIR }

    -- deviceName is what NovelCheck calls this e-reader (the model, unless set).
    function NovelCheck:deviceName()
        local name = trim(self.settings:readSetting("device_name"))
        if name == "" then
            name = (Device.model or "KOReader device")
        end
        return name
    end

    function NovelCheck:deviceNameDialog()
        local InputDialog = require("ui/widget/inputdialog")
        local dialog
        dialog = InputDialog:new{
            title = _("Name of this device"),
            input = self:deviceName(),
            buttons = { {
                { text = _("Cancel"), callback = function() UIManager:close(dialog) end },
                { text = _("Save"), is_enter_default = true, callback = function()
                    self.settings:saveSetting("device_name", trim(dialog:getInputText()))
                    self.settings:flush()
                    UIManager:close(dialog)
                end },
            } },
        }
        UIManager:show(dialog)
        dialog:onShowKeyboard()
    end

    local BOOK_EXT = { epub = true, pdf = true, mobi = true, azw3 = true, azw = true, fb2 = true, cbz = true, txt = true, djvu = true }
    local MAX_BOOKS = 500

    -- scanBooks lists the books under dir (without folder names): file name,
    -- format, size and KOReader's fingerprint of the file.
    -- bookProps is a book's own title and author. It asks KOReader's book info
    -- (which knows opened books and cached ones) and, for up to PROPS_OPENS books
    -- per scan, opens the book itself. Answers are kept in a small cache file
    -- (by fingerprint), so after a few reports every book is known.
    local PROPS_OPENS = 25

    function NovelCheck:bookProps(path, md5)
        if not self.props_cache then
            self.props_cache = LuaSettings:open(DataStorage:getSettingsDir() .. "/novelcheck_props.lua")
        end
        local cached = md5 ~= "" and self.props_cache:readSetting(md5) or nil
        if cached then
            return cached.t, cached.a
        end
        local info = self.ui and self.ui.bookinfo
        if not info then
            return nil
        end
        local may_open = (self.props_opens or 0) < PROPS_OPENS
        local ok, props = pcall(info.getDocProps, info, path, nil, not may_open)
        if may_open then
            self.props_opens = (self.props_opens or 0) + 1
        end
        if ok and type(props) == "table" and type(props.title) == "string" and props.title ~= "" then
            local author = tostring(props.authors or ""):match("([^\n]+)") or ""
            if author == "N/A" then
                author = ""
            end
            if md5 ~= "" then
                self.props_cache:saveSetting(md5, { t = props.title, a = author })
            end
            return props.title, author
        end
        return nil
    end

    function NovelCheck:scanBooks(dir, books, depth, no_md5)
        if #books >= MAX_BOOKS or depth > 4 then return end
        if depth == 0 then
            self.props_opens = 0
        end
        for entry in lfs.dir(dir) do
            if entry:sub(1, 1) ~= "." and not entry:match("%.sdr$") then
                local path = dir .. "/" .. entry
                local attr = lfs.attributes(path)
                if attr and attr.mode == "directory" then
                    self:scanBooks(path, books, depth + 1, no_md5)
                elseif attr and attr.mode == "file" then
                    local ext = entry:match("%.([^.]+)$")
                    if ext and BOOK_EXT[ext:lower()] then
                        local sum = ""
                        if not no_md5 then
                            local ok, got = pcall(util.partialMD5, path)
                            sum = ok and got or ""
                        end
                        local ptitle, pauthor
                        if not no_md5 then
                            ptitle, pauthor = self:bookProps(path, sum)
                        end
                        table.insert(books, {
                            filename = entry,
                            path = path,
                            title = ptitle or (entry:gsub("%.[^.]+$", ""):gsub("[-_]", " ")),
                            author = pauthor or "",
                            format = ext:lower(),
                            size = attr.size,
                            md5 = sum,
                        })
                    end
                end
                if #books >= MAX_BOOKS then return end
            end
        end
    end

    -- withoutPaths drops the folder of each book: NovelCheck is told file names only.
    function NovelCheck:withoutPaths(books)
        local out = {}
        for _i, b in ipairs(books) do
            table.insert(out, { filename = b.filename, title = b.title, author = b.author, format = b.format, size = b.size, md5 = b.md5 })
        end
        if self.props_cache then
            self.props_cache:flush()
        end
        return out
    end

    -- reportQuietly tells NovelCheck what is on the device now, without messages
    -- (after books were deleted). It does nothing when not connected or offline.
    function NovelCheck:reportQuietly()
        if not self:isConnected() then return end
        pcall(function()
            local books = {}
            self:scanBooks(self:downloadDir(), books, 0)
            local code = self:request("/kosync/plugin/device", nil, json.encode{ device = self:deviceName(), plugin_version = VERSION, books = self:withoutPaths(books) })
            self:noteReported(code)
        end)
    end

    -- noteReported remembers which plugin version NovelCheck last heard of, so
    -- a freshly updated plugin reports again at once (the Profile page shows it).
    function NovelCheck:noteReported(code)
        if code == 200 then
            self.settings:saveSetting("reported_version", VERSION)
            self.settings:flush()
        end
    end

    function NovelCheck:reportDevice()
        if not self:isConnected() then
            self:connectDialog()
            return
        end
        NetworkMgr:runWhenOnline(function()
            UIManager:show(InfoMessage:new{ text = _("Listing the books on this device..."), timeout = 1 })
            UIManager:nextTick(function()
                local books = {}
                local ok = pcall(self.scanBooks, self, self:downloadDir(), books, 0)
                if not ok then
                    UIManager:show(InfoMessage:new{ text = _("Could not read the books folder.") })
                    return
                end
                local body = json.encode{ device = self:deviceName(), plugin_version = VERSION, books = self:withoutPaths(books) }
                local code, answer = self:request("/kosync/plugin/device", nil, body)
                self:noteReported(code)
                if code == 200 then
                    local good, data = pcall(json.decode, answer)
                    local text = _("Reported to NovelCheck:") .. " " .. tostring(#books) .. " " .. _("books")
                    if good and type(data) == "table" and data.matched then
                        text = text .. "\n" .. tostring(data.matched) .. " " .. _("are in your NovelCheck library.")
                    end
                    if good and type(data) == "table" and data.added and data.added > 0 then
                        text = text .. "\n" .. tostring(data.added) .. " " .. _("added to your Nightstand.")
                    end
                    if #books >= MAX_BOOKS then
                        text = text .. "\n" .. _("(Only the first 500 books are listed.)")
                    end
                    UIManager:show(InfoMessage:new{ text = text })
                elseif code == 401 then
                    UIManager:show(InfoMessage:new{ text = _("NovelCheck did not accept the username or sync code.") })
                else
                    UIManager:show(InfoMessage:new{ text = _("Could not report to NovelCheck.") .. "\n" .. tostring(code or answer) })
                end
            end)
        end)
    end

    function NovelCheck:downloadDir()
        local dir = G_reader_settings:readSetting("download_dir") or G_reader_settings:readSetting("home_dir")
        if dir and lfs.attributes(dir, "mode") == "directory" then
            return dir
        end
        return DataStorage:getDataDir()
    end
end
