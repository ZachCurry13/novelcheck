// A third set of trophies (reading totals, streaks, sittings, authors, variety, Hardcover goals), added to
// the list in trophies.js with its own builder so they look and level up the same way.
import { dayKey } from "./statscalc.js";

const HOUR = 3600e3;
const WEEK = 7 * 864e5;

// sittings: runs of hours in a row with reading in each, as seconds (the statistics come by the hour).
function sittings(rows) {
  const perHour = new Map();
  for (const r of rows) perHour.set(r.ms, (perHour.get(r.ms) || 0) + r.sec);
  const out = [];
  let last = -Infinity;
  for (const [ms, sec] of [...perHour].sort((a, b) => a[0] - b[0])) {
    if (ms - last <= HOUR && out.length) out[out.length - 1] += sec;
    else out.push(sec);
    last = ms;
  }
  return out;
}

// weekRun: the most weeks in a row (Monday to Sunday) with any reading.
function weekRun(rows) {
  const monday = (ms) => {
    const d = new Date(ms);
    return new Date(d.getFullYear(), d.getMonth(), d.getDate() - ((d.getDay() + 6) % 7)).getTime();
  };
  const weeks = [...new Set(rows.map((r) => monday(r.ms)))].sort((a, b) => a - b);
  let best = 0, run = 0, prev = null;
  for (const w of weeks) {
    run = prev != null && Math.round((w - prev) / WEEK) === 1 ? run + 1 : 1;
    prev = w;
    best = Math.max(best, run);
  }
  return best;
}

export function trophies3(m, f, add) {
  // Reading totals
  add("Reading", "days", "📆", "Regular", "Days with reading", f.days.size, [30, 100, 365, 730, 1000]);
  add("Reading", "opened", "📖", "Curious", "Different books opened", Object.keys(m.books).length, [10, 25, 50, 100, 250]);

  // Streaks
  add("Streaks", "weeks", "🗓️", "Week after week", "Most weeks in a row with reading", weekRun(m.rows), [4, 12, 26, 52]);
  const weekends = new Set(m.rows.filter((r) => [0, 6].includes(new Date(r.ms).getDay())).map((r) => dayKey(r.ms)));
  add("Streaks", "weekends", "🛋️", "Weekend reader", "Weekend days with reading", weekends.size, [10, 50, 100, 200]);

  // Sittings
  const sits = sittings(m.rows);
  add("Sittings", "sitting", "🪑", "Settled in", "Longest sitting, in hours", Math.floor(Math.max(0, ...sits) / 3600), [1, 2, 3, 5]);
  add("Sittings", "longsits", "☕", "Long sittings", "Sittings of two hours or more", sits.filter((s) => s >= 7200).length, [1, 10, 25, 50]);

  // Authors and series
  add("Authors and series", "authors", "🖋️", "Well introduced", "Different authors finished", f.byAuthor.size, [5, 10, 25, 50, 100]);
  const series = new Set(f.fin.map((b) => b.series).filter(Boolean));
  add("Authors and series", "seriesstart", "🚪", "Series starter", "Different series you finished a book in", series.size, [1, 3, 5, 10]);

  // Variety
  add("Variety", "nonfiction", "🎓", "Fact finder", "Non-fiction books finished", f.fin.filter((b) => b.kind === "nonfiction").length, [1, 5, 10, 25]);

  // Hardcover goals, when connected
  const goals = (m.goals || []).filter((g) => g.goal > 0);
  if (goals.length) add("Hardcover goals", "goals", "🎯", "Goal getter", "Hardcover reading goals reached", goals.filter((g) => (g.progress || 0) >= g.goal).length, [1, 2, 3, 5]);
}
