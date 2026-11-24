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
function spice(level) { return level == null ? "Not rated yet" : "Spice level " + level + " (" + (level >= 4 ? "Open Door" : level >= 2 ? "Closed Door" : "No Spice") + ")"; }
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
    note(state.books.length ? "" : list === "up-next" ? "Your Nightstand is empty." : list === "kindle-mine" ? "Nothing yet: tap Sync now." : "No books found.");
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
  if (state.list === "kindle-mine") { // the books on this Kindle: their rating, and where they are
    what.appendChild(el("div", "meta", (b.status === "pending" || b.status === "processing" ? "Being rated…" : spice(b.level) + (b.old_rules ? " (older rating)" : "")) +
      (b.format === "cloud" ? " · In your Amazon library" : b.percent > 0 ? " · " + Math.round(b.percent * 100) + "% read" : "")));
    if (b.id) what.onclick = function () { details(b.id); };
    r.appendChild(what);
    return r;
  }
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

window.addEventListener("resize", function () { applyScale(); });
if (window.kindle && window.kindle.appmgr) window.kindle.appmgr.ongo = chrome; // the Kindle shows the app: set its top bar

document.addEventListener("DOMContentLoaded", function () {
  applyScale();
  chrome();
  if (!NC.server) return note("This copy has no settings. Download it again from NovelCheck.");
  if (NC.device === "Kindle") NC.device = "Kindle (Amazon reader)"; // its own name, so the plugin's report on the same Kindle is never replaced
  $("find").onsubmit = function (e) {
    e.preventDefault();
    var q = $("q").value.replace(/^\s+|\s+$/g, "");
    if (q.length > 1) show("search", q);
  };
  var tabs = document.querySelectorAll(".tabs button");
  for (var i = 0; i < tabs.length; i++) tabs[i].onclick = function () { show(this.getAttribute("data-list")); };
  $("report").onclick = function () { note("Syncing…"); syncNow(true); };
  $("prev").onclick = function () { state.page--; rows(); };
  $("next").onclick = function () { state.page++; rows(); };
  show("up-next");
  checkUpdate();
  syncNow(false); // by itself at most every 12 hours
});
