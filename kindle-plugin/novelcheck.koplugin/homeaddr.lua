--[[
NovelCheck plugin: the home address. Besides the main address (the one that works anywhere, through the internet)
NovelCheck can have a home address: the server's address on the home network, like http://192.168.1.20:8080. At
home the plugin talks to the server straight over Wi-Fi, without the internet, the tunnel or a name lookup (the
lookups were what failed when the Kindle's Wi-Fi wavered). Before using the home address it knocks once (a 2-second
connection try) and remembers the answer for ten minutes, so away from home a request waits 2 seconds at most, once
every ten minutes, then goes to the main address. It adds its functions to the NovelCheck plugin class (see main.lua).
]]

local socket = require("socket")

local KNOCK_SECONDS = 2
local REMEMBER_SECONDS = 600

local function trim(s)
    return ((s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

-- hostPort reads the host and port of an address ("http://192.168.1.20:8080" -> "192.168.1.20", 8080).
local function hostPort(base)
    local scheme, rest = base:match("^(https?)://(.+)$")
    if not scheme then
        return nil
    end
    local hostport = rest:match("^([^/]+)") or ""
    local host, port = hostport:match("^%[?([^%]]+)%]?:(%d+)$")
    if not host then
        host, port = hostport, (scheme == "https" and 443 or 80)
    end
    return host, tonumber(port)
end

return function(NovelCheck)
    -- homeReachable says whether the home address answers a knock (remembered for ten minutes).
    function NovelCheck:homeReachable(home)
        local now = os.time()
        local seen = self.home_seen
        if seen and seen.base == home and now - seen.at < REMEMBER_SECONDS then
            return seen.ok
        end
        local ok = false
        local host, port = hostPort(home)
        if host then
            local tcp = socket.tcp()
            if tcp then
                tcp:settimeout(KNOCK_SECONDS)
                ok = tcp:connect(host, port) == 1
                tcp:close()
            end
        end
        self.home_seen = { base = home, at = now, ok = ok }
        self:dlog("net", "home address " .. (ok and "answers" or "does not answer"))
        return ok
    end

    -- homeFailed forgets a good knock when a real request to the home address then failed.
    function NovelCheck:homeFailed()
        local home = trim(self.settings:readSetting("server_home"))
        if home == "" then
            home = trim(self.settings:readSetting("server_home_auto"))
        end
        self.home_seen = { base = home, at = os.time(), ok = false }
    end

    -- addresses lists where to send a request, in order: the home address when it answers, the main address, then
    -- the backup address.
    function NovelCheck:addresses()
        local out = {}
        local home = trim(self.settings:readSetting("server_home"))
        if home == "" then
            home = trim(self.settings:readSetting("server_home_auto")) -- from the server's own setting
        end
        if home ~= "" and self:homeReachable(home) then
            out[#out + 1] = home
        end
        for _i, key in ipairs{ "server", "server_alt" } do
            local base = trim(self.settings:readSetting(key))
            if base ~= "" and base ~= home then
                out[#out + 1] = base
            end
        end
        return out
    end
end
