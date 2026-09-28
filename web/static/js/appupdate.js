// Keeping phones on the newest NovelCheck. A home-screen app can stay open
// for days, so whenever it comes back to the screen (at most every few
// minutes) it asks the server which version it has, and reloads if that's
// newer than the one running: before anything is tapped, never mid-typing.
// The version is the build in this file's own address (/v/<build>/js/…);
// the server sends the same build as index.html's ETag.
const running = (import.meta.url.match(/\/v\/([^/]+)\//) || [])[1];
const CHECK_EVERY = 5 * 60 * 1000;

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

export function keepUpToDate() {
  if (!("serviceWorker" in navigator)) return;
  navigator.serviceWorker.register("/sw.js").then((reg) => {
    let checked = Date.now();
    document.addEventListener("visibilitychange", async () => {
      if (document.visibilityState !== "visible" || Date.now() - checked < CHECK_EVERY) return;
      checked = Date.now();
      reg.update().catch(() => {});
      if (await newerOnServer()) location.reload();
    });
  }).catch(() => {});
}
