// Admin → Library cleanup: everything for keeping the library tidy in one place. The guided
// cleanup (cleanup.js) runs the Calibre steps in order; below it each tool on its own, with
// how much is waiting. The buttons marked data-act are handled by the Admin page.
import { get, post } from "./api.js";
import { esc, attempt, fmtNum } from "./ui.js";
import { mountCleanup } from "./cleanup.js";
import { renderSameBooks } from "./samebooks.js";
import { renderHCClean } from "./hcclean.js";
import { renderUpload } from "./uploadbooks.js";
import { renderCalTrash } from "./caltrash.js";

const row = (icon, title, note, count, action) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-3">
  <div class="min-w-0 flex-1"><p class="font-semibold">${icon} ${esc(title)}${count === null ? "" : count ? ` <span class="rounded-full bg-amber-900/60 px-2 text-xs text-amber-200">${fmtNum(count)}</span>` : ` <span class="text-xs text-emerald-400">✓ nothing waiting</span>`}</p>
    <p class="text-xs text-slate-400">${note}</p></div>${action}</li>`;

const btn = (act, label) => `<button type="button" data-act="${act}" class="btn-secondary py-1 text-sm">${label}</button>`;
const link = (href, label) => `<a href="${href}" class="btn-secondary py-1 text-sm">${label}</a>`;

export async function renderCleanupTab(host, state) {
  host.innerHTML = `<div class="space-y-4"><div id="cu-guided" class="card"></div>
    <div class="card space-y-2"><h2 class="text-lg font-semibold">Each tool on its own</h2><ul id="cu-tools" class="space-y-2"></ul></div>
    <div class="card space-y-2"><details id="cu-trash"><summary class="cursor-pointer font-semibold">Calibre's recycle bin: what was removed, and put a book back</summary>
      <div id="cu-trash-body" class="mt-2"></div></details></div>
    <div class="card space-y-2"><details id="cu-twice"><summary class="cursor-pointer font-semibold">Books listed twice in NovelCheck (with the evidence)</summary>
      <div id="cu-twice-body" class="mt-2"></div></details></div>
    <div class="card space-y-2"><details id="cu-hc"><summary class="cursor-pointer font-semibold">Fix Calibre details from Hardcover (series, titles, authors, ISBN, descriptions, genres)</summary>
      <div id="cu-hc-body" class="mt-2"></div></details></div>
    <div class="card space-y-2"><details id="cu-abs"><summary class="cursor-pointer font-semibold">Fix Audiobookshelf details from Hardcover (titles, authors, series, ISBN, ASIN, narrators, covers)</summary>
      <div id="cu-abs-body" class="mt-2"></div></details></div>
    <div class="card space-y-2"><details id="cu-mem"><summary class="cursor-pointer font-semibold">Remembered ratings and covers (and how to forget them)</summary>
      <div id="cu-mem-body" class="mt-2"></div></details></div>
    <div class="card space-y-2"><details id="cu-ku"><summary class="cursor-pointer font-semibold">Books on your Kindle NovelCheck doesn't know</summary>
      <div id="cu-ku-body" class="mt-2 text-sm text-slate-300"></div></details></div>
    <div class="card space-y-2"><details id="cu-kb"><summary class="cursor-pointer font-semibold">Books copied from Kindles</summary>
      <div id="cu-kb-body" class="mt-2 text-sm text-slate-300"></div></details></div>
    <div class="card space-y-2"><details id="cu-up"><summary class="cursor-pointer font-semibold">⬆️ Upload e-books to Calibre and audiobooks to Audiobookshelf (experimental)</summary>
      <div id="cu-up-body" class="mt-2"></div></details></div></div>`;
  mountCleanup(host.querySelector("#cu-guided"));
  // paintTools draws only the list of tools (their counts and buttons), so the rest of the page stays as it is
  // and keeps its place when a tool was started.
  const paintTools = async () => {
  const [s, c, cv] = await Promise.all([attempt(() => get("/api/admin/status")), attempt(() => get("/api/admin/cleanup")), attempt(() => get("/api/admin/covers/missing"))]);
  const job = cv?.job || {};
  const g = s?.genres || {};
  host.querySelector("#cu-tools").innerHTML = [
    row("👯", "Books listed twice in NovelCheck", "The same book entered twice (for example on a list before the e-book arrived). Joining moves everything onto one entry.", c?.pairs, `<button type="button" data-cu="twice" class="btn-secondary py-1 text-sm">Review below</button>`),
    row("🔁", "Duplicates in Calibre", "The same book as two or more Calibre entries. Keeps the best copy (EPUB first) and moves the rest to Calibre's recycle bin.", c?.calibre_dups ?? null, link("#/duplicates", "Open")),
    row("🏷️", "Tidy titles in Calibre", "Titles carrying track or series numbers like “01 - …”. NovelCheck already shows them tidied.", s?.title_fixes, btn("tidy-titles", "Tidy…")),
    row("🗂️", "Formats", "Keep only the formats you choose (say EPUB), convert books with no EPUB, undo for 7 days.", c?.remove_files, `<button type="button" data-cu="formats" class="btn-secondary py-1 text-sm">Formats…</button>`),
    row("🔖", "Genres", "Library books Calibre has no tags for. They are filled in by the AI by themselves; this runs it now.", g.missing, g.running ? btn("genres-stop", "Stop") : btn("genres-fill", "Fill in now")),
    row("📦", "Box sets", "Several books in one file: check them and split them into their books.", s?.box_sets, btn("box-sets", "Check and split")),
    row("🖼️", "Books with no cover", job.running ? `Looking now: ${job.checked} of ${job.total} checked, ${job.found} found${job.current ? ` · ${esc(job.current)}` : ""}.` : "Books Calibre has no cover for show a drawn one. This looks for the real cover in Open Library and Google Books in the background (it also runs once a day), and only uses one whose title matches.", cv?.count ?? null, job.running ? `<button type="button" data-cu="covers-stop" class="btn-secondary py-1 text-sm">Stop</button>` : `<span class="flex flex-wrap gap-2"><button type="button" data-cu="covers-review" class="btn-primary py-1 text-sm">Review one by one</button><button type="button" data-cu="covers-find" class="btn-secondary py-1 text-sm">Find covers now</button></span>`),
    row("🖼️", "Covers reported as wrong", "Covers your family flagged.", s?.cover_reports, btn("cover-reports", "Review")),
    row("🗑️", "Delete requests", "Books your family asked to remove.", s?.pending_deletes, link("#/deletions", "Review")),
    row("🔄", "Read the Calibre library now", "Re-reads Calibre so NovelCheck matches what is there (it also happens by itself).", null, btn("sync", "Read now")),
    row("📖", "Kindle books to match", "Books on your e-readers NovelCheck could not match. Each person does their own, under Settings.", null, link("#/settings?s=kindle", "Open Settings")),
  ].join("");
  };
  await paintTools();
  host.onclick = async (e) => {
    const k = e.target.closest("[data-cu]")?.dataset.cu;
    if (k === "formats") import("./formats.js").then((m) => m.openFormats());
    else if (k === "covers-review") import("./coverreview.js").then((m) => m.openCoverReview(paintTools));
    else if (k === "covers-find" || k === "covers-stop") {
      if (await attempt(() => post(`/api/admin/covers/${k === "covers-find" ? "find" : "stop"}`), k === "covers-find" ? "Looking for covers in the background" : "Stopping")) paintTools();
    }
    else if (e.target.closest("[data-act]")) setTimeout(paintTools, 1500); // a tool the Admin page starts: show its new state
    else if (k === "twice") {
      const d = host.querySelector("#cu-twice");
      d.open = true;
      d.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  };
  host.querySelector("#cu-trash").addEventListener("toggle", (e) => {
    if (e.target.open) renderCalTrash(host.querySelector("#cu-trash-body")); // read again each time it is opened
  });
  host.querySelector("#cu-up").addEventListener("toggle", (e) => {
    const body = host.querySelector("#cu-up-body");
    if (e.target.open && !body.dataset.done) {
      body.dataset.done = "1";
      renderUpload(body);
    }
  });
  host.querySelector("#cu-hc").addEventListener("toggle", (e) => {
    const body = host.querySelector("#cu-hc-body");
    if (e.target.open && !body.dataset.done) {
      body.dataset.done = "1";
      renderHCClean(body);
    }
  });
  host.querySelector("#cu-ku").addEventListener("toggle", async (e) => {
    if (!e.target.open) return;
    const body = host.querySelector("#cu-ku-body");
    const load = async () => {
      const r = await get("/api/admin/kindle-unknown").catch(() => null);
      if (!r) { body.textContent = "Couldn't load that just now."; return; }
      if (!r.books.length) { body.textContent = "Nothing waiting. Book files the Kindle backup finds that NovelCheck has no book for show up here: tick the ones to add. Nothing is added by itself."; return; }
      body.innerHTML = `<p class="mb-2">The Kindle backup found these book files, and NovelCheck has no book with that title. Tick the ones to <b>add</b> (they join NovelCheck under the title and author shown and go on your Nightstand). Many are probably books you already have under another name: leave those out.</p>
        <ul class="max-h-96 divide-y divide-slate-800 overflow-auto">${r.books.map((b) => `<li class="flex items-center gap-2 py-1.5"><input type="checkbox" data-ku="${b.id}">
          <span class="min-w-0"><span class="block break-words">${esc(b.title || b.filename)}${b.author ? ` <span class="text-slate-400">· ${esc(b.author)}</span>` : ""}</span>
          <span class="block break-words text-xs text-slate-500">${esc(b.path)}</span></span></li>`).join("")}</ul>
        <p class="mt-2 flex flex-wrap gap-2"><button type="button" data-ku-add class="btn-secondary py-1 text-sm">Add the ticked books</button>
        <button type="button" data-ku-drop class="btn-ghost py-1 text-sm">Leave the ticked ones out (don't ask again)</button></p>`;
      body.onclick = async (ev) => {
        const add = ev.target.closest("[data-ku-add]"), drop = ev.target.closest("[data-ku-drop]");
        if (!add && !drop) return;
        const ids = [...body.querySelectorAll("[data-ku]:checked")].map((c) => Number(c.dataset.ku));
        if (!ids.length) return;
        await attempt(() => post("/api/admin/kindle-unknown", add ? { add: ids } : { drop: ids }), add ? "Added" : "Left out");
        load();
      };
    };
    load();
  });
  host.querySelector("#cu-kb").addEventListener("toggle", async (e) => {
    if (!e.target.open) return;
    const body = host.querySelector("#cu-kb-body");
    const r = await get("/api/admin/kindle-backup").catch(() => null);
    const mb = (b) => (b >= 1e9 ? `${(b / 1e9).toFixed(1)} GB` : `${Math.max(1, Math.round(b / 1e6))} MB`);
    const q = (d) => `user=${encodeURIComponent(d.user)}&device=${encodeURIComponent(d.name)}`;
    body.innerHTML = !r ? "Couldn't load that just now." : !r.devices.length ? "Nothing copied yet. The KOReader plugin copies every book on a Kindle here when it goes online (NovelCheck &gt; Copy my Kindle to NovelCheck now)."
      : `${r.devices.map((d, i) => `<div class="mb-3"><p><b>${esc(d.name)}</b>: ${d.files} book${d.files === 1 ? "" : "s"}, ${mb(d.bytes)}.</p>
          <p class="mt-1 flex flex-wrap gap-2"><a class="btn-secondary py-1 text-sm" href="/api/admin/kindle-backup/zip?${q(d)}">⬇ Download everything (ZIP)</a>
          <button type="button" data-kb-list="${i}" class="btn-ghost py-1 text-sm">Show the files</button></p><div data-kb-files="${i}" class="mt-2"></div></div>`).join("")}
        <p class="mt-2 text-xs text-slate-400">The files are in the <b>${esc(r.folder)}</b>, exactly as they were on the Kindle (protected ones too, marked 🔒: they only open on a device with your Kindle account). Nothing is converted or unlocked.</p>`;
    body.onclick = async (ev) => {
      const b = ev.target.closest("[data-kb-list]");
      if (!b) return;
      const d = r.devices[Number(b.dataset.kbList)], box = body.querySelector(`[data-kb-files="${b.dataset.kbList}"]`);
      box.textContent = "Loading…";
      const l = await get(`/api/admin/kindle-backup/files?${q(d)}`).catch(() => null);
      box.innerHTML = !l ? "Couldn't load that just now." : `<ul class="max-h-80 divide-y divide-slate-800 overflow-auto text-xs">${l.files.map((f) =>
        `<li class="flex items-center justify-between gap-2 py-1"><span class="min-w-0 break-words">${f.locked ? "🔒 " : ""}${esc(f.path)}</span>
          <a class="shrink-0 underline" href="/api/admin/kindle-backup/file?${q(d)}&path=${encodeURIComponent(f.path)}">${mb(f.size)} ⬇</a></li>`).join("")}</ul>`;
    };
  });
  host.querySelector("#cu-mem").addEventListener("toggle", (e) => {
    if (e.target.open) import("./memorycard.js").then((m) => m.renderMemory(host.querySelector("#cu-mem-body"))); // fresh each time
  });
  host.querySelector("#cu-abs").addEventListener("toggle", (e) => {
    if (e.target.open) import("./absclean.js").then((m) => m.renderABSClean(host.querySelector("#cu-abs-body"))); // fresh each time
  });
  host.querySelector("#cu-twice").addEventListener("toggle", (e) => {
    if (e.target.open && !host.querySelector("#cu-twice-body").dataset.done) {
      host.querySelector("#cu-twice-body").dataset.done = "1";
      renderSameBooks(host.querySelector("#cu-twice-body"), undefined, state);
    }
  });
}
