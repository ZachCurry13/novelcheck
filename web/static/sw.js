// NovelCheck service worker: caches the app shell for offline launch and
// home-screen installs. API responses are never cached (they are private and
// sent with Cache-Control: no-store).
const CACHE = "novelcheck-shell-v61";
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
  "/js/adminsettings.js",
  "/js/adminstats.js",
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
  "/js/ollamamodels.js",
  "/js/titlefix.js",
  "/js/suggest.js",
  "/js/taste.js",
  "/js/covers.js",
  "/js/genres.js",
  "/js/problems.js",
  "/js/libraries.js",
  "/js/libraryselect.js",
  "/js/mobilenav.js",
  "/js/content.js",
  "/js/blurb.js",
  "/js/discover.js",
  "/js/discoveradmin.js",
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
  "/js/deepscanlists.js",
  "/js/appupdate.js",
  "/js/shelf.js",
  "/js/photo.js",
  "/js/activity.js",
  "/js/livestatus.js",
  "/js/autorateadmin.js",
  "/js/collections.js",
  "/js/collectionai.js",
  "/js/bookcollections.js",
  "/js/seasonchips.js",
  "/js/kidcollections.js",
  "/js/bookchoices.js",
  "/js/adminnav.js",
  "/js/discovertabs.js",
  "/js/profileextras.js",
  "/js/events.js",
  "/js/eventnew.js",
  "/js/eventshelf.js",
  "/js/librarycard.js",
  "/js/aitoolsadmin.js",
  "/js/appearance.js",
  "/js/deepreader.js",
  "/js/recentsearches.js",
  "/js/safemodebanner.js",
  "/js/barcode.js",
  "/js/wishlist.js",
  "/js/delivery.js",
  "/vendor/qrcode.esm.js",
];

self.addEventListener("install", (event) => {
  // cache: "reload" skips copies the browser may still hold from before (they
  // used to be cached for an hour), so an install never stores old files.
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL.map((u) => new Request(u, { cache: "reload" })))).then(() => self.skipWaiting()));
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

// Network first, so a new version shows at once; the cache is for offline.
// index.html loads this build's files at /v/<build>/…, cached under that
// address; the plain address (precached above) is the offline fallback.
self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || url.origin !== location.origin || url.pathname.startsWith("/api/")) return;
  const plain = url.pathname.replace(/^\/v\/[^/]+(?=\/)/, "");
  event.respondWith(
    fetch(event.request)
      .then((res) => {
        if (res.ok && SHELL.includes(plain)) {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(event.request, copy));
        }
        return res;
      })
      .catch(() => caches.match(event.request)
        .then((r) => r || caches.match(plain))
        .then((r) => r || (event.request.mode === "navigate" ? caches.match("/index.html") : Response.error()))),
  );
});
