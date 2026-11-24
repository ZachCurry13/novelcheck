// The Library's filters, laid out like a bookshop website: on a computer a panel beside the books, always open
// (pepper squares, then short pick lists with counts, then More filters); on a phone the same panel slides up
// from the bottom behind "Filters". Above the books: the search box, Sort, a chip for every filter that is on
// (tap × to remove it), and a "⋯" menu for the rare actions.
import { esc, HIDE_LABELS, FILTER_IDEA_URL, AGE_GROUPS, canManage, can } from "./ui.js";
import { peppersHTML, checksHTML, pillsHTML, sortPillsHTML, pepperLabel } from "./libfacets.js";
import { on } from "./modules.js";
import { hideBoxes } from "./customflags.js";
import { hidePicker } from "./content.js";
import { audioFilterHTML, audioSortHTML } from "./libaudio.js";

const field = (label, control, extra = "") => `<label class="field ${extra}"><span>${esc(label)}</span>${control}</label>`;

// SORTS are the orders a list can have (the event's own order first, on an event).
export const SORTS = (event) => [...(event ? [["list", "Event order"]] : []), ["title", "Title"], ["author", "Author"],
  ["rating", "Highest rated on Hardcover"], ["recent", "Recently added"], ["mild", "Fewest peppers"], ["spicy", "Most peppers"]];

