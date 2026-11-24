/* NovelCheck Kindle app: what it keeps on the Kindle, so the lists, the ratings of the books on the Kindle and the books'
   pages show without Wi-Fi (with the day they were saved), and what it does about downloads without a connection: they
   wait, and start when Wi-Fi is back (while the app is open). ES5 only. */
var LISTS = { "up-next": 1, "new": 1, "discover": 1, "collections": 1, "kindle-mine": 1 }; // lists worth keeping (not a search)
var MAX_BOOKS = 400, MAX_INFO = 80;

function lsGet(k) { try { return JSON.parse(localStorage.getItem(k)); } catch (e) { return null; } }
function lsPut(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); return true; } catch (e) { return false; } }
function lsDrop(k) { try { localStorage.removeItem(k); } catch (e) {} }

var DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
var MONTHS3 = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
// dateText: "Mon Nov 24, 10:05 AM"
function dateText(ms) {
  var d = new Date(ms), h = d.getHours(), m = d.getMinutes();
  return DAYS[d.getDay()] + " " + MONTHS3[d.getMonth()] + " " + d.getDate() + ", " + (h % 12 || 12) + ":" + (m < 10 ? "0" : "") + m + (h < 12 ? " AM" : " PM");
}

function cacheList(list, books, header) {
  if (LISTS[list]) lsPut("nc-l-" + list, { at: new Date().getTime(), books: books.slice(0, MAX_BOOKS), header: header || "" });
}
function cachedList(list) { return LISTS[list] ? lsGet("nc-l-" + list) : null; }

// cacheInfo keeps a book's page (the newest MAX_INFO); when the storage is full the oldest half goes.
function cacheInfo(id, b) {
  var ix = lsGet("nc-i-index") || [], at = -1;
  for (var i = 0; i < ix.length; i++) if (ix[i] === id) at = i;
  if (at >= 0) ix.splice(at, 1);
  ix.push(id);
  while (ix.length > MAX_INFO) lsDrop("nc-i-" + ix.shift());
  if (!lsPut("nc-i-" + id, { at: new Date().getTime(), b: b })) {
    var cut = ix.splice(0, Math.ceil(ix.length / 2));
    for (var j = 0; j < cut.length; j++) if (cut[j] !== id) lsDrop("nc-i-" + cut[j]);
    lsPut("nc-i-" + id, { at: new Date().getTime(), b: b });
    ix.push(id);
  }
  lsPut("nc-i-index", ix);
}
function cachedInfo(id) { return lsGet("nc-i-" + id); }

// warmCache keeps the Kindle's own books (their ratings and pages) for when there is no Wi-Fi: the My Kindle list, then
// each downloaded book's page, one at a time and a few per opening.
function warmCache() {
  api("GET", "/kosync/plugin/kindle-mine?device=" + encodeURIComponent(NC.device), null, function (err, data) {
    if (err || !data || !data.books) return;
    cacheList("kindle-mine", data.books);
    var todo = [];
    for (var i = 0; i < data.books.length && todo.length < 40; i++) {
      var b = data.books[i];
      if (b.id && b.format !== "cloud" && !cachedInfo(b.id)) todo.push(b.id);
    }
    (function next() {
      var id = todo.shift();
      if (!id) return;
      api("GET", "/kosync/plugin/info/" + id, null, function (e2, info) {
        if (e2 || !info) return;
        cacheInfo(id, info);
        setTimeout(next, 1500);
      });
    })();
  });
}

// ----- no Wi-Fi: downloads wait
function pendingList() { return lsGet("nc-pending") || []; }
function isPending(id) {
  var p = pendingList();
  for (var i = 0; i < p.length; i++) if (p[i].id === id) return true;
  return false;
}
function addPending(b) { if (!isPending(b.id)) { var p = pendingList(); p.push({ id: b.id, title: b.title, author: b.author }); lsPut("nc-pending", p); } }
function dropPending(id) {
  var p = pendingList(), out = [];
  for (var i = 0; i < p.length; i++) if (p[i].id !== id) out.push(p[i]);
  lsPut("nc-pending", out);
}

// queueDownload: the download starts by itself when Wi-Fi is back.
function queueDownload(b, btn) {
  addPending(b);
  btn.disabled = true;
  btn.textContent = "Waiting for Wi-Fi";
  note("No connection. “" + b.title + "” will download when Wi-Fi is back (keep NovelCheck open).");
  logLine("download waiting for Wi-Fi: " + b.title);
}

var retrying = false, retryAt = 0;
function retryPending() {
  var p = pendingList();
  if (!p.length || retrying || new Date().getTime() < retryAt) return;
  retrying = true;
  var b = p[0];
  api("GET", "/kosync/plugin/version", null, function (err, d, status) {
    if (status === 0) { // still no connection
      retrying = false;
      retryAt = new Date().getTime() + 20000;
      return;
    }
    setOffline(false);
    note("Wi-Fi is back: downloading “" + b.title + "”…");
    afterDownload = function () {
      retrying = false;
      if (got[b.id]) { dropPending(b.id); setTimeout(retryPending, 500); } else retryAt = new Date().getTime() + 30000;
      if (state.list) rows();
    };
    download(b, { disabled: false, textContent: "" });
  });
}
setInterval(retryPending, 30000);
window.addEventListener("online", retryPending);
