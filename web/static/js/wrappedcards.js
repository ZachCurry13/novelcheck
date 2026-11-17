// Wrapped, up front: two big cards at the top of Reading stats → Highlights, one for last month and one for the
// year so far, and a picker for any other month or year. A tap plays the story (the page's own data-play). A
// "New" tag shows until that Wrapped has been played on this device. The colors come from the theme
// (--nc-wrap-* in web/static/themes), the look from .wcard in tailwind.input.css.
import { esc } from "./ui.js";
import { fmtDur } from "./statscalc.js";
import { bounds, summary } from "./statsperiods.js";

const KEY = "novelcheck.wrappedSeen";
const read = () => {
  try {
    return JSON.parse(localStorage.getItem(KEY) || "[]");
  } catch {
    return [];
  }
};
const idOf = (period, offset) => `${period}:${new Date(bounds(period, offset).from).getFullYear()}-${new Date(bounds(period, offset).from).getMonth()}`;

// markWrappedSeen notes that this Wrapped was played (so its New tag goes).
export function markWrappedSeen(period, offset) {
  try {
    const seen = read();
    const id = idOf(period, offset);
    if (!seen.includes(id)) localStorage.setItem(KEY, JSON.stringify([...seen, id].slice(-80)));
  } catch {
    /* private mode: the tag stays */
  }
}

const label = (period, offset) => {
  const from = new Date(bounds(period, offset).from);
  return period === "year" ? String(from.getFullYear()) : from.toLocaleDateString(undefined, { month: "long", year: "numeric" });
};
const books = (n) => (n ? ` · ${n.toLocaleString()} book${n === 1 ? "" : "s"}` : "");

function card(period, offset, kicker, sum) {
  const fresh = !read().includes(idOf(period, offset));
  return `<button type="button" class="wcard" data-play="${period}:${offset}" aria-label="Play Wrapped: ${esc(label(period, offset))}">
    ${fresh ? `<span class="wcard-new">New</span>` : ""}<small>${esc(kicker)}</small><b>${esc(label(period, offset))}</b>
    <span class="wcard-stat">${esc(fmtDur(sum.sec))}${books(sum.finished)}</span><span class="wcard-play">Tap to play <span aria-hidden="true">▶</span></span></button>`;
}

// wrappedCardsHTML is the block for the top of Highlights, or "" for someone with no timed reading to play.
export function wrappedCardsHTML(m) {
  const lastMonth = summary(m, "month", -1), year = summary(m, "year", 0);
  const cards = [];
  if (lastMonth.sec > 0) cards.push(card("month", -1, "Wrapped", lastMonth));
  if (year.sec > 0) cards.push(card("year", 0, "Wrapped so far", year));
  if (!cards.length) return "";
  const options = (period, from, to) => {
    const out = [];
    for (let off = from; off >= to; off--) if (summary(m, period, off).sec > 0) out.push(`<option value="${period}:${off}">${esc(label(period, off))}</option>`);
    return out.join("");
  };
  const months = options("month", -2, -48), years = options("year", -1, -12);
  return `<section class="wcards" aria-label="Wrapped"><div class="wcards-row">${cards.join("")}</div>
    ${months || years ? `<select class="input wcards-pick" data-wrapped-pick aria-label="Another month or year"><option value="">Another month…</option>
      ${months ? `<optgroup label="Months">${months}</optgroup>` : ""}${years ? `<optgroup label="Years">${years}</optgroup>` : ""}</select>` : ""}</section>`;
}
