// Unified dashboard: browse and filter books across every catalog.
import { get, post, qs } from "./api.js";
import { $, esc, attempt, toast, classChip, flagChips, ageChip, HIDE_LABELS, FILTER_IDEA_URL, AGE_GROUPS, canManage } from "./ui.js";
import { openCalibreRemoval } from "./calibreremove.js";
import { openBook } from "./bookdialog.js";
import { pepperOptions, openPepperGuide, whyChip } from "./peppers.js";
import { on } from "./modules.js";
import { loadFlags, customChips, hideBoxes } from "./customflags.js";
import { deepChip } from "./deepscan.js";
import { seriesText } from "./titlefix.js";
import { coverImg } from "./covers.js";

const PAGE = 60;

export async function renderLibrary(view, state) {
  const manager = canManage(state.user);
  const [catalogs, , facets] = await Promise.all([attempt(() => get("/api/catalogs")).then((c) => c || []), loadFlags(true),
    attempt(() => get("/api/books/facets")).then((f) => f || { genres: [], kinds: [], authors: [], series: [] })]);
  const catOpts = catalogs.map((c) => `<option value="${c.id}">${esc(c.name)} (${c.book_count})</option>`).join("");
  const opt = (v, label, n) => `<option value="${esc(v)}">${esc(label)}${n === undefined ? "" : ` (${n.toLocaleString()})`}</option>`;
  view.innerHTML = `
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
      <select name="catalog" class="input filter-more"><option value="">All catalogs</option>${catOpts}</select>
      <select name="overlap_with" class="input filter-more" title="Only books also present in this catalog">
        <option value="">…also in (overlap)</option>${catOpts}</select>
      <select name="spice" class="input filter-more" title="Peppers: how much romance and sexual content">
        <option value="">Any peppers</option>${pepperOptions(null)}
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
        <option value="multi">2+ formats</option>
        <option value="dupes">Duplicates</option>
        <option value="none">No file</option>
      </select>
      <select name="sort" class="input filter-more">
        <option value="title">Sort: Title</option><option value="author">Sort: Author</option>
        <option value="recent">Sort: Recently added</option>
      </select>
      <div class="filter-more col-span-full flex flex-wrap items-center gap-x-5 gap-y-2">
        <span class="label mb-0" title="Books with these are hidden (unless a parent marked them OK)">Hide:</span>
        ${Object.entries(HIDE_LABELS).map(([k, v]) =>
          `<label class="toggle"><input type="checkbox" name="hide" value="${k}"> ${esc(v)}</label>`).join("")}
        ${hideBoxes()}
        <label class="toggle"><input type="checkbox" name="multi"> Only books in 2+ catalogs</label>
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
    <p id="result-count" class="mb-3 text-sm text-slate-400"></p>
    <div id="grid" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"></div>
    <div class="mt-6 text-center"><button id="more-btn" class="btn-secondary hidden">Load more</button></div>`;

  const form = $("#filters", view);
  const grid = $("#grid", view);
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
      exclude: fd.getAll("hide").join(","),
      limit: PAGE,
    };
  }

  async function load(reset) {
    if (reset) offset = 0;
    const data = await attempt(() => get("/api/books" + qs({ ...params(), offset })));
    if (!data) return;
    if (reset) grid.innerHTML = "";
    grid.insertAdjacentHTML("beforeend", data.books.map((b) => card(b, on(state.user, "queue"))).join(""));
    offset += data.books.length;
    $("#result-count", view).textContent = `${data.total.toLocaleString()} book${data.total === 1 ? "" : "s"}`;
    $("#more-btn", view).classList.toggle("hidden", offset >= data.total);
    if (!data.total) grid.innerHTML = `<p class="text-slate-400">No books match these filters.</p>`;
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
  $("#more-btn", view).addEventListener("click", () => load(false));
  grid.addEventListener("click", async (e) => {
    const q = e.target.closest("[data-queue]");
    if (q) {
      e.stopPropagation();
      await attempt(() => post("/api/queue", { book_id: Number(q.dataset.queue) }), "Added to Up Next");
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
      toast(`Queued ${r.queued} books for analysis`);
      setTimeout(() => load(true), 400);
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
  for (const k of ["q", "author", "series", "genre", "kind"]) {
    if (preset.get(k) && form.elements[k]) form.elements[k].value = preset.get(k);
  }
  if ([...preset.keys()].some((k) => k !== "q")) form.classList.add("filters-open");
  countFilters();
  await load(true);
}

function card(b, queueOn) {
  const cats = b.catalogs ? b.catalogs.split(", ").map((c) => `<span class="chip-cat">${esc(c)}</span>`).join(" ") : "";
  return `
    <article data-book="${b.id}" class="card cursor-pointer transition hover:ring-indigo-600 flex flex-col gap-2">
      <div class="flex items-start justify-between gap-3">
        ${coverImg(b.id, "h-24 w-16")}
        <div class="min-w-0 flex-1">
          <h3 class="font-semibold leading-tight line-clamp-2">${esc(b.title)}</h3>
          <p class="text-sm text-slate-400 truncate">${esc(b.author || "Unknown author")}${seriesText(b) ? ` · <span class="text-sky-300">${esc(seriesText(b))}</span>` : ""}</p>
        </div>
        ${queueOn ? `<button data-queue="${b.id}" title="Add to Up Next" class="btn-ghost px-2 py-1 text-lg">＋</button>` : ""}
      </div>
      <div class="flex flex-wrap gap-1">${classChip(b)} ${deepChip(b)} ${whyChip(b)} ${ageChip(b)} ${flagChips(b)} ${customChips(b)}</div>
      ${b.summary_verdict ? `<p class="text-sm text-slate-300 line-clamp-3">${esc(b.summary_verdict)}</p>` : ""}
      <div class="mt-auto flex flex-wrap items-center gap-1">${cats} ${formatChips(b)}</div>
    </article>`;
}

// File formats (EPUB, AZW3…) and a warning when Calibre has the book twice.
export function formatChips(b) {
  const fmts = b.formats ? b.formats.split(",").map((f) => `<span class="chip-fmt">${esc(f)}</span>`).join(" ") : "";
  const del = b.delete_requests ? `<span class="chip-dup" title="Someone asked to delete this book">🗑 Delete requested</span>` : "";
  const dup = b.calibre_copies > 1
    ? `<span class="chip-dup" title="This book is in Calibre ${b.calibre_copies} times">⚠ ${b.calibre_copies}× in Calibre</span>` : "";
  return `${fmts} ${dup} ${del}`;
}
