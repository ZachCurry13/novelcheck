/* NovelCheck Kindle app: updating itself. On start it asks NovelCheck for the newest version; when there is one, it
   has utild fetch the new files into both copies of the app (the one in extensions and the one the Kindle runs) and
   reloads, by itself. If that fails, the Update button stays to try again. config.js (the account) is never replaced.
   ES5 only. */

// newer says whether version a is newer than b ("1.10.0" > "1.9.0").
function newer(a, b) {
  var x = String(a).split("."), y = String(b || "0").split(".");
  for (var i = 0; i < 3; i++) {
    var p = +x[i] || 0, q = +y[i] || 0;
    if (p !== q) return p > q;
  }
  return false;
}

function checkUpdate() {
  api("GET", "/kosync/plugin/kindle-app", null, function (err, v) {
    if (err || !v || !newer(v.version, NC.version)) return;
    var b = $("update");
    b.textContent = "Update to " + v.version;
    b.hidden = false;
    b.onclick = function () { update(v, b); };
    var tried = "";
    try { tried = localStorage.getItem("nc-auto-update") || ""; } catch (e) {}
    if (tried === v.version) return; // tried by itself once already: the button is there to try again
    try { localStorage.setItem("nc-auto-update", v.version); } catch (e) {}
    logLine("updating by itself from " + NC.version + " to " + v.version);
    update(v, b);
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
  var cmd = "rm -f " + out + "; ok=1; for f in " + list + "; do curl -sfL -H " + sh("x-auth-user: " + NC.user) +
    " -H " + sh("x-auth-key: " + NC.key) + " -o " + src + "/$f.new " + sh(NC.server + "/kosync/plugin/kindle-app/") + "\"$f\" || ok=0; done; " +
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
  });
}
