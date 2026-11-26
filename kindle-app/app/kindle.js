/* NovelCheck Kindle app: talking to the server and to the Kindle. ES5 only: the Kindle's app runner
   (Mesquite) stops at const, let, arrow functions, fetch and Promise. */
var NC = window.NC || {}; // set by config.js: server, user, key, device

// sh quotes one word for the shell.
function sh(s) {
  return "'" + String(s).replace(/'/g, "'\\''") + "'";
}

// api calls the server with the plugin's sign-in; done(err, data, status). err is a sentence to show.
function api(method, path, body, done) {
  var x = new XMLHttpRequest();
  x.open(method, NC.server + path, true);
  x.setRequestHeader("x-auth-user", NC.user);
  x.setRequestHeader("x-auth-key", NC.key);
  x.setRequestHeader("Accept", "application/json");
  if (body) x.setRequestHeader("Content-Type", "application/json");
  x.timeout = 20000;
  x.onreadystatechange = function () {
    if (x.readyState !== 4) return;
    var data = null;
    try { data = JSON.parse(x.responseText); } catch (e) {}
    // any answer at all means NovelCheck can be reached: a leftover "Offline" line goes
    if (x.status > 0 && typeof state !== "undefined" && state.offline && typeof setOffline === "function") setOffline(false);
    if (x.status === 0) {
      if (path.indexOf("/plugin/debug") < 0) logLine(method + " " + path.split("?")[0] + ": no connection");
      return done("No connection. Turn off Airplane Mode and try again.", null, 0);
    }
    if (x.status >= 400) {
      if (path.indexOf("/plugin/debug") < 0) logLine(method + " " + path.split("?")[0] + " -> " + x.status + " " + ((data && data.message) || ""));
      return done((data && data.message) || ("NovelCheck answered " + x.status + "."), data, x.status);
    }
    done(null, data, x.status);
  };
  x.send(body ? JSON.stringify(body) : null);
}

// cover shows a book's cover in img (size "l" for a book's page, else the small one). Covers need the sign-in, so they
// come by XHR as bytes turned into a data: address (the Kindle's browser has no blob: addresses for pictures). One
// at a time, so a list of covers doesn't crowd a slow Wi-Fi. A typed array isn't passed to apply (old engines refuse).
var coverQueue = [], coverBusy = false, coverLogged = 0;
var coverVer = {}; // books whose cover was just changed: a new address so the old picture is not reused
function cover(img, id, size) {
  coverQueue.push({ img: img, id: id, size: size || "s" });
  nextCover();
}
// coverFrom shows a picture from another address of NovelCheck (a candidate cover's preview) the same way.
function coverFrom(img, path) {
  coverQueue.push({ img: img, id: "x", size: "s", path: path });
  nextCover();
}

function nextCover() {
  if (coverBusy) return;
  var job = coverQueue.shift();
  if (!job) return;
  coverBusy = true;
  var img = job.img, id = job.id;
  var done = function (why) {
    if (why && job.path) { logLine("cover preview: " + why); coverBusy = false; return nextCover(); } // no file fallback for a preview
    if (why) { logLine("cover " + id + ": " + why + " (trying the Kindle's own download)"); return fileCover(job, function (ok) {
      if (!ok && img.parentNode && img.className !== "thumb") img.parentNode.removeChild(img);
      coverBusy = false;
      nextCover();
    }); }
    coverBusy = false;
    nextCover();
  };
  if (!window.Uint8Array || !window.btoa) return done("this browser can't show covers (no Uint8Array or btoa)");
  var x = new XMLHttpRequest();
  x.open("GET", NC.server + (job.path || "/kosync/plugin/cover/" + id + "?size=" + job.size + (coverVer[id] ? "&v=" + coverVer[id] : "")), true);
  x.setRequestHeader("x-auth-user", NC.user);
  x.setRequestHeader("x-auth-key", NC.key);
  x.responseType = "arraybuffer";
  x.onload = function () {
    var type = x.getResponseHeader("Content-Type") || "";
    if (x.status !== 200) return done("status " + x.status);
    if (type.indexOf("image/") !== 0) return done("not a picture (" + type + ")");
    try {
      var bytes = new Uint8Array(x.response), parts = [];
      for (var i = 0; i < bytes.length; i++) parts.push(String.fromCharCode(bytes[i]));
      img.src = "data:" + type.split(";")[0] + ";base64," + window.btoa(parts.join(""));
      if (coverLogged++ < 3) logLine("cover " + id + " ok, " + bytes.length + " bytes, " + type.split(";")[0]);
      done("");
    } catch (e) { done("couldn't turn it into a picture: " + e); }
  };
  x.onerror = function () { done("network error"); };
  x.send();
}

// fileCover is the second way to a cover: the Kindle's own curl (through KindleForge's helper) saves it as a file in the
// app's folder and the page shows that file. Used when the first way failed.
function fileCover(job, done) {
  var dir = "/var/local/mesquite/NovelCheck/covers", name = "c" + job.id + "_" + job.size + ".jpg", out = "/tmp/nc-cover-" + job.id + ".txt";
  var cmd = "mkdir -p " + dir + "; rm -f " + out + "; if curl -sfL -H " + sh("x-auth-user: " + NC.user) + " -H " + sh("x-auth-key: " + NC.key) +
    " -o " + dir + "/" + name + " " + sh(NC.server + "/kosync/plugin/cover/" + job.id + "?size=" + job.size) + "; then echo ok > " + out + "; else rm -f " + dir + "/" + name + "; echo failed > " + out + "; fi; echo END >> " + out;
  if (!run(cmd)) return done(false);
  waitFor(out, 20, function (text) {
    var ok = !!text && text.indexOf("ok") >= 0;
    if (ok) job.img.src = "file://" + dir + "/" + name;
    logLine("cover " + job.id + " by file: " + (ok ? "saved" : "failed"));
    done(ok);
  });
}

// logLine notes what the app did or what went wrong (the last 150 lines, kept on the Kindle); sendLog sends them to
// NovelCheck's bug log, so an admin sees what the app is doing without asking.
function logLine(text) {
  var lines = [];
  try { lines = JSON.parse(localStorage.getItem("nc-log")) || []; } catch (e) {}
  lines.push(new Date().toISOString().slice(0, 19).replace("T", " ") + " " + text);
  try { localStorage.setItem("nc-log", JSON.stringify(lines.slice(-150))); } catch (e) {}
}

// An error nobody caught (a bug in the app) goes in the log and is sent, at most once in ten minutes.
var lastErrorSent = 0;
window.onerror = function (msg, src, line) {
  logLine("error: " + msg + " (" + String(src || "").split("/").pop() + ":" + line + ")");
  var now = new Date().getTime();
  if (NC.server && now - lastErrorSent > 600000) { lastErrorSent = now; sendLog("an error in the app"); }
  return false;
};

function sendLog(why) {
  var lines = [];
  try { lines = JSON.parse(localStorage.getItem("nc-log")) || []; } catch (e) {}
  if (!lines.length) return;
  api("POST", "/kosync/plugin/debug", { device: NC.device, version: "app " + NC.version,
    text: "KINDLE APP LOG (" + why + ")\n" + navigator.userAgent + "\n\n" + lines.join("\n") }, function (err) {
    if (!err) try { localStorage.setItem("nc-log", "[]"); } catch (e) {}
  });
}

// run has KindleForge's helper (utild) run a shell command as the Kindle; false when there is no helper.
function run(cmd) {
  var k = window.kindle || (window.top && window.top.kindle);
  if (!k || !k.messaging) return false;
  setTimeout(function () { k.messaging.sendStringMessage("com.kindlemodding.utild", "runCMD", cmd); }, 10); // repaint first
  return true;
}

// readFile reads a text file on the Kindle (as KindleForge does: through a hidden frame); "" when missing.
function readFile(path, done) {
  var f = document.createElement("iframe");
  var over = false;
  f.style.display = "none";
  f.src = "file://" + path + "?t=" + new Date().getTime();
  function finish(text) {
    if (over) return;
    over = true;
    if (f.parentNode) f.parentNode.removeChild(f);
    done(text);
  }
  f.onload = function () {
    var text = "";
    try { text = f.contentDocument.body.textContent || ""; } catch (e) {}
    finish(text);
  };
  setTimeout(function () { finish(""); }, 3000);
  document.body.appendChild(f);
}

// waitFor reads path every 2 seconds until it holds a line ending in "END", up to limit seconds; done(text or null).
function waitFor(path, limit, done) {
  var start = new Date().getTime();
  (function again() {
    readFile(path, function (text) {
      var at = text.lastIndexOf("END");
      if (at >= 0) return done(text.slice(0, at));
      if (new Date().getTime() - start > limit * 1000) return done(null);
      setTimeout(again, 2000);
    });
  })();
}

// chrome sets the Kindle's top bar (the title and a close button), in the newer or the older firmware's way, and lets
// the Kindle offer to turn on Wi-Fi (as KindleForge does).
var menuBound = false;
function chrome() {
  try {
    var k = window.kindle;
    if (k.chrome && k.chrome.isDecanterChromeEnabled) {
      // the ⋯ button next to the X opens the Kindle's own menu (as KindleForge's does); a pick comes back as an event
      var items = [], n = 0;
      var add = function (id, label) { items.push({ id: id, state: "enabled", handling: "notifyApp", label: label, position: n++ }); };
      add("NC_SYNC", "Sync now");
      add("NC_BACKUP", "Back up my books to NovelCheck");
      add("NC_RESTORE", "Restore books from NovelCheck");
      add("NC_HIDDEN", "Books I hid");
      add("NC_PLUGIN", "Install the KOReader plugin");
      add("NC_DEST", typeof destLabel === "function" ? destLabel() : "Downloads go to…");
      add("NC_CRASH", "Delete crash logs");
      add("NC_CHECK", "Something not working? Send a check");
      add("NC_MENU", "Menu: search and lists");
      add("NC_RELOAD", "Reload");
      k.messaging.sendMessage("com.lab126.chromebar", "configureChrome", {
        appId: "com.novelcheck.kindle",
        topNavBar: { template: "title", title: "NovelCheck", buttons: [{ id: "KPP_MORE", state: "enabled", handling: "system" }, { id: "KPP_CLOSE", state: "enabled", handling: "system" }] },
        systemMenu: { clientParams: { profile: { name: "default", items: items, selectionMode: "none", closeOnUse: true } } }
      });
      if (!menuBound) {
        menuBound = true;
        k.messaging.receiveMessage("systemMenuItemSelected", function (type, id) { if (typeof menuPick === "function") menuPick(id); });
      }
    } else {
      k.messaging.sendMessage("com.lab126.pillow", "configureChrome", {
        appId: "com.novelcheck.kindle",
        searchBar: { clientParams: { profile: { name: "default", buttons: [{ id: "menu", state: "enabled", handling: "system" }] } } }
      });
    }
    k.net.ensureConnection("all", false, function () {});
  } catch (e) {}
}

// pager shows ▲ ▼ when the page is longer than the screen; each moves it by most of a screen at once (e-ink redraws a
// scrolling page over and over, which makes scrolling slow).
function pager() {
  var p = document.getElementById("pager");
  if (!p) return;
  var h = window.innerHeight || 1024;
  p.className = document.body.scrollHeight > h + 10 ? "on" : "";
  document.getElementById("pgup").onclick = function () { window.scrollBy(0, -Math.round(h * 0.85)); };
  document.getElementById("pgdn").onclick = function () { window.scrollBy(0, Math.round(h * 0.85)); };
}

// applyScale grows the page to the screen: the Kindle's browser reports a bigger window than the 758 x 1024 the
// layout is drawn for, and every size in style.css is in rem (KindleForge's way).
function applyScale() {
  var w = window.innerWidth || 758, h = window.innerHeight || 1024;
  var scale = Math.max(1, Math.min(2.2, Math.min(w / 758, h / 1024)));
  document.documentElement.style.fontSize = Math.round(16 * scale * 100) / 100 + "px";
}
