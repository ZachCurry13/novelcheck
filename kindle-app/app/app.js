/* NovelCheck Kindle app: the screens. ES5 only (see kindle.js). */
var PER_PAGE = 6;
var state = { books: [], page: 0, list: "up-next", levels: {}, offline: false }; // levels: the spice levels shown ({"1": true, "2": true}); none ticked = all
var got = {}; // books downloaded on this Kindle, by id
try { got = JSON.parse(localStorage.getItem("nc-got")) || {}; } catch (e) {}

function $(id) { return document.getElementById(id); }
function note(text) { $("note").textContent = text || ""; }
// setOffline shows (or clears) the line that says there is no connection and which saved copy is on screen.
function setOffline(on, at) {
  state.offline = on;
  var s = $("status");
  s.textContent = on ? "Offline" + (at ? ": showing what was saved " + dateText(at) : "") + ". Downloads wait for Wi-Fi." : "";
  s.hidden = !on;
}
function el(tag, cls, text) {
  var e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
}
function anyLevel() { for (var k in state.levels) if (state.levels[k]) return true; return false; }
function shown() {
  if (!anyLevel() || state.list === "collections") return state.books;
  var out = [];
  for (var i = 0; i < state.books.length; i++) if (state.books[i].level != null && state.levels[state.books[i].level]) out.push(state.books[i]);
  return out;
}
function badge(level) { return level == null ? "Not rated yet" : "Spice " + level + " · " + (level >= 4 ? "Open Door" : level >= 2 ? "Closed Door" : "No Spice"); }
function spice(level) { return level == null ? "Not rated yet" : "Spice level " + level + " (" + (level >= 4 ? "Open Door" : level >= 2 ? "Closed Door" : "No Spice") + ")"; }
function onKindle(b) { return got[b.id] || b.on_device; }

// paintSynced says how long ago the last sync was ("Synced 2 h ago").
function paintSynced() {
  var at = 0;
  try { at = +localStorage.getItem("nc-synced") || 0; } catch (e) {}
  var m = at ? Math.round((new Date().getTime() - at) / 60000) : -1;
  var e = $("synced");
  e.className = "meta" + (m < 0 || m >= 720 ? " stale" : "");
  e.textContent = m < 0 ? "Not synced yet" : m < 2 ? "Synced just now" : m < 120 ? "Synced " + m + " min ago" : m < 720 ? "Synced " + Math.round(m / 60) + " h ago" :
    "Not up to date. Last synced " + dateText(at);
}

function show(list, query) {
  state.list = list;
  var tabs = document.querySelectorAll(".tabs button[data-list]");
  var tab = list === "collection" ? "collections" : list; // a collection's books stay under Collections
  for (var i = 0; i < tabs.length; i++) tabs[i].className = tabs[i].getAttribute("data-list") === tab ? "on" : "";
  var path = list === "search" ? "/kosync/plugin/search?q=" + encodeURIComponent(query) :
    list === "collection" ? "/kosync/plugin/collection/" + encodeURIComponent(query) : "/kosync/plugin/" + list;
  var cached = cachedList(list);
  if (cached && cached.books) { // what was saved shows at once; the fresh list replaces it
    state.books = cached.books;
    state.header = cached.header || "";
    state.page = 0;
    rows();
  }
  note("Loading…");
  api("GET", path + (path.indexOf("?") > 0 ? "&" : "?") + "device=" + encodeURIComponent(NC.device), null, function (err, data, status) {
    if (state.list !== list) return; // another tab was opened meanwhile
    if (err) {
      if (status === 0) setOffline(true, cached && cached.at);
      if (cached && cached.books) return note(status === 0 ? "" : err);
      return offline(err);
    }
    setOffline(false);
    state.header = "";
    if (list === "discover") { // rows of books the family has; each book remembers its row's name
      state.books = [];
      var rowsIn = (data && data.rows) || [];
      for (var r = 0; r < rowsIn.length; r++) for (var k = 0; k < rowsIn[r].books.length; k++) {
        rowsIn[r].books[k].section = (rowsIn[r].icon ? rowsIn[r].icon + " " : "") + rowsIn[r].title;
        state.books.push(rowsIn[r].books[k]);
      }
    } else if (list === "collections") {
      state.books = (data && data.collections) || [];
      for (var c = 0; c < state.books.length; c++) state.books[c].collection = true;
    } else {
      state.books = (data && data.books) || [];
      if (list === "collection") state.header = ((data.icon ? data.icon + " " : "") + data.name) + (data.about ? ": " + data.about : "");
    }
    cacheList(list, state.books, state.header);
    state.page = 0;
    note(state.books.length ? "" : list === "up-next" ? "Your Nightstand is empty." : list === "kindle-mine" ? "Nothing yet: tap Sync now." :
      list === "discover" ? "Nothing on Discover is in your library yet." : list === "collections" ? "No collections yet." : "No books found.");
    rows();
  });
}

