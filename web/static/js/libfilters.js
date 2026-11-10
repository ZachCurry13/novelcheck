// The Library's filter bar: one search box with Sort, a Filters button that opens a panel (grouped and
// labelled, full-width fields), a chip for every filter that is on (tap × to remove it), and a "⋯" menu
// for the rare actions. The common case is a clean single row.
import { esc, HIDE_LABELS, FILTER_IDEA_URL, AGE_GROUPS, canManage, can } from "./ui.js";
import { pepperOptions } from "./peppers.js";
import { on } from "./modules.js";
import { hideBoxes } from "./customflags.js";
import { hidePicker } from "./content.js";
import { audioFilterHTML, audioSortHTML } from "./libaudio.js";

const field = (label, control, extra = "") => `<label class="field ${extra}"><span>${esc(label)}</span>${control}</label>`;

export function filtersHTML({ facets, catOpts, event, state }) {
  const manager = canManage(state.user);
  const opt = (v, label, n) => `<option value="${esc(v)}">${esc(label)}${n === undefined ? "" : ` (${n.toLocaleString()})`}</option>`;
  const admin = [
    can(state.user, "calibre") ? `<button type="button" id="remove-btn">Remove hidden books from Calibre…</button>` : "",
    manager ? `<a href="#/duplicates">Find duplicates</a>` : "",
    can(state.user, "calibre") ? `<a href="#/deletions">Delete requests</a>` : "",
    manager ? `<button type="button" id="batch-btn">Analyze the next batch of books</button>` : "",
  ].filter(Boolean).join("");
  return `<form id="filters" class="card mb-4 grid gap-3">
    <div class="fbar">
      <input name="q" type="search" placeholder="Search title, author, series or tag" class="input min-w-0 flex-1" aria-label="Search">
      <select name="sort" class="input fsort" aria-label="Sort">
        ${event ? `<option value="list">The event's order</option>` : ""}
        <option value="title">Title</option><option value="author">Author (last name)</option>
        <option value="recent">Recently added</option><option value="mild">Fewest peppers</option>
        ${audioSortHTML(facets)}
      </select>
      <button type="button" id="filters-toggle" class="btn-secondary" aria-expanded="false">Filters</button>
      ${admin ? `<details class="fmenu"><summary class="btn-secondary" aria-label="More actions" title="More actions">⋯</summary><div class="fmenu-list">${admin}</div></details>` : ""}
    </div>
    <div id="filter-chips" class="fchips" aria-live="polite"></div>
    <div class="fpanel filter-more">
      <div class="fgroup"><h3>Find</h3>
        ${field("Genre", `<select name="genre" class="input"><option value="">Any genre</option>${(facets.genres || []).map((g) => opt(g.key, g.label, g.count)).join("")}</select>`)}
        ${field("Fiction or nonfiction", `<select name="kind" class="input"><option value="">Both</option>${(facets.kinds || []).filter((k) => k.count).map((k) => opt(k.key, k.label, k.count)).join("")}</select>`)}
        ${field("Author", `<input name="author" placeholder="Any author" class="input">`, "combo-field")}
        ${field("Series", `<input name="series" placeholder="Any series" class="input">`, "combo-field")}
      </div>
      <div class="fgroup"><h3>Where it is</h3>
        ${field("Library", `<select name="catalog" class="input"><option value="">All libraries</option>${catOpts}</select>`)}
        ${field("Also in", `<select name="overlap_with" class="input" title="Only books also in this library"><option value="">Any other library</option>${catOpts}</select>`)}
        ${field("File format", `<select name="format" class="input" title="File format">
          <option value="">Any format</option>
          ${["epub", "azw3", "mobi", "kfx", "pdf"].map((f) => `<option value="${f}">${f.toUpperCase()}</option>`).join("")}
          <option value="paper">📕 Paper books</option>
          ${(facets.narrators || []).length ? `<option value="audio">🎧 Audiobooks</option>` : ""}
          <option value="multi">2+ formats</option><option value="dupes">Duplicates</option><option value="none">No file</option></select>`)}
        ${audioFilterHTML(facets)}
        ${field("At my Libby library", `<select name="libby_audio" class="input" title="Books you don't have as an audiobook, checked against your libraries in the background"><option value="">Any</option><option value="1">🎧 The audiobook is there</option><option value="now">🎧 The audiobook is available now</option></select>`)}
      </div>
      <div class="fgroup"><h3>Content</h3>
        ${field("Peppers", `<select name="spice" class="input" title="Peppers: how much romance and sexual content">
          <option value="">Any peppers</option>${pepperOptions(null)}
          <option value="review">⚠ Needs review (the AI wasn't sure)</option>
          <option value="old">Older rating (not on pepper scale)</option><option value="Pending">Not rated yet</option><option value="failed">Rating failed</option></select>`)}
        ${field("Age group", `<select name="age" class="input" title="Books a parent rated for this age group or younger">
          <option value="">Any age group</option>
          ${AGE_GROUPS.map(([l, n, r]) => `<option value="${l}">Suitable for ${n} (${r})</option>`).join("")}
          <option value="unset">Age group not set yet</option></select>`, on(state.user, "parents") ? "" : "module-off")}
      </div>
      <details class="fgroup fwide fmore"><summary>More: hide content and other options</summary>
      <div class="fgroup"><h3 title="Books with these are hidden (unless a parent marked them OK)">Hide content</h3>
        ${hidePicker()}
      </div>
      <div class="fgroup"><h3>Hide books that have…</h3>
        <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
          ${Object.entries(HIDE_LABELS).map(([k, v]) => `<label class="toggle"><input type="checkbox" name="hide" value="${k}"> ${esc(v)}</label>`).join("")}
          ${hideBoxes()}
          ${event ? `<label class="toggle"><input type="checkbox" name="not_owned"> Hide books we have</label>` : ""}
          <label class="toggle"><input type="checkbox" name="multi"> Only books in 2+ libraries</label>
          <label class="toggle" title="Books whose whole text was read by the AI"><input type="checkbox" name="deep"> 🧬 Deep Scanned only</label>
        </div>
        <div class="flex flex-wrap gap-4 text-xs"><button type="button" id="pepper-help" class="text-slate-400 underline">🌶️ What do the peppers mean?</button>
          <a href="${FILTER_IDEA_URL}" target="_blank" rel="noopener noreferrer" class="text-slate-500 underline">Missing a filter? Suggest one</a></div>
      </div>
      </details>
      <div class="fgroup fwide fdone"><button type="button" class="btn-primary" data-filters-done>Done</button></div>
    </div>
  </form>`;
}

const LABELS = { genre: "Genre", kind: "Type", author: "Author", series: "Series", catalog: "Library", overlap_with: "Also in", spice: "Peppers", age: "Age",
  format: "Format", libby_audio: "Libby", narrator: "Narrator", min_hours: "At least (hours)", max_hours: "At most (hours)" };

// bindChips keeps a chip for each filter that is on, under the search bar; × clears it.
export function bindChips(form, onChange) {
  const box = form.querySelector("#filter-chips");
  const chips = [];
  const draw = () => {
    chips.length = 0;
    for (const el of form.elements) {
      if (!el.name || el.name === "q" || el.name === "sort" || el.type === "button") continue;
      if (el.type === "checkbox") {
        if (el.checked) chips.push({ el, text: `${el.name === "hide" ? "Hide: " : ""}${el.closest("label")?.textContent.trim() || el.value}` });
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
    else c.el.value = "";
    c.el.dispatchEvent(new Event("input", { bubbles: true }));
  });
  form.addEventListener("input", draw);
  form.addEventListener("change", draw);
  draw();
  return draw;
}
