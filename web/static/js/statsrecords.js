// Reading stats → Records: a tile for this week, month, year and all time (with the change against the
// period before, and "Play it as Wrapped"), a year's awards, then your personal bests as a list.
// The numbers come from records.js and statsperiods.js; days and hours are in this device's time zone.
import { esc } from "./ui.js";
import { fmtDur } from "./statscalc.js";
import { summary } from "./statsperiods.js";
import { awards, records, yearsOf } from "./records.js";

const n = (x) => Number(x).toLocaleString();
const TILES = [["week", "This week", "last week"], ["month", "This month", "last month"], ["year", "This year", "last year"], ["all", "All time", ""]];

// change: how this period compares with the one before, as a short line.
function change(now, before, versus) {
  if (!versus) return "";
  if (!before) return now ? `<p class="text-xs text-slate-400">Nothing ${versus}</p>` : "";
  const pct = Math.round(((now - before) / before) * 100);
  if (!pct) return `<p class="text-xs text-slate-400">The same as ${versus}</p>`;
  return `<p class="text-xs ${pct > 0 ? "text-emerald-300" : "text-rose-300"}"><span aria-hidden="true">${pct > 0 ? "▲" : "▼"}</span> ${Math.abs(pct)}% ${pct > 0 ? "more" : "less"} than ${versus}</p>`;
}

function tile(m, [period, label, versus]) {
  const now = summary(m, period, 0);
  const before = versus ? summary(m, period, -1) : null;
  const canPlay = period === "month" || period === "year";
  // With no reading time (a reader whose numbers come from an imported list), the books finished lead instead.
  const timed = now.sec > 0 || before?.sec > 0;
  const listPages = now.elsewhere.reduce((t, d) => t + (m.books[d.book]?.pages || 0), 0);
  const books = `${n(now.finished)} book${now.finished === 1 ? "" : "s"} finished`;
  const head = now.sec ? fmtDur(now.sec) : now.finished ? books : "Nothing yet";
  const sub = now.sec ? `${n(now.finished)} finished · ${n(now.pages)} pages · ${n(now.days)} day${now.days === 1 ? "" : "s"}`
    : now.finished && listPages ? `${n(listPages)} pages in those books` : "";
  return `<li class="card flex flex-col gap-1">
    <p class="text-sm text-slate-400">${label}</p>
    <p class="text-2xl font-semibold leading-tight">${head}</p>
    ${sub ? `<p class="text-sm text-slate-300">${sub}</p>` : ""}
    ${timed ? change(now.sec, before?.sec, versus) : change(now.finished, before?.finished, versus)}
    ${canPlay && timed ? `<div class="mt-auto flex flex-wrap gap-1 pt-2">
      <button type="button" data-play="${period}:0" class="btn-tonal px-4 py-1.5" ${now.sec ? "" : "disabled"}>Play it as Wrapped</button>
      <button type="button" data-play="${period}:-1" class="btn-text py-1.5" ${before?.sec ? "" : "disabled"}>${versus[0].toUpperCase()}${versus.slice(1)}</button></div>` : ""}</li>`;
}

const row = (icon, title, value, note) => `<li class="list-row"><span class="text-2xl" aria-hidden="true">${icon}</span>
  <div class="min-w-0 flex-1"><p class="font-medium leading-snug">${esc(title)}</p>${note ? `<p class="break-words text-sm text-slate-400">${esc(note)}</p>` : ""}</div>
  <p class="max-w-[45%] break-words text-right font-semibold">${esc(value)}</p></li>`;

// recordsTab is the whole tab. s.year is the year picked for awards and bests (null: all time).
export function recordsTab(m, s) {
  const years = yearsOf(m);
  const year = s.year;
  const chip = (y, text) => `<button type="button" data-year="${y ?? ""}" aria-pressed="${y === year}" class="chip-filter">${text}</button>`;
  const aw = year == null ? [] : awards(m, year);
  const rec = records(m, year);
  return `<ul class="mb-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">${TILES.map((t) => tile(m, t)).join("")}</ul>
    <div class="mb-3 flex flex-wrap gap-2" role="group" aria-label="Year">${chip(null, "All time")}${years.map((y) => chip(y, y)).join("")}</div>
    ${aw.length ? `<section class="card mb-4"><h2 class="mb-1 text-base font-medium">Your ${year} awards</h2>
      <ul class="list">${aw.map((a) => row(a.icon, a.title, a.winner, a.why || "")).join("")}</ul></section>` : ""}
    <section class="card mb-4"><h2 class="mb-1 text-base font-medium">${year == null ? "Personal bests, all time" : `Personal bests in ${year}`}</h2>
      ${rec.length ? `<ul class="list">${rec.map((r) => row(r.icon, r.title, r.value, r.note)).join("")}</ul>` : `<p class="text-sm text-slate-400">No reading recorded ${year == null ? "yet" : `in ${year}`}.</p>`}</section>
    <p class="text-xs text-slate-500">A book counts as finished when KOReader's statistics show 97% or more. Sittings and hours are counted by the hour, as KOReader records them. A period still under way is compared with the whole one before it.</p>`;
}
