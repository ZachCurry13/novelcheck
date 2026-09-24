// NovelCheck service worker: caches the app shell for offline launch and
// home-screen installs. API responses are never cached (they are private and
// sent with Cache-Control: no-store).
const CACHE = "novelcheck-shell-v21";
const SHELL = [
  "/",
  "/index.html",
  "/css/app.css",
  "/manifest.json",
  "/icons/icon.svg",
  "/icons/icon-192.png",
  "/icons/icon-512.png",
  "/vendor/Sortable.min.js",
  "/vendor/jszip.min.js",
  "/js/app.js",
  "/js/api.js",
  "/js/ui.js",
  "/js/peppers.js",
  "/js/copy.js",
  "/js/diagnose.js",
  "/js/errors.js",
  "/js/library.js",
  "/js/bookdialog.js",
  "/js/queue.js",
  "/js/scanner.js",
  "/js/importlist.js",
  "/js/bookmeta.js",
  "/js/admin.js",
  "/js/calibrepicker.js",
  "/js/verdictform.js",
  "/js/markdown.js",
  "/js/whatsnew.js",
  "/js/guide.js",
  "/js/updatebanner.js",
  "/js/llmpresets.js",
  "/js/remoteaccess.js",
  "/js/ollamahelper.js",
  "/js/ollamaorder.js",
  "/js/calibreremove.js",
  "/js/calibreserver.js",
  "/js/system.js",
  "/js/usage.js",
  "/js/duplicates.js",
  "/js/charts.js",
  "/js/notifications.js",
  "/js/booknotes.js",
  "/js/users.js",
  "/js/profile.js",
];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

// Network-first for the shell so updates land immediately; cache as fallback.
self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || url.origin !== location.origin || url.pathname.startsWith("/api/")) return;
  event.respondWith(
    fetch(event.request)
      .then((res) => {
        if (res.ok && SHELL.includes(url.pathname)) {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(event.request, copy));
        }
        return res;
      })
      .catch(() => caches.match(event.request).then((r) => r || caches.match("/index.html"))),
  );
});
