// The series page's gaps: a missing number gets a search of the person's Libby libraries (and Add to Nightstand
// from there); novellas and half-numbers are labelled so they aren't mistaken for full novels.
import { esc } from "./ui.js";
import { libbySearchBox } from "./libbysearch.js";

// isNovella: a half-number (1.5, 0.5) or a title that says so.
export const isNovella = (b) => (b.series_index && b.series_index % 1 !== 0) || /\b(novella|short story|stories|prequel story)\b/i.test(b.title || "");

export const novellaChip = (b) => (isNovella(b) ? `<span class="chip-pending" title="A shorter side story, not one of the full novels">Novella, not a full novel</span>` : "");

export function gapRow(n, canSearch) {
  return `<li class="rounded-xl border border-dashed border-slate-700 p-3 text-sm text-slate-400" data-gap="${n}">
    <div class="flex items-center gap-3"><span class="w-8 shrink-0 text-center font-semibold">#${n}</span><span class="flex-1">Not in your libraries</span>
    ${canSearch ? `<button type="button" data-gapfind class="btn-secondary py-1 text-sm">Find it</button>` : ""}</div><div data-gaphost class="mt-2 hidden"></div></li>`;
}

// bindGap handles a tap on "Find it": opens the Libby search for this series and number.
export function bindGap(e, seriesName) {
  const btn = e.target.closest("[data-gapfind]");
  if (!btn) return false;
  const li = btn.closest("[data-gap]");
  const host = li.querySelector("[data-gaphost]");
  host.classList.toggle("hidden");
  if (!host.firstChild) libbySearchBox(host, `${seriesName} ${li.dataset.gap}`, true);
  return true;
}

