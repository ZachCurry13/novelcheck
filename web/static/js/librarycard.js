// A book's card in the Library: cover, title, chips, blurb, and where the
// family has it.
import { esc, classChip, flagChips, ageChip, hcStars } from "./ui.js";
import { whyChip } from "./peppers.js";
import { customChips } from "./customflags.js";
import { contentIcons } from "./content.js";
import { deepChip } from "./deepscan.js";
import { seriesText } from "./titlefix.js";
import { coverImg } from "./covers.js";
import { audioChip } from "./audioui.js";

// A book already in your Nightstand.
export const IN_QUEUE = `<span class="px-2 py-1 text-lg text-emerald-300" title="In your Nightstand" aria-label="In your Nightstand">✓</span>`;

// reviewChip marks a rating the AI wasn't sure of, until a parent looks.
export function reviewChip(b) {
  return b.status === "analyzed" && b.confidence === "low" && !b.approved
    ? `<span class="chip-dup" title="The AI wasn't sure of this rating: worth a look, or a re-rate on the big model">Not sure</span>` : "";
}

// card is one book, as a bookshop shows it: the cover (with its pepper level, and ＋ for the Nightstand),
// the title and author, a few chips, and an event's Claim buttons (extra). The blurb, formats and libraries
// are on the book's own page.
export function card(b, queueOn, extra = "") {
  const rated = b.spice_level !== null && b.spice_level !== undefined;
  return `
    <article data-book="${b.id}" data-state="${esc(b.status || "")}" class="lib-tile">
      <div class="tile-cover">${coverImg(b.id, "tile-img")}
        ${rated ? `<span class="lvl lvl-${b.spice_level} tile-lvl" title="Level ${b.spice_level}">${b.spice_level}</span>` : ""}
        ${!queueOn ? "" : b.in_queue ? `<span class="tile-add">${IN_QUEUE}</span>` : `<button data-queue="${b.id}" title="Add to your Nightstand" aria-label="Add to your Nightstand" class="tile-add tile-add-btn">＋</button>`}
      </div>
      <h3 class="tile-title">${esc(b.title)}</h3>
      <p class="tile-author">${esc(b.author || "Unknown author")}${seriesText(b) ? ` · <span class="text-sky-300">${esc(seriesText(b))}</span>` : ""}</p>
      <div data-chips class="tile-chips">${rated ? "" : classChip(b)} ${reviewChip(b)} ${deepChip(b)} ${whyChip(b)} ${ageChip(b)} ${flagChips(b)} ${customChips(b)} ${contentIcons(b)}</div>
      ${extra}
      <div class="tile-meta">${hcStars(b)} ${audioChip(b)} ${b.formats && /AUDIO/.test(b.formats) ? "" : `<span data-libaudio="${b.id}"></span>`}${b.delete_requests ? ` <span class="chip-dup">Delete requested</span>` : ""}</div>
    </article>`;
}

// File formats (EPUB, AZW3…) and a warning when Calibre has the book twice.
export function formatChips(b) {
  const fmts = b.formats ? b.formats.split(",").filter((f) => f !== "AUDIO").map((f) => `<span class="chip-fmt">${f === "PAPER" ? "Paper" : esc(f)}</span>`).join(" ") : "";
  const del = b.delete_requests ? `<span class="chip-dup" title="Someone asked to delete this book">Delete requested</span>` : "";
  const dup = b.calibre_copies > 1
    ? `<span class="chip-dup" title="This book is in Calibre ${b.calibre_copies} times">${b.calibre_copies}× in Calibre</span>` : "";
  return `${audioChip(b)} ${fmts} ${dup} ${del} ${b.formats && /AUDIO/.test(b.formats) ? "" : `<span data-libaudio="${b.id}"></span>`}`;
}
