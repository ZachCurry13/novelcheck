// Profile: "My reading", a few numbers from your KOReader reading statistics
// with a way into the full Reading stats page. No data yet? It says how to
// get it.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { byDay, fmtDur, prepare, startOfDay, streaks, within } from "./statscalc.js";

const big = (n, label) => `<div class="rounded-lg bg-slate-800/60 p-3 text-center"><p class="text-lg font-bold">${n}</p><p class="text-xs text-slate-400">${label}</p></div>`;

export async function renderReadingSummary(host) {
  const data = await get("/api/me/reading-stats").catch(() => null);
  const m = data && prepare(data);
  if (!m || !m.rows.length) {
    host.innerHTML = `<h2 class="text-lg font-semibold">📊 My reading</h2>
      <p class="text-sm text-slate-400">Your reading time, streaks and calendar show up here once your Kindle's reading statistics sync.</p>
      <a href="#/settings?s=kindle" class="btn-secondary inline-flex">Set up my Kindle</a>`;
    return;
  }
  const now = startOfDay(Date.now());
  const monthStart = new Date(new Date().getFullYear(), new Date().getMonth(), 1).getTime();
  const month = within(m, monthStart);
  const week = within(m, now - 6 * 864e5);
  const year = within(m, new Date(new Date().getFullYear(), 0, 1).getTime());
  const st = streaks(byDay(m.rows));
  host.innerHTML = `<h2 class="text-lg font-semibold">📊 My reading</h2>
    <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">${big(esc(fmtDur(week.sec)), "last 7 days")}${big(esc(fmtDur(month.sec)), "this month")}
      ${big(`${st.current} day${st.current === 1 ? "" : "s"}`, `reading streak (best ${st.longest})`)}${big(year.finished, "books finished this year")}</div>
    <a href="#/stats" class="btn-secondary inline-flex">📊 Open Reading stats</a>`;
}
