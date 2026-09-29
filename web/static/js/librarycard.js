// A book's card in the Library: cover, title, chips, blurb, and where the
// family has it.
import { esc, classChip, flagChips, ageChip } from "./ui.js";
import { whyChip } from "./peppers.js";
import { customChips } from "./customflags.js";
import { contentIcons } from "./content.js";
import { deepChip } from "./deepscan.js";
import { seriesText } from "./titlefix.js";
import { coverImg } from "./covers.js";
import { blurbHTML } from "./blurb.js";

// A book already in your Up Next.
export const IN_QUEUE = `<span class="px-2 py-1 text-lg text-emerald-300" title="In your Up Next" aria-label="In your Up Next">✓</span>`;

// Lists that aren't libraries (Discover's and the events').
const LISTS = ["Discover", "Events"];

// reviewChip marks a rating the AI wasn't sure of, until a parent looks.
export function reviewChip(b) {
  return b.status === "analyzed" && b.confidence === "low" && !b.approved
    ? `<span class="chip-dup" title="The AI wasn't sure of this rating: worth a look, or a re-rate on the big model">⚠ Not sure</span>` : "";
}

// card is one book; extra is more to show above its libraries (an event's
// Claim buttons).
export function card(b, queueOn, extra = "") {
  const cats = b.catalogs ? b.catalogs.split(", ").filter((c) => !LISTS.includes(c)).map((c) => `<span class="chip-cat">${esc(c)}</span>`).join(" ") : "";
  return `
    <article data-book="${b.id}" data-state="${esc(b.status || "")}" class="card cursor-pointer transition hover:ring-indigo-600 flex flex-col gap-2">
      <div class="flex items-start justify-between gap-3">
        ${coverImg(b.id, "h-24 w-16")}
        <div class="min-w-0 flex-1">
          <h3 class="font-semibold leading-tight line-clamp-2">${esc(b.title)}</h3>
          <p class="text-sm text-slate-400 truncate">${esc(b.author || "Unknown author")}${seriesText(b) ? ` · <span class="text-sky-300">${esc(seriesText(b))}</span>` : ""}</p>
        </div>
        ${!queueOn ? "" : b.in_queue ? IN_QUEUE : `<button data-queue="${b.id}" title="Add to Up Next" aria-label="Add to Up Next" class="btn-ghost px-2 py-1 text-lg">＋</button>`}
      </div>
      <div data-chips class="flex flex-wrap gap-1">${classChip(b)} ${reviewChip(b)} ${deepChip(b)} ${whyChip(b)} ${ageChip(b)} ${flagChips(b)} ${customChips(b)} ${contentIcons(b)}</div>
      ${blurbHTML(b)}
      ${extra}
      <div class="mt-auto flex flex-wrap items-center gap-1">${cats} ${formatChips(b)}</div>
    </article>`;
}

// File formats (EPUB, AZW3…) and a warning when Calibre has the book twice.
export function formatChips(b) {
  const fmts = b.formats ? b.formats.split(",").map((f) => `<span class="chip-fmt">${f === "PAPER" ? "📕 Paper" : esc(f)}</span>`).join(" ") : "";
  const del = b.delete_requests ? `<span class="chip-dup" title="Someone asked to delete this book">🗑 Delete requested</span>` : "";
  const dup = b.calibre_copies > 1
    ? `<span class="chip-dup" title="This book is in Calibre ${b.calibre_copies} times">⚠ ${b.calibre_copies}× in Calibre</span>` : "";
  return `${fmts} ${dup} ${del}`;
}
