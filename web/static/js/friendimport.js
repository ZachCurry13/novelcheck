// A friend's account brings in her own reading from a Goodreads (or
// StoryGraph) export: the books on her "read" shelf go to her own library and
// her reading stats, Wrapped and trophies. Nothing goes to the family's.
import { post } from "./api.js";
import { attempt } from "./ui.js";
import { parseCSV, detect, rowsToBooks } from "./importfile.js";

export const friendImportHTML = (again) => `<div class="card space-y-2 text-sm text-slate-300" data-friend-import>
  <p>${again ? "Read more since? Bring in your list again: books already there are kept." : "Bring in your reading from Goodreads and NovelCheck fills in your stats, Wrapped and trophies."}</p>
  ${again ? "" : `<p class="text-xs text-slate-400">On Goodreads: My Books → Import and export → Export library, then pick the file it gives you. A StoryGraph export works too.</p>`}
  <label class="btn-secondary inline-flex cursor-pointer">Choose the file<input type="file" accept=".csv,text/csv" class="hidden" data-friend-file></label>
  <p data-friend-msg class="text-xs text-slate-400"></p></div>`;

// bindFriendImport reads the chosen file and sends her "read" shelf.
export function bindFriendImport(host, onDone) {
  const input = host.querySelector("[data-friend-file]");
  const msg = host.querySelector("[data-friend-msg]");
  input?.addEventListener("change", async () => {
    const file = input.files?.[0];
    if (!file) return;
    const rows = parseCSV(await file.text());
    if (rows.length < 2) return (msg.textContent = "That file has no books in it.");
    const { cols } = detect(rows[0]);
    if (cols.title < 0) return (msg.textContent = "That doesn't look like a Goodreads or StoryGraph export.");
    const books = rowsToBooks(rows.slice(1), cols, null).filter((b) => b.read);
    if (!books.length) return (msg.textContent = "No books on the \"read\" shelf in that file.");
    msg.textContent = `Bringing in ${books.length} books…`;
    const r = await attempt(() => post("/api/me/import", { books }));
    msg.textContent = r ? `Brought in ${r.imported} books.` : "";
    if (r) onDone?.();
  });
}
