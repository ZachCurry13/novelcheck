// Reading stats → Trophies: how many levels you have earned, what is next (with a ring and a likely
// date), then every trophy with its named levels ticked off. The trophies themselves are in trophies.js.
import { esc } from "./ui.js";
import { achievements, LEVELS } from "./trophies.js";

const n = (x) => Number(x).toLocaleString();
const DAY = 864e5;
const LOOK_BACK = 90; // days of recent reading the likely date is worked out from
const RING = 2 * Math.PI * 24;

// earlier is the same reading as it stood `days` ago: later hours left out, and books finished since then not yet finished.
function earlier(m, days) {
  const cut = Date.now() - days * DAY;
  const last = new Map();
  for (const r of m.rows) last.set(String(r.book), Math.max(last.get(String(r.book)) || 0, r.ms));
  const books = Array.isArray(m.books) ? m.books.map((b, i) => ((last.get(String(i)) || 0) >= cut ? { ...b, percent: 0 } : b)) : m.books;
  return { ...m, books, rows: m.rows.filter((r) => r.ms < cut) };
}

// likely: when the next level comes at the pace of the last 90 days, or "" when there is no pace to go by.
function likely(a, before) {
  const gained = a.value - (before?.value ?? a.value);
  if (gained <= 0 || a.next == null) return "";
  const days = Math.ceil((a.next - a.value) / (gained / LOOK_BACK));
  if (days > 730) return "";
  if (days <= 1) return "any day now";
  return `around ${new Date(Date.now() + days * DAY).toLocaleDateString(undefined, days > 300 ? { month: "short", year: "numeric" } : { month: "short", day: "numeric" })}`;
}

const ring = (pct) => `<svg viewBox="0 0 56 56" class="h-14 w-14 flex-none -rotate-90" role="img" aria-label="${pct}% of the way">
  <circle cx="28" cy="28" r="24" fill="none" stroke-width="5" class="ring-track"></circle>
  <circle cx="28" cy="28" r="24" fill="none" stroke-width="5" stroke-linecap="round" class="ring-fill" stroke-dasharray="${((pct / 100) * RING).toFixed(1)} ${RING.toFixed(1)}"></circle></svg>`;

const levelName = (i) => LEVELS[i] || `Level ${i + 1}`;

function nextUp(list, m) {
  const before = new Map(achievements(earlier(m, LOOK_BACK)).map((a) => [a.id, a]));
  const open = list.filter((a) => a.next != null && a.value > 0).sort((x, y) => y.pct - x.pct).slice(0, 4);
  if (!open.length) return "";
  return `<section class="mb-6"><h2 class="mb-2 text-base font-medium">Next up</h2><ul class="grid gap-3 sm:grid-cols-2">${open.map((a) => {
    const when = likely(a, before.get(a.id));
    return `<li class="card flex items-center gap-4"><div class="relative flex-none">${ring(a.pct)}<span class="absolute inset-0 flex items-center justify-center text-xl" aria-hidden="true">${a.icon}</span></div>
      <div class="min-w-0"><p class="font-medium leading-snug">${esc(a.title)} · ${levelName(a.level)}</p>
        <p class="text-sm text-slate-400">${esc(a.what)}: ${n(a.value)} of ${n(a.next)}</p>
        <p class="text-sm text-indigo-300">${n(a.next - a.value)} to go${when ? ` · ${when}` : ""}</p></div></li>`;
  }).join("")}</ul></section>`;
}

const trophy = (a) => `<li class="list-row items-start" data-trophy="${esc(a.id)}"><span class="text-2xl" aria-hidden="true">${a.icon}</span>
  <div class="min-w-0 flex-1 space-y-1.5"><p class="font-medium leading-snug">${esc(a.title)}</p>
    <p class="text-sm text-slate-400">${esc(a.what)}: <b class="text-slate-200">${n(a.value)}</b>${a.next ? ` · next at ${n(a.next)}` : " · every level earned"}</p>
    <ul class="flex flex-wrap gap-1.5">${a.tiers.map((t, i) => `<li class="chip ${i < a.level ? "bg-amber-900/60 text-amber-200" : "bg-slate-800 text-slate-400"}">${i < a.level ? `<span aria-hidden="true">✓</span><span class="sr-only">Earned: </span>` : ""}${levelName(i)} · ${n(t)}</li>`).join("")}</ul></div></li>`;

export function trophiesTab(m) {
  const list = achievements(m);
  const groups = [...new Set(list.map((a) => a.group))];
  const earned = list.reduce((t, a) => t + a.level, 0);
  const total = list.reduce((t, a) => t + a.tiers.length, 0);
  return `<section class="card mb-6 space-y-2"><p class="text-2xl font-semibold">${n(earned)} of ${n(total)} earned</p>
      <div class="meter-track bg-slate-800"><div class="meter-fill bg-amber-400" data-pct="${total ? Math.round((earned / total) * 100) : 0}"></div></div>
      <p class="text-sm text-slate-400">${n(list.filter((a) => a.level).length)} of ${n(list.length)} trophies started. Each one has named levels to earn.</p></section>
    ${nextUp(list, m)}
    ${groups.map((g) => {
      const mine = list.filter((a) => a.group === g);
      return `<section class="card mb-4"><h2 class="mb-1 flex items-baseline justify-between text-base font-medium">${esc(g)}<span class="text-sm font-normal text-slate-400">${mine.reduce((t, a) => t + a.level, 0)} of ${mine.reduce((t, a) => t + a.tiers.length, 0)}</span></h2>
        <ul class="list">${mine.map(trophy).join("")}</ul></section>`;
    }).join("")}
    <p class="text-xs text-slate-500">A book counts when KOReader's statistics show you finished it (97% or more). Likely dates assume you keep reading as you did in the last ${LOOK_BACK} days.</p>`;
}
