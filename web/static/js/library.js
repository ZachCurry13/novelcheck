// Unified dashboard: browse and filter books across every catalog.
import { get, post, qs } from "./api.js";
import { $, esc, attempt, toast, classChip, flagChips, ageChip, HIDE_LABELS, FILTER_IDEA_URL, AGE_GROUPS, canManage } from "./ui.js";
import { openCalibreRemoval } from "./calibreremove.js";
import { openBook } from "./bookdialog.js";
import { pepperOptions, openPepperGuide } from "./peppers.js";

const PAGE = 60;

export async function renderLibrary(view, state) {
  const manager = canManage(state.user);
  const catalogs = (await attempt(() => get("/api/catalogs"))) || [];
  const catOpts = catalogs.map((c) => `<option value="${c.id}">${esc(c.name)} (${c.book_count})</option>`).join("");
  view.innerHTML = `
    <form id="filters" class="card mb-4 grid gap-3 md:grid-cols-4 xl:grid-cols-8">
      <input name="q" type="search" placeholder="Search title or author" class="input md:col-span-2">
      <select name="catalog" class="input"><option value="">All catalogs</option>${catOpts}</select>
      <select name="overlap_with" class="input" title="Only books also present in this catalog">
        <option value="">…also in (overlap)</option>${catOpts}</select>
      <select name="spice" class="input" title="Peppers: how much romance and sexual content">
        <option value="">Any peppers</option>${pepperOptions(null)}
        <option value="old">Older rating (not on pepper scale)</option><option value="Pending">Not rated yet</option>
      </select>
      <select name="age" class="input" title="Books a parent rated for this age group or younger">
        <option value="">Any age group</option>
        ${AGE_GROUPS.map(([l, n, r]) => `<option value="${l}">Suitable for ${n} (${r})</option>`).join("")}
        <option value="unset">Age group not set yet</option>
      </select>
      <select name="format" class="input" title="File format">
        <option value="">Any format</option>
        ${["epub", "azw3", "mobi", "kfx", "pdf"].map((f) => `<option value="${f}">${f.toUpperCase()}</option>`).join("")}
        <option value="multi">2+ formats</option>
        <option value="dupes">Duplicates</option>
        <option value="none">No file</option>
      </select>
      <select name="sort" class="input">
        <option value="title">Sort: Title</option><option value="author">Sort: Author</option>
        <option value="recent">Sort: Recently added</option>
      </select>
      <div class="col-span-full flex flex-wrap items-center gap-x-5 gap-y-2">
        <span class="label mb-0" title="Books with these are hidden (unless a parent marked them OK)">Hide:</span>
        ${Object.entries(HIDE_LABELS).map(([k, v]) =>
          `<label class="toggle"><input type="checkbox" name="hide" value="${k}"> ${esc(v)}</label>`).join("")}
        <label class="toggle"><input type="checkbox" name="multi"> Only books in 2+ catalogs</label>
        <button type="button" id="pepper-help" class="text-xs text-slate-400 underline">🌶️ What do the peppers mean?</button>
        <a href="${FILTER_IDEA_URL}" target="_blank" rel="noopener noreferrer" class="text-xs text-slate-500 underline">Missing a filter? Suggest one</a>
        <span class="ml-auto flex flex-wrap gap-2">
          ${state.user.role === "admin" ? `<button type="button" id="remove-btn" class="btn-ghost text-xs">Remove hidden books from Calibre…</button>` : ""}
          ${manager ? `<a href="#/duplicates" class="btn-ghost text-xs">Find duplicates</a>` : ""}
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
      spice: fd.get("spice") === "Pending" ? "" : fd.get("spice"),
      classification: fd.get("spice") === "Pending" ? "Pending" : "",
      age: fd.get("age"),
      format: fd.get("format"),
      sort: fd.get("sort"),
      multi: fd.get("multi") === "on",
      exclude: fd.getAll("hide").join(","),
      limit: PAGE,
    };
  }

  async function load(reset) {
    if (reset) offset = 0;
    const data = await attempt(() => get("/api/books" + qs({ ...params(), offset })));
    if (!data) return;
    if (reset) grid.innerHTML = "";
    grid.insertAdjacentHTML("beforeend", data.books.map(card).join(""));
    offset += data.books.length;
    $("#result-count", view).textContent = `${data.total.toLocaleString()} book${data.total === 1 ? "" : "s"}`;
    $("#more-btn", view).classList.toggle("hidden", offset >= data.total);
    if (!data.total) grid.innerHTML = `<p class="text-slate-400">No books match these filters.</p>`;
  }

  let debounce;
  form.addEventListener("input", () => {
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
  await load(true);
}

function card(b) {
  const cats = b.catalogs ? b.catalogs.split(", ").map((c) => `<span class="chip-cat">${esc(c)}</span>`).join(" ") : "";
  return `
    <article data-book="${b.id}" class="card cursor-pointer transition hover:ring-indigo-600 flex flex-col gap-2">
      <div class="flex items-start justify-between gap-2">
        <div class="min-w-0">
          <h3 class="font-semibold leading-tight line-clamp-2">${esc(b.title)}</h3>
          <p class="text-sm text-slate-400 truncate">${esc(b.author || "Unknown author")}</p>
        </div>
        <button data-queue="${b.id}" title="Add to Up Next" class="btn-ghost px-2 py-1 text-lg">＋</button>
      </div>
      <div class="flex flex-wrap gap-1">${classChip(b)} ${ageChip(b)} ${flagChips(b)}</div>
      ${b.summary_verdict ? `<p class="text-sm text-slate-300 line-clamp-3">${esc(b.summary_verdict)}</p>` : ""}
      <div class="mt-auto flex flex-wrap items-center gap-1">${cats} ${formatChips(b)}</div>
    </article>`;
}

// File formats (EPUB, AZW3…) and a warning when Calibre has the book twice.
export function formatChips(b) {
  const fmts = b.formats ? b.formats.split(",").map((f) => `<span class="chip-fmt">${esc(f)}</span>`).join(" ") : "";
  const dup = b.calibre_copies > 1
    ? `<span class="chip-dup" title="This book is in Calibre ${b.calibre_copies} times">⚠ ${b.calibre_copies}× in Calibre</span>` : "";
  return `${fmts} ${dup}`;
}
