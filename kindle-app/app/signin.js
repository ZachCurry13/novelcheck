/* NovelCheck Kindle app: first-run sign-in, for a copy installed from KindleForge (no config.js made for an account).
   Asks for NovelCheck's address, the name and password, and a name for this Kindle; NovelCheck answers with the key the app
   signs in with (never the password or sync code is kept), which is written to config.js, and the app reloads. ES5 only. */
function signin() {
  var s = $("signin"), msg = $("si-msg");
  $("lists").hidden = true;
  document.getElementsByTagName("header")[0].hidden = true;
  s.hidden = false;
  function say(t) { msg.textContent = t; }
  $("si-go").onclick = function () {
    var server = $("si-server").value.replace(/\s+/g, "").replace(/\/+$/, "");
    var user = $("si-user").value.replace(/^\s+|\s+$/g, ""), pw = $("si-pass").value;
    if (!server || !user || !pw) return say("Fill in all three.");
    if (!/^https?:\/\//.test(server)) server = "https://" + server;
    say("Signing in…");
    var x = new XMLHttpRequest();
    x.open("POST", server + "/kosync/plugin/app-signin", true);
    x.setRequestHeader("Content-Type", "application/json");
    x.timeout = 20000;
    x.onreadystatechange = function () {
      if (x.readyState !== 4) return;
      var r = {};
      try { r = JSON.parse(x.responseText); } catch (e) {}
      if (x.status === 0) return say("Can't reach " + server + ". Check the address and the Wi-Fi.");
      if (x.status !== 200 || !r.key) return say(r.message || "Could not sign in (" + x.status + ").");
      var cfg = { server: server, user: r.user, key: r.key, device: $("si-device").value.replace(/["\\\/]/g, " ").replace(/^\s+|\s+$/g, "") || "Kindle", version: r.version };
      var text = "// Made by the app's sign-in.\nvar NC = " + JSON.stringify(cfg) + ";\n";
      var out = "/tmp/nc-signin.txt";
      var cmd = "printf '%s' " + sh(text) + " > /mnt/us/extensions/novelcheck-app/config.js && cp /mnt/us/extensions/novelcheck-app/config.js /var/local/mesquite/NovelCheck/config.js; echo $? > " + out + "; echo END >> " + out;
      if (!run("rm -f " + out + "; " + cmd)) return say("This needs KindleForge's helper. Open KindleForge once, then try again.");
      say("Saving…");
      waitFor(out, 20, function (res) {
        if (res === null || res.replace(/\s+/g, "") !== "0") return say("Could not save the sign-in on this Kindle.");
        window.location.reload();
      });
    };
    x.send(JSON.stringify({ username: user, password: pw }));
  };
}
