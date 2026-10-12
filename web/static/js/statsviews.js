// The Reading stats views: each returns HTML for the page to show.
import { esc } from "./ui.js";
import { coverImg } from "./covers.js";
import { ago } from "./progress.js";
import { byDay, byHourOfDay, byWeekday, dayKey, fmtDur, keyDate, startOfDay, streaks, timeline, within } from "./statscalc.js";

const card = (inner) => `<div class="card space-y-2">${inner}</div>`;
const big = (n, label) => `<div class="rounded-lg bg-slate-800/60 p-3 text-center"><p class="text-xl font-bold">${n}</p><p class="text-xs text-slate-400">${label}</p></div>`;
const title = (m, i) => esc(m.books[i]?.title || "Unknown book");
const row = (name, value) => `<li class="flex justify-between gap-2"><span class="truncate">${name}</span><span class="shrink-0 text-slate-400">${value}</span></li>`;

// bars draws values as columns, with a tooltip on each.
function bars(values, labels, cls = "fill-indigo-500") {
  const W = 300, H = 80, n = values.length, top = Math.max(...values, 1), bw = W / n;
  const rects = values.map((v, i) => {
    const h = v ? Math.max(2, (v / top) * (H - 4)) : 0;
    return `<rect x="${(i * bw + 1).toFixed(1)}" y="${(H - h).toFixed(1)}" width="${(bw - 2).toFixed(1)}" height="${h.toFixed(1)}" rx="2" class="${cls}"><title>${esc(labels[i])}: ${esc(fmtDur(v))}</title></rect>`;
  }).join("");
  return `<svg viewBox="0 0 ${W} ${H}" class="h-24 w-full" role="img" aria-label="${esc(labels.map((l, i) => `${l} ${fmtDur(values[i])}`).join(", "))}">
    <line x1="0" x2="${W}" y1="${H}" y2="${H}" class="stroke-slate-700"/>${rects}</svg>`;
}
const hourLabel = (h) => `${h % 12 || 12}${h < 12 ? "a" : "p"}`;
const hourAxis = `<div class="flex justify-between text-[10px] text-slate-500"><span>12a</span><span>6a</span><span>12p</span><span>6p</span><span>11p</span></div>`;

export function nowView(m) {
  const last = m.rows.reduce((a, r) => (!a || r.ms > a.ms ? r : a), null);
  const today = within(m, startOfDay(Date.now()));
  const week = within(m, startOfDay(Date.now()) - 6 * 864e5);
  const st = streaks(byDay(m.rows));
  const b = last && m.books[last.book];
  const cur = b ? card(`<p class="label">📖 Reading now</p><div class="flex gap-3">
      <div class="w-16 shrink-0">${b.book_id ? coverImg(b.book_id, "h-24 w-full") : ""}</div>
      <div class="min-w-0 space-y-1"><p class="font-semibold leading-tight">${esc(b.title)}</p><p class="truncate text-sm text-slate-400">${esc(b.author)}</p>
        <div class="h-1.5 overflow-hidden rounded-full bg-slate-800"><div class="h-full rounded-full bg-indigo-500" data-pct="${Math.round(b.percent * 100)}"></div></div>
        <p class="text-xs text-slate-400">${Math.round(b.percent * 100)}% · ${fmtDur(b.read_seconds)} in total · last read ${esc(ago(last.ms / 1000))}</p></div></div>`) : "";
  return `${cur}${card(`<p class="label">Today and this week</p><div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
    ${big(fmtDur(today.sec), "today")}${big(today.pages.toLocaleString(), "pages today")}${big(fmtDur(week.sec), "last 7 days")}
    ${big(`${st.current} day${st.current === 1 ? "" : "s"}`, `reading streak (best ${st.longest})`)}</div>`)}`;
}

export function timelineView(m, key) {
  const t = timeline(m, key);
  const d = keyDate(key);
  const isToday = key === dayKey(Date.now());
  const total = t.hours.reduce((a, b) => a + b, 0);
  return card(`<div class="flex items-center justify-between gap-2">
      <button type="button" data-day="-1" class="btn-ghost px-3 py-1" aria-label="Previous day">‹</button>
      <p class="text-center font-semibold">${isToday ? "Today, " : ""}${esc(d.toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric" }))}</p>
      <button type="button" data-day="1" class="btn-ghost px-3 py-1" aria-label="Next day" ${isToday ? "disabled" : ""}>›</button></div>
    ${total ? `${bars(t.hours, t.hours.map((_, h) => hourLabel(h)))}${hourAxis}
      <p class="text-sm text-slate-300">${fmtDur(total)} of reading</p>
      <ul class="space-y-1 text-sm">${t.books.map((b) => row(title(m, b.book), fmtDur(b.sec))).join("")}</ul>`
    : `<p class="py-6 text-center text-sm text-slate-500">No reading recorded this day.</p>`}`);
}

