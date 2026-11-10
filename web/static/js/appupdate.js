// Keeping phones on the newest NovelCheck. A home-screen app can stay open for days, so it asks the server
// which version it has: when it comes back to the screen (at most every couple of minutes) and now and then
// while it is open. If the server's is newer, a small bar at the bottom says so with a Reload button: nothing
// reloads by itself, so nobody loses what they were typing.
// The version is the build in this file's own address (/v/<build>/js/…); the server sends the same build as
// index.html's ETag.
const running = (import.meta.url.match(/\/v\/([^/]+)\//) || [])[1];
const CHECK_EVERY = 2 * 60 * 1000;

async function newerOnServer() {
  if (!running) return false; // not loaded from a versioned address (development)
  try {
    const res = await fetch("/", { method: "HEAD", cache: "no-store" });
    const build = (res.headers.get("ETag") || "").replace(/"/g, "");
    return res.ok && build !== "" && build !== running;
  } catch {
    return false; // offline: keep going
  }
}

function showBar() {
  if (document.getElementById("update-bar")) return;
  const bar = document.createElement("div");
  bar.id = "update-bar";
  bar.setAttribute("role", "status");
  bar.innerHTML = `<span>A new version of NovelCheck is ready.</span><button type="button" class="btn-primary py-1">Reload</button><button type="button" class="btn-ghost py-1" aria-label="Not now">✕</button>`;
  const [reload, later] = bar.querySelectorAll("button");
  reload.addEventListener("click", () => location.reload());
  later.addEventListener("click", () => bar.remove());
  document.body.append(bar);
}

// showUpdateBar is for testing: it draws the bar at once.
export const showUpdateBar = showBar;

export function keepUpToDate() {
  if (!("serviceWorker" in navigator)) return;
  navigator.serviceWorker.register("/sw.js").then((reg) => {
    let checked = 0;
    const check = async () => {
      if (document.visibilityState !== "visible" || Date.now() - checked < CHECK_EVERY) return;
      checked = Date.now();
      reg.update().catch(() => {});
      if (await newerOnServer()) showBar();
    };
    document.addEventListener("visibilitychange", check);
    setInterval(check, CHECK_EVERY);
    setTimeout(check, 15000); // soon after opening: the phone may have had the app open a long while
  }).catch(() => {});
}
