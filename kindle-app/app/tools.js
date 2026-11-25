/* NovelCheck Kindle app: this Kindle's tools, from the Menu and from NovelCheck's Kindle remote (a task is done when the
   app opens or Sync now is pressed): back up the books to NovelCheck, restore them, delete the Kindle's crash logs, send books
   and the Nightstand, a self-test. Books move with the Kindle's own curl through KindleForge's helper. ES5 only. */
var APP_TASKS = { remove_store: 1, download_books: 1, download_nightstand: 1, crashlogs: 1, backup: 1, selftest: 1 }; // report, highlights and stats are the sync itself: left for the plugin to answer
var MAX_UPLOAD = 95 * 1024 * 1024; // the tunnel refuses bigger bodies
var AUTH = " -H " + sh("x-auth-user: " + NC.user) + " -H " + sh("x-auth-key: " + NC.key);
var STORE_NAME = /(^|[_ -])B[0-9A-Z]{9}([_. -]|$)|_EBOK|_PDOC|_SAMPLE/i; // a file named by Amazon's code instead of the title
var toolBusy = false;

// shellOut runs cmd (which must write its answer to out, ending with END) and gives the text, or null.
function shellOut(cmd, out, limit, done) {
  if (!run("rm -f " + out + "; " + cmd + "; echo END >> " + out)) return done(null);
  waitFor(out, limit, done);
}

