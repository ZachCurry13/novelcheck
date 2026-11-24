// Kindle page (main menu): one Kindle at a time (the KOReader plugin's and the Kindle app's reports together),
// its books with cover, spice level and progress, the ones that left it, and what you can ask it to do.
import { get } from "./api.js";
import { esc, attempt } from "./ui.js";
import { ago } from "./progress.js";
import { renderLeftBooks } from "./leftbooks.js";
import { renderKindleTasks } from "./kindletasks.js";

const isApp = (d) => String(d.plugin_version || "").startsWith("app ");
const door = (n) => (n >= 4 ? "Open Door" : n >= 2 ? "Closed Door" : "No Spice");
const FILTERS = [
  ["all", "All", () => true],
  ["file", "Downloaded", (b) => b.format !== "cloud"],
  ["cloud", "Amazon cloud", (b) => b.format === "cloud"],
  ["0", "No Spice", (b) => b.level != null && b.level < 2],
  ["closed", "Closed Door", (b) => b.level != null && b.level >= 2 && b.level < 4],
  ["open", "Open Door", (b) => b.level != null && b.level >= 4],
  ["none", "Not rated", (b) => b.level == null],
];

function rowHTML(b) {
  const pct = Math.round((b.percent || 0) * 100);
  const chip = b.level != null ? `<span class="chip-cat">Spice ${b.level} · ${door(b.level)}</span>`
    : b.id ? `<span class="chip-wait">Being rated…</span>` : `<span class="chip-none">Not in your library</span>`;
  const name = esc(b.title);
  return `<li class="flex items-center gap-3 py-2">
    ${b.id ? `<img class="h-16 w-11 shrink-0 rounded object-cover" loading="lazy" alt="" src="/api/books/${b.id}/cover">` : `<span class="h-16 w-11 shrink-0 rounded bg-slate-800"></span>`}
    <span class="min-w-0 flex-1"><b class="block truncate">${b.id ? `<a href="#/library?book=${b.id}" class="hover:underline">${name}</a>` : name}</b>
      <span class="block truncate text-xs text-slate-400">${esc(b.author || "Unknown author")}</span>
      ${chip} <span class="chip-none">${b.format === "cloud" ? "Amazon cloud" : "Downloaded"}</span>
      ${pct > 0 && b.format !== "cloud" ? `<span class="mt-1 block h-1 rounded bg-slate-800"><span class="block h-1 rounded bg-indigo-500" data-w="${pct}"></span></span>` : ""}</span></li>`;
}

export async function renderKindle(view) {
  view.innerHTML = `<h1 class="page-title mb-1">Kindle</h1><div id="k-body"><p class="text-slate-400">Loading…</p></div>`;
  const s = await attempt(() => get("/api/me/reading-sync"));
  const body = view.querySelector("#k-body");
  if (!s) return;
  const devices = s.devices || [], links = s.links || {};
  const groups = new Map();
  for (const d of devices) {
    const g = links[d.name] || d.name;
    groups.set(g, [...(groups.get(g) || []), d]);
  }
  if (!groups.size) {
    body.innerHTML = `<div class="card text-sm text-slate-300">No Kindle has reported yet. Install the KOReader plugin or the Kindle app from <a href="#/settings" class="underline">Settings</a>, then press Sync now on the Kindle.</div>`;
    return;
  }
  const names = [...groups.keys()];
  const saved = sessionStorage.getItem("nc-kindle");
  let current = names.includes(saved) ? saved : names[0];
  let filter = "all";

  async function show() {
    const ds = groups.get(current);
    const r = await attempt(() => get("/api/me/kindle-library?kindle=" + encodeURIComponent(current)));
    const books = (r && r.books) || [];
    const test = FILTERS.find((f) => f[0] === filter)[2];
    const shown = books.filter(test);
    const last = Math.max(...ds.map((d) => d.reported_at || 0));
    body.innerHTML = `${names.length > 1 ? `<label class="mb-2 block text-sm text-slate-400">Kindle <select id="k-pick" class="input ml-1 py-1">${names.map((n) => `<option ${n === current ? "selected" : ""}>${esc(n)}</option>`).join("")}</select></label>` : ""}
      <div class="card mb-3 text-sm"><b>${esc(current)}</b>
        ${ds.map((d) => `<span class="${isApp(d) && String(d.plugin_version).slice(4) !== s.app_latest ? "chip-wait" : "chip-none"}">${isApp(d) ? "Kindle app " + esc(String(d.plugin_version).slice(4)) : "KOReader plugin " + esc(d.plugin_version)}</span>`).join(" ")}
        <p class="text-xs text-slate-400">Last report ${esc(ago(last))} · ${books.length} book${books.length === 1 ? "" : "s"} · ${books.filter((b) => b.format === "cloud").length} only in the Amazon library</p></div>
      <div class="mb-2 flex flex-wrap gap-1">${FILTERS.map(([k, label]) => `<button type="button" data-f="${k}" class="chip-filter${k === filter ? " on" : ""}">${label}</button>`).join("")}</div>
      <ul class="card divide-y divide-slate-800">${shown.map(rowHTML).join("") || `<li class="py-2 text-sm text-slate-400">No books match.</li>`}</ul>
      <section id="k-left" class="mt-4"></section>
      <section class="card mt-4 space-y-2"><h2 class="font-semibold">Ask this Kindle to…</h2><div id="k-tasks"></div></section>`;
    body.querySelectorAll("[data-w]").forEach((e) => (e.style.width = e.dataset.w + "%")); // inline style="" is refused by the page policy
    body.querySelector("#k-pick")?.addEventListener("change", (e) => { current = e.target.value; sessionStorage.setItem("nc-kindle", current); show(); });
    body.querySelectorAll("[data-f]").forEach((b) => b.addEventListener("click", () => { filter = b.dataset.f; show(); }));
    renderLeftBooks(body.querySelector("#k-left"));
    const plugin = ds.find((d) => !isApp(d));
    renderKindleTasks(body.querySelector("#k-tasks"), (plugin || ds[0]).name);
  }
  show();
}