function offline(err) {
  note("");
  var m = $("main");
  m.innerHTML = "";
  m.appendChild(el("p", "", err));
  var again = el("button", "main", "Try again");
  again.onclick = function () { show(state.list === "search" ? "up-next" : state.list === "collection" ? "collections" : state.list); };
  m.appendChild(again);
  $("pages").hidden = true;
}

function rows() {
  window.scrollTo(0, 0);
  var m = $("main");
  m.innerHTML = "";
  var list = shown();
  var pages = Math.max(1, Math.ceil(list.length / PER_PAGE));
  var part = list.slice(state.page * PER_PAGE, (state.page + 1) * PER_PAGE);
  if (state.list === "collection") { // one collection: its name, and the way back
    var head = el("div", "sect", state.header || "Collection");
    m.appendChild(head);
    var back = el("button", "", "‹ All collections");
    back.onclick = function () { show("collections"); };
    m.appendChild(back);
  }
  var last = "";
  for (var i = 0; i < part.length; i++) {
    if (part[i].section && part[i].section !== last) { // Discover: the row's name above its books
      m.appendChild(el("div", "sect", part[i].section));
      last = part[i].section;
    }
    m.appendChild(row(part[i]));
  }
  if (!part.length && state.books.length) m.appendChild(el("p", "meta", "No books at the spice levels picked here. Tap All."));
  var pills = document.querySelectorAll(".pills button");
  for (var j = 0; j < pills.length; j++) { var lv = pills[j].getAttribute("data-lvl"); pills[j].className = (lv === "" ? !anyLevel() : state.levels[lv]) ? "on" : ""; }
  var missing = 0;
  for (var q = 0; q < state.books.length; q++) if (!onKindle(state.books[q])) missing++;
  $("dlall").hidden = !(state.list === "up-next" && missing > 0);
  $("pages").hidden = pages < 2;
  $("page").textContent = (state.page + 1) + " of " + pages;
  $("prev").disabled = state.page === 0;
  $("next").disabled = state.page >= pages - 1;
  if (!fitting) fit();
}

// collectionRow is one collection in the Collections list.
function collectionRow(b) {
  var r = el("div", "row");
  var what = el("div", "what");
  what.appendChild(el("div", "title", (b.icon ? b.icon + " " : "") + b.name));
  var meta = el("div", "meta");
  if (b.ai) meta.appendChild(el("span", "badge", "Made by AI"));
  meta.appendChild(document.createTextNode((b.ai ? " " : "") + b.books + " book" + (b.books === 1 ? "" : "s")));
  what.appendChild(meta);
  if (b.about) what.appendChild(el("div", "by", b.about));
  what.onclick = function () { show("collection", b.id); };
  var open = el("button", "act", "Open");
  open.onclick = what.onclick;
  r.appendChild(open);
  r.appendChild(what);
  return r;
}

