// 🗂 Series: every series in the family's libraries (with how far you've
// read), and one series in order: the numbers you don't have, what you've
// read, and Next up with ＋ Up Next.
import { get, post } from "./api.js";
import { $, esc, attempt, toast, classChip } from "./ui.js";
import { coverImg } from "./covers.js";
import { openBook } from "./bookdialog.js";
import { on } from "./modules.js";
import { loadContent, contentIcons } from "./content.js";
import { shelfTabsHTML } from "./shelftabs.js";

const q = () => new URLSearchParams(location.hash.split("?")[1] || "");
// FormatIndex writes a series number: 3, or 2.5 for a novella between books.
const FormatIndex = (i) => (Number.isInteger(i) ? String(i) : String(Number(i.toFixed(1))));

export async function renderSeries(view, state) {
  const name = q().get("name");
  if (name) return renderOne(view, state, name);
  view.innerHTML = `${shelfTabsHTML("series")}
    <h1 class="mb-1 text-2xl font-bold">🗂 Series</h1>
    <p class="mb-3 text-sm text-slate-400">Every series in your libraries, in order. Open one to see which books you have, what you've read, and what's next.</p>
    <input id="series-q" type="search" class="input mb-3" placeholder="Find a series" autocomplete="off">
    <ul id="series" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"><li class="text-slate-400">Loading…</li></ul>`;
  const list = (await attempt(() => get("/api/series"))) || [];
  const paint = (text) => {
    const want = text.trim().toLowerCase();
    const shown = list.filter((s) => !want || s.name.toLowerCase().includes(want));
    $("#series", view).innerHTML = shown.map((s) => `<li><a href="#/series?name=${encodeURIComponent(s.name)}" class="card flex items-center gap-3 hover:ring-indigo-600">
      <span class="flex shrink-0 gap-1">${s.covers.map((id) => coverImg(id, "h-16 w-11")).join("")}</span>
      <span class="min-w-0 flex-1"><span class="block font-semibold leading-snug">${esc(s.name)}</span>
        <span class="block text-xs text-slate-400">${s.books} book${s.books === 1 ? "" : "s"}${s.top > s.books ? ` (up to #${FormatIndex(s.top)})` : ""}${s.read ? ` · you've read ${s.read}` : ""}${s.reading ? " · reading one" : ""}</span>
        ${s.read ? `<span class="mt-1 block h-1.5 overflow-hidden rounded-full bg-slate-800"><span class="block h-full rounded-full bg-emerald-500" data-pct="${Math.round((100 * s.read) / s.books)}"></span></span>` : ""}</span>
      <span aria-hidden="true">›</span></a></li>`).join("")
      || `<li class="card text-sm text-slate-400">${list.length ? "No series match." : "No series yet. Books get their series from Calibre (the Series field)."}</li>`;
    // Widths are set here: the page's strict security rules allow no inline styles in HTML.
    view.querySelectorAll("[data-pct]").forEach((el) => (el.style.width = `${el.dataset.pct}%`));
  };
  paint("");
  $("#series-q", view).addEventListener("input", (e) => paint(e.target.value));
}

async function renderOne(view, state, name) {
  const [data] = await Promise.all([attempt(() => get(`/api/series/one?name=${encodeURIComponent(name)}`)), loadContent()]);
  if (!data) return (location.hash = "#/series");
  const queueOn = on(state.user, "queue");
  const rows = [
    ...data.books.map((b) => ({ n: b.series_index, b })),
    ...data.gaps.map((n) => ({ n, gap: true })),
  ].sort((a, b) => (a.n || 1e9) - (b.n || 1e9));
  const read = data.books.filter((b) => b.my_status === "finished").length;
  const status = (b) => ({ finished: `<span class="chip-none">✓ Read</span>`, reading: `<span class="chip-busy">▶ Reading</span>`,
    queued: `<span class="chip-cat">In Up Next</span>` })[b.my_status] || "";
  view.innerHTML = `${shelfTabsHTML("series")}
    <a href="#/series" class="mb-2 inline-block text-sm text-slate-400 hover:text-white">← All series</a>
    <h1 class="text-2xl font-bold">🗂 ${esc(data.name)}</h1>
    <p class="mb-4 text-sm text-slate-400">${data.books.length} book${data.books.length === 1 ? "" : "s"} in your libraries${read ? ` · you've read ${read}` : ""}
      · <a href="#/library?series=${encodeURIComponent(data.name)}" class="underline">show them in the Library</a></p>
    <ol id="books" class="space-y-2">${rows.map(({ n, b, gap }) => gap
      ? `<li class="flex items-center gap-3 rounded-xl border border-dashed border-slate-700 p-3 text-sm text-slate-500">
          <span class="w-8 shrink-0 text-center font-bold">#${n}</span><span>Not in your libraries</span></li>`
      : `<li class="card flex gap-3 p-3 ${b.id === data.next ? "ring-2 ring-indigo-500" : ""}" data-book="${b.id}">
          <span class="w-8 shrink-0 pt-1 text-center font-bold text-slate-400">${b.series_index ? `#${FormatIndex(b.series_index)}` : "–"}</span>
          <button type="button" data-open class="shrink-0">${coverImg(b.id, "h-20 w-14")}</button>
          <div class="min-w-0 flex-1 space-y-1">
            ${b.id === data.next ? `<p class="text-xs font-semibold uppercase tracking-wide text-indigo-300">Next up</p>` : ""}
            <button type="button" data-open class="text-left font-semibold leading-snug line-clamp-2">${esc(b.title)}</button>
            <p class="truncate text-xs text-slate-400">${esc(b.author || "")}</p>
            <div class="flex flex-wrap gap-1">${classChip(b)} ${contentIcons(b)} ${status(b)}</div>
            ${queueOn && !b.my_status ? `<button type="button" data-queue class="btn-secondary py-1 text-sm">＋ Up Next</button>` : ""}
          </div></li>`).join("")}</ol>`;
  view.onclick = async (e) => {
    const li = e.target.closest("[data-book]");
    if (!li) return;
    const id = Number(li.dataset.book);
    if (e.target.closest("[data-open]")) return openBook(id, state, () => renderOne(view, state, name));
    const add = e.target.closest("[data-queue]");
    if (add && (await attempt(() => post("/api/queue", { book_id: id })))) {
      add.outerHTML = `<span class="chip-cat">In Up Next</span>`;
      toast("Added to Up Next", false, { label: "View Up Next", href: "#/queue" });
    }
  };
}
