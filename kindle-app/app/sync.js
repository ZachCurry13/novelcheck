/* NovelCheck Kindle app: Sync now. In one go: the remote's tasks for this Kindle (removing Amazon store books a
   parent picked), the library report (store books are rated by NovelCheck; Amazon's reader's progress and finished
   marks feed the Nightstand and Hardcover), highlights from "My Clippings.txt" and the words looked up in the
   dictionary. On opening it syncs by itself at most every 12 hours; nothing runs once the app is closed.
   With the KOReader plugin on the same Kindle, the books only KOReader reads are left to the plugin. ES5 only. */
var SYNC_EVERY = 12 * 3600 * 1000;

function stored(k) { try { return localStorage.getItem(k); } catch (e) { return null; } }
function keep(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }
function day(unix) { return new Date(unix * 1000).toISOString().slice(0, 10); }
function stamp(d) { return d.toISOString().slice(0, 19).replace("T", " "); }

// syncNow: force for the button; otherwise only when the last sync is over 12 hours old.
function syncNow(force) {
  if (!force && new Date().getTime() - (+stored("nc-synced") || 0) < SYNC_EVERY) return;
  var said = [];
  doTasks(said, function () {
    report(said, function (ok, left) {
      sendHighlights(said, function () {
        sendWords(said, function () {
          if (ok) { keep("nc-synced", String(new Date().getTime())); got = {}; keep("nc-got", "{}"); } // the Kindle's own list is the truth now
          paintSynced();
          note(said.join(" "));
          logLine("sync" + (force ? " (button)" : "") + ": " + said.join(" "));
          // a sync that went fine is reported once a day; one that did not, at once
          if (!ok || new Date().getTime() - (+stored("nc-log-sent") || 0) > 86400000) { keep("nc-log-sent", String(new Date().getTime())); sendLog("after a sync"); }
          if (left && left.length) showLeft(left); // books that left the Kindle: keep or remove them
          if (!force) autoBackup(); // the automatic sync also sends new books to NovelCheck, at most once a day
        });
      });
    });
  });
}

// doTasks does what was asked from NovelCheck's Kindle remote for this Kindle (tools.js runs each kind), then the sync goes on.
function doTasks(said, done) {
  api("GET", "/kosync/plugin/tasks?device=" + encodeURIComponent(NC.device), null, function (err, data) {
    var tasks = [];
    var all = (!err && data && data.tasks) || [];
    for (var i = 0; i < all.length; i++) if (APP_TASKS[all[i].kind]) tasks.push(all[i]);
    (function next() {
      var t = tasks.shift();
      if (!t) return done();
      runTask(t, said, next);
    })();
  });
}

// finishTask tells NovelCheck how a task went.
function finishTask(t, status, note, next) {
  api("POST", "/kosync/plugin/tasks/result", { id: t.id, status: status, note: note }, next);
}

// removeStore removes the Amazon store books a parent picked in NovelCheck's Kindle remote.
function removeStore(t, said, next) {
  var places = {}, rm = [], missing = 0;
  try { places = JSON.parse(stored("nc-places")) || {}; } catch (e) {}
  var names = (t.args && t.args.filenames) || [];
  for (var j = 0; j < names.length; j++) {
    var p = places[names[j]];
    if (p && p.indexOf("/mnt/us/documents/") === 0 && p.indexOf("..") < 0) rm.push("rm -f " + sh(p)); else missing++;
  }
  var out = "/tmp/nc-task.txt";
  if (!rm.length || !run("rm -f " + out + "; " + rm.join("; ") + "; lipc-set-prop com.lab126.scanner doFullScan 1; echo ok > " + out + "; echo END >> " + out)) {
    return finishTask(t, "skipped", "None of those books were found on the Kindle.", next);
  }
  waitFor(out, 60, function (text) {
    var ok = text && text.indexOf("ok") >= 0;
    if (ok) said.push("Removed " + rm.length + " book" + (rm.length === 1 ? "" : "s") + " (still in your Amazon library).");
    finishTask(t, ok ? "done" : "failed", ok ? "Removed " + rm.length + (missing ? "; " + missing + " not found" : "") : "The Kindle couldn't remove them.", next);
  });
}