export function filtersHTML({ facets, catOpts, catalogs, event, state }) {
  const manager = canManage(state.user);
  const opt = (v, label, n) => `<option value="${esc(v)}">${esc(label)}${n === undefined ? "" : ` (${n.toLocaleString()})`}</option>`;
  const admin = [
    can(state.user, "calibre") ? `<button type="button" id="remove-btn">Remove hidden books from Calibre…</button>` : "",
    manager ? `<a href="#/duplicates">Find duplicates</a>` : "",
    can(state.user, "calibre") ? `<a href="#/deletions">Delete requests</a>` : "",
    manager ? `<button type="button" id="batch-btn">Analyze the next batch of books</button>` : "",
  ].filter(Boolean).join("");
  return `<form id="filters" class="mb-4 grid gap-3">
    <div class="fbar">
      <input name="q" type="search" placeholder="${event ? "Search this event" : "Search title, author, series or tag"}" class="fsearch" aria-label="Search">
      <select name="sort" class="fsort" aria-label="Sort">
        ${SORTS(event).map(([v, l]) => `<option value="${v}">Sort: ${l}</option>`).join("")}
        ${audioSortHTML(facets)}
      </select>
      <button type="button" id="filters-toggle" class="fbtn" aria-expanded="false">Filters</button>
      ${admin ? `<details class="fmenu"><summary class="btn-secondary" aria-label="More actions" title="More actions">⋯</summary><div class="fmenu-list">${admin}</div></details>` : ""}
    </div>
    <div id="filter-chips" class="fchips" aria-live="polite"></div>
    <aside class="fpanel filter-more" aria-label="Filters">
      <div class="fhead"><b>Filters</b><button type="button" data-clear-all class="facet-link">Clear all</button></div>
      ${peppersHTML()}
      ${checksHTML("Genre", "genre", (facets.genres || []).filter((g) => g.count).map((g) => ({ value: g.key, label: g.label, count: g.count })))}
      ${checksHTML("Library", "catalog", (catalogs || []).map((c) => ({ value: String(c.id), label: c.name, count: c.book_count })))}
      ${on(state.user, "parents") ? pillsHTML("Age group", "age", [["", "Any"], ...AGE_GROUPS.map(([l, n]) => [String(l), n]), ["unset", "Not set"]]) : ""}
      ${sortPillsHTML(event)}
      <details class="facet fmore"><summary><span>More</span><span class="fmore-arrow" aria-hidden="true">▸</span>
        <span class="fmore-sum">Format, series, author, Libby, hide content…</span></summary><div class="fmore-body">
        ${pillsHTML("Fiction or nonfiction", "kind", [["", "Both"], ...(facets.kinds || []).filter((k) => k.count).map((k) => [k.key, k.label])], "facet-sub")}
        ${field("Author", `<input name="author" placeholder="Any author" class="input">`, "combo-field")}
        ${field("Series", `<input name="series" placeholder="Any series" class="input">`, "combo-field")}
        ${field("Also in", `<select name="overlap_with" class="input" title="Only books also in this library"><option value="">Any other library</option>${catOpts}</select>`)}
        ${field("File format", `<select name="format" class="input" title="File format">
          <option value="">Any format</option>
          ${["epub", "azw3", "mobi", "kfx", "pdf"].map((f) => `<option value="${f}">${f.toUpperCase()}</option>`).join("")}
          <option value="paper">Paper books</option>
          ${(facets.narrators || []).length ? `<option value="audio">Audiobooks</option>` : ""}
          <option value="multi">2+ formats</option><option value="dupes">Duplicates</option><option value="none">No file</option></select>`)}
        ${audioFilterHTML(facets)}
        ${field("Books I've read", `<select name="read" class="input"><option value="">Any</option><option value="1">Only books I've read</option><option value="0">Only books I haven't read</option></select>`)}
        ${field("At my Libby library", `<select name="libby_audio" class="input" title="Books you don't have as an audiobook, checked against your libraries in the background"><option value="">Any</option><option value="1">The audiobook is there</option><option value="now">The audiobook is available now</option></select>`)}
        ${field("Rating", `<select name="rating" class="input"><option value="">Any</option>
          <option value="review">Needs review (the AI wasn't sure)</option><option value="old">Older rating (not on pepper scale)</option>
          <option value="Pending">Not rated yet</option><option value="failed">Rating failed</option></select>`)}
        <div class="space-y-2"><h3 class="facet-h" title="Books with these are hidden (unless a parent marked them OK)">Hide content</h3>${hidePicker()}</div>
        <div class="space-y-2"><h3 class="facet-h">Hide books that have…</h3>
          ${Object.entries(HIDE_LABELS).map(([k, v]) => `<label class="toggle"><input type="checkbox" name="hide" value="${k}"> ${esc(v)}</label>`).join("")}
          ${hideBoxes()}
          ${event ? `<label class="toggle"><input type="checkbox" name="not_owned"> Hide books we have</label>` : ""}
          <label class="toggle"><input type="checkbox" name="multi"> Only books in 2+ libraries</label>
          <label class="toggle" title="Books whose whole text was read by the AI"><input type="checkbox" name="deep"> Deep Scanned only</label></div>
        <a href="${FILTER_IDEA_URL}" target="_blank" rel="noopener noreferrer" class="facet-link">Missing a filter? Suggest one</a>
      </div></details>
      <div class="fdone"><button type="button" class="btn-primary" data-filters-done>Show the books</button></div>
    </aside>
  </form>`;
}

const LABELS = { genre: "Genre", kind: "Type", author: "Author", series: "Series", catalog: "Library", overlap_with: "Also in", spice: "Peppers", age: "Age", rating: "Rating", lf: "At my library",
  format: "Format", read: "Read", libby_audio: "Libby", narrator: "Narrator", min_hours: "At least (hours)", max_hours: "At most (hours)" };

// bindChips keeps a chip for each filter that is on, under the search bar; × clears it.
export function bindChips(form, onChange) {
  const box = form.querySelector("#filter-chips");
  const chips = [];
  const draw = () => {
    chips.length = 0;
    for (const el of form.elements) {
      if (!el.name || el.name === "q" || el.name === "sort" || el.type === "button") continue;
      if (el.type === "checkbox") {
        const what = el.closest("label")?.querySelector("[data-l]")?.textContent.trim() || el.closest("label")?.textContent.trim() || el.value;
        if (el.checked) chips.push({ el, text: `${el.name === "hide" ? "Hide: " : ""}${what}` });
      } else if (el.type === "radio") {
        if (el.name !== "sortp" && el.checked && el.value !== "") chips.push({ el, text: `${LABELS[el.name] || el.name}: ${el.closest("label")?.textContent.trim() || el.value}` });
      } else if (el.name === "spice") {
        if (el.value) chips.push({ el, text: pepperLabel(el.value) });
      } else if (el.value !== "") {
        const shown = el.tagName === "SELECT" ? el.selectedOptions[0]?.textContent.replace(/\s*\([\d,]+\)$/, "").trim() : el.value;
        chips.push({ el, text: `${LABELS[el.name] || el.name}: ${shown}` });
      }
    }
    box.innerHTML = chips.map((c, i) => `<button type="button" class="fchip" data-chip="${i}" title="Remove this filter">${esc(c.text)} <span aria-hidden="true">×</span></button>`).join("");
  };
  box.addEventListener("click", (e) => {
    const c = chips[Number(e.target.closest("[data-chip]")?.dataset.chip)];
    if (!c) return;
    if (c.el.type === "checkbox") c.el.checked = false;
    else if (c.el.type === "radio") c.el.form.querySelector(`[name="${c.el.name}"][value=""]`).checked = true;
    else c.el.value = "";
    c.el.dispatchEvent(new Event("input", { bubbles: true }));
  });
  form.addEventListener("input", draw);
  form.addEventListener("change", draw);
  draw();
  return draw;
}
