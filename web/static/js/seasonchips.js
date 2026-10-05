// Seasonal shelves as a row of chips (Library, Discover): the ones in season
// first, a link to all collections, and "🗓️ More seasons" for the rest. Each
// opens the Library filtered to that shelf.
import { esc } from "./ui.js";

const chipCls = "chip shrink-0 min-h-[2.25rem] px-3 text-sm";

export function seasonChipsHTML(seasons, active = "") {
  const chip = (s, more) => `<a href="#/library?season=${s.key}" ${more ? "data-more-season" : ""}
    class="${chipCls} ${s.key === active ? "bg-indigo-600 text-white" : s.now ? "bg-amber-900/50 text-amber-100 ring-1 ring-amber-700/60" : "bg-slate-800 text-slate-300"}${more && s.key !== active ? " hidden" : ""}"
    title="${esc(s.name)}${s.now ? " (in season now)" : ""}">${s.icon} ${esc(s.name)}</a>`;
  const now = seasons.filter((s) => s.now);
  const rest = seasons.filter((s) => !s.now);
  // py-1: a scrolling row clips what sticks out, like the in-season chips' outline.
  return `<div data-season-chips class="-mx-4 mb-2 flex gap-2 overflow-x-auto px-4 py-1">
    ${now.map((s) => chip(s, false)).join("")}
    <a href="#/collections" class="${chipCls} bg-slate-800 text-slate-300">📚 Collections</a>
    ${rest.length ? `<button type="button" data-more-seasons class="${chipCls} bg-slate-800 text-slate-400">🗓️ More seasons</button>` : ""}
    ${rest.map((s) => chip(s, true)).join("")}
  </div>`;
}

// bindSeasonChips makes "More seasons" show the rest.
export function bindSeasonChips(host) {
  host.addEventListener("click", (e) => {
    const more = e.target.closest("[data-more-seasons]");
    if (!more) return;
    host.querySelectorAll("[data-more-season]").forEach((a) => a.classList.remove("hidden"));
    more.remove();
  });
}
