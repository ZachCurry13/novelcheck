--[[
NovelCheck plugin: "Back up my Kindle" (apps, plugins, settings and books: everything on the Kindle's user storage
except hidden and system folders and caches; see backupSkip on the server) and the older "Copy my Kindle" (only the
books). Both send what NovelCheck does not have yet; a file is sent again when its size or change time differs.

"Copy my Kindle". Looks through the whole device (every folder, not only the books KOReader
opened) for e-book files and sends the ones NovelCheck does not have yet, one at a time, as they are: nothing is
unlocked or changed. NovelCheck keeps every file and adds the unlocked ones to Calibre. Files that only look like
books (this plugin's logs, KOReader's own folders) are left out. It resumes where it stopped, so a dropped
connection costs nothing. It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local DataStorage = require("datastorage")
local InfoMessage = require("ui/widget/infomessage")
local UIManager = require("ui/uimanager")
local _ = require("gettext")
local json = require("json")
local lfs = require("libs/libkoreader-lfs")
local ltn12 = require("ltn12")
local socket = require("socket")
local socketutil = require("socketutil")
local http = require("socket.http")
local https = require("ssl.https")
local md5 = require("ffi/sha2").md5

local EXTS = { epub = true, pdf = true, mobi = true, azw = true, azw3 = true, azw1 = true, kfx = true, kepub = true, fb2 = true, cbz = true, prc = true }
local MAX_FILES = 5000
local MAX_BYTES = 1024 * 1024 * 1024
local RUN_SECONDS = 20 * 60

local function skipDir(name)
    local l = name:lower()
    return l:sub(1, 1) == "." or l == "koreader" or l == "system" or l == "novelcheck.koplugin" or l:match("%.sdr$") ~= nil
end

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function encode(s)
    return (s:gsub("[^%w%-%._~/]", function(c) return string.format("%%%02X", c:byte()) end))
end

return function(NovelCheck)
    -- findBooks lists every book file under dir: {path, size}.
    function NovelCheck:findBooks(dir, out, depth)
        if depth > 12 or #out >= MAX_FILES then
            return
        end
        local ok, iter, state = pcall(lfs.dir, dir)
        if not ok then
            return
        end
        for name in iter, state do
            if name ~= "." and name ~= ".." and not skipDir(name) then
                local path = dir .. "/" .. name
                local attr = lfs.attributes(path)
                if attr and attr.mode == "directory" then
                    self:findBooks(path, out, depth + 1)
                elseif attr and attr.mode == "file" and EXTS[(name:match("%.([^.]+)$") or ""):lower()]
                    and attr.size >= 2048 and attr.size <= MAX_BYTES then
                    table.insert(out, { path = path, size = attr.size })
                end
            end
        end
    end

    local function skipBackupDir(name, rel)
        local l = name:lower()
        if l:sub(1, 1) == "." or l == "system" or l == "trash" or l == "lost+found" then
            return true
        end
        local r = (rel .. "/" .. name):lower()
        return r == "/koreader/cache" or r == "/koreader/ota"
    end

    -- findAll lists every file under dir (relative path kept in rel): {path, size, mtime}.
    function NovelCheck:findAll(dir, rel, out, depth)
        if depth > 16 or #out >= 20000 then
            return
        end
        local ok, iter, state = pcall(lfs.dir, dir)
        if not ok then
            return
        end
        for name in iter, state do
            if name ~= "." and name ~= ".." then
                local path = dir .. "/" .. name
                local attr = lfs.attributes(path)
                if attr and attr.mode == "directory" then
                    if not skipBackupDir(name, rel) then
                        self:findAll(path, rel .. "/" .. name, out, depth + 1)
                    end
                elseif attr and attr.mode == "file" and attr.size <= MAX_BYTES and name:sub(1, 1) ~= "." and not name:match("%.part$") and not name:match("%.new$") and not name:find("_crash_", 1, true) then
                    table.insert(out, { path = path, size = attr.size, mtime = attr.modification or 0 })
                end
            end
        end
    end

    local function put(self, base, path, file, size)
        local url = base .. path
        local fetch = url:match("^https") and https.request or http.request
        local fh = io.open(file, "rb")
        if not fh then
            return nil, "can't open"
        end
        socketutil:set_timeout(15, 600)
        local chunks = {}
        local ok, code = pcall(function()
            return socket.skip(1, fetch{
                url = url, method = "PUT",
                headers = {
                    ["x-auth-user"] = trim(self.settings:readSetting("user")),
                    ["x-auth-key"] = md5(trim(self.settings:readSetting("code"))),
                    ["content-length"] = tostring(size),
                    ["content-type"] = "application/octet-stream",
                },
                source = ltn12.source.file(fh),
                sink = ltn12.sink.table(chunks),
            })
        end)
        socketutil:reset_timeout()
        pcall(function() fh:close() end)
        if not ok then
            return nil, tostring(code)
        end
        return code, table.concat(chunks)
    end

    function NovelCheck:putBook(path, file, size, backup, mtime)
        local q = "/kosync/plugin/kindle/file?device=" .. encode(self:deviceName()) .. "&path=" .. encode(path)
            .. (backup and ("&mode=backup&mtime=" .. tostring(mtime or 0)) or "")
        local code, body = put(self, self.settings:readSetting("server"), q, file, size)
        local alt = trim(self.settings:readSetting("server_alt"))
        if type(code) ~= "number" and alt ~= "" then
            code, body = put(self, alt, q, file, size)
        end
        return code, body
    end

    function NovelCheck:kindleRoot()
        if lfs.attributes("/mnt/us", "mode") == "directory" then
            return "/mnt/us"
        end
        return self:libraryDir()
    end

    -- backupKindle is copyKindle for everything on the Kindle.
    function NovelCheck:backupKindle(quiet)
        return self:copyKindle(quiet, true)
    end

    -- copyKindle works out what is missing and sends it a file at a time (scheduled, so the screen stays usable).
    -- backup: every file of the Kindle (apps, plugins, settings, books), not only the books.
    function NovelCheck:copyKindle(quiet, backup)
        if self.kindle_copy_running then
            return
        end
        local files = {}
        if backup then
            self:findAll(self:kindleRoot(), "", files, 0)
        else
            self:findBooks(self:kindleRoot(), files, 0)
        end
        local todo, sizes, mtimes = {}, {}, {}
        for _i, f in ipairs(files) do
            sizes[f.path], mtimes[f.path] = f.size, f.mtime
        end
        -- ask in batches (a Kindle with KOReader holds thousands of files)
        local i = 1
        while i <= #files do
            local list = {}
            for j = i, math.min(i + 1999, #files) do
                list[#list + 1] = { path = files[j].path, size = files[j].size, mtime = backup and files[j].mtime or nil }
            end
            i = i + 2000
            local code, answer = self:request("/kosync/plugin/kindle/check", nil, json.encode{ device = self:deviceName(), mode = backup and "backup" or nil, files = list })
            if code ~= 200 then
                self:dlog("error", "kindle copy: NovelCheck answered " .. tostring(code))
                if not quiet then
                    UIManager:show(InfoMessage:new{ text = (code == 401) and _("NovelCheck did not accept the username or sync code.")
                        or (_("Could not reach NovelCheck.") .. "\n" .. tostring(code or answer)) })
                end
                return
            end
            local good, data = pcall(json.decode, answer)
            for _k, p in ipairs((good and type(data) == "table" and type(data.send) == "table") and data.send or {}) do
                todo[#todo + 1] = p
            end
        end
        if #todo == 0 then
            if backup then
                self.settings:saveSetting("last_backup", os.time())
                self.settings:flush()
            end
            if not quiet then
                UIManager:show(InfoMessage:new{ text = backup and _("This Kindle is already backed up to NovelCheck.") or _("Everything on this Kindle is already copied to NovelCheck.") })
            end
            return
        end
        self.kindle_copy_running = true
        local started, sent, failed, n = os.time(), 0, 0, 0
        local function finish(more)
            self.kindle_copy_running = false
            if backup and more == 0 and failed == 0 then
                self.settings:saveSetting("last_backup", os.time())
                self.settings:flush()
            end
            self:dlog("info", (backup and "kindle backup" or "kindle copy") .. ": sent " .. sent .. ", failed " .. failed .. ", left " .. tostring(more))
            if not quiet or sent > 0 then
                UIManager:show(InfoMessage:new{ text = (backup and _("Backed up to NovelCheck:") or _("Copied to NovelCheck:")) .. " " .. sent .. " " .. _("files")
                    .. ((failed > 0) and (", " .. failed .. " " .. _("failed")) or "")
                    .. ((more > 0) and ("\n" .. more .. " " .. _("left for next time.")) or "") })
            end
        end
        local function step()
            n = n + 1
            if n > #todo then
                return finish(0)
            end
            if os.time() - started > RUN_SECONDS or not self:isConnected() then
                return finish(#todo - n + 1)
            end
            local path = todo[n]
            local c = self:putBook(path, path, lfs.attributes(path, "size") or sizes[path] or 0, backup, lfs.attributes(path, "modification") or mtimes[path])
            if c == 200 then
                sent = sent + 1
            else
                failed = failed + 1
                if c == 401 then
                    return finish(#todo - n)
                end
            end
            UIManager:scheduleIn(0.1, step)
        end
        UIManager:scheduleIn(0.1, step)
    end
end