function row(b) {
  if (b.collection) return collectionRow(b);
  var r = el("div", "row");
  if (b.id) { // a small cover; the book's page has the big one
    var th = el("img", "thumb");
    th.alt = "";
    r.appendChild(th);
    cover(th, b.id, "s");
  }
  var what = el("div", "what");
  what.appendChild(el("div", "title", b.title));
  what.appendChild(el("div", "by", b.author));
  var more = [];
  if (b.series) more.push(b.series);
  if (b.stars) more.push("\u2605 " + (Math.round(b.stars * 10) / 10));
  if (b.genres && b.genres.length) more.push(b.genres.join(", "));
  if (more.length) what.appendChild(el("div", "meta", more.join(" \u00b7 ")));
  if (state.list === "kindle-mine") { // the books on this Kindle: their rating, and where they are
    var meta = el("div", "meta");
    meta.appendChild(el("span", "badge", b.status === "pending" || b.status === "processing" ? "Being rated…" : badge(b.level) + (b.old_rules ? " (older)" : "")));
    meta.appendChild(document.createTextNode((b.format === "cloud" ? " In your Amazon library" : b.percent > 0 ? " " + Math.round(b.percent * 100) + "% read" : "")));
    what.appendChild(meta);
    if (b.id) what.onclick = function () { details(b.id); };
    r.appendChild(what);
    return r;
  }
  var line = el("div", "meta");
  line.appendChild(el("span", "badge", badge(b.level)));
  what.appendChild(line);
  what.onclick = function () { details(b.id); };
  var dl = downloadButton(b);
  dl.className += " act"; // floats right, so it goes in before the text
  r.appendChild(dl);
  r.appendChild(what);
  return r;
}

function downloadButton(b, main) {
  var waiting = !onKindle(b) && isPending(b.id);
  var btn = el("button", main ? "main" : "", onKindle(b) ? "On Kindle" : waiting ? "Waiting for Wi-Fi" : "Download");
  btn.disabled = !!onKindle(b) || waiting;
  btn.onclick = function () { download(b, btn); };
  return btn;
}

// download asks the server for a Kindle copy (it may convert first), then has utild save it into documents/NovelCheck.
var afterDownload = null; // Download all: what runs when a download has ended, well or not
function ended() { var f = afterDownload; if (f) f(); }

function download(b, btn, tries) {
  tries = tries || 0;
  if (state.offline) { // known to be offline: wait for Wi-Fi instead of failing
    queueDownload(b, btn);
    retryPending();
    return ended();
  }
  btn.disabled = true;
  btn.textContent = "Getting…";
  api("GET", "/kosync/plugin/kindle/" + b.id + "?check=1", null, function (err, data, status) {
    if (err) {
      if (status === 0) { setOffline(true, 0); queueDownload(b, btn); return ended(); }
      btn.disabled = false;
      btn.textContent = "Download";
      note(err);
      return ended();
    }
    if (status === 202) {
      note((data && data.message) || "Getting it ready for Kindle…");
      if (tries > 100) { note("This is taking too long. Try again later."); return ended(); }
      return setTimeout(function () { download(b, btn, tries + 1); }, 6000);
    }
    save(b, data.file, btn);
  });
}

// thumbnail gives a downloaded book its cover in the Kindle's library. For a book copied onto it, the Kindle often
// shows a plain tile and looks for the picture in system/thumbnails, named after the book's key and type in its
// library database (the way Calibre does it). Once the Kindle has listed the book, the cover from NovelCheck goes
// there. It runs in the background after the download reports back, and only with a real picture (never the drawn
// stand-in).
function thumbnail(b, path) {
  if (!/^[A-Za-z0-9\/._-]+$/.test(path)) return "";
  var snap = "/tmp/nc_th.db", dir = "/mnt/us/system/thumbnails";
  return "; ( for i in 1 2 3 4 5 6; do sleep 5; cp /var/local/cc.db " + snap + "; [ -e /var/local/cc.db-wal ] && cp /var/local/cc.db-wal " + snap + "-wal; " +
    "K=$(sqlite3 " + snap + " \"SELECT p_cdeKey || '_' || p_cdeType FROM Entries WHERE p_location='" + path + "' LIMIT 1\"); rm -f " + snap + "*; " +
    "[ -n \"$K\" ] && break; done; if [ -n \"$K\" ]; then mkdir -p " + dir + "; T=\"" + dir + "/thumbnail_${K}_portrait.jpg\"; " +
    "if curl -sfL -H " + sh("x-auth-user: " + NC.user) + " -H " + sh("x-auth-key: " + NC.key) + " -o \"$T.part\" -w '%{content_type}' " +
    sh(NC.server + "/kosync/plugin/cover/" + b.id + "?size=l") + " | grep -q jpeg; then mv \"$T.part\" \"$T\"; else rm -f \"$T.part\"; fi; fi ) >/dev/null 2>&1 &";
}

