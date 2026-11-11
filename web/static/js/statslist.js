// Reading stats → Highlights for a reader whose numbers come from an imported list (Goodreads, StoryGraph…)
// or Hardcover, with no KOReader reading: everything the list says about the books finished in a period.
// The books are m.done (statselsewhere.js); days are in this device's time zone.
import { esc } from "./ui.js";
import { GENRES } from "./genres.js";
import { bounds, count, pageBuckets } from "./statsperiods.js";
import { card, bigNum, barRows, donut, columns } from "./statsparts.js";
import { coverImg } from "./covers.js";

const n = (x) => Number(x).toLocaleString();
const plural = (k, one) => `${n(k)} ${one}${k === 1 ? "" : "s"}`;
const gname = (g) => GENRES[g] || g;
const FORMATS = { paper: "Paper", ebook: "E-book", audio: "Audiobook" };
const stars = (v) => `${"★".repeat(Math.round(v))}${"☆".repeat(5 - Math.round(v))}`;

// byMonth: a point per month between the first and last of the books (at most the last 24).
function byMonth(done, from, to) {
  if (!done.length) return [];
  const first = new Date(Math.max(from, Math.min(...done.map((d) => d.ms))));
  const last = new Date(Math.min(Number.isFinite(to) ? to - 1 : Date.now(), Math.max(Date.now(), ...done.map((d) => d.ms))));
  const out = [];
  for (let d = new Date(first.getFullYear(), first.getMonth(), 1); d <= last; d = new Date(d.getFullYear(), d.getMonth() + 1, 1)) {
    out.push({ key: `${d.getFullYear()}-${d.getMonth()}`, label: d.toLocaleDateString(undefined, { month: "short" }), books: 0, pages: 0 });
  }
  const at = new Map(out.map((p) => [p.key, p]));
  for (const d of done) {
    const x = new Date(d.ms);
    const p = at.get(`${x.getFullYear()}-${x.getMonth()}`);
    if (p) {
      p.books++;
      p.pages += d.b.pages || 0;
    }
  }
  return out.slice(-24);
}

