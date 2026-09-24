// Admin helper: remove the books your Hide filters catch from Calibre itself.
// NovelCheck never deletes files; it builds a Calibre search that selects
// exactly those books, so Calibre removes them properly (with its own
// recycle bin and database updates).
import { get, post, qs } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";

export async function openCalibreRemoval(filters, onDone) {
  const data = await attempt(() => get("/api/admin/calibre/removal" + qs(filters)));
  if (!data) return;
  const dlg = $("#book-dialog");
  const list = data.books.slice(0, 100).map((b) => `<li>${esc(b.title)} <span class="text-slate-500">· ${esc(b.author)}</span></li>`).join("");
  dlg.innerHTML = `
    <div class="max-h-[85vh] space-y-4 overflow-y-auto p-6 text-sm">
      <div class="flex items-start justify-between gap-4">
        <h2 class="text-xl font-bold">Remove hidden books from Calibre</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button>
      </div>
      ${data.count === 0 ? `<p class="text-slate-300">No Calibre books match these Hide filters (books marked OK are never included).</p>` : `
      <p class="text-slate-300"><b>${data.count}</b> Calibre book${data.count === 1 ? "" : "s"} match your Hide filters.
        Books a parent marked OK are left out. Check the list first:</p>
      <ul class="max-h-48 list-disc overflow-y-auto rounded-lg bg-slate-800/60 py-2 pl-8">${list}</ul>
      ${data.count > 100 ? `<p class="text-xs text-slate-500">Showing the first 100.</p>` : ""}
      ${data.one_click ? `
      <div class="rounded-lg bg-rose-950/40 p-3 ring-1 ring-rose-900">
        <label class="toggle"><input type="checkbox" id="cal-checked"> I've checked the list above</label>
        <button type="button" data-remove class="btn-danger mt-2" disabled>Remove these ${data.count} book${data.count === 1 ? "" : "s"} from Calibre</button>
        <p class="mt-1 text-xs text-slate-400">They go to Calibre's recycle bin, so you can restore them in Calibre.</p>
      </div>
      <p class="text-xs text-slate-500">Or do it by hand in Calibre:</p>` : `
      <p class="text-xs text-slate-400">Tip: set up <b>Admin → Calibre Library → One-click removal</b> to remove books straight from here.</p>`}
      <div>
        <span class="label">Calibre search</span>
        <textarea id="cal-search" readonly rows="3" class="input font-mono text-xs">${esc(data.search)}</textarea>
        <button type="button" data-copy class="btn-primary mt-2">Copy search</button>
      </div>
      <ol class="list-decimal space-y-1 pl-5 text-slate-300">
        <li>Open <b>Calibre</b> on your computer, or its web interface.</li>
        <li>Paste the search into Calibre's search bar and press <b>Enter</b>. Exactly these books appear.</li>
        <li>Select them all (<b>Ctrl+A</b>, or <b>⌘A</b> on a Mac) and press <b>Delete</b> (<b>Remove books</b>). Calibre asks you to confirm.</li>
        <li>Back in NovelCheck, click <b>Admin → Sync Calibre now</b> so they disappear here too.</li>
      </ol>
      <p class="text-xs text-slate-500">Calibre keeps deleted books in its recycle bin for a while, so a mistake can be undone there.</p>`}
    </div>`;
  dlg.onclick = async (e) => {
    if (e.target === dlg || e.target.closest("[data-close]")) return dlg.close();
    if (e.target.id === "cal-checked") {
      $("[data-remove]", dlg).disabled = !e.target.checked;
      return;
    }
    const rm = e.target.closest("[data-remove]");
    if (rm) {
      if (!confirm(`Remove ${data.count} book(s) from your Calibre library? They go to Calibre's recycle bin.`)) return;
      rm.disabled = true;
      rm.textContent = "Removing…";
      const r = await attempt(() => post("/api/admin/calibre/remove", { ...filters, expected_count: data.count }));
      if (r) {
        dlg.close();
        toast(`Removed ${r.removed} book(s) from Calibre${r.skipped ? ` (${r.skipped} were already gone)` : ""}. Syncing…`);
        onDone?.();
      } else {
        rm.disabled = false;
        rm.textContent = `Remove these ${data.count} books from Calibre`;
      }
      return;
    }
    if (e.target.closest("[data-copy]")) {
      const ta = $("#cal-search", dlg);
      try {
        await navigator.clipboard.writeText(ta.value);
      } catch {
        ta.select();
        document.execCommand("copy");
      }
      toast("Search copied. Paste it into Calibre's search bar");
    }
  };
  dlg.showModal();
}