function save(b, file, btn) {
  var dir = "/mnt/us/documents/NovelCheck", out = "/tmp/nc-dl-" + b.id + ".txt";
  var part = dir + "/" + file + ".part";
  var cmd = "mkdir -p " + dir + "; rm -f " + out + "; if curl -sfL -H " + sh("x-auth-user: " + NC.user) +
    " -H " + sh("x-auth-key: " + NC.key) + " -o " + sh(part) + " " + sh(NC.server + "/kosync/plugin/kindle/" + b.id) +
    "; then mv " + sh(part) + " " + sh(dir + "/" + file) + "; lipc-set-prop com.lab126.scanner doFullScan 1; echo ok > " + out +
    "; else rm -f " + sh(part) + "; echo failed > " + out + "; fi; echo END >> " + out + thumbnail(b, dir + "/" + file);
  if (!run(cmd)) {
    btn.disabled = false;
    btn.textContent = "Download";
    return note("Saving books needs KindleForge's helper. Open KindleForge once, then try again.");
  }
  note("Downloading " + b.title + "…");
  waitFor(out, 600, function (text) {
    if (text && text.indexOf("ok") >= 0) {
      got[b.id] = true;
      logLine("downloaded " + b.title + " as " + file);
      try { localStorage.setItem("nc-got", JSON.stringify(got)); } catch (e) {}
      btn.textContent = "On Kindle";
      note(b.title + " is in your Kindle library.");
      return ended();
    }
    btn.disabled = false;
    btn.textContent = "Download";
    note("The download failed. Check Wi-Fi and try again.");
    setTimeout(ended, 0);
    logLine("download failed: " + b.title + " (" + file + "): " + (text == null ? "no answer from the Kindle helper" : text.replace(/\s+/g, " ")));
    sendLog("a download failed");
  });
}

// downloadAll downloads the Nightstand books that aren't on this Kindle yet, one after another (the plugin's
// "Download Nightstand").
function downloadAll() {
  var todo = [], done = 0, current = null;
  for (var i = 0; i < state.books.length; i++) if (!onKindle(state.books[i])) todo.push(state.books[i]);
  var total = todo.length;
  if (!total) return note("Everything on your Nightstand is already on this Kindle.");
  $("dlall").disabled = true;
  var next = function () {
    if (current && got[current.id]) done++;
    current = todo.shift();
    if (!current) {
      afterDownload = null;
      $("dlall").disabled = false;
      logLine("download all: " + done + " of " + total);
      return note("Downloaded " + done + " of " + total + " Nightstand books.");
    }
    note("Downloading " + (total - todo.length) + " of " + total + ": " + current.title + "…");
    download(current, { disabled: false, textContent: "" });
  };
  afterDownload = next;
  next();
}

