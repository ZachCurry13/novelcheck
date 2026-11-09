// Nightstand: two books on your Nightstand that may be the same book (for example
// "Percy Jackson 1 - The Lightning Thief" and "The Lightning Thief"), with the evidence and a
// button to join them keeping either one. Doubtful pairs ask first. Only people who may merge
// books see it.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";
import { openBook } from "./bookdialog.js";
import { badge, reasons, confirmJoin, suggestion } from "./samebooks.js";

const line = (b) => `<button type="button" data-open="${b.id}" class="text-left font-semibold underline decoration-dotted" title="Open this book to look at it">${esc(b.title)}</button>${b.author ? ` <span class="text-slate-400">· ${esc(b.author)}</span>` : ""}${b.series ? ` <span class="text-sky-300">· ${esc(b.series)}${b.series_index ? ` #${b.series_index}` : ""}</span>` : ""}`;

export async function renderTwins(box, items, state, reload) {
  const user = state.user;
  box.innerHTML = "";
  if (!["admin", "editor"].includes(user.role)) return;
  const ids = new Set(items.map((i) => i.book_id));
  if (ids.size < 2) return;
  const r = await get("/api/admin/same-books").catch(() => null);
  const pairs = (r?.pairs || []).filter((p) => ids.has(p.a.id) && ids.has(p.b.id));
  if (!pairs.length) return;
  box.innerHTML = `<div class="mb-4 space-y-2 rounded-lg border border-amber-700/50 bg-amber-950/30 p-3">
    <p class="text-sm text-amber-200">👯 ${pairs.length === 1 ? "Two books here may be" : `${pairs.length} pairs here may be`} the same book. Check the evidence, then choose which one to keep: the joined book shows that one's title and details, and everything from both (your place, rating and shelves) moves onto it.</p>
    ${pairs.map((p, i) => `<div data-i="${i}" class="space-y-2 rounded-lg bg-slate-800/60 p-2 text-sm">
      ${badge(p)}<p>1. ${line(p.a)}<br>2. ${line(p.b)}</p>${reasons(p)}${suggestion(p)}
      <div class="flex flex-wrap gap-2"><button type="button" data-twin="merge" data-keep="a" class="btn-primary py-1 text-sm">Join, keep 1</button>
        <button type="button" data-twin="merge" data-keep="b" class="btn-primary py-1 text-sm">Join, keep 2</button>
        <button type="button" data-twin="not" class="btn-ghost py-1 text-sm">Not the same</button></div></div>`).join("")}</div>`;
  box.onclick = async (e) => {
    const open = e.target.closest("[data-open]");
    if (open) return openBook(Number(open.dataset.open), state, reload);
    const btn = e.target.closest("[data-twin]");
    const row = btn?.closest("[data-i]");
    if (!row) return;
    const p = pairs[Number(row.dataset.i)];
    if (btn.dataset.twin === "merge" && !confirmJoin(p)) return;
    btn.disabled = true;
    const keep = btn.dataset.keep ? (btn.dataset.keep === "a" ? p.a.id : p.b.id) : 0;
    if (await attempt(() => post(`/api/admin/same-books/${btn.dataset.twin}`, { a: p.a.id, b: p.b.id, keep }), btn.dataset.twin === "merge" ? "Joined" : "Okay, kept apart")) reload();
    else btn.disabled = false;
  };
}
