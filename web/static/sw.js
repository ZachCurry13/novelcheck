// NovelCheck service worker: caches the app shell for offline launch and
// home-screen installs. API responses are never cached (they are private and
// sent with Cache-Control: no-store).
const CACHE = "novelcheck-shell-v31";
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
  "/js/modules.js",
  "/js/peppers.js",
  "/js/copy.js",
  "/js/diagnose.js",
  "/js/errors.js",
  "/js/deletions.js",
  "/js/library.js",
  "/js/bookdialog.js",
  "/js/queue.js",
  "/js/scanner.js",
  "/js/importlist.js",
  "/js/importfile.js",
  "/js/importguides.js",
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
  "/js/push.js",
  "/js/customflags.js",
  "/js/check.js",
  "/js/deepscan.js",
  "/js/deepscanadmin.js",
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

// Phone notifications (see internal/push): show what the server sent.
self.addEventListener("push", (event) => {
  let msg = {};
  try {
    msg = event.data ? event.data.json() : {};
  } catch {
    msg = { body: event.data ? event.data.text() : "" };
  }
  event.waitUntil(self.registration.showNotification(msg.title || "NovelCheck", {
    body: msg.body || "",
    icon: "/icons/icon-192.png",
    badge: "/icons/icon-192.png",
    tag: msg.tag || undefined,
    data: { url: msg.url || "/" },
  }));
});

// Tapping a notification opens (or focuses) NovelCheck on the right page.
self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = new URL(event.notification.data?.url || "/", self.location.origin).href;
  event.waitUntil(self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((wins) => {
    const win = wins.find((w) => w.url.startsWith(self.location.origin));
    if (!win) return self.clients.openWindow(url);
    return win.focus().then((w) => (w && w.navigate ? w.navigate(url) : w)).catch(() => self.clients.openWindow(url));
  }));
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