export function listReport(m, s) {
  const b = bounds(s.period, s.offset);
  const done = m.done.filter((d) => d.ms >= b.from && d.ms < b.to).map((d) => ({ ...d, b: m.books[d.book] })).sort((x, y) => y.ms - x.ms);
  if (!done.length) {
    return `<div class="card text-center text-slate-300"><p class="text-lg font-semibold">No books finished in this period.</p>
      <p class="mt-1 text-sm text-slate-400">Pick another period: your list has ${plural(m.done.length, "finished book")} in all.</p></div>`;
  }
  const books = done.map((d) => d.b);
  const pages = books.reduce((t, x) => t + (x.pages || 0), 0);
  const rated = books.filter((x) => x.rating > 0);
  const avg = rated.length ? rated.reduce((t, x) => t + x.rating, 0) / rated.length : 0;
  const authors = count(books.filter((x) => x.author), (x) => x.author);
  const withPages = books.filter((x) => x.pages > 0);
  const parts = [`<div class="card grid grid-cols-2 gap-4 sm:grid-cols-3">${[
    bigNum("🏁", n(books.length), "Books finished"), bigNum("📄", n(pages), "Pages in those books"), bigNum("✍️", n(authors.length), "Authors"),
    rated.length ? bigNum("⭐", avg.toFixed(1), `Average of your ${plural(rated.length, "rating")}`) : "",
    withPages.length ? bigNum("📏", n(Math.round(pages / withPages.length)), "Pages in an average book") : "",
    bigNum("🔁", n(books.reduce((t, x) => t + Math.max(0, (x.reads || 1) - 1), 0)), "Re-reads"),
  ].join("")}</div>`];

  parts.push(card("Books you finished", `You finished ${plural(books.length, "book")}`,
    `<div class="flex gap-2 overflow-x-auto pb-1">${books.slice(0, 40).map((x) => (x.book_id ? `<span class="shrink-0" title="${esc(x.title)}">${coverImg(x.book_id, "h-28 w-20")}</span>` : "")).join("")}</div>`));

  const pts = byMonth(done, b.from, b.to);
  if (pts.length > 1) {
    const best = pts.reduce((a, p) => (p.books > a.books ? p : a));
    parts.push(card("Books by month", `Your biggest month had ${plural(best.books, "book")}`, columns(pts.map((p) => ({ label: p.label, value: p.books })), (v) => plural(v, "book"))));
    if (pages) parts.push(card("Pages by month", `${n(pages)} pages in all`, columns(pts.map((p) => ({ label: p.label, value: p.pages })), (v) => `${n(v)} pages`, "fill-emerald-500")));
  }

  if (rated.length) {
    const rows = [5, 4, 3, 2, 1].map((k) => ({ k, v: rated.filter((x) => Math.round(x.rating) === k).length }));
    parts.push(card("Your ratings", `You gave ${plural(rows[0].v, "book")} five stars`, barRows(rows.map((r) => ({ label: stars(r.k), value: r.v, text: plural(r.v, "book") })), { colour: false })));
  }

  const formats = count(books.filter((x) => x.format), (x) => x.format);
  if (formats.length) parts.push(card("How you read", `Mostly ${(FORMATS[formats[0].label] || formats[0].label).toLowerCase()}s`, donut(formats.map((f) => ({ label: FORMATS[f.label] || f.label, value: f.n })))));

  const fic = books.filter((x) => x.kind === "fiction").length, non = books.filter((x) => x.kind === "nonfiction").length;
  if (fic || non) parts.push(card("Fiction and non-fiction", fic >= non ? "You mostly read fiction." : "You mostly read non-fiction.",
    donut([{ label: "Fiction", value: fic }, { label: "Non-fiction", value: non }, { label: "Not sure", value: books.length - fic - non }].filter((p) => p.value > 0))));

  const genres = count(books, (x) => x.genres || []);
  if (genres.length) parts.push(card("Genres", `${esc(gname(genres[0].label))} is your top genre`, barRows(genres.slice(0, 8).map((g) => ({ label: gname(g.label), value: g.n, text: plural(g.n, "book") })))));

  if (authors.length) parts.push(card("Authors", `${esc(authors[0].label)} is your most-read author`, barRows(authors.slice(0, 8).map((a) => ({ label: a.label, value: a.n, text: plural(a.n, "book") })))));

  const buckets = pageBuckets(books);
  if (buckets.some((x) => x.n)) {
    const top = buckets.reduce((a, x) => (x.n > a.n ? x : a));
    const long = withPages.slice().sort((x, y) => y.pages - x.pages)[0], short = withPages.slice().sort((x, y) => x.pages - y.pages)[0];
    parts.push(card("Book length", `Most of your books are ${top.label} pages`, columns(buckets.map((x) => ({ label: x.label, value: x.n })), (v) => plural(v, "book"))
      + (long ? `<p class="text-sm text-slate-400">Longest: ${esc(long.title)} (${n(long.pages)} pages). Shortest: ${esc(short.title)} (${n(short.pages)} pages).</p>` : "")));
  }

  parts.push(card("Every book", `${plural(books.length, "book")}, newest first`, `<ul class="list">${done.map((d) => `<li class="list-row">
    <div class="min-w-0 flex-1"><p class="font-medium leading-snug">${esc(d.b.title)}</p><p class="text-sm text-slate-400">${esc(d.b.author || "")}${d.b.pages ? ` · ${n(d.b.pages)} pages` : ""}${d.b.format ? ` · ${FORMATS[d.b.format] || esc(d.b.format)}` : ""}</p></div>
    <div class="text-right text-sm"><p>${new Date(d.ms).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}</p>${d.b.rating ? `<p class="text-amber-300" aria-label="${d.b.rating} stars">${stars(d.b.rating)}</p>` : ""}</div></li>`).join("")}</ul>`));
  return parts.join("");
}