const HEAT = ["fill-slate-800", "fill-indigo-900", "fill-indigo-700", "fill-indigo-500", "fill-indigo-300"];
const CELL = ["bg-slate-800", "bg-indigo-900", "bg-indigo-700", "bg-indigo-500", "bg-indigo-300 text-slate-900"];
const level = (sec) => (!sec ? 0 : sec < 600 ? 1 : sec < 1800 ? 2 : sec < 3600 ? 3 : 4);

export function calendarView(m, month) {
  const days = byDay(m.rows);
  const first = new Date(month.getFullYear(), month.getMonth(), 1);
  const count = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  const cells = [];
  for (let i = 0; i < first.getDay(); i++) cells.push(`<span></span>`);
  for (let d = 1; d <= count; d++) {
    const key = dayKey(new Date(month.getFullYear(), month.getMonth(), d).getTime());
    const sec = days.get(key)?.sec || 0;
    cells.push(`<button type="button" data-open-day="${key}" class="${CELL[level(sec)]} aspect-square rounded text-xs" title="${esc(fmtDur(sec))}">${d}</button>`);
  }
  return `${card(`<div class="flex items-center justify-between gap-2">
      <button type="button" data-month="-1" class="btn-ghost px-3 py-1" aria-label="Previous month">‹</button>
      <p class="font-semibold">${esc(month.toLocaleDateString(undefined, { month: "long", year: "numeric" }))}</p>
      <button type="button" data-month="1" class="btn-ghost px-3 py-1" aria-label="Next month">›</button></div>
    <div class="grid grid-cols-7 gap-1 text-center text-[10px] text-slate-500">${["S", "M", "T", "W", "T", "F", "S"].map((x) => `<span>${x}</span>`).join("")}</div>
    <div class="grid grid-cols-7 gap-1">${cells.join("")}</div>
    <p class="text-xs text-slate-500">Darker means longer. Tap a day for its timeline.</p>`)}
    ${card(`<p class="label">The last year</p>${yearHeat(days)}`)}`;
}

// yearHeat is a column per week for the last 53 weeks.
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

export const RANGES = { week: "7 days", month: "30 days", year: "Year", all: "All time" };

export function totalsView(m, range) {
  const now = startOfDay(Date.now());
  const from = { week: now - 6 * 864e5, month: now - 29 * 864e5, year: now - 364 * 864e5, all: 0 }[range];
  const t = within(m, from);
  return card(`<div class="flex flex-wrap gap-1">${Object.entries(RANGES).map(([k, v]) =>
      `<button type="button" data-range="${k}" class="${k === range ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">${v}</button>`).join("")}</div>
    <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">${big(fmtDur(t.sec), "reading time")}${big(t.pages.toLocaleString(), "pages")}
      ${big(t.books.length, "books read")}${big(t.finished, "finished")}</div>
    <p class="text-xs text-slate-500">${t.days} reading day${t.days === 1 ? "" : "s"}${t.days ? ` · ${fmtDur(t.sec / t.days)} on an average one` : ""}</p>
    ${t.books.length ? `<p class="label pt-2">Most read</p><ul class="space-y-1 text-sm">${t.books.slice(0, 10).map((b) => row(title(m, b.book), fmtDur(b.sec))).join("")}</ul>` : ""}`);
}

export function habitsView(m) {
  const days = byDay(m.rows);
  const st = streaks(days);
  const best = [...days].sort((a, b) => b[1].sec - a[1].sec)[0];
  const wk = byWeekday(m.rows);
  const names = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const hours = byHourOfDay(m.rows);
  const peak = hours.indexOf(Math.max(...hours));
  return `${card(`<p class="label">When you read</p>${bars(hours, hours.map((_, h) => hourLabel(h)))}${hourAxis}
      <p class="text-sm text-slate-300">Most around <b>${hourLabel(peak)}</b>.</p>`)}
    ${card(`<p class="label">Which days</p>${bars(wk, names, "fill-emerald-500")}
      <div class="flex justify-between text-[10px] text-slate-500">${names.map((n) => `<span>${n}</span>`).join("")}</div>`)}
    ${card(`<div class="grid grid-cols-2 gap-2">${big(`${st.longest} days`, "longest streak")}${best ? big(fmtDur(best[1].sec), `best day, ${esc(keyDate(best[0]).toLocaleDateString())}`) : ""}</div>`)}`;
}
