// Reading stats, the Books tab and the pages under it: books by week or month
// (like KOReader's lists), the books read in one of them, one book's page, and
// one day.
import { esc } from "./ui.js";
import { coverImg } from "./covers.js";
import { dayKey, fmtDur, keyDate, timeline } from "./statscalc.js";
import { agoText, bookDetail, booksIn, listPeriods, periodLabel } from "./statsperiods.js";
import { columns } from "./statsparts.js";

const n = (x) => Number(x).toLocaleString();
const plural = (k, one, many = `${one}s`) => `${n(k)} ${k === 1 ? one : many}`;
const longDate = (ms) => new Date(ms).toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric", year: "numeric" });

const row = (left, sub, right, attrs) => `<li><button type="button" ${attrs} class="flex w-full items-start justify-between gap-3 py-3 text-left">
  <span class="min-w-0"><span class="block break-words font-medium">${esc(left)}</span>${sub ? `<span class="block text-sm text-slate-400">${esc(sub)}</span>` : ""}</span>
  <span class="shrink-0 text-right text-sm tabular-nums text-slate-300">${right}</span></button></li>`;

const list = (rows) => `<ul class="card divide-y divide-slate-800 !py-1">${rows.join("")}</ul>`;
const heading = (t, sub = "") => `<div class="mb-2"><h2 class="text-xl font-semibold">${esc(t)}</h2>${sub ? `<p class="text-sm text-slate-400">${esc(sub)}</p>` : ""}</div>`;

export function booksTab(m, s) {
  const periods = listPeriods(m, s.kind);
  return `<div class="mb-3 flex gap-2">${[["week", "By week"], ["month", "By month"]].map(([k, l]) =>
    `<button type="button" data-kind="${k}" class="${k === s.kind ? "btn-primary" : "btn-secondary"} px-3 py-1.5 text-sm">${l}</button>`).join("")}</div>
    ${list(periods.map((p) => row(periodLabel(s.kind, p.start), plural(p.books.size, "book"), `${fmtDur(p.sec)}<br><span class="text-slate-400">${plural(p.pages, "page")}</span>`, `data-period-open="${s.kind}:${p.start}"`)))}`;
}

export function periodPage(m, kind, start) {
  const books = booksIn(m, kind, start);
  return `${heading(`Books read in ${periodLabel(kind, start)}`, `${plural(books.length, "book")}`)}
    ${list(books.map((b) => row(m.books[b.book].title, m.books[b.book].author, `${fmtDur(b.sec)}<br><span class="text-slate-400">${plural(b.pages, "page")}</span>`, `data-book="${b.book}"`)))}`;
}

const dl = (rows) => `<dl class="card divide-y divide-slate-800 !py-1">${rows.filter(Boolean).map(([k, v]) =>
  `<div class="flex items-start justify-between gap-3 py-2.5"><dt class="text-slate-300">${esc(k)}</dt><dd class="text-right font-medium tabular-nums">${v}</dd></div>`).join("")}</dl>`;

export function bookPage(m, i, readonly = false) {
  const d = bookDetail(m, i);
  const b = d.book;
  const pct = Math.round((b.percent || 0) * 100);
  const lastPage = b.pages ? `${n(Math.round((b.percent || 0) * b.pages))} / ${n(b.pages)} (${pct}%)` : `${pct}%`;
  return `<div class="mb-4 flex gap-4">${b.book_id ? `<div class="w-24 shrink-0">${coverImg(b.book_id, "h-36 w-24")}</div>` : ""}
      <div class="min-w-0 space-y-2"><h2 class="break-words text-2xl font-bold leading-tight">${esc(b.title)}</h2>
        <p class="text-slate-400">${esc(b.author || "")}</p>
        <div class="h-2 overflow-hidden rounded-full bg-slate-800"><div class="h-full rounded-full bg-indigo-500" data-pct="${pct}"></div></div>
        <p class="text-sm text-slate-400">${pct}% read${b.series ? ` · ${esc(b.series)}` : ""}</p>
        ${b.book_id ? `<button type="button" data-open-lib="${b.book_id}" class="btn-secondary px-3 py-1 text-sm">Open in your library</button>` : ""}</div></div>
    ${dl([["Time spent reading", fmtDur(d.sec)], ["Days reading this book", n(d.days)], ["Average time per day", fmtDur(d.avgDay)],
      d.first ? ["Book start date", `${esc(longDate(d.first))}<br><span class="text-sm font-normal text-slate-400">${esc(agoText(d.first))}</span>`] : null,
      d.last ? ["Last read date", `${esc(longDate(d.last))}<br><span class="text-sm font-normal text-slate-400">${esc(agoText(d.last))}</span>`] : null,
      ["Last read page / total pages", lastPage], ["Pages turned", n(d.pages)],
      d.pages ? ["Average time per page", `${Math.round(d.secPerPage)} sec`] : null,
      ...(readonly ? [] : [["Highlights", n(b.highlights || 0)], ["Notes", n(b.notes || 0)]])])}
    ${d.perDay.length ? `<h3 class="mb-1 mt-5 text-lg font-semibold">Days you read it</h3>${list(d.perDay.slice(0, 20).map((p) =>
      row(keyDate(p.key).toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric", year: "numeric" }), "", `${fmtDur(p.sec)}<br><span class="text-slate-400">${plural(p.pages, "page")}</span>`, `data-day-key="${p.key}"`)))}` : ""}`;
}

export function dayPage(m, key) {
  const t = timeline(m, key);
  const total = t.hours.reduce((a, b) => a + b, 0);
  const isToday = key === dayKey(Date.now());
  const hourPts = t.hours.map((v, h) => ({ label: h % 6 === 0 ? `${h % 12 || 12}${h < 12 ? "a" : "p"}` : "", value: v }));
  return `<div class="mb-3 flex items-center justify-between gap-2">
      <button type="button" data-shift="-1" class="btn-ghost px-3 py-1" aria-label="Previous day">‹</button>
      <h2 class="text-center text-xl font-semibold">${esc(keyDate(key).toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" }))}</h2>
      <button type="button" data-shift="1" class="btn-ghost px-3 py-1" aria-label="Next day" ${isToday ? "disabled" : ""}>›</button></div>
    ${total ? `<div class="card space-y-2"><p class="text-lg">You read <b>${fmtDur(total)}</b> this day.</p>${columns(hourPts, fmtDur)}</div>
      ${list(t.books.map((b) => row(m.books[b.book]?.title || "Unknown book", "", fmtDur(b.sec), `data-book="${b.book}"`)))}`
    : `<div class="card text-center text-slate-400">No reading recorded this day.</div>`}`;
}
