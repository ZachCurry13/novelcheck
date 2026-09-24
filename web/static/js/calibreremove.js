// Admin helper: remove the books your Hide filters catch from Calibre itself.
// NovelCheck never deletes files; it builds a Calibre search that selects
// exactly those books, so Calibre removes them properly (with its own
// recycle bin and database updates).
import { get, qs } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";

export async function openCalibreRemoval(filters) {
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
