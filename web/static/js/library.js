// Unified dashboard: browse and filter books across every catalog.
import { get, post, qs } from "./api.js";
import { $, esc, attempt, toast, HIDE_LABELS, FILTER_IDEA_URL, AGE_GROUPS, canManage } from "./ui.js";
import { openCalibreRemoval } from "./calibreremove.js";
import { openBook } from "./bookdialog.js";
import { pepperOptions, openPepperGuide } from "./peppers.js";
import { on } from "./modules.js";
import { loadFlags, hideBoxes } from "./customflags.js";
import { loadContent, hidePicker, bindHidePicker } from "./content.js";
import { setupSelect } from "./libraryselect.js";
import { card, IN_QUEUE } from "./librarycard.js";
import { eventShelf } from "./eventshelf.js";
import { recentSearches } from "./recentsearches.js";
import { liveCards } from "./livestatus.js";
import { refreshActivity } from "./activity.js";
import { seasonChipsHTML, bindSeasonChips } from "./seasonchips.js";
import { shelfBanner } from "./collections.js";

const PAGE = 60;

export async function renderLibrary(view, state) {
  const manager = canManage(state.user);
  const [catalogs, , facets, , shelves] = await Promise.all([attempt(() => get("/api/catalogs")).then((c) => c || []), loadFlags(true),
    attempt(() => get("/api/books/facets")).then((f) => f || { genres: [], kinds: [], authors: [], series: [] }), loadContent(),
    get("/api/collections").catch(() => ({ collections: [], seasons: [] }))]);
  // A collection, seasonal shelf or event chosen elsewhere (#/library?collection=3, ?season=advent, ?event=5).
  const shelf = new URLSearchParams(location.hash.split("?")[1] || "");
  const event = shelf.get("event") || "";
  const collection = event ? "" : shelf.get("collection") || "";
  const season = collection || event ? "" : shelf.get("season") || "";
  const catOpts = catalogs.map((c) => `<option value="${c.id}">${esc(c.name)} (${c.book_count})</option>`).join("");
  const opt = (v, label, n) => `<option value="${esc(v)}">${esc(label)}${n === undefined ? "" : ` (${n.toLocaleString()})`}</option>`;
  view.innerHTML = `
    ${event ? "" : seasonChipsHTML(shelves.seasons || [], season)}
    <div id="shelf-banner"></div>
    <form id="filters" class="card mb-4 grid gap-3 md:grid-cols-4 xl:grid-cols-8">
      <div class="flex gap-2 md:col-span-2">
        <input name="q" type="search" placeholder="Search title, author, series or tag" class="input min-w-0 flex-1">
        <button type="button" id="filters-toggle" class="btn-secondary filters-toggle" aria-expanded="false">Filters</button>
      </div>
      <select name="genre" class="input filter-more"><option value="">Any genre</option>${(facets.genres || []).map((g) => opt(g.key, g.label, g.count)).join("")}</select>
      <select name="kind" class="input filter-more"><option value="">Fiction &amp; nonfiction</option>${(facets.kinds || []).filter((k) => k.count).map((k) => opt(k.key, k.label, k.count)).join("")}</select>
      <input name="author" list="author-list" placeholder="Author" class="input filter-more" autocomplete="off">
      <input name="series" list="series-list" placeholder="Series" class="input filter-more" autocomplete="off">
      <datalist id="author-list">${(facets.authors || []).map((a) => `<option value="${esc(a)}"></option>`).join("")}</datalist>
      <datalist id="series-list">${(facets.series || []).map((s) => `<option value="${esc(s)}"></option>`).join("")}</datalist>
      <select name="catalog" class="input filter-more"><option value="">All libraries</option>${catOpts}</select>
      <select name="overlap_with" class="input filter-more" title="Only books also in this library">
        <option value="">…also in (overlap)</option>${catOpts}</select>
      <select name="spice" class="input filter-more" title="Peppers: how much romance and sexual content">
        <option value="">Any peppers</option>${pepperOptions(null)}
        <option value="review">⚠ Needs review (the AI wasn't sure)</option>
        <option value="old">Older rating (not on pepper scale)</option><option value="Pending">Not rated yet</option>
        <option value="failed">Rating failed</option>
      </select>
      <select name="age" class="input filter-more${on(state.user, "parents") ? "" : " module-off"}" title="Books a parent rated for this age group or younger">
        <option value="">Any age group</option>
        ${AGE_GROUPS.map(([l, n, r]) => `<option value="${l}">Suitable for ${n} (${r})</option>`).join("")}
        <option value="unset">Age group not set yet</option>
      </select>
      <select name="format" class="input filter-more" title="File format">
        <option value="">Any format</option>
        ${["epub", "azw3", "mobi", "kfx", "pdf"].map((f) => `<option value="${f}">${f.toUpperCase()}</option>`).join("")}
        <option value="paper">📕 Paper books</option>
        <option value="multi">2+ formats</option>
        <option value="dupes">Duplicates</option>
        <option value="none">No file</option>
      </select>
      <select name="sort" class="input filter-more">
        ${event ? `<option value="list">Sort: The event's order</option>` : ""}
        <option value="title">Sort: Title</option><option value="author">Sort: Author (last name)</option>
        <option value="recent">Sort: Recently added</option><option value="mild">Sort: Fewest peppers</option>
      </select>
      <div class="filter-more col-span-full"><span class="label" title="Books with these are hidden (unless a parent marked them OK)">Hide content</span>
        ${hidePicker()}</div>
      <div class="filter-more col-span-full flex flex-wrap items-center gap-x-5 gap-y-2">
        <span class="label mb-0" title="Books with these are hidden (unless a parent marked them OK)">Hide:</span>
        ${Object.entries(HIDE_LABELS).map(([k, v]) =>
          `<label class="toggle"><input type="checkbox" name="hide" value="${k}"> ${esc(v)}</label>`).join("")}
        ${hideBoxes()}
        ${event ? `<label class="toggle"><input type="checkbox" name="not_owned"> Hide books we have</label>` : ""}
        <label class="toggle"><input type="checkbox" name="multi"> Only books in 2+ libraries</label>
        <label class="toggle" title="Books whose whole text was read by the AI"><input type="checkbox" name="deep"> 🧬 Deep Scanned only</label>
        <button type="button" id="pepper-help" class="text-xs text-slate-400 underline">🌶️ What do the peppers mean?</button>
        <a href="${FILTER_IDEA_URL}" target="_blank" rel="noopener noreferrer" class="text-xs text-slate-500 underline">Missing a filter? Suggest one</a>
        <span class="ml-auto flex flex-wrap gap-2">
          ${state.user.role === "admin" ? `<button type="button" id="remove-btn" class="btn-ghost text-xs">Remove hidden books from Calibre…</button>` : ""}
          ${manager ? `<a href="#/duplicates" class="btn-ghost text-xs">Find duplicates</a>` : ""}
          ${state.user.role === "admin" ? `<a href="#/deletions" class="btn-ghost text-xs">Delete requests</a>` : ""}
          ${manager ? `<button type="button" id="batch-btn" class="btn-secondary">Analyze next batch</button>` : ""}
        </span>
      </div>
    </form>
    <div class="mb-3 flex items-center justify-between gap-2"><p id="result-count" class="text-sm text-slate-400"></p>
      <button type="button" id="select-toggle" class="btn-ghost py-1 text-sm" title="Pick several books to add to Up Next, Deep Scan or delete">☑ Select</button></div>
    <div id="grid" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"></div>
    <div class="mt-6 text-center"><button id="more-btn" class="btn-secondary hidden">Load more</button></div>`;

  const form = $("#filters", view);
  const recent = recentSearches(form.q, () => load(true));
  if (!event) bindSeasonChips(view.querySelector("[data-season-chips]"));
  if (collection || season) shelfBanner($("#shelf-banner", view), { collection, season, seasons: shelves.seasons }, state, () => load(true));
  const evShelf = event ? await eventShelf($("#shelf-banner", view), event, state, () => load(true)) : null;
  const cardFor = (b) => card(b, on(state.user, "queue"), evShelf?.extra(b) || "");
  const grid = $("#grid", view);
  const sel = setupSelect(view, grid, state, () => load(true), collection || season ? { collection_id: Number(collection) || 0, season } : null);
  let offset = 0;

  function params() {
    const fd = new FormData(form);
    return {
      q: fd.get("q"),
      catalog: fd.get("catalog"),
      overlap_with: fd.get("overlap_with"),
      spice: ["Pending", "failed"].includes(fd.get("spice")) ? "" : fd.get("spice"),
      status: fd.get("spice") === "failed" ? "error" : "",
      classification: fd.get("spice") === "Pending" ? "Pending" : "",
      age: fd.get("age"),
      format: fd.get("format"),
      genre: fd.get("genre"),
      kind: fd.get("kind"),
      author: fd.get("author"),
      series: fd.get("series"),
      sort: fd.get("sort"),
      multi: fd.get("multi") === "on",
      deep: fd.get("deep") === "on",
      not_owned: fd.get("not_owned") === "on",
      event,
      exclude: fd.getAll("hide").join(","),
      collection,
      season,
      limit: PAGE,
    };
  }

  async function load(reset) {
    if (reset) offset = 0;
    const data = await attempt(() => get("/api/books" + qs({ ...params(), offset })));
    if (!data) return;
    if (reset) grid.innerHTML = "";
    grid.insertAdjacentHTML("beforeend", data.books.map(cardFor).join(""));
    offset += data.books.length;
    if (reset && data.total) recent.remember(form.q.value);
    $("#result-count", view).textContent = `${data.total.toLocaleString()} book${data.total === 1 ? "" : "s"}`;
    $("#more-btn", view).classList.toggle("hidden", offset >= data.total);
    if (!data.total) grid.innerHTML = `<p class="text-slate-400">No books match these filters.</p>`;
    sel.repaint();
  }

  // Phones show just the search box; "Filters (n)" opens the rest and counts
  // how many are in use, so a hidden filter is never a surprise.
  const toggle = $("#filters-toggle", view);
  const countFilters = () => {
    const fd = new FormData(form);
    const n = [...fd.entries()].filter(([k, v]) => k !== "q" && k !== "sort" && v !== "").length;
    toggle.textContent = n ? `Filters (${n})` : "Filters";
  };
  toggle.addEventListener("click", () => {
    toggle.setAttribute("aria-expanded", String(form.classList.toggle("filters-open")));
  });

  let debounce;
  form.addEventListener("input", () => {
    countFilters();
    clearTimeout(debounce);
    debounce = setTimeout(() => load(true), 250);
  });
  form.addEventListener("submit", (e) => e.preventDefault());
  bindHidePicker(form);
  $("#more-btn", view).addEventListener("click", () => load(false));
  grid.addEventListener("click", async (e) => {
    if (evShelf && e.target.closest("[data-ev]")) return void evShelf.click(e); // an event's Claim buttons
    const picked = e.target.closest("[data-book]");
    if (picked && sel.clicked(picked, e)) return; // selecting several books
    const q = e.target.closest("[data-queue]");
    if (q) {
      e.stopPropagation();
      if (await attempt(() => post("/api/queue", { book_id: Number(q.dataset.queue) }))) {
        q.outerHTML = IN_QUEUE;
        toast("Added to Up Next", false, { label: "View Up Next", href: "#/queue" });
      }
      return;
    }
    const c = e.target.closest("[data-book]");
    if (c) openBook(Number(c.dataset.book), state, () => load(true));
  });
  $("#pepper-help", view).addEventListener("click", openPepperGuide);
  $("#remove-btn", view)?.addEventListener("click", () => {
    const fd = new FormData(form);
    openCalibreRemoval({ hide: fd.getAll("hide").join(","), q: fd.get("q"), classification: fd.get("spice") === "Pending" ? "Pending" : "" },
      () => setTimeout(() => load(true), 1500));
  });
  const batch = $("#batch-btn", view);
  if (batch) {
    batch.addEventListener("click", async () => {
      const r = await attempt(() => post("/api/admin/analyze-batch"));
      if (!r) return;
      toast(`Queued ${r.queued} books for rating`);
      setTimeout(() => load(true), 400);
      refreshActivity();
    });
  }
  // "Show in Library" from Rating errors opens this page filtered to failures.
  try {
    if (sessionStorage.getItem("nc:libfilter") === "failed") form.spice.value = "failed";
    sessionStorage.removeItem("nc:libfilter");
  } catch {
    /* private mode */
  }
  // Links such as #/library?author=… (from a book's window) open with that filter set.
  const preset = new URLSearchParams(location.hash.split("?")[1] || "");
  for (const k of ["q", "author", "series", "genre", "kind", "catalog", "spice"]) {
    if (preset.get(k) && form.elements[k]) form.elements[k].value = preset.get(k);
  }
  if ([...preset.keys()].some((k) => !["q", "collection", "season", "event"].includes(k))) form.classList.add("filters-open");
  countFilters();
  await load(true);
  // Parents see ratings arrive on the cards as the AI works.
  const stopLive = manager ? liveCards(grid, cardFor, () => sel.repaint()) : null;
  if (stopLive) refreshActivity();
  return () => {
    stopLive?.();
    sel.cleanup?.();
  };
}
