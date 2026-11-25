/* NovelCheck Kindle app: updating itself. On start it asks NovelCheck for its newest version and, with the Kindle's
   own curl, GitHub's releases for theirs (so the app can be newer than the person's server: GitHub's is taken only
   when the server is at least the version the release says it needs). The newer one is fetched with utild into both
   copies of the app (the one in extensions and the one the Kindle runs), and the app reloads, by itself. If that
   fails, the Update button stays to try again. config.js (the account) is never replaced. ES5 only. */
var GH = "https://github.com/ZachCurry13/novelcheck/releases/";

// newer says whether version a is newer than b ("1.10.0" > "1.9.0").
function newer(a, b) {
  var x = String(a).split("."), y = String(b || "0").split(".");
  for (var i = 0; i < 3; i++) {
    var p = +x[i] || 0, q = +y[i] || 0;
    if (p !== q) return p > q;
  }
  return false;
}

// github reads the newest release's app manifest with the Kindle's curl (a page can't read GitHub across origins);
// done(null) when GitHub can't be reached or the manifest isn't what it should be.
function github(done) {
  var out = "/tmp/nc-gh.txt";
  if (!run("rm -f " + out + "; curl -sfL -m 20 " + sh(GH + "latest/download/kindle-app.json") + " >> " + out + " 2>/dev/null; echo END >> " + out)) return done(null);
  waitFor(out, 30, function (text) {
    var g = null;
    try { g = JSON.parse(text); } catch (e) {}
    var ok = g && /^[0-9.]+$/.test(g.version) && /^[0-9.]+$/.test(g.min_server) && g.prefix === "kindle-app-" && g.files && g.files.length &&
      typeof g.base === "string" && g.base.indexOf(GH + "download/") === 0 && /^[A-Za-z0-9._\/:-]+$/.test(g.base);
    done(ok ? g : null);
  });
}

function checkUpdate() {
  api("GET", "/kosync/plugin/kindle-app?v=" + encodeURIComponent(NC.version || "") + "&device=" + encodeURIComponent(NC.device || ""), null, function (err, v) {
    var ver = $("ver"), base = "v" + (NC.version || "?");
    github(function (g) {
      var best = null;
      if (!err && v && newer(v.version, NC.version)) best = { version: v.version, files: v.files, from: "server" };
      // GitHub's is taken when it is newer than both, and the server is new enough for it ("dev" is a build from source)
      if (g && newer(g.version, NC.version) && (!best || newer(g.version, best.version)) && !err && v && v.server &&
          (v.server === "dev" || !newer(g.min_server, v.server))) {
        best = { version: g.version, files: g.files, from: "github", base: g.base, prefix: g.prefix };
      }
      if (!best) {
        ver.textContent = base + (err ? " · couldn't check for updates" : " · up to date");
        return;
      }
      ver.textContent = base + " · v" + best.version + " is ready" + (best.from === "github" ? " (from GitHub)" : "");
      var b = $("update");
      b.textContent = "Update to " + best.version;
      b.hidden = false;
      b.onclick = function () { update(best, b); };
      var tried = "";
      try { tried = localStorage.getItem("nc-auto-update") || ""; } catch (e) {}
      if (tried === best.version) return; // tried by itself once already: the button is there to try again
      try { localStorage.setItem("nc-auto-update", best.version); } catch (e) {}
      logLine("updating by itself from " + NC.version + " to " + best.version + " (" + best.from + ")");
      update(best, b);
    });
  });
}

function update(v, btn) {
  var names = [];
  for (var i = 0; i < (v.files || []).length; i++) {
    if (/^[A-Za-z0-9._-]+$/.test(v.files[i]) && v.files[i] !== "config.js") names.push(v.files[i]); // they go into a shell command
  }
  if (!names.length || !/^[0-9.]+$/.test(v.version)) return note("That update can't be installed.");
  var src = "/mnt/us/extensions/novelcheck-app", dst = "/var/local/mesquite/NovelCheck", out = "/tmp/nc-update.txt";
  var list = names.join(" ");
  // from GitHub: the release's own address, no sign-in; from the server: the account's sign-in
  var from = v.from === "github" ? "-o " + src + "/$f.new " + sh(v.base + v.prefix) + "\"$f\""
    : "-H " + sh("x-auth-user: " + NC.user) + " -H " + sh("x-auth-key: " + NC.key) + " -o " + src + "/$f.new " + sh(NC.server + "/kosync/plugin/kindle-app/") + "\"$f\"";
  var cmd = "rm -f " + out + "; ok=1; for f in " + list + "; do curl -sfL " + from + " || ok=0; done; " +
    "if [ $ok = 1 ]; then for f in " + list + "; do mv " + src + "/$f.new " + src + "/$f; cp " + src + "/$f " + dst + "/$f; done; " +
    "sed -i 's/\"version\":\"[^\"]*\"/\"version\":\"" + v.version + "\"/' " + src + "/config.js " + dst + "/config.js; echo ok > " + out +
    "; else rm -f " + src + "/*.new; echo failed > " + out + "; fi; echo END >> " + out;
  if (!run(cmd)) return note("Updating needs KindleForge's helper. Open KindleForge once, then try again.");
  btn.disabled = true;
  btn.textContent = "Updating…";
  waitFor(out, 120, function (text) {
    if (text && text.indexOf("ok") >= 0) {
      note("Updated. Restarting…");
      return setTimeout(function () { window.location.reload(); }, 1000);
    }
    btn.disabled = false;
    btn.textContent = "Update to " + v.version;
    note("The update failed. Check Wi-Fi and try again.");
    logLine("update to " + v.version + " failed");
    sendLog("an update failed");
  });
}
