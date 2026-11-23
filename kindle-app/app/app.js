/* NovelCheck Kindle app: the screens. ES5 only (see kindle.js). */
var PER_PAGE = 6;
var state = { books: [], page: 0, list: "up-next" };
var got = {}; // books downloaded on this Kindle, by id
try { got = JSON.parse(localStorage.getItem("nc-got")) || {}; } catch (e) {}

function $(id) { return document.getElementById(id); }
function note(text) { $("note").textContent = text || ""; }
function el(tag, cls, text) {
  var e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
}
function spice(level) { return level == null ? "Not rated yet" : "Spice " + level + "/5"; }
function onKindle(b) { return got[b.id] || b.on_device; }

function show(list, query) {
  state.list = list;
  var tabs = document.querySelectorAll(".tabs button");
  for (var i = 0; i < tabs.length; i++) tabs[i].className = tabs[i].getAttribute("data-list") === list ? "on" : "";
  var path = list === "search" ? "/kosync/plugin/search?q=" + encodeURIComponent(query) : "/kosync/plugin/" + list;
  note("Loading…");
  api("GET", path + (path.indexOf("?") > 0 ? "&" : "?") + "device=" + encodeURIComponent(NC.device), null, function (err, data) {
    if (err) return offline(err);
    state.books = (data && data.books) || [];
    state.page = 0;
    note(state.books.length ? "" : list === "up-next" ? "Your Nightstand is empty." : "No books found.");
    rows();
  });
}

function offline(err) {
  note("");
  var m = $("main");
  m.innerHTML = "";
  m.appendChild(el("p", "", err));
  var again = el("button", "main", "Try again");
  again.onclick = function () { show(state.list === "search" ? "up-next" : state.list); };
  m.appendChild(again);
  $("pages").hidden = true;
}

function rows() {
  var m = $("main");
  m.innerHTML = "";
  var pages = Math.max(1, Math.ceil(state.books.length / PER_PAGE));
  var part = state.books.slice(state.page * PER_PAGE, (state.page + 1) * PER_PAGE);
  for (var i = 0; i < part.length; i++) m.appendChild(row(part[i]));
  $("pages").hidden = pages < 2;
  $("page").textContent = (state.page + 1) + " of " + pages;
  $("prev").disabled = state.page === 0;
  $("next").disabled = state.page >= pages - 1;
}

function row(b) {
  var r = el("div", "row");
  var what = el("div", "what");
  what.appendChild(el("div", "title", b.title));
  what.appendChild(el("div", "by", b.author));
  what.appendChild(el("div", "meta", spice(b.level)));
  what.onclick = function () { details(b.id); };
  r.appendChild(what);
  r.appendChild(downloadButton(b));
  return r;
}

function downloadButton(b, main) {
  var btn = el("button", main ? "main" : "", onKindle(b) ? "On Kindle" : "Download");
  btn.disabled = !!onKindle(b);
  btn.onclick = function () { download(b, btn); };
  return btn;
}

function details(id) {
  note("Loading…");
  api("GET", "/kosync/plugin/info/" + id, null, function (err, b) {
    if (err) return note(err);
    note("");
    var m = $("main");
    m.innerHTML = "";
    $("pages").hidden = true;
    var d = el("div", "book");
    var img = el("img");
    d.appendChild(img);
    cover(img, id);
    d.appendChild(el("h2", "", b.title));
    d.appendChild(el("div", "by", b.author + (b.series ? " · " + b.series + (b.series_index ? " #" + b.series_index : "") : "")));
    d.appendChild(el("p", "title", spice(b.level) + (b.reason ? ": " + b.reason : "")));
    if (b.verdict) d.appendChild(el("p", "", b.verdict));
    if (b.summary) d.appendChild(el("p", "", b.summary));
    var acts = el("div", "actions");
    var back = el("button", "", "Back");
    back.onclick = rows;
    if (b.url) acts.appendChild(downloadButton(b, true));
    else acts.appendChild(el("span", "meta", "No file to download."));
    acts.appendChild(back);
    d.appendChild(acts);
    m.appendChild(d);
  });
}

// download asks the server for a Kindle copy (it may convert first), then has utild save it into documents/NovelCheck.
function download(b, btn, tries) {
  tries = tries || 0;
  btn.disabled = true;
  btn.textContent = "Getting…";
  api("GET", "/kosync/plugin/kindle/" + b.id + "?check=1", null, function (err, data, status) {
    if (err) {
      btn.disabled = false;
      btn.textContent = "Download";
      return note(err);
    }
    if (status === 202) {
      note((data && data.message) || "Getting it ready for Kindle…");
      if (tries > 100) return note("This is taking too long. Try again later.");
      return setTimeout(function () { download(b, btn, tries + 1); }, 6000);
    }
    save(b, data.file, btn);
  });
}

