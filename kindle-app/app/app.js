/* NovelCheck Kindle app: the screens. ES5 only (see kindle.js). */
var PER_PAGE = 6;
var state = { books: [], page: 0, list: "up-next", levels: {}, offline: false, show: "" }; // levels: the spice levels shown ({"1": true, "2": true}); none ticked = all
var got = {}; // books downloaded on this Kindle, by id
try { got = JSON.parse(localStorage.getItem("nc-got")) || {}; } catch (e) {}

function $(id) { return document.getElementById(id); }
// The Kindle's own helper (KindleForge's utild) saves books and reads the library; without an answer from it nothing can.
var HELPER_HINT = "This Kindle's helper isn't answering. Open KindleForge once (it starts the helper), then open NovelCheck again. Without KindleForge on this Kindle the app can't save books or read its library.";
function note(text) { $("note").textContent = text || ""; }

// ----- where you are: the top bar has ☰ Menu at the left (on a list) or ‹ Back (inside a book, the menu, a collection)
var backTo = null; // what ‹ Back does; null on a list, where the button opens the menu
var TITLES = { "up-next": "Nightstand", "new": "New", "discover": "Discover", "collections": "Collections", "kindle-mine": "My Kindle", "search": "Check a book" };
function paintNav(title) {
  $("navbtn").textContent = backTo ? "‹ Back" : "☰ Menu";
  $("title").textContent = title || "NovelCheck";
}
function showLists(listView) { // listView: the filters and the page buttons belong to a list, not to a book
  $("menu").hidden = true;
  $("lists").hidden = false;
  $("filters").hidden = !listView;
}
// paintCounts puts how many books were in each list the last time it was opened on the Menu's buttons.
function paintCounts() {
  var names = ["up-next", "new", "discover", "collections", "kindle-mine"];
  for (var i = 0; i < names.length; i++) {
    var c = +stored("nc-count-" + names[i]);
    var e = $("c-" + names[i]);
    if (e) e.textContent = c ? String(c) : "";
  }
}
function openMenu() {
  paintCounts();
  $("lists").hidden = true;
  $("menu").hidden = false;
  note("");
  backTo = function () { backTo = null; if (state.books && state.books.length) rows(); else show(state.list || "up-next"); };
  paintNav("Menu");
  window.scrollTo(0, 0);
}
// goBack: a book's page returns to the list it was opened from
function goBack() { if (backTo) backTo(); else openMenu(); }
// setOffline shows (or clears) the line that says there is no connection and which saved copy is on screen.
function setOffline(on, at) {
  state.offline = on;
  var s = $("status");
  s.textContent = on ? "Can't reach NovelCheck right now" + (at ? ". Showing what was saved " + dateText(at) : "") + ". Downloads wait until it can be reached." : "";
  s.hidden = !on;
}
function el(tag, cls, text) {
  var e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
}
// ----- sorting a list (not Discover or Collections, which are in rows of their own)
var SORTS = [["recent", "Recent"], ["title", "Name A to Z"], ["author", "Author"], ["spice_up", "Spice, low to high"], ["spice_down", "Spice, high to low"], ["stars", "Hardcover rating"]];
var sortKey = stored("nc-sort") || "recent";
function sortLabel(k) { for (var i = 0; i < SORTS.length; i++) if (SORTS[i][0] === k) return SORTS[i][1]; return "Recent"; }
function plain(t) { return String(t || "").toLowerCase().replace(/^(the|a|an) /, "").replace(/[^a-z0-9]+/g, " ").replace(/^ +/, ""); }
function lastNameKey(a) { var f = String(a || "").toLowerCase().replace(/[^a-z ,]+/g, " ").replace(/ +/g, " ").replace(/^ | $/g, "").split(" "); return f[f.length - 1] || ""; }
function sorted(books) {
  if (sortKey === "recent" || state.list === "discover" || state.list === "collections") return books;
  var idx = [];
  for (var i = 0; i < books.length; i++) idx.push({ b: books[i], i: i });
  var num = function (v, none) { return v == null ? none : v; };
  idx.sort(function (x, y) {
    var a = x.b, b = y.b, d = 0;
    if (sortKey === "title") d = plain(a.title) < plain(b.title) ? -1 : plain(a.title) > plain(b.title) ? 1 : 0;
    else if (sortKey === "author") { d = lastNameKey(a.author) < lastNameKey(b.author) ? -1 : lastNameKey(a.author) > lastNameKey(b.author) ? 1 : 0; if (!d) d = plain(a.title) < plain(b.title) ? -1 : 1; }
    else if (sortKey === "spice_up") d = num(a.level, 99) - num(b.level, 99);
    else if (sortKey === "spice_down") d = num(b.level, -1) - num(a.level, -1);
    else if (sortKey === "stars") d = (b.stars || 0) - (a.stars || 0);
    return d || x.i - y.i; // ties keep the list's own order
  });
  var out = [];
  for (var j = 0; j < idx.length; j++) out.push(idx[j].b);
  return out;
}
function setSort(k) {
  sortKey = k;
  keep("nc-sort", k);
  state.page = 0;
  if (!$("lists").hidden && !$("filters").hidden) rows();
}
// menuPick runs what was chosen in the Kindle's own ⋯ menu (kindle.js chrome).
function menuPick(id) {
  if (id === "NC_CHECK") return sendCheck();
  if (id.indexOf("NC_") === 0 && typeof TOOLS !== "undefined" && (TOOLS[id] || id === "NC_HIDDEN" || id === "NC_DEST")) return runTool(id);
  if (id === "NC_SYNC") { note("Syncing…"); return syncNow(true); }
  if (id === "NC_MENU") return openMenu();
  if (id === "NC_RELOAD") window.location.reload();
}
// sendCheck has the Kindle check itself and send the result to NovelCheck (the ⋯ menu's "Something not working?").
function sendCheck() {
  if (!run("sh /mnt/us/extensions/novelcheck-app/check.sh")) return note("The check needs KindleForge's helper. Open KindleForge once, then try again.");
  note("Checking this Kindle… A message appears when it's done, and the result goes to NovelCheck.");
}

