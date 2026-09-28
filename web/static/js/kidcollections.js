// Users & Rules: a kid's account can be limited to chosen collections (they
// then see no other books anywhere; their content rules still apply).
import { $, $$, esc } from "./ui.js";

export function collectionLimitHTML(u, cols) {
  const chosen = new Set(u.collections || []);
  return `<details class="rounded-lg bg-slate-800/40 px-3 py-2" ${u.only_collections ? "open" : ""}>
    <summary class="cursor-pointer py-1 text-sm">📚 Only books in chosen collections${u.only_collections ? ` <span class="chip-cat">on</span>` : ""}</summary>
    <label class="toggle mt-2 min-h-[2.5rem]"><input type="checkbox" data-only-collections ${u.only_collections ? "checked" : ""}>
      Only show books in these collections (content rules still apply)</label>
    ${cols.length ? `<div class="grid gap-1 sm:grid-cols-2">${cols.map((c) => `<label class="toggle min-h-[2.5rem]">
      <input type="checkbox" data-collection value="${c.id}" ${chosen.has(c.id) ? "checked" : ""}> ${esc(c.icon)} ${esc(c.name)} <span class="text-slate-500">(${c.books})</span></label>`).join("")}</div>`
      : `<p class="text-xs text-slate-400">No collections yet. Make some on the <a href="#/collections" class="underline">📚 Collections</a> page.</p>`}
  </details>`;
}

// readCollectionLimit adds a kid card's choices to the save body.
export function readCollectionLimit(cardEl, body) {
  const only = $("[data-only-collections]", cardEl);
  if (!only) return;
  body.only_collections = only.checked;
  body.collections = $$("[data-collection]:checked", cardEl).map((cb) => Number(cb.value));
}
