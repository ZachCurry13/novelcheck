// Reading stats: the books an imported list put on your "read" shelf without a day (Goodreads leaves "Date
// Read" empty when none was set), so they can't be counted in a year. Give each its day here; it then counts.
import { put } from "./api.js";
import { attempt, esc } from "./ui.js";

const today = () => new Date().toISOString().slice(0, 10);

export function undatedHTML(list = []) {
  const help = `<p class="text-xs text-slate-400">Read a book that isn't counted at all? Open it in the <a href="#/library" class="underline">Library</a> and press <b>✓ I read this</b>.</p>`;
  if (!list.length) return `<div class="mb-4">${help}</div>`;
  return `<section class="card-outlined mb-4 space-y-2">
    <h2 class="text-base font-medium">${list.length} finished book${list.length === 1 ? "" : "s"} with no day</h2>
    <p class="text-sm text-slate-300">Your list says you read ${list.length === 1 ? "this" : "these"} but not when, so ${list.length === 1 ? "it isn't" : "they aren't"} counted in any year. Give the day you finished (a guess is fine) and press Save.</p>
    <ul class="list">${list.map((b) => `<li class="space-y-2 py-3" data-undated="${b.book_id}">
      <div><p class="break-words font-medium">${esc(b.title)}</p><p class="text-sm text-slate-400">${esc(b.author || "")}</p></div>
      <div class="flex flex-wrap items-center gap-2"><input type="date" class="input w-auto flex-1 py-1.5" max="${today()}" aria-label="Day you finished ${esc(b.title)}">
        <button type="button" data-undated-save class="btn-tonal px-4 py-1.5">Save</button></div></li>`).join("")}</ul>
    ${help}</section>`;
}

// saveUndated stores the day picked on a row; it says whether anything was saved.
export async function saveUndated(button) {
  const row = button.closest("[data-undated]");
  const day = row?.querySelector("input")?.value;
  if (!day) {
    row?.querySelector("input")?.focus();
    return false;
  }
  return Boolean(await attempt(() => put(`/api/books/${row.dataset.undated}/finished`, { day }), "Counted"));
}