function anyLevel() { for (var k in state.levels) if (state.levels[k]) return true; return false; }
// shown is the list with this screen's filters applied: the spice levels ticked, and all / on my Kindle / not on it.
function shown() {
  var out = [];
  var plain = state.list === "collections" || state.list === "discover";
  for (var i = 0; i < state.books.length; i++) {
    var b = state.books[i];
    if (!plain || state.list === "discover") {
      if (anyLevel() && !(b.level != null && state.levels[b.level])) continue;
    }
    if (!plain && state.show && (state.show === "on") !== !!onKindle(b)) continue;
    out.push(b);
  }
  return sorted(out);
}
// the same names as on the web and in KOReader
var SPICE_NAMES = ["No Romance", "Sweet Romance", "Romantic", "Steamy Closed-Door", "Explicit / Open Door", "Very Explicit / Erotica-Level"];
function badge(level) { return level == null ? "Not rated yet" : "Spice " + level + " · " + (SPICE_NAMES[level] || ""); }
function spice(level) { return level == null ? "Not rated yet" : "Spice level " + level + ": " + (SPICE_NAMES[level] || ""); }
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
  state.query = list === "search" ? query : "";
  showLists(true);
  var tabs = document.querySelectorAll(".tabs button[data-list]");
  var tab = list === "collection" ? "collections" : list; // a collection's books stay under Collections
  for (var i = 0; i < tabs.length; i++) tabs[i].className = tabs[i].getAttribute("data-list") === tab ? "on" : "";
  var path = list === "search" ? "/kosync/plugin/search?q=" + encodeURIComponent(query) :
    list === "collection" ? "/kosync/plugin/collection/" + encodeURIComponent(query) : "/kosync/plugin/" + list;
  var cached = list === "search" ? null : cachedList(list); // a search is for this query only
  if (cached && cached.books && cached.books.length) { // what was saved shows at once; the fresh list replaces it
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
      if (cached && cached.books && cached.books.length) return note(status === 0 ? "" : err);
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
    if (list !== "search") cacheList(list, state.books, state.header);
    keep("nc-count-" + list, String(state.books.length)); // the Menu shows how many are in each list
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
  showLists(true);
  backTo = state.list === "collection" ? function () { show("collections"); } : null;
  paintNav(state.list === "collection" ? (state.header || "Collection").split(":")[0] : TITLES[state.list]);
  window.scrollTo(0, 0);
  var m = $("main");
  m.innerHTML = "";
  var list = shown();
  var pages = Math.max(1, Math.ceil(list.length / PER_PAGE));
  var part = list.slice(state.page * PER_PAGE, (state.page + 1) * PER_PAGE);
  if (state.list === "collection") { // one collection: its name, and the way back
    var head = el("div", "sect", state.header || "Collection");
    m.appendChild(head);
  }
  var last = "";
  for (var i = 0; i < part.length; i++) {
    if (part[i].section && part[i].section !== last) { // Discover: the row's name above its books
      m.appendChild(el("div", "sect", part[i].section));
      last = part[i].section;
    }
    m.appendChild(row(part[i]));
  }
  if (!part.length && state.books.length) m.appendChild(el("p", "meta", "No books match these filters. Tap All."));
  if (!state.books.length) m.appendChild(el("p", "", state.list === "up-next" ? "Your Nightstand is empty. Add books to it from New or Discover, or in NovelCheck on the web." :
    state.list === "kindle-mine" ? "Nothing yet: tap Sync now." : state.list === "collections" ? "No collections yet." : state.list === "discover" ? "Nothing on Discover is in your library yet." : "No books found."));
  if (state.list === "search" && state.query) { // not in the library: look it up online, as Check a book does
    var up = el("button", "main", "Not here? Look it up online");
    up.onclick = function () { lookupOnline(state.query); };
    m.appendChild(up);
  }
  var pills = document.querySelectorAll(".pills button[data-lvl]");
  var noSort = state.list === "discover" || state.list === "collections";
  $("sortrow").hidden = noSort;
  $("showrow").hidden = noSort;
  var sorts = document.querySelectorAll(".pills button[data-sort]");
  for (var s1 = 0; s1 < sorts.length; s1++) sorts[s1].className = sorts[s1].getAttribute("data-sort") === sortKey ? "on" : "";
  var shows = document.querySelectorAll(".pills button[data-show]");
  for (var s2 = 0; s2 < shows.length; s2++) shows[s2].className = shows[s2].getAttribute("data-show") === state.show ? "on" : "";
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
  if (b.no_file) { // in the library with nothing to download here: its page says why and offers a request
    dl = el("button", "", "Details");
    dl.onclick = function () { details(b.id); };
  }
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
  if (stored("nc-dest") === "koreader") return saveForKOReader(b, btn); // the book as it is, in KOReader's folder, like the plugin
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
    sh(server() + "/kosync/plugin/cover/" + b.id + "?size=l") + " | grep -q jpeg; then mv \"$T.part\" \"$T\"; else rm -f \"$T.part\"; fi; fi ) >/dev/null 2>&1 &";
}

