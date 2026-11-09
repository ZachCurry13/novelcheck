// Admin → Library cleanup: everything for keeping the library tidy in one place. The guided
// cleanup (cleanup.js) runs the Calibre steps in order; below it each tool on its own, with
// how much is waiting. The buttons marked data-act are handled by the Admin page.
import { get } from "./api.js";
import { esc, attempt, fmtNum } from "./ui.js";
import { mountCleanup } from "./cleanup.js";
import { renderSameBooks } from "./samebooks.js";

const row = (icon, title, note, count, action) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-3">
  <div class="min-w-0 flex-1"><p class="font-semibold">${icon} ${esc(title)}${count === null ? "" : count ? ` <span class="rounded-full bg-amber-900/60 px-2 text-xs text-amber-200">${fmtNum(count)}</span>` : ` <span class="text-xs text-emerald-400">✓ nothing waiting</span>`}</p>
    <p class="text-xs text-slate-400">${note}</p></div>${action}</li>`;

const btn = (act, label) => `<button type="button" data-act="${act}" class="btn-secondary py-1 text-sm">${label}</button>`;
const link = (href, label) => `<a href="${href}" class="btn-secondary py-1 text-sm">${label}</a>`;

export async function renderCleanupTab(host) {
  host.innerHTML = `<div class="space-y-4"><div id="cu-guided" class="card"></div>
    <div class="card space-y-2"><h2 class="text-lg font-semibold">🧰 Each tool on its own</h2><ul id="cu-tools" class="space-y-2"></ul></div>
    <div class="card space-y-2"><details id="cu-twice"><summary class="cursor-pointer font-semibold">👯 Books listed twice in NovelCheck (with the evidence)</summary>
      <div id="cu-twice-body" class="mt-2"></div></details></div></div>`;
  mountCleanup(host.querySelector("#cu-guided"));
  const [s, c] = await Promise.all([attempt(() => get("/api/admin/status")), attempt(() => get("/api/admin/cleanup"))]);
  const g = s?.genres || {};
  host.querySelector("#cu-tools").innerHTML = [
    row("👯", "Books listed twice in NovelCheck", "The same book entered twice (for example on a list before the e-book arrived). Joining moves everything onto one entry.", c?.pairs, `<button type="button" data-cu="twice" class="btn-secondary py-1 text-sm">Review below</button>`),
    row("🔁", "Duplicates in Calibre", "The same book as two or more Calibre entries. Keeps the best copy (EPUB first) and moves the rest to Calibre's recycle bin.", c?.calibre_dups ?? null, link("#/duplicates", "Open")),
    row("🏷️", "Tidy titles in Calibre", "Titles carrying track or series numbers like “01 - …”. NovelCheck already shows them tidied.", s?.title_fixes, btn("tidy-titles", "Tidy…")),
    row("🗂️", "Formats", "Keep only the formats you choose (say EPUB), convert books with no EPUB, undo for 7 days.", c?.remove_files, `<button type="button" data-cu="formats" class="btn-secondary py-1 text-sm">Formats…</button>`),
    row("🔖", "Genres", "Library books Calibre has no tags for. They are filled in by the AI by themselves; this runs it now.", g.missing, g.running ? btn("genres-stop", "Stop") : btn("genres-fill", "Fill in now")),
    row("📦", "Box sets", "Several books in one file: check them and split them into their books.", s?.box_sets, btn("box-sets", "Check and split")),
    row("🖼️", "Covers reported as wrong", "Covers your family flagged.", s?.cover_reports, btn("cover-reports", "Review")),
    row("🗑️", "Delete requests", "Books your family asked to remove.", s?.pending_deletes, link("#/deletions", "Review")),
    row("🔄", "Read the Calibre library now", "Re-reads Calibre so NovelCheck matches what is there (it also happens by itself).", null, btn("sync", "Read now")),
    row("📖", "Kindle books to match", "Books on your e-readers NovelCheck could not match. Each person does their own, under Settings.", null, link("#/settings?s=kindle", "Open Settings")),
  ].join("");
  host.onclick = async (e) => {
    const k = e.target.closest("[data-cu]")?.dataset.cu;
    if (k === "formats") import("./formats.js").then((m) => m.openFormats());
    else if (k === "twice") {
      const d = host.querySelector("#cu-twice");
      d.open = true;
      d.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  };
  host.querySelector("#cu-twice").addEventListener("toggle", (e) => {
    if (e.target.open && !host.querySelector("#cu-twice-body").dataset.done) {
      host.querySelector("#cu-twice-body").dataset.done = "1";
      renderSameBooks(host.querySelector("#cu-twice-body"));
    }
  });
}
