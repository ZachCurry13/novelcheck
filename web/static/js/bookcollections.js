// The book window's collections: the ones the book is in (tap one to see
// it), and for parents "＋ Add to a collection" and ✕ to take it out.
import { get, post, del } from "./api.js";
import { esc, attempt } from "./ui.js";
import { openNewCollection } from "./collectionai.js";

const chip = (c, manager) => `<span class="chip-cat gap-1" data-in="${c.id}">
  <a href="#/library?collection=${c.id}" data-close>${esc(c.icon)} ${esc(c.name)}</a>
  ${manager ? `<button type="button" data-uncollect="${c.id}" class="ml-1 px-1 text-slate-400 hover:text-white" title="Take it out of ${esc(c.name)}" aria-label="Take it out of ${esc(c.name)}">✕</button>` : ""}</span>`;

export function bookCollectionsHTML(cols, manager) {
  if (!cols.length && !manager) return "";
  return `<div id="book-collections"><span class="label">Collections</span>
    <div data-chips class="flex flex-wrap items-center gap-2">${cols.map((c) => chip(c, manager)).join("")}
      ${manager ? `<select data-add-collection class="input w-auto py-1 text-sm" aria-label="Add to a collection"><option value="">＋ Add to a collection…</option></select>` : ""}</div></div>`;
}

export async function bindBookCollections(dlg, bookId, manager, cols) {
  const host = dlg.querySelector("#book-collections");
  if (!host || !manager) return;
  const sel = host.querySelector("[data-add-collection]");
  const all = ((await get("/api/collections").catch(() => null))?.collections) || [];
  const fill = () => {
    const have = new Set([...host.querySelectorAll("[data-in]")].map((el) => Number(el.dataset.in)));
    sel.innerHTML = `<option value="">＋ Add to a collection…</option>`
      + all.filter((c) => !have.has(c.id)).map((c) => `<option value="${c.id}">${esc(c.icon)} ${esc(c.name)}${c.kind === "idea" ? " (idea)" : ""}</option>`).join("")
      + `<option value="new">＋ New collection…</option>`;
  };
  const addTo = async (c) => {
    if (!(await attempt(() => post(`/api/collections/${c.id}/books`, { ids: [bookId] }), `Added to ${c.name}`))) return;
    sel.insertAdjacentHTML("beforebegin", chip(c, true));
    fill();
  };
  fill();
  sel.onchange = () => {
    const v = sel.value;
    sel.value = "";
    if (v === "new") {
      openNewCollection(async (r) => {
        const data = await get("/api/collections").catch(() => null);
        const c = data?.collections.find((x) => x.id === r.id);
        if (c) {
          all.push(c);
          addTo(c);
        }
      });
    } else if (v) {
      addTo(all.find((c) => String(c.id) === v));
    }
  };
  host.addEventListener("click", async (e) => {
    const id = e.target.closest("[data-uncollect]")?.dataset.uncollect;
    if (id && (await attempt(() => del(`/api/collections/${id}/books/${bookId}`), "Taken out"))) {
      host.querySelector(`[data-in="${id}"]`)?.remove();
      fill();
    }
  });
}