// saveForKOReader downloads the book in its own format (EPUB, no conversion) into the NovelCheck folder inside KOReader's home
// folder (read from KOReader's settings, as the plugin does), where KOReader finds it. The file name comes from the server.
function saveForKOReader(b, btn) {
  var out = "/tmp/nc-dl-" + b.id + ".txt", tmp = "/tmp/nc-ko-" + b.id + ".part", hdr = "/tmp/nc-ko-" + b.id + ".hdr";
  var cmd = "rm -f " + out + " " + hdr + "; H=$(sed -n 's/.*\\[\"home_dir\"\\] = \"\\(.*\\)\".*/\\1/p' /mnt/us/koreader/settings.reader.lua 2>/dev/null | head -1); " +
    "[ -d \"$H\" ] || H=/mnt/us/documents; D=\"${H%/}/NovelCheck\"; mkdir -p \"$D\"; " +
    "if curl -sfL -D " + hdr + " -H " + sh("x-auth-user: " + NC.user) + " -H " + sh("x-auth-key: " + NC.key) + " -o " + tmp + " " + sh(server() + "/kosync/plugin/book/" + b.id + "/book") +
    "; then F=$(sed -n 's/^[Cc]ontent-[Dd]isposition:.*filename=\"\\(.*\\)\".*/\\1/p' " + hdr + " | tr -d '\\r' | tr '/' '_'); [ -n \"$F\" ] || F=book-" + b.id + ".epub; mv " + tmp + " \"$D/$F\"; echo ok > " + out +
    "; else rm -f " + tmp + "; echo failed > " + out + "; fi; rm -f " + hdr + "; echo END >> " + out;
  if (!run(cmd)) {
    btn.disabled = false;
    btn.textContent = "Download";
    return note("Saving books needs KindleForge's helper. Open KindleForge once, then try again.");
  }
  note("Downloading " + b.title + " for KOReader…");
  waitFor(out, 600, function (text) {
    if (text && text.indexOf("ok") >= 0) {
      got[b.id] = true;
      logLine("downloaded " + b.title + " for KOReader");
      try { localStorage.setItem("nc-got", JSON.stringify(got)); } catch (e) {}
      btn.textContent = "On Kindle";
      note(b.title + " is in KOReader's NovelCheck folder.");
      return ended();
    }
    btn.disabled = false;
    btn.textContent = "Download";
    note("The download failed. Check Wi-Fi and try again.");
    logLine("KOReader download failed: " + b.title);
    setTimeout(ended, 0);
  });
}