// backupKindle sends the Kindle's books to NovelCheck that it doesn't have yet, one at a time. Amazon's own books (protected) are
// listed by the server as not wanted. done(sent, wanted, tooBig, failed).
function backupKindle(done) {
  var list = "/tmp/nc-bk-list.txt";
  note("Listing the books on this Kindle…");
  shellOut("find /mnt/us/documents -type f 2>/dev/null | while IFS= read -r f; do printf '%s\\t%s\\n' \"$(wc -c < \"$f\")\" \"$f\"; done > " + list, list, 120, function (text) {
    if (text == null) return done(-1, 0, 0, 0);
    var files = [], lines = text.split("\n");
    for (var i = 0; i < lines.length; i++) {
      var f = lines[i].split("\t");
      // Amazon's dictionaries, and books named by Amazon's code (B0…_EBOK.azw: bought books, still in the Amazon library), are not backed up
      if (f.length === 2 && +f[0] >= 0 && f[1].indexOf("/mnt/us/") === 0 && f[1].indexOf("..") < 0 && !/\/dictionaries\//i.test(f[1]) && !STORE_NAME.test(f[1].split("/").pop())) files.push({ path: f[1], size: +f[0] });
    }
    note("Asking NovelCheck which of " + files.length + " files it needs…");
    api("POST", "/kosync/plugin/kindle/check", { device: NC.device, files: files }, function (err, r) {
      if (err || !r) return done(-1, 0, 0, 0);
      var send = r.send || [], sizes = {}, sent = 0, big = 0, failed = 0, total = send.length;
      for (var k = 0; k < files.length; k++) sizes[files[k].path] = files[k].size;
      (function next() {
        var path = send.shift();
        if (!path || failed >= 5) return done(sent, total, big, failed);
        if (sizes[path] > MAX_UPLOAD) { big++; return next(); }
        note("Backing up " + (total - send.length) + " of " + total + ": " + path.split("/").pop() + "…");
        var out = "/tmp/nc-bk-one.txt";
        shellOut("if curl -sf -m 900 -T " + sh(path) + AUTH + " " + sh(NC.server + "/kosync/plugin/kindle/file?device=" + encodeURIComponent(NC.device) + "&path=" + encodeURIComponent(path)) +
          " > /dev/null; then echo ok > " + out + "; else echo failed > " + out + "; fi", out, 920, function (t) {
          if (t && t.indexOf("ok") >= 0) { sent++; failed = 0; } else failed++;
          next();
        });
      })();
    });
  });
}

// paintBackup says when this Kindle's books were last sent to NovelCheck (by the button or by itself).
function paintBackup() {
  var t = +stored("nc-last-bk") || 0;
  $("lastbk").textContent = t ? "Books last backed up: " + new Date(t).toISOString().slice(0, 16).replace("T", " ") + " UTC" : "Books not backed up yet.";
}

// autoBackup is backupKindle by itself, at most every 24 hours, after an automatic sync (books NovelCheck already has are skipped).
function autoBackup() {
  if (toolBusy || new Date().getTime() - (+stored("nc-auto-bk") || 0) < 24 * 3600 * 1000) return;
  toolBusy = true;
  backupKindle(function (sent, wanted) {
    toolBusy = false;
    if (sent < 0) return logLine("automatic backup could not run");
    keep("nc-auto-bk", String(new Date().getTime()));
    keep("nc-last-bk", String(new Date().getTime()));
    paintBackup();
    logLine("automatic backup: " + sent + " of " + wanted + " new books sent");
    if (sent) note("Sent " + sent + " new book" + (sent === 1 ? "" : "s") + " to NovelCheck.");
  });
}

// restoreKindle puts back the books of this Kindle's backup that are not on it (nothing is replaced).
function restoreKindle(done) {
  note("Looking at the backup…");
  api("GET", "/kosync/plugin/restore?device=" + encodeURIComponent(NC.device), null, function (err, r) {
    if (err || !r) return done(-1, 0);
    var todo = [], all = r.files || [];
    for (var i = 0; i < all.length; i++) {
      var p = all[i].path;
      if (/^documents\/.+\.(azw3?|kfx|mobi|prc|pdf|epub|kepub|txt)$/i.test(p) && p.indexOf("..") < 0) todo.push(p);
    }
    var total = todo.length, put = 0;
    (function next() {
      var rel = todo.shift();
      if (!rel) {
        if (put) run("lipc-set-prop com.lab126.scanner doFullScan 1");
        return done(put, total);
      }
      note("Checking " + (total - todo.length) + " of " + total + "…");
      var dest = "/mnt/us/" + rel, out = "/tmp/nc-rs-one.txt";
      shellOut("if [ -e " + sh(dest) + " ]; then echo have > " + out + "; else mkdir -p \"$(dirname " + sh(dest) + ")\"; if curl -sf -m 900 " + AUTH + " -o " + sh(dest + ".part") + " " +
        sh(NC.server + "/kosync/plugin/restore/file?device=" + encodeURIComponent(NC.device) + "&path=" + encodeURIComponent(rel)) + "; then mv " + sh(dest + ".part") + " " + sh(dest) +
        "; echo ok > " + out + "; else rm -f " + sh(dest + ".part") + "; echo failed > " + out + "; fi; fi", out, 920, function (t) {
        if (t && t.indexOf("ok") >= 0) { put++; note("Restored " + put + ": " + rel.split("/").pop()); }
        next();
      });
    })();
  });
}

// crashLogs deletes the Kindle's crash reports in its top folder (names with _crash_); done(count) or -1.
function crashLogs(done) {
  var out = "/tmp/nc-crash.txt";
  shellOut("n=$(ls /mnt/us/*_crash_* 2>/dev/null | wc -l); rm -f /mnt/us/*_crash_*; echo $n > " + out, out, 30, function (t) {
    done(t == null ? -1 : parseInt(t, 10) || 0);
  });
}

// downloadBooks downloads books one after another (the ones already on the Kindle count as done); done(downloaded, total).
function downloadBooks(books, done) {
  var todo = books.slice(), got0 = 0, total = todo.length;
  (function nextBook() {
    var b = todo.shift();
    if (!b) { afterDownload = null; return done(got0, total); }
    if (onKindle(b)) { got0++; return nextBook(); }
    afterDownload = function () { if (got[b.id]) got0++; nextBook(); };
    download(b, { disabled: false, textContent: "" });
  })();
}

// runTask does one task from NovelCheck's Kindle remote and reports how it went.
function runTask(t, said, next) {
  var kind = t.kind;
  if (kind === "remove_store") return removeStore(t, said, next);
  if (kind === "selftest") {
    var started = run("sh /mnt/us/extensions/novelcheck-app/check.sh");
    return finishTask(t, started ? "done" : "failed", started ? "The check was started; its result goes to NovelCheck's bug log." : "The check needs KindleForge's helper.", next);
  }
  if (kind === "crashlogs") return crashLogs(function (n) {
    if (n >= 0) said.push(n ? "Deleted " + n + " crash log" + (n === 1 ? "" : "s") + "." : "");
    finishTask(t, n >= 0 ? "done" : "failed", n < 0 ? "The Kindle couldn't do it." : n ? "Deleted " + n + " crash logs." : "There were no crash logs.", next);
  });
  if (kind === "backup") return backupKindle(function (sent, wanted, big, failed) {
    if (sent >= 0) said.push("Backed up " + sent + " of " + wanted + " books.");
    finishTask(t, sent < 0 || failed >= 5 ? "failed" : "done", sent < 0 ? "The Kindle couldn't list its books or reach NovelCheck." : "Backed up " + sent + " of " + wanted + " books" + (big ? "; " + big + " too big to send" : "") + (failed ? "; " + failed + " failed" : "") + ".", next);
  });
  if (kind === "download_books") return downloadBooks((t.args && t.args.books) || [], function (n, total) {
    said.push("Downloaded " + n + " of " + total + " books you sent.");
    finishTask(t, n === total ? "done" : "failed", "Downloaded " + n + " of " + total + ".", next);
  });
  if (kind === "download_nightstand") return api("GET", "/kosync/plugin/up-next?device=" + encodeURIComponent(NC.device), null, function (err, data) {
    var books = (!err && data && data.books) || [];
    downloadBooks(books, function (n, total) {
      said.push("Downloaded " + n + " of " + total + " Nightstand books.");
      finishTask(t, err ? "failed" : "done", err ? "NovelCheck couldn't be reached." : "Downloaded " + n + " of " + total + ".", next);
    });
  });
  finishTask(t, "skipped", "The Kindle app doesn't do that one.", next);
}

// The Menu's "This Kindle" buttons.
function bindTools() {
  var guard = function (id, fn) {
    $(id).onclick = function () {
      if (toolBusy) return note("Another job is still running. Wait for it to finish.");
      toolBusy = true;
      var btn = this;
      btn.disabled = true;
      fn(function () { toolBusy = false; btn.disabled = false; });
    };
  };
  guard("t-backup", function (end) {
    backupKindle(function (sent, wanted, big, failed) {
      note(sent < 0 ? "The backup couldn't start. Check Wi-Fi, and that KindleForge's helper is there." :
        "Backed up " + sent + " of " + wanted + " books that NovelCheck needed" + (big ? "; " + big + " were too big to send" : "") + (failed ? "; " + failed + " failed" : "") + ".");
      logLine("backup: " + sent + " of " + wanted);
      if (sent >= 0) { keep("nc-last-bk", String(new Date().getTime())); paintBackup(); }
      end();
    });
  });
  guard("t-restore", function (end) {
    restoreKindle(function (put, total) {
      note(put < 0 ? "Couldn't reach NovelCheck." : total === 0 ? "NovelCheck has no backup of this Kindle yet." : put ? "Restored " + put + " book" + (put === 1 ? "" : "s") + ". They show in the library in a moment." : "Nothing to restore: every backed-up book is already here.");
      end();
    });
  });
  $("t-hidden").onclick = showHidden;
  paintBackup();
  var dest = function () { $("t-dest").textContent = stored("nc-dest") === "koreader" ? "Downloads go to: KOReader (tap to change)" : "Downloads go to: Kindle library (tap to change)"; };
  dest();
  $("t-dest").onclick = function () {
    keep("nc-dest", stored("nc-dest") === "koreader" ? "kindle" : "koreader");
    dest();
    note(stored("nc-dest") === "koreader" ? "New downloads are EPUBs in KOReader's NovelCheck folder." : "New downloads are Kindle books in the Kindle library.");
  };
  guard("t-crash", function (end) {
    crashLogs(function (n) {
      note(n < 0 ? "The Kindle couldn't do that. Open KindleForge once, then try again." : n ? "Deleted " + n + " crash log" + (n === 1 ? "" : "s") + "." : "There are no crash logs on this Kindle.");
      end();
    });
  });
}

// showHidden lists the books this person hid with "Don't show this book", each with "Show again".
function showHidden() {
  note("Looking…");
  api("GET", "/kosync/plugin/hidden", null, function (err, data) {
    if (err) return note(err);
    var books = (data && data.books) || [];
    note(books.length ? "" : "You haven't hidden any books.");
    showLists(false);
    backTo = function () { backTo = null; openMenu(); };
    paintNav("Books I hid");
    window.scrollTo(0, 0);
    var m = $("main");
    m.innerHTML = "";
    $("pages").hidden = true;
    for (var i = 0; i < books.length; i++) {
      (function (b) {
        var r = el("div", "row");
        var show = el("button", "act", "Show again");
        show.onclick = function () {
          show.disabled = true;
          api("POST", "/kosync/plugin/unhide", { book_id: b.id }, function (e2) {
            if (e2) { show.disabled = false; return note(e2); }
            m.removeChild(r);
            note("“" + b.title + "” will show in your lists again.");
          });
        };
        r.appendChild(show);
        var what = el("div", "what");
        what.appendChild(el("div", "title", b.title));
        what.appendChild(el("div", "by", b.author));
        r.appendChild(what);
        m.appendChild(r);
      })(books[i]);
    }
  });
}
