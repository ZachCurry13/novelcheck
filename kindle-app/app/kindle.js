/* NovelCheck Kindle app: talking to the server and to the Kindle. ES5 only: the Kindle's app runner
   (Mesquite) stops at const, let, arrow functions, fetch and Promise. */
var NC = window.NC || {}; // set by config.js: server, user, key, device

// sh quotes one word for the shell.
function sh(s) {
  return "'" + String(s).replace(/'/g, "'\\''") + "'";
}

// api calls the server with the plugin's sign-in; done(err, data, status). err is a sentence to show.
function api(method, path, body, done) {
  var x = new XMLHttpRequest();
  x.open(method, NC.server + path, true);
  x.setRequestHeader("x-auth-user", NC.user);
  x.setRequestHeader("x-auth-key", NC.key);
  x.setRequestHeader("Accept", "application/json");
  if (body) x.setRequestHeader("Content-Type", "application/json");
  x.timeout = 20000;
  x.onreadystatechange = function () {
    if (x.readyState !== 4) return;
    var data = null;
    try { data = JSON.parse(x.responseText); } catch (e) {}
    if (x.status === 0) return done("No connection. Turn off Airplane Mode and try again.", null, 0);
    if (x.status >= 400) return done((data && data.message) || ("NovelCheck answered " + x.status + "."), data, x.status);
    done(null, data, x.status);
  };
  x.send(body ? JSON.stringify(body) : null);
}

// cover shows a book's cover in img (covers need the sign-in, so they come by XHR).
function cover(img, id) {
  var x = new XMLHttpRequest();
  x.open("GET", NC.server + "/kosync/plugin/cover/" + id + "?size=l", true);
  x.setRequestHeader("x-auth-user", NC.user);
  x.setRequestHeader("x-auth-key", NC.key);
  x.responseType = "blob";
  x.onload = function () {
    if (x.status === 200 && window.URL) img.src = window.URL.createObjectURL(x.response);
    else img.parentNode && img.parentNode.removeChild(img);
  };
  x.onerror = function () { img.parentNode && img.parentNode.removeChild(img); };
  x.send();
}

// run has KindleForge's helper (utild) run a shell command as the Kindle; false when there is no helper.
function run(cmd) {
  var k = window.kindle || (window.top && window.top.kindle);
  if (!k || !k.messaging) return false;
  k.messaging.sendStringMessage("com.kindlemodding.utild", "runCMD", cmd);
  return true;
}

// readFile reads a text file on the Kindle (as KindleForge does: through a hidden frame); "" when missing.
function readFile(path, done) {
  var f = document.createElement("iframe");
  var over = false;
  f.style.display = "none";
  f.src = "file://" + path + "?t=" + new Date().getTime();
  function finish(text) {
    if (over) return;
    over = true;
    if (f.parentNode) f.parentNode.removeChild(f);
    done(text);
  }
  f.onload = function () {
    var text = "";
    try { text = f.contentDocument.body.textContent || ""; } catch (e) {}
    finish(text);
  };
  setTimeout(function () { finish(""); }, 3000);
  document.body.appendChild(f);
}

// waitFor reads path every 2 seconds until it holds a line ending in "END", up to limit seconds; done(text or null).
function waitFor(path, limit, done) {
  var start = new Date().getTime();
  (function again() {
    readFile(path, function (text) {
      var at = text.lastIndexOf("END");
      if (at >= 0) return done(text.slice(0, at));
      if (new Date().getTime() - start > limit * 1000) return done(null);
      setTimeout(again, 2000);
    });
  })();
}

// chrome sets the Kindle's top bar: the title and a close button.
function chrome() {
  try {
    window.kindle.messaging.sendMessage("com.lab126.chromebar", "configureChrome", {
      appId: "com.novelcheck.kindle",
      topNavBar: { template: "title", title: "NovelCheck", buttons: [{ id: "KPP_CLOSE", state: "enabled", handling: "system" }] }
    });
  } catch (e) {}
}