function save(b, file, btn) {
  var dir = "/mnt/us/documents/NovelCheck", out = "/tmp/nc-dl-" + b.id + ".txt";
  var part = dir + "/" + file + ".part";
  var cmd = "mkdir -p " + dir + "; rm -f " + out + "; if curl -sfL -H " + sh("x-auth-user: " + NC.user) +
    " -H " + sh("x-auth-key: " + NC.key) + " -o " + sh(part) + " " + sh(server() + "/kosync/plugin/kindle/" + b.id) +
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
    note(text == null ? HELPER_HINT : "The download failed. Check Wi-Fi and try again.");
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
  showLists(false);
  backTo = function () { rows(); };
  paintNav("Left your Kindle");
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
  if (!NC.server) return signin(); // installed from KindleForge: asks for the sign-in once
  // its own name ("Zach Paperwhite (Amazon reader)"), so the plugin's report of the same Kindle is never replaced
  if (!/ \(Amazon reader\)$/.test(NC.device)) NC.device = (NC.device || "Kindle") + " (Amazon reader)";
  $("navbtn").onclick = goBack;
  state.show = stored("nc-show") || "";
  var sortChips = document.querySelectorAll(".pills button[data-sort]");
  for (var sc = 0; sc < sortChips.length; sc++) sortChips[sc].onclick = function () { setSort(this.getAttribute("data-sort")); };
  var showChips = document.querySelectorAll(".pills button[data-show]");
  for (var sh = 0; sh < showChips.length; sh++) showChips[sh].onclick = function () {
    state.show = this.getAttribute("data-show");
    keep("nc-show", state.show);
    state.page = 0;
    rows();
  };
  bindTools();
  $("wantbtn").onclick = showWant;
  $("find").onsubmit = function (e) {
    e.preventDefault();
    var q = $("q").value.replace(/^\s+|\s+$/g, "");
    if (q.length > 1) show("search", q);
  };
  var tabs = document.querySelectorAll(".tabs button[data-list]");
  for (var i = 0; i < tabs.length; i++) tabs[i].onclick = function () { show(this.getAttribute("data-list")); };
  var pills = document.querySelectorAll(".pills button[data-lvl]");
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
  if (!stored("nc-tidy-1") && run("rm -f '/mnt/us/documents/NovelCheck-check.txt' '/mnt/us/documents/NovelCheck-Check.sh'; lipc-set-prop com.lab126.scanner doFullScan 1")) {
    keep("nc-tidy-1", "1"); // app 1.2.0 put a check "book" and its result in the library: take them out once
  }
  $("prev").onclick = function () { state.page--; rows(); };
  $("next").onclick = function () { state.page++; rows(); };
  openMenu(); // the app opens on Check a book, with the lists under it
  checkUpdate();
  setTimeout(warmCache, 6000); // keep the Kindle's books and their ratings for when there is no Wi-Fi
  syncNow(false); // by itself at most every 12 hours
});