// showLeft asks about books that left the Kindle: keep them in NovelCheck, remove them from her library (only she
// stops seeing them), or remove them and ask for them to be deleted for good (the main admin decides).
function showLeft(list) {
  var m = $("main");
  m.innerHTML = "";
  $("pages").hidden = true;
  m.appendChild(el("h2", "", "These left your Kindle"));
  m.appendChild(el("p", "meta", "Keep them in NovelCheck, or remove them. Their rating stays either way."));
  var choices = [["keep", "Keep"], ["hide", "Remove from my library"], ["delete", "Remove, ask to delete"]];
  for (var i = 0; i < list.length; i++) {
    (function (b) {
      var r = el("div", "book");
      r.appendChild(el("div", "title", b.title));
      r.appendChild(el("div", "by", b.author));
      var acts = el("div", "actions");
      for (var j = 0; j < choices.length; j++) {
        (function (c) {
          var btn = el("button", c[0] === "keep" ? "" : "main", c[1]);
          btn.onclick = function () {
            api("POST", "/kosync/plugin/left", { book_id: b.id, choice: c[0] }, function (err) {
              if (err) return note(err);
              m.removeChild(r);
              note(c[0] === "keep" ? "Kept." : c[0] === "hide" ? "Removed from your library." : "Removed; the main admin will decide about deleting it.");
              if (!m.querySelector(".book")) show("up-next");
            });
          };
          acts.appendChild(btn);
        })(choices[j]);
      }
      r.appendChild(acts);
      m.appendChild(r);
    })(list[i]);
  }
}

window.addEventListener("resize", function () { applyScale(); pager(); });
setInterval(pager, 1500); // pages change size as lists and books load
if (window.kindle && window.kindle.appmgr) window.kindle.appmgr.ongo = chrome; // the Kindle shows the app: set its top bar

document.addEventListener("DOMContentLoaded", function () {
  applyScale();
  chrome();
  if (!NC.server) return note("This copy has no settings. Download it again from NovelCheck.");
  // its own name ("Zach Paperwhite (Amazon reader)"), so the plugin's report of the same Kindle is never replaced
  if (!/ \(Amazon reader\)$/.test(NC.device)) NC.device = (NC.device || "Kindle") + " (Amazon reader)";
  $("find").onsubmit = function (e) {
    e.preventDefault();
    var q = $("q").value.replace(/^\s+|\s+$/g, "");
    if (q.length > 1) show("search", q);
  };
  var tabs = document.querySelectorAll(".tabs button[data-list]");
  for (var i = 0; i < tabs.length; i++) tabs[i].onclick = function () { show(this.getAttribute("data-list")); };
  var pills = document.querySelectorAll(".pills button");
  for (var k = 0; k < pills.length; k++) pills[k].onclick = function () {
    var lv = this.getAttribute("data-lvl");
    if (lv === "") state.levels = {}; else state.levels[lv] = !state.levels[lv]; // several levels can be ticked together
    state.page = 0;
    rows();
  };
  $("dlall").onclick = downloadAll;
  $("ver").textContent = "v" + (NC.version || "?");
  $("who").textContent = "Signed in as " + (NC.user || "?") + " · " + String(NC.device || "").replace(/ \(Amazon reader\)$/, "");
  paintSynced();
  setInterval(paintSynced, 60000);
  logLine("app " + NC.version + " opened as " + NC.device);
  $("report").onclick = function () { note("Syncing…"); syncNow(true); };
  $("check").onclick = function () {
    if (!run("sh /mnt/us/extensions/novelcheck-app/check.sh")) return note("The check needs KindleForge's helper. Open KindleForge once, then try again.");
    note("Checking this Kindle… A message appears when it's done, and the result goes to NovelCheck.");
  };
  if (!stored("nc-tidy-1") && run("rm -f '/mnt/us/documents/NovelCheck-check.txt' '/mnt/us/documents/NovelCheck-Check.sh'; lipc-set-prop com.lab126.scanner doFullScan 1")) {
    keep("nc-tidy-1", "1"); // app 1.2.0 put a check "book" and its result in the library: take them out once
  }
  $("prev").onclick = function () { state.page--; rows(); };
  $("next").onclick = function () { state.page++; rows(); };
  show("up-next");
  checkUpdate();
  setTimeout(warmCache, 6000); // keep the Kindle's books and their ratings for when there is no Wi-Fi
  syncNow(false); // by itself at most every 12 hours
});
