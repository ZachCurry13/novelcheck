// Nightstand: two books on your Nightstand that look like the same book (for example
// "Percy Jackson 1 - The Lightning Thief" and "The Lightning Thief"), with a button to
// join them. Only people who may merge books see it.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";

const line = (b) => `<b>${esc(b.title)}</b>${b.author ? ` <span class="text-slate-400">· ${esc(b.author)}</span>` : ""}`;

export async function renderTwins(box, items, user, reload) {
  box.innerHTML = "";
  if (!["admin", "editor"].includes(user.role)) return;
  const ids = new Set(items.map((i) => i.book_id));
  if (ids.size < 2) return;
  const r = await get("/api/admin/same-books").catch(() => null);
  const pairs = (r?.pairs || []).filter((p) => ids.has(p.a.id) && ids.has(p.b.id));
  if (!pairs.length) return;
  box.innerHTML = `<div class="mb-4 space-y-2 rounded-lg border border-amber-700/50 bg-amber-950/30 p-3">
    <p class="text-sm text-amber-200">👯 ${pairs.length === 1 ? "Two books here look" : `${pairs.length} pairs here look`} like the same book. Joining keeps one entry with everything from both (your place, rating and shelves).</p>
    ${pairs.map((p) => `<div data-a="${p.a.id}" data-b="${p.b.id}" class="space-y-2 rounded-lg bg-slate-800/60 p-2 text-sm">
      <p>${line(p.a)}<br>${line(p.b)}</p>
      <div class="flex flex-wrap gap-2"><button type="button" data-twin="merge" class="btn-primary py-1 text-sm">Join them</button>
        <button type="button" data-twin="not" class="btn-ghost py-1 text-sm">Not the same</button></div></div>`).join("")}</div>`;
  box.onclick = async (e) => {
    const btn = e.target.closest("[data-twin]");
    const row = btn?.closest("[data-a]");
    if (!row) return;
    btn.disabled = true;
    const body = { a: Number(row.dataset.a), b: Number(row.dataset.b) };
    if (await attempt(() => post(`/api/admin/same-books/${btn.dataset.twin}`, body), btn.dataset.twin === "merge" ? "Joined" : "Okay, kept apart")) reload();
    else btn.disabled = false;
  };
}
