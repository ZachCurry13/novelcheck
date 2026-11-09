// Library → Find duplicates → "Listed twice": entries that look like the same
// book (for example one added to a reading list before the e-book arrived).
// Merging moves everything (Nightstand, wishlist, ratings, copies) onto one.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

const side = (b) => `<div class="min-w-0 flex-1 rounded-lg bg-slate-800/60 p-3 text-sm">
    <p class="break-words font-semibold">${esc(b.title)}</p><p class="text-slate-400">${esc(b.author || "No author")}</p>
    <p class="mt-1 flex flex-wrap gap-1 text-xs">${b.owned ? `<span class="chip-none">In a library</span>` : `<span class="chip-pending">Not in a library</span>`}
      ${b.formats ? b.formats.split(",").map((f) => `<span class="chip-fmt">${esc(f)}</span>`).join(" ") : ""}
      ${b.wanted ? `<span class="text-slate-400">on ${b.wanted} list${b.wanted === 1 ? "" : "s"}</span>` : ""}</p></div>`;

export async function renderSameBooks(host, onChange) {
  const r = await attempt(() => get("/api/admin/same-books"));
  if (!r) return;
  if (!r.pairs.length) {
    host.innerHTML = `<p class="text-sm text-slate-400">No books are listed twice. When one is added to a list before it's in the library, NovelCheck joins them by itself once the library has it.</p>`;
    return;
  }
  host.innerHTML = `<p class="mb-2 text-sm text-slate-400">${r.pairs.length} possible pair${r.pairs.length === 1 ? "" : "s"}. <b>Join them</b> if it's one book: everything moves onto one entry and the other title is remembered. <b>Not the same</b> hides the pair for good.</p>
    <ul class="space-y-3">${r.pairs.map((p) => `<li class="card space-y-2" data-a="${p.a.id}" data-b="${p.b.id}">
      <div class="flex flex-col gap-2 sm:flex-row">${side(p.a)}${side(p.b)}</div>
      <div class="flex flex-wrap gap-2"><button type="button" data-same="merge" class="btn-primary py-1 text-sm">Join them</button>
        <button type="button" data-same="not" class="btn-ghost py-1 text-sm">Not the same</button></div></li>`).join("")}</ul>`;
  host.onclick = async (e) => {
    const act = e.target.closest("[data-same]")?.dataset.same;
    const li = e.target.closest("li");
    if (!act || !li) return;
    const body = { a: Number(li.dataset.a), b: Number(li.dataset.b) };
    if (await attempt(() => post(`/api/admin/same-books/${act}`, body), act === "merge" ? "Joined" : "Okay, kept apart")) {
      li.remove();
      if (!host.querySelector("li")) renderSameBooks(host, onChange);
      onChange?.();
    } else toast("That didn't work", true);
  };
}
