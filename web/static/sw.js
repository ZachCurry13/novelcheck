// NovelCheck service worker: caches the app shell for offline launch and
// home-screen installs. API responses are not cached, except the few a reader
// needs offline (MINE, below), which are cleared on sign-out and profile switch.
const CACHE = "novelcheck-shell-v242";
const MINE = "novelcheck-mine";
const SHELL = [
  "/",
  "/index.html",
  "/css/app.css",
  "/css/bookshelf.css",
  "/themes/ember.css",
  "/themes/paperback.css",
  "/themes/classic.css",
  "/themes/fonts.css",
  "/fonts/Literata-var.woff2",
  "/fonts/InstrumentSans-var.woff2",
  "/manifest.json",
  "/icons/icon.svg",
  "/icons/icon-192.png",
  "/icons/icon-512.png",
  "/fonts/MaterialSymbolsRounded-subset.woff2",
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
  "/js/adminfields.js",
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
  "/js/modeljobs.js",
  "/js/records.js",
  "/js/caltrash.js",
  "/js/ollamamodels.js",
  "/js/titlefix.js",
  "/js/bookrename.js",
  "/js/viewasbanner.js",
  "/js/libfacets.js",
  "/js/discoverfilters.js",
  "/js/hcextras.js",
  "/js/highlights.js",
  "/js/coverpicker.js",
  "/js/leftbooks.js",
  "/js/kindletasks.js",
  "/js/kindleadmin.js",
  "/js/kindlegroups.js",
  "/js/contentprefs.js",
  "/js/fixbooks.js",
  "/js/yearbanner.js",
  "/js/libmenu.js",
  "/js/pulltorefresh.js",
  "/js/whatsnewpopup.js",
  "/js/wrappedcards.js",
  "/js/memorycard.js",
  "/js/trialrate.js",
  "/js/bookshelf.js",
  "/js/bookshelfart.js",
  "/js/bookshelffall.js",
  "/js/summaryprefs.js",
  "/js/absclean.js",
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
  "/js/activityrec.js",
  "/js/inflight.js",
  "/js/hcclean.js",
  "/js/servercheck.js",
  "/js/uploadbooks.js",
  "/js/checkasync.js",
  "/js/activitycard.js",
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
  "/js/eventlists.js",
  "/js/friendimport.js",
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
  "/js/series.js",
  "/js/seriesgap.js",
  "/js/shelftabs.js",
  "/js/sameas.js",
  "/js/boxsets.js",
  "/js/formats.js",
  "/js/shelfcheck.js",
  "/js/progress.js",
  "/js/koreaderbooks.js",
  "/js/kidreading.js",
  "/js/family.js",
  "/js/familysettings.js",
  "/js/adminaccess.js",
  "/js/syncstatus.js",
  "/js/pluginmanage.js",
  "/js/tabs.js",
  "/js/readers.js",
  "/js/sharing.js",
  "/js/settings.js",
  "/js/profilesummary.js",
  "/js/kindlebooks.js",
  "/js/stats.js",
  "/js/kindle.js",
  "/js/home.js",
  "/js/health.js",
  "/js/accuracy.js",
  "/js/disagree.js",
  "/js/dropbooks.js",
  "/js/statscalc.js",
  "/js/statsperiods.js",
  "/js/statsparts.js",
  "/js/statsreport.js",
  "/js/statsbooks.js",
  "/js/statscard.js",
  "/js/audioui.js",
  "/js/bookaudio.js",
  "/js/player.js",
  "/js/playerextras.js",
  "/js/absadmin.js",
  "/js/absaccount.js",
  "/js/listening.js",
  "/js/libaudio.js",
  "/js/debugrec.js",
  "/js/samebooks.js",
  "/js/cleanreview.js",
  "/js/cleanup.js",
  "/js/trophies.js",
  "/js/trophies2.js",
  "/js/trophies3.js",
  "/js/trophyview.js",
  "/js/statsrecords.js",
  "/js/statselsewhere.js",
  "/js/statslist.js",
  "/js/finished.js",
  "/js/marks.js",
  "/js/coverreview.js",
  "/js/statsundated.js",
  "/js/search.js",
  "/js/searchindex.js",
  "/js/stars.js",
  "/js/wrapped.js",
  "/js/wrappedslides.js",
  "/js/wrappedstory.js",
  "/js/combo.js",
  "/js/libfilters.js",
  "/js/jobbanner.js",
  "/js/cleanuptab.js",
  "/js/devlink.js",
  "/js/devsettings.js",
  "/js/dialogback.js",
  "/js/viewstate.js",
  "/js/queuetwins.js",
  "/js/statscalendar.js",
  "/js/hardcover.js",
  "/js/libby.js",
  "/js/libbysearch.js",
  "/vendor/qrcode.esm.js",
  "/reader.html",
  "/js/reader.js",
  "/js/readerbars.js",
  "/js/readermenu.js",
  "/js/offline.js",
  ...["view", "epub", "epubcfi", "mobi", "fb2", "comic-book", "fixed-layout", "footnotes", "overlayer", "paginator",
    "progress", "search", "text-walker", "vendor/zip", "vendor/fflate"].map((f) => `/vendor/foliate/${f}.js`),
];