// report reads a copy of the Kindle's library database (the Kindle keeps the real one open) and sends it.
function report(said, done) {
  var out = "/tmp/nc-books.txt", snap = "/tmp/nc_cc_snap.db";
  var cmd = "rm -f " + out + " " + snap + "*; [ -d /mnt/us/koreader/plugins/novelcheck.koplugin ] && echo PLUGIN > " + out + "; " +
    "cp /var/local/cc.db " + snap + " 2>> " + out + "; [ -e /var/local/cc.db-wal ] && cp /var/local/cc.db-wal " + snap + "-wal 2>> " + out + "; " +
    "sqlite3 -separator '\t' " + snap + " \"SELECT p_cdeKey, p_titles_0_nominal, COALESCE(p_location,''), COALESCE(p_percentFinished,0), p_cdeType, " +
    "COALESCE(p_readState,0), COALESCE(p_lastAccess,0), COALESCE(j_credits,'') FROM Entries WHERE p_type='Entry:Item' AND p_cdeType IN ('EBOK','PDOC')\" >> " + out + " 2>> " + out +
    "; rm -f " + snap + "*; echo END >> " + out;
  if (!run(cmd)) { said.push("Syncing needs KindleForge's helper. Open KindleForge once, then try again."); return done(false); }
  note("Reading your Kindle library…");
  waitFor(out, 150, function (text) {
    if (text == null) { // no answer in time: keep what the file says (or that it is empty) in the log, so the cause shows in the bug link
      return readFile(out, function (t) {
        logLine("library read: no answer in 150 s; the file holds: " + (t ? t.replace(/\s+/g, " ").slice(0, 300) : "nothing"));
        said.push("Couldn't read the Kindle library.");
        done(false);
      });
    }
    var bad = [], rows = text.split("\n");
    for (var q = 0; q < rows.length; q++) if (rows[q] && rows[q].indexOf("\t") < 0 && rows[q] !== "PLUGIN") bad.push(rows[q]);
    if (bad.length) logLine("library read: " + bad.join(" ").slice(0, 300)); // an error from the Kindle's own tools
    var lib = parseLibrary(text);
    keep("nc-places", JSON.stringify(lib.places));
    note("Sending " + lib.books.length + " books…");
    api("POST", "/kosync/plugin/device", { device: NC.device, plugin_version: "app " + NC.version, books: lib.books }, function (err, r) {
      if (err) { said.push(err); return done(false); }
      said.push("Reported " + r.books + " books" + (lib.plugin ? " (KOReader's are the plugin's)" : "") + ". " +
        (r.matched === 1 ? "1 is" : r.matched + " are") + " in NovelCheck; new store books are being rated.");
      done(true, r.left);
    });
  });
}

// parseLibrary turns the database lines (key, title, file, percent, type, read state, last opened, credits) into the
// plugin's device report. Store books carry their Amazon ID as "asin:…"; places remembers each file's full path.
function parseLibrary(text) {
  var lines = text.split("\n"), books = [], places = {}, plugin = /^PLUGIN/.test(text);
  for (var i = 0; i < lines.length; i++) {
    var f = lines[i].split("\t");
    if (f.length < 8 || !f[1]) continue;
    // a file that isn't a book (KUAL's launcher .azw2, a .sh scriptlet, a dictionary…) is not reported
    if (f[2] && !/\.(azw|azw3|kfx|kfx-zip|mobi|prc|pdf|epub|kepub|txt|docx?|rtf|cbz|cbr)$/i.test(f[2])) continue;
    var pct = +f[3] || 0, at = +f[6] || 0;
    if (plugin && f[4] === "PDOC" && pct === 0) continue; // a book KOReader reads: the plugin reports it
    if (at > 1e11) at = Math.round(at / 1000); // milliseconds on some firmware
    var author = "";
    try { author = JSON.parse(f.slice(7).join("\t"))[0].name.display || ""; } catch (e) {}
    var file = f[2] ? f[2].split("/").pop() : f[1].replace(/[\/\\]/g, " ") + " [" + f[0] + "]";
    var finished = pct >= 100 || f[5] === "2";
    var b = { filename: file, title: f[1], author: author, format: f[2] ? (file.split(".").pop() || "").toLowerCase() : "cloud",
      percent: Math.min(1, pct / 100), opened_at: at, finished: finished, finished_on: finished && at ? day(at) : "" };
    if (f[4] === "EBOK" && f[0]) b.md5 = "asin:" + f[0];
    if (f[2]) places[file] = f[2];
    books.push(b);
  }
  return { books: books, places: places, plugin: plugin };
}

// sendHighlights sends what was highlighted and noted in Amazon's reader ("My Clippings.txt"), when it changed.
function sendHighlights(said, done) {
  readFile("/mnt/us/documents/My%20Clippings.txt", function (text) {
    var books = parseClippings(text || ""), n = 0;
    for (var i = 0; i < books.length; i++) n += books[i].highlights.length;
    if (!n || String(n) === stored("nc-hl-count")) return done();
    api("POST", "/kosync/plugin/highlights", { device: NC.device, books: books.slice(0, 500) }, function (err, r) {
      if (!err) {
        keep("nc-hl-count", String(n));
        if (r.added) said.push(r.added + " new highlight" + (r.added === 1 ? "" : "s") + ".");
      }
      done();
    });
  });
}

