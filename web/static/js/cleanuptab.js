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
  host.querySelector("#cu-twice").addEventListener("toggle", (e) => {
    if (e.target.open && !host.querySelector("#cu-twice-body").dataset.done) {
      host.querySelector("#cu-twice-body").dataset.done = "1";
      renderSameBooks(host.querySelector("#cu-twice-body"), undefined, state);
    }
  });
}