// The private answers kept for offline reading, newest last, each kind trimmed to its limit: the books opened in
// the reader (5 per device), their places and pages, the Nightstand and its covers, and who is signed in.
const KEEP = [
  [/^\/api\/books\/\d+\/download$/, 5],
  [/^\/api\/books\/\d+\/position$/, 50],
  [/^\/api\/books\/\d+$/, 50],
  [/^\/api\/books\/\d+\/cover$/, 50],
  [/^\/api\/(me|queue|content)$/, 3],
  // read-only pages, so a server that is down shows what was last seen (with a banner saying so): kept by the whole
  // address, since a list is asked many ways
  [/^\/api\/(home|books|books\/facets|catalogs|collections|requests)$/, 40],
  [/^\/api\/collections\/\d+$/, 20],
  [/^\/api\/me\/(kindles|kindle-library)$/, 30], // the Kindle page: which Kindles, and what each holds
  // the other read-only pages: Discover, Events, the wish list, Series, Stats, notices, the e-reader shelf, box sets
  [/^\/api\/(discover|events|wishlist|series|series\/one|notifications|books\/states|listening|box-sets|me\/reading-stats|me\/koreader-books)$/, 20],
];
const LISTED = /^\/api\/(home|books|books\/facets|catalogs|collections|requests|me\/kindles|me\/kindle-library|discover|events|wishlist|series|series\/one|notifications|books\/states|listening|box-sets|me\/reading-stats|me\/koreader-books)$/;
const keyOf = (url) => (LISTED.test(url.pathname) || url.pathname === "/api/queue" ? url.pathname + url.search : url.pathname);
// A server that is down answers with an error page (Cloudflare's 502, 521, 530 …), not with nothing: that counts as down.
const down = (res) => res.status >= 500 && res.status < 600;
// stale marks an answer from the kept copies, so the page can say the server is offline.
async function stale(r) {
  const h = new Headers(r.headers);
  h.set("X-NovelCheck-Offline", "1");
  if (r.headers.get("date")) h.set("X-NovelCheck-Saved", r.headers.get("date")); // when this copy was saved, for the banner
  return new Response(await r.blob(), { status: r.status, statusText: r.statusText, headers: h });
}
const kept = (path) => KEEP.find(([re]) => re.test(path));

async function keep(key, res) {
  const c = await caches.open(MINE);
  await c.delete(key);
  await c.put(key, res);
  const path = key.split("?")[0];
  const [re, max] = kept(path);
  const same = (await c.keys()).filter((k) => re.test(new URL(k.url).pathname));
  await Promise.all(same.slice(0, Math.max(0, same.length - max)).map((k) => c.delete(k)));
}

// A fresh Nightstand also keeps its books' pages and covers, so it opens offline.
async function keepNightstand(res) {
  const items = await res.json().catch(() => []);
  for (const i of Array.isArray(items) ? items : []) {
    for (const path of [`/api/books/${i.book_id}`, `/api/books/${i.book_id}/cover`]) {
      const r = await fetch(path, { credentials: "same-origin", headers: { "X-NovelCheck": "1" } }).catch(() => null);
      if (r?.ok) await keep(path, r);
    }
  }
}

self.addEventListener("install", (event) => {
  // cache: "reload" skips copies the browser may still hold from before (they
  // used to be cached for an hour), so an install never stores old files.
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL.map((u) => new Request(u, { cache: "reload" })))).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE && k !== MINE).map((k) => caches.delete(k))))
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
  if (event.request.method !== "GET" || url.origin !== location.origin) return;
  if (url.pathname.startsWith("/api/")) {
    // Downloads are kept only for the reader (?keep=1), and the Nightstand only as the plain list.
    const keepIt = kept(url.pathname) && (!url.pathname.endsWith("/download") || url.searchParams.has("keep")) &&
      (url.pathname !== "/api/queue" || !url.search || url.search === "?hidden=1"); // the plain Nightstand, and its put-aside list
    if (!keepIt) return;
    event.respondWith(fetch(event.request)
      .then((res) => {
        if (down(res)) throw new Error("the server is down");
        if (res.ok && !url.pathname.endsWith("/cover")) { // covers are kept only for the Nightstand (keepNightstand)
          const copy = res.clone(), list = url.pathname === "/api/queue" && !url.search ? res.clone() : null;
          event.waitUntil(keep(keyOf(url), copy).then(() => list && keepNightstand(list)));
        }
        return res;
      })
      .catch(() => caches.open(MINE).then((c) => c.match(keyOf(url))).then((r) => (r ? stale(r) : Response.error()))));
    return;
  }
  const plain = url.pathname.replace(/^\/v\/[^/]+(?=\/)/, "");
  event.respondWith(
    fetch(event.request)
      .then((res) => {
        if (down(res)) throw new Error("the server is down"); // Cloudflare's error page: use the kept copy instead
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
