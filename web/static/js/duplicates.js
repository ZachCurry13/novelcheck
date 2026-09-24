// Library → Find duplicates: books that are in Calibre more than once, with
// each copy's formats and sizes. Admins can move the extra copies to
// Calibre's recycle bin in one click (through the Content server).
import { get, post } from "./api.js";
import { $, $$, esc, attempt, toast } from "./ui.js";

const MB = 1 << 20;
const fmtSize = (b) => (!b ? "" : b >= MB ? `${(b / MB).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`);

function entryHTML(g, e, isAdmin) {
  const keep = e.calibre_id === g.keep;
  const total = e.files.reduce((a, f) => a + f.size, 0);
  const files = e.files.length
    ? e.files.map((f) => `<span class="chip-fmt" title="${esc(fmtSize(f.size))}">${esc(f.format.toUpperCase())}</span>`).join(" ")
    : `<span class="text-xs text-slate-500">no file</span>`;
  return `<li class="flex flex-wrap items-start gap-3 rounded-lg bg-slate-800/50 p-3">
    <label class="toggle shrink-0"><input type="checkbox" data-remove="${esc(e.calibre_id)}" data-book="${g.book_id}" ${keep ? "" : "checked"}> Remove</label>
    <div class="min-w-0 flex-1 space-y-1">
      <p class="flex flex-wrap items-center gap-2 text-sm"><b>Calibre #${esc(e.calibre_id)}</b> ${files}
        ${total ? `<span class="text-xs text-slate-400">${fmtSize(total)}</span>` : ""}
        ${keep ? `<span class="chip-none">✓ Suggested keep</span>` : ""}</p>
      ${isAdmin ? e.files.map((f) => `<p class="break-all text-xs text-slate-500">${esc(f.path)}</p>`).join("") : ""}
    </div></li>`;
}

export async function renderDuplicates(view, state) {
  const isAdmin = state.user.role === "admin";
  view.innerHTML = `
    <a href="#/library" class="text-sm text-slate-400 underline">← Library</a>
    <h1 class="mb-1 mt-2 text-2xl font-bold">Duplicates in Calibre</h1>
    <p class="mb-4 max-w-3xl text-sm text-slate-400">Books that are in Calibre more than once (same title and author).
      NovelCheck suggests keeping the copy with the best formats (EPUB first), then the most files, then the largest.
      Change the ticks if you'd rather keep a different copy. Removed copies go to Calibre's recycle bin.</p>
    <div id="dup-body"><p class="text-slate-400">Looking for duplicates…</p></div>`;
  const body = $("#dup-body", view);

  async function load() {
    const data = await attempt(() => get("/api/admin/calibre/duplicates"));
    if (!data) return;
    if (!data.groups.length) {
      body.innerHTML = `<div class="card text-slate-300">✓ No duplicates found. Every book is in Calibre only once.</div>`;
      return;
    }
    const how = data.can_remove
      ? `<button id="dup-remove" class="btn-primary">Remove selected copies</button>`
      : isAdmin
        ? `<p class="text-sm text-slate-400">To remove them from here, turn on <a href="#/admin" class="underline">Admin → Calibre Library → One-click removal</a>. Or copy the search into Calibre's search bar and delete them there.</p>`
        : `<p class="text-sm text-slate-400">Only an admin can remove books from Calibre. You can copy the search into Calibre's search bar to find them.</p>`;
    body.innerHTML = `
      <div class="card mb-4 flex flex-wrap items-center gap-3">
        <p class="mr-auto"><b>${data.groups.length}</b> book${data.groups.length === 1 ? "" : "s"} with duplicates ·
          <b>${data.extra}</b> extra cop${data.extra === 1 ? "y" : "ies"} · <span id="dup-count"></span></p>
        <button id="dup-copy" class="btn-secondary">Copy Calibre search</button>
        ${how}
      </div>
      <div class="space-y-3">${data.groups.map((g) => `
        <section class="card space-y-2">
          <h2 class="font-semibold">${esc(g.title)} <span class="font-normal text-slate-400">· ${esc(g.author || "Unknown author")}</span></h2>
          <ul class="space-y-2">${g.entries.map((e) => entryHTML(g, e, isAdmin)).join("")}</ul>
        </section>`).join("")}</div>`;
    const picked = () => $$("[data-remove]:checked", body).map((c) => c.dataset.remove);
    const count = () => ($("#dup-count", body).textContent = `${picked().length} selected to remove`);
    count();
    body.addEventListener("change", (e) => {
      const box = e.target.closest("[data-remove]");
      if (!box) return;
      // Never let every copy of a book be ticked.
      const group = $$(`[data-book="${box.dataset.book}"]`, body);
      if (group.every((c) => c.checked)) {
        box.checked = false;
        toast("Keep at least one copy of each book", true);
      }
      count();
    });
    $("#dup-copy", body).addEventListener("click", async () => {
      const ids = picked();
      if (!ids.length) return toast("Tick at least one copy", true);
      const search = ids.map((id) => `id:${id}`).join(" or ");
      try {
        await navigator.clipboard.writeText(search);
        toast("Copied. Paste it into Calibre's search bar, select all, and press Delete.");
      } catch {
        prompt("Copy this into Calibre's search bar:", search);
      }
    });
    $("#dup-remove", body)?.addEventListener("click", async (e) => {
      const ids = picked();
      if (!ids.length) return toast("Tick at least one copy", true);
      if (!confirm(`Move ${ids.length} duplicate cop${ids.length === 1 ? "y" : "ies"} to Calibre's recycle bin? The other copy of each book stays.`)) return;
      e.target.disabled = true;
      e.target.textContent = "Removing…";
      const r = await attempt(() => post("/api/admin/calibre/duplicates/remove", { remove: ids }));
      e.target.disabled = false;
      e.target.textContent = "Remove selected copies";
      if (!r) return;
      toast(`Removed ${r.removed} duplicate${r.removed === 1 ? "" : "s"}${r.skipped ? ` (${r.skipped} were already gone)` : ""}. Re-syncing Calibre…`);
      setTimeout(load, 2500);
    });
  }
  await load();
}
