// Unified dashboard: browse and filter books across every catalog.
import { get, post, qs } from "./api.js";
import { $, esc, attempt, toast, canManage, can } from "./ui.js";
import { openCalibreRemoval } from "./calibreremove.js";
import { libbySearchBox } from "./libbysearch.js";
import { openBook } from "./bookdialog.js";
import { openPepperGuide } from "./peppers.js";
import { on } from "./modules.js";
import { loadFlags } from "./customflags.js";
import { loadContent, bindHidePicker } from "./content.js";
import { setupSelect } from "./libraryselect.js";
import { card, IN_QUEUE } from "./librarycard.js";
import { eventShelf } from "./eventshelf.js";
import { recentSearches } from "./recentsearches.js";
import { liveCards } from "./livestatus.js";
import { refreshActivity } from "./activity.js";
import { seasonChipsHTML, bindSeasonChips } from "./seasonchips.js";
import { shelfBanner } from "./collections.js";
import { audioParams } from "./libaudio.js";
import { filtersHTML, bindChips } from "./libfilters.js";
import { combo } from "./combo.js";
import { isBack, saveForm, restoreForm } from "./viewstate.js";
import { markKeptAudio } from "./libby.js";

const PAGE = 60;

export async function renderLibrary(view, state) {
  const manager = canManage(state.user);
  const [catalogs, , facets, , shelves] = await Promise.all([attempt(() => get("/api/catalogs")).then((c) => c || []), loadFlags(true),
    attempt(() => get("/api/books/facets")).then((f) => f || { genres: [], kinds: [], authors: [], series: [], narrators: [] }), loadContent(),
    get("/api/collections").catch(() => ({ collections: [], seasons: [] }))]);
  // A collection, seasonal shelf or event chosen elsewhere (#/library?collection=3, ?season=advent, ?event=5).
  const shelf = new URLSearchParams(location.hash.split("?")[1] || "");
  const event = shelf.get("event") || "";
  const collection = event ? "" : shelf.get("collection") || "";
  const season = collection || event ? "" : shelf.get("season") || "";
  const catOpts = catalogs.map((c) => `<option value="${c.id}">${esc(c.name)} (${c.book_count})</option>`).join("");
  view.innerHTML = `
    ${event ? "" : seasonChipsHTML(shelves.seasons || [], season)}
    <div id="shelf-banner"></div>
    ${filtersHTML({ facets, catOpts, event, state })}
    <div class="mb-3 flex items-center justify-between gap-2"><p id="result-count" class="text-sm text-slate-400"></p>
      <button type="button" id="select-toggle" class="btn-ghost py-1 text-sm" title="Pick several books to add to your Nightstand, Deep Scan or delete">Select</button></div>
    <div id="grid" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"></div>
    <div class="mt-6 text-center"><button id="more-btn" class="btn-secondary hidden">Load more</button></div>`;

  const form = $("#filters", view);
  combo(form.author, () => facets.authors);
  combo(form.series, () => facets.series);
  const redrawChips = bindChips(form);
  form.addEventListener("click", (e) => {
    if (e.target.closest(".fmenu-list")) e.target.closest("details").open = false; // pick an action, the menu closes
  });
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
      ...audioParams(fd),
      sort: fd.get("sort"),
      multi: fd.get("multi") === "on",
      deep: fd.get("deep") === "on",
      not_owned: fd.get("not_owned") === "on",
      libby_audio: fd.get("libby_audio") || "",
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
    if (!data.total) {
      grid.innerHTML = `<p class="text-slate-400">No books match these filters.</p>${form.q.value.trim() && state.user.role !== "restricted" ? `<div class="card mt-3"><p class="mb-2 font-semibold">Look in your Libby libraries</p><div data-lsbox></div></div>` : ""}`;
      const box = grid.querySelector("[data-lsbox]");
      if (box) libbySearchBox(box, form.q.value.trim());
    }
    sel.repaint();
    markKeptAudio(grid); // 🎧 at your library, from what the background check kept
  }

  // Phones show just the search box; "Filters (n)" opens the rest and counts
  // how many are in use, so a hidden filter is never a surprise.
  const toggle = $("#filters-toggle", view);
  const countFilters = () => {
    const fd = new FormData(form);
    const n = [...fd.entries()].filter(([k, v]) => k !== "q" && k !== "sort" && v !== "").length;
    toggle.textContent = n ? `Filters (${n})` : "Filters";
  };
  const setOpen = (open) => {
    form.classList.toggle("filters-open", open);
    toggle.setAttribute("aria-expanded", String(open));
  };
  toggle.addEventListener("click", () => setOpen(!form.classList.contains("filters-open")));
  form.addEventListener("click", (e) => {
    if (e.target.closest("[data-filters-done]")) setOpen(false);
  });
  document.addEventListener("pointerdown", (e) => { // a tap anywhere outside the panel closes it
    if (form.isConnected && form.classList.contains("filters-open") && !form.contains(e.target) && !e.target.closest(".combo-list")) setOpen(false);
  });
  form.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && form.classList.contains("filters-open")) setOpen(false);
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
        toast("Added to your Nightstand", false, { label: "View Nightstand", href: "#/queue" });
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
  for (const k of ["q", "author", "series", "genre", "kind", "catalog", "spice", "narrator", "format", "sort"]) {
    if (preset.get(k) && form.elements[k]) form.elements[k].value = preset.get(k);
  }
  // Back from a book's series, an author's page and so on: the search and filters are as you left them
  // (the chips under the search box show which are on; the panel stays closed).
  if (isBack() && [...preset.keys()].length === 0) restoreForm("library", form);
  form.addEventListener("input", () => saveForm("library", form));
  form.addEventListener("change", () => saveForm("library", form));
  countFilters();
  redrawChips();
  await load(true);
  // Parents see ratings arrive on the cards as the AI works.
  const stopLive = manager ? liveCards(grid, cardFor, () => sel.repaint()) : null;
  if (stopLive) refreshActivity();
  return () => {
    stopLive?.();
    sel.cleanup?.();
  };
}