var MONTHS = { January: 1, February: 2, March: 3, April: 4, May: 5, June: 6, July: 7, August: 8, September: 9, October: 10, November: 11, December: 12 };
function two(n) { return (n < 10 ? "0" : "") + n; }

// parseClippings reads "My Clippings.txt": entries split by "==========", each "Title (Author)", a line like
// "- Your Highlight on page 3 | Location 40-42 | Added on Tuesday, November 4, 2025 9:12:03 PM", then the text.
// A note joins the highlight it was made on.
function parseClippings(text) {
  var parts = text.replace(/﻿/g, "").split(/\r?\n==========\r?\n?/), byKey = {}, books = [];
  for (var i = 0; i < parts.length; i++) {
    var lines = parts[i].replace(/^\s+/, "").split(/\r?\n/);
    if (lines.length < 3 || !/^- /.test(lines[1]) || /Bookmark/.test(lines[1])) continue;
    var head = lines[0].match(/^(.*) \(([^()]*)\)\s*$/), title = head ? head[1] : lines[0], author = head ? head[2] : "";
    var body = lines.slice(2).join("\n").replace(/^\s+|\s+$/g, ""), meta = lines[1];
    var loc = meta.match(/Location (\d+)(?:-(\d+))?/), page = meta.match(/page (\S+)/), when = meta.match(/Added on \w+, (\w+) (\d+), (\d+) (\d+):(\d+):(\d+) (AM|PM)/);
    var at = "";
    if (when && MONTHS[when[1]]) {
      var h = (+when[4] % 12) + (when[7] === "PM" ? 12 : 0);
      at = when[3] + "-" + two(MONTHS[when[1]]) + "-" + two(+when[2]) + " " + two(h) + ":" + when[5] + ":" + when[6];
    }
    var key = title + "|" + author, book = byKey[key];
    if (!book) { book = byKey[key] = { filename: title, title: title, author: author, highlights: [] }; books.push(book); }
    var end = loc ? +(loc[2] || loc[1]) : -1;
    if (/Note/.test(meta)) {
      var last = book.highlights[book.highlights.length - 1];
      if (last && last.end === end && !last.note) { last.note = body; continue; }
      book.highlights.push({ text: "", note: body, page: page ? page[1] : "", datetime: at, end: end });
    } else if (body) {
      book.highlights.push({ text: body, note: "", page: page ? page[1] : "", datetime: at, end: end });
    }
  }
  return books;
}

// sendWords sends the words looked up in the dictionary (Vocabulary Builder), when there are new ones.
function sendWords(said, done) {
  var out = "/tmp/nc-words.txt", snap = "/tmp/nc_vocab.db";
  var cmd = "rm -f " + out + " " + snap + "*; if [ -e /mnt/us/system/vocabulary/vocab.db ]; then cp /mnt/us/system/vocabulary/vocab.db " + snap + "; " +
    "sqlite3 -separator '\t' " + snap + " \"SELECT b.title, b.authors, w.word, l.timestamp, l.usage FROM LOOKUPS l JOIN WORDS w ON w.id = l.word_key " +
    "JOIN BOOK_INFO b ON b.id = l.book_key\" > " + out + "; rm -f " + snap + "*; fi; echo END >> " + out;
  if (!run(cmd)) return done();
  waitFor(out, 60, function (text) {
    var lines = (text || "").split("\n"), byKey = {}, books = [], n = 0;
    for (var i = 0; i < lines.length; i++) {
      var f = lines[i].split("\t");
      if (f.length < 5 || !f[0] || !f[2]) continue;
      var key = f[0] + "|" + f[1];
      if (!byKey[key]) { byKey[key] = { title: f[0], author: f[1], words: [] }; books.push(byKey[key]); }
      byKey[key].words.push({ word: f[2], at: +f[3] ? stamp(new Date(+f[3])) : "", usage: f.slice(4).join(" ") });
      n++;
    }
    if (!n || String(n) === stored("nc-words-count")) return done();
    api("POST", "/kosync/plugin/kindle-words", { books: books }, function (err, r) {
      if (!err) {
        keep("nc-words-count", String(n));
        if (r.added) said.push(r.added + " looked-up word" + (r.added === 1 ? "" : "s") + ".");
      }
      done();
    });
  });
}
