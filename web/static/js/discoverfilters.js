// Discover's filters: the same side panel as the Library (pepper squares, genres as tick boxes), plus what
// only Discover has: what your Libby library lends, and whether to show books the family already has. They
// narrow every list on the page at once; a list with nothing left is folded away.
import { esc } from "./ui.js";
import { GENRES } from "./genres.js";
import { peppersHTML, checksHTML, bindFacets } from "./libfacets.js";
import { bindChips } from "./libfilters.js";

export function discoverFiltersHTML(owned) {
  return `<form id="filters" class="mb-4 grid gap-3" autocomplete="off">
    <div class="fbar"><input name="q" type="search" placeholder="Search these lists" class="fsearch" aria-label="Search these lists">
      <button type="button" id="filters-toggle" class="fbtn" aria-expanded="false">Filters</button></div>
    <div id="filter-chips" class="fchips" aria-live="polite"></div>
    <aside class="fpanel filter-more" aria-label="Filters">
      <div class="fhead"><b>Filters</b><button type="button" data-clear-all class="facet-link">Clear all</button></div>
      ${peppersHTML()}
      <div data-genre-host></div>
      <section class="facet hidden" data-libby-host><h3>At my library <span id="lf-status" class="facet-sum"></span></h3>
        <div class="pills">${[["", "All"], ["here", "Lends it"], ["audio", "Audiobook"], ["now", "Available now"]].map(([v, l], i) =>
          `<label class="pill"><input type="radio" name="lf" value="${v}"${i ? "" : " checked"}><span>${l}</span></label>`).join("")}</div></section>
      <section class="facet"><label class="facet-row"><input type="checkbox" id="show-owned" ${owned ? "checked" : ""}>
        <span class="facet-box" aria-hidden="true"></span><span class="facet-l" data-l>Also show books we already have</span><span id="owned-count" class="facet-n"></span></label></section>
      <div class="fdone"><button type="button" class="btn-primary" data-filters-done>Show the books</button></div>
    </aside></form>`;
}

const levels = (v) => String(v || "").split(",").filter((x) => /^[0-5]$/.test(x)).map(Number);

// bindDiscoverFilters wires the panel to the lists in rows; refill() is called after the lists are drawn
// (the genres to offer come from the books shown).
export function bindDiscoverFilters(form, rows, count) {
  bindChips(form);
  const paint = bindFacets(form);
  const toggle = form.querySelector("#filters-toggle");
  const setOpen = (open) => {
    form.classList.toggle("filters-open", open);
    toggle.setAttribute("aria-expanded", String(open));
  };
  toggle.addEventListener("click", () => setOpen(!form.classList.contains("filters-open")));
  form.addEventListener("click", (e) => {
    if (e.target.closest("[data-filters-done]")) setOpen(false);
  });
  form.addEventListener("submit", (e) => e.preventDefault());

  const apply = () => {
    const fd = new FormData(form);
    const q = String(fd.get("q") || "").trim().toLowerCase();
    const lv = levels(fd.get("spice"));
    const gs = fd.getAll("genre");
    let shown = 0;
    rows.querySelectorAll("[data-book]").forEach((c) => {
      const lvl = c.dataset.lvl === "" ? null : Number(c.dataset.lvl);
      const ok = (!q || c.dataset.text.includes(q)) && (!lv.length || lv.includes(lvl))
        && (!gs.length || gs.some((g) => c.dataset.genres.includes(`,${g},`)));
      c.toggleAttribute("data-out", !ok);
      if (ok && !c.classList.contains("hidden")) shown++;
    });
    rows.querySelectorAll("section").forEach((s) => s.toggleAttribute("data-out", ![...s.querySelectorAll("[data-book]")].some((c) => !c.hasAttribute("data-out") && !c.classList.contains("hidden"))));
    const n = [...fd.entries()].filter(([k, v]) => !["q", "lf"].includes(k) && v !== "").length + (fd.get("lf") ? 1 : 0);
    toggle.textContent = n ? `Filters · ${n}` : "Filters";
    const total = rows.querySelectorAll("[data-book]").length;
    count.textContent = shown === total ? `${total} books` : `${shown} of ${total} books`;
    const done = form.querySelector("[data-filters-done]");
    if (done) done.textContent = shown ? `Show ${shown} book${shown === 1 ? "" : "s"}` : "No books match: change a filter";
  };
  form.addEventListener("input", apply);
  form.addEventListener("change", apply);

  // refill offers the genres of the books on the page, most common first, keeping what was ticked.
  const refill = () => {
    const ticked = new FormData(form).getAll("genre");
    const n = new Map();
    rows.querySelectorAll("[data-book]").forEach((c) => c.dataset.genres.split(",").filter((g) => GENRES[g]).forEach((g) => n.set(g, (n.get(g) || 0) + 1)));
    const items = [...n].sort((a, b) => b[1] - a[1]).map(([g, k]) => ({ value: g, label: GENRES[g], count: k }));
    form.querySelector("[data-genre-host]").innerHTML = checksHTML("Genre", "genre", items);
    form.querySelectorAll('input[name="genre"]').forEach((b) => (b.checked = ticked.includes(b.value)));
    paint();
    apply();
  };
  return { apply, refill, libbyHost: form.querySelector("[data-libby-host]") };
}

// cardData is what a Discover card carries for the filters: its level, genres and words to search.
export const cardData = (b) => {
  const rated = b.status === "analyzed" && b.spice_level !== null && b.spice_level !== undefined;
  return `data-lvl="${rated ? b.spice_level : ""}" data-genres=",${esc((b.genres || "").replace(/^,|,$/g, ""))}," data-text="${esc(`${b.title} ${b.author || ""} ${b.series || ""}`.toLowerCase())}"`;
};
