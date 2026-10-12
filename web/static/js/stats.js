// 📊 Reading stats: your KOReader reading as a timeline, calendar, totals and
// habits, all in this device's own time zone.
import { get } from "./api.js";
import { $, esc } from "./ui.js";
import { ago, setBars } from "./progress.js";
import { dayKey, keyDate, prepare } from "./statscalc.js";
import { calendarView, habitsView, nowView, timelineView, totalsView } from "./statsviews.js";

const TABS = { now: "Now", timeline: "Timeline", calendar: "Calendar", totals: "Totals", habits: "Habits" };

export async function renderStats(view) {
  view.innerHTML = `<h1 class="mb-1 text-2xl font-bold">📊 Reading stats</h1><p class="text-slate-400">Loading…</p>`;
  const data = await get("/api/me/reading-stats").catch(() => null);
  if (!data) {
    view.innerHTML = `<h1 class="mb-1 text-2xl font-bold">📊 Reading stats</h1><p class="text-slate-400">Couldn't load your stats just now.</p>`;
    return;
  }
  const m = prepare(data);
  if (!m.rows.length) {
    const have = data.synced_at && m.books.length
      ? `<p>NovelCheck has your statistics file (${m.books.length} book${m.books.length === 1 ? "" : "s"}, synced ${esc(ago(data.synced_at))}) but it holds no page-by-page reading yet. Open a book in KOReader, read a few pages, then press <b>Reading statistics → Synchronize now</b> again.</p>` : "";
    view.innerHTML = `<h1 class="mb-2 text-2xl font-bold">📊 Reading stats</h1>
      <div class="card space-y-2 text-sm text-slate-300"><p>No reading recorded yet.</p>${have}
      <p>In KOReader, set up <b>Cloud storage</b> and <b>Reading statistics → Cloud sync</b> (Settings → KOReader setup has the steps), then press <b>Reading statistics → Synchronize now</b>. Your days and hours appear here after the next sync.</p>
      <a href="#/profile" class="btn-secondary inline-flex">Back to Profile</a></div>`;
    return;
  }
  const s = { tab: "now", day: dayKey(Date.now()), month: new Date(), range: "month" };
  s.month.setDate(1);
  const draw = () => {
    const body = { now: () => nowView(m), timeline: () => timelineView(m, s.day), calendar: () => calendarView(m, s.month),
      totals: () => totalsView(m, s.range), habits: () => habitsView(m) }[s.tab]();
    view.innerHTML = `<h1 class="mb-2 text-2xl font-bold">📊 Reading stats</h1>
      <div class="mb-3 flex flex-wrap gap-1">${Object.entries(TABS).map(([k, v]) =>
        `<button type="button" data-tab="${k}" class="${k === s.tab ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">${esc(v)}</button>`).join("")}</div>
      <div class="space-y-3">${body}</div>
      <p class="mt-4 text-xs text-slate-500">Days and hours use this device's time zone. Pages are different pages seen; time is time with a page open (long pauses are cut short).</p>`;
    setBars(view);
  };
  view.onclick = (e) => {
    const t = e.target.closest("[data-tab],[data-day],[data-month],[data-open-day],[data-range]");
    if (!t) return;
    if (t.dataset.tab) s.tab = t.dataset.tab;
    else if (t.dataset.day) {
      const d = keyDate(s.day);
      d.setDate(d.getDate() + Number(t.dataset.day));
      s.day = dayKey(d.getTime());
    } else if (t.dataset.month) s.month = new Date(s.month.getFullYear(), s.month.getMonth() + Number(t.dataset.month), 1);
    else if (t.dataset.openDay) {
      s.day = t.dataset.openDay;
      s.tab = "timeline";
    } else if (t.dataset.range) s.range = t.dataset.range;
    draw();
  };
  draw();
  $("h1", view)?.focus?.();
}