function save(b, file, btn) {
  var dir = "/mnt/us/documents/NovelCheck", out = "/tmp/nc-dl-" + b.id + ".txt";
  var part = dir + "/" + file + ".part";
  var cmd = "mkdir -p " + dir + "; rm -f " + out + "; if curl -sfL -H " + sh("x-auth-user: " + NC.user) +
    " -H " + sh("x-auth-key: " + NC.key) + " -o " + sh(part) + " " + sh(NC.server + "/kosync/plugin/kindle/" + b.id) +
    "; then mv " + sh(part) + " " + sh(dir + "/" + file) + "; lipc-set-prop com.lab126.scanner doFullScan 1; echo ok > " + out +
    "; else rm -f " + sh(part) + "; echo failed > " + out + "; fi; echo END >> " + out;
  if (!run(cmd)) {
    btn.disabled = false;
    btn.textContent = "Download";
    return note("Saving books needs KindleForge's helper. Open KindleForge once, then try again.");
  }
  note("Downloading " + b.title + "…");
  waitFor(out, 600, function (text) {
    if (text && text.indexOf("ok") >= 0) {
      got[b.id] = true;
      try { localStorage.setItem("nc-got", JSON.stringify(got)); } catch (e) {}
      btn.textContent = "On Kindle";
      return note(b.title + " is in your Kindle library.");
    }
    btn.disabled = false;
    btn.textContent = "Download";
    note("The download failed. Check Wi-Fi and try again.");
  });
}

// report sends NovelCheck the books in the Kindle's library (on this Kindle and in the Amazon cloud),
// read from a copy of the library database (the Kindle keeps the real one open).
function report() {
  var out = "/tmp/nc-books.txt", snap = "/tmp/nc_cc_snap.db";
  var cmd = "rm -f " + out + " " + snap + "*; cp /var/local/cc.db " + snap + "; [ -e /var/local/cc.db-wal ] && cp /var/local/cc.db-wal " + snap + "-wal; " +
    "sqlite3 -separator '\t' " + snap + " \"SELECT p_cdeKey, p_titles_0_nominal, COALESCE(p_location,''), COALESCE(p_percentFinished,0), " +
    "COALESCE(j_credits,'') FROM Entries WHERE p_type='Entry:Item' AND p_cdeType IN ('EBOK','PDOC')\" > " + out + "; rm -f " + snap + "*; echo END >> " + out;
  if (!run(cmd)) return note("Reporting needs KindleForge's helper. Open KindleForge once, then try again.");
  note("Reading your Kindle library…");
  waitFor(out, 90, function (text) {
    if (text == null) return note("Couldn't read the Kindle library.");
    var books = parseLibrary(text);
    note("Sending " + books.length + " books…");
    api("POST", "/kosync/plugin/device", { device: NC.device, plugin_version: "app " + NC.version, books: books }, function (err, r) {
      if (err) return note(err);
      note("Reported " + r.books + " books. " + (r.matched === 1 ? "1 is" : r.matched + " are") + " in NovelCheck.");
    });
  });
}

// parseLibrary turns the database lines (key, title, file, percent, credits) into the plugin's device report.
function parseLibrary(text) {
  var lines = text.split("\n"), books = [];
  for (var i = 0; i < lines.length; i++) {
    var f = lines[i].split("\t");
    if (f.length < 5 || !f[1] || /\.sh$/.test(f[2])) continue;
    var author = "";
    try { author = JSON.parse(f.slice(4).join("\t"))[0].name.display || ""; } catch (e) {}
    var file = f[2] ? f[2].split("/").pop() : f[1].replace(/[\/\\]/g, " ") + " [" + f[0] + "]";
    var ext = f[2] ? (file.split(".").pop() || "").toLowerCase() : "cloud";
    books.push({ filename: file, title: f[1], author: author, format: ext, percent: Math.min(1, (+f[3] || 0) / 100) });
  }
  return books;
}

document.addEventListener("DOMContentLoaded", function () {
  chrome();
  if (!NC.server) return note("This copy has no settings. Download it again from NovelCheck.");
  $("find").onsubmit = function (e) {
    e.preventDefault();
    var q = $("q").value.replace(/^\s+|\s+$/g, "");
    if (q.length > 1) show("search", q);
  };
  var tabs = document.querySelectorAll(".tabs button");
  for (var i = 0; i < tabs.length; i++) tabs[i].onclick = function () { show(this.getAttribute("data-list")); };
  $("report").onclick = report;
  $("prev").onclick = function () { state.page--; rows(); };
  $("next").onclick = function () { state.page++; rows(); };
  show("up-next");
});
