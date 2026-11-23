// Settings → Kindle & KOReader: books that left your e-reader, waiting for your choice (keep it, remove it from your
// library, or remove it and ask for it to be deleted for good), and the books you removed, with Show again.
// Only the main admin deletes a book's files; its rating and cover always stay (api/leftbooks.go).
import { get, post, del } from "./api.js";
import { esc, attempt } from "./ui.js";

const CHOICES = [["keep", "Keep"], ["hide", "Remove from my library"], ["delete", "Remove, and ask to delete it"]];

export async function renderLeftBooks(host) {
  const r = await get("/api/me/left").catch(() => null);
  if (!r || (!r.left.length && !r.hidden.length)) { host.innerHTML = ""; return; }
  const name = (b) => `<b>${esc(b.title)}</b>${b.author ? ` <span class="text-slate-400">· ${esc(b.author)}</span>` : ""}`;
  host.innerHTML = `<div class="space-y-2 border-t border-slate-800 pt-2 text-sm">
    ${r.left.length ? `<p class="label">Left your e-reader (${r.left.length})</p>
      <p class="text-xs text-slate-400">Keep them in NovelCheck, or remove them. Removing only hides a book from you; asking to delete puts it on the main admin's list.</p>
      <ul class="divide-y divide-slate-800">${r.left.map((b) => `<li class="py-1.5">${name(b)}
        <span class="mt-1 flex flex-wrap gap-1">${CHOICES.map(([c, l]) => `<button type="button" data-left="${b.id}" data-choice="${c}" class="btn-ghost py-0.5 text-xs">${l}</button>`).join("")}</span></li>`).join("")}</ul>` : ""}
    ${r.hidden.length ? `<details><summary class="label cursor-pointer">Removed from my library (${r.hidden.length})</summary>
      <ul class="mt-1 divide-y divide-slate-800">${r.hidden.map((b) => `<li class="flex items-center justify-between gap-2 py-1"><span class="min-w-0">${name(b)}</span>
        <button type="button" data-unhide="${b.id}" class="btn-ghost shrink-0 py-0.5 text-xs">Show again</button></li>`).join("")}</ul></details>` : ""}</div>`;
  host.onclick = async (e) => {
    const l = e.target.closest("[data-left]"), u = e.target.closest("[data-unhide]");
    if (l && await attempt(() => post("/api/me/left", { book_id: Number(l.dataset.left), choice: l.dataset.choice }), "Done")) renderLeftBooks(host);
    if (u && await attempt(() => del(`/api/me/hidden-books/${u.dataset.unhide}`), "It shows again")) renderLeftBooks(host);
  };
}
