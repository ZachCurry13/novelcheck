// Reading stats, the Calendar tab: a month you can move through (darker = more
// reading, tap a day to see it) and the last year at a glance.
import { esc } from "./ui.js";
import { byDay, dayKey, fmtDur } from "./statscalc.js";

const HEAT = ["fill-slate-800", "fill-indigo-900", "fill-indigo-700", "fill-indigo-500", "fill-indigo-300"];
const CELL = ["bg-slate-800", "bg-indigo-900", "bg-indigo-700", "bg-indigo-500", "bg-indigo-300 text-slate-900"];
const level = (sec) => (!sec ? 0 : sec < 600 ? 1 : sec < 1800 ? 2 : sec < 3600 ? 3 : 4);

export function calendarTab(m, month) {
  const days = byDay(m.rows);
  const first = new Date(month.getFullYear(), month.getMonth(), 1);
  const count = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  const cells = [];
  for (let i = 0; i < first.getDay(); i++) cells.push(`<span></span>`);
  for (let d = 1; d <= count; d++) {
    const key = dayKey(new Date(month.getFullYear(), month.getMonth(), d).getTime());
    const sec = days.get(key)?.sec || 0;
    cells.push(`<button type="button" data-day-key="${key}" class="${CELL[level(sec)]} aspect-square rounded text-sm" title="${esc(fmtDur(sec))}">${d}</button>`);
  }
  const total = [...days].filter(([k]) => k.startsWith(`${month.getFullYear()}-${String(month.getMonth() + 1).padStart(2, "0")}`)).reduce((a, [, e]) => a + e.sec, 0);
  return `<div class="card space-y-2"><div class="flex items-center justify-between gap-2">
      <button type="button" data-month="-1" class="btn-ghost px-3 py-1" aria-label="Previous month">‹</button>
      <div class="text-center"><h2 class="text-xl font-semibold">${esc(month.toLocaleDateString(undefined, { month: "long", year: "numeric" }))}</h2><p class="text-sm text-slate-400">${total ? `${fmtDur(total)} of reading` : "No reading this month"}</p></div>
      <button type="button" data-month="1" class="btn-ghost px-3 py-1" aria-label="Next month">›</button></div>
    <div class="grid grid-cols-7 gap-1 text-center text-xs text-slate-500">${["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"].map((x) => `<span>${x}</span>`).join("")}</div>
    <div class="grid grid-cols-7 gap-1">${cells.join("")}</div>
    <p class="text-xs text-slate-500">Darker means longer. Tap a day to see it.</p></div>
    <div class="card space-y-2"><h2 class="text-xl font-semibold">The last year</h2>${yearHeat(days)}</div>`;
}

function yearHeat(days) {
  const end = new Date();
  end.setHours(0, 0, 0, 0);
  const total = 52 * 7 + end.getDay();
  const startMs = end.getTime() - total * 864e5;
  let rects = "";
  for (let i = 0; i <= total; i++) {
    const d = new Date(startMs + i * 864e5);
    const sec = days.get(dayKey(d.getTime()))?.sec || 0;
    rects += `<rect x="${Math.floor(i / 7) * 11}" y="${(i % 7) * 11}" width="9" height="9" rx="2" class="${HEAT[level(sec)]}"><title>${esc(d.toLocaleDateString())}: ${esc(fmtDur(sec))}</title></rect>`;
  }
  return `<svg viewBox="0 0 ${53 * 11} 77" class="w-full" role="img" aria-label="Reading by day for the last year">${rects}</svg>`;
}
