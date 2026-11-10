// More trophies (reading habit, book shape, series and year), added to the list in trophies.js. `add` is that
// file's own builder, so these look and level up the same way.
import { dayKey } from "./statscalc.js";

const FINISHED = 0.97;

// spans: when each book was first and last read (ms), from the page-by-page rows.
function spans(m) {
  const s = new Map();
  for (const r of m.rows) {
    const k = String(r.book);
    const e = s.get(k) || { first: r.ms, last: r.ms };
    e.first = Math.min(e.first, r.ms);
    e.last = Math.max(e.last, r.ms);
    s.set(k, e);
  }
  return s;
}

export function moreTrophies(m, f, add) {
  const sp = spans(m);
  const fin = [...sp].map(([k, e]) => ({ b: m.books[k], ...e })).filter((x) => x.b && (x.b.percent || 0) >= FINISHED);

  // Habit
  const oneDay = fin.filter((x) => dayKey(x.first) === dayKey(x.last)).length;
  add("Habit", "oneday", "⚡", "One sitting", "Books finished the day you started them", oneDay, [1, 5, 10, 25]);
  const lunch = new Set();
  for (const r of m.rows) {
    const h = new Date(r.ms).getHours();
    if (h >= 11 && h < 14) lunch.add(dayKey(r.ms));
  }
  add("Habit", "lunch", "🥪", "Lunch break reader", "Days you read between 11 and 2", lunch.size, [10, 50, 100, 200]);
  add("Habit", "streak7", "🔥", "On a roll", "Longest streak, in days", f.streak, [7, 14, 30, 60, 100]);

  // Book shape
  const byMonth = new Map();
  for (const x of fin) {
    const d = new Date(x.last);
    const key = `${d.getFullYear()}-${d.getMonth()}`;
    byMonth.set(key, (byMonth.get(key) || 0) + 1);
  }
  add("Book shape", "month", "🗓️", "Hot streak", "Most books finished in one month", Math.max(0, ...byMonth.values()), [3, 5, 8, 10]);
  add("Book shape", "long", "🧱", "Doorstop", "Books of 800+ pages finished", fin.filter((x) => (x.b.pages || 0) >= 800).length, [1, 3, 5, 10]);
  add("Book shape", "short", "🍬", "Quick read", "Books under 100 pages finished", fin.filter((x) => x.b.pages && x.b.pages < 100).length, [1, 5, 10]);

  // Series and year
  const series = new Map();
  for (const x of fin) if (x.b.series) series.set(x.b.series, (series.get(x.b.series) || 0) + 1);
  add("Series and year", "seriesrun", "🗂️", "Series marathon", "Most books of one series finished", Math.max(0, ...series.values()), [3, 5, 8, 12]);
  const year = new Date().getFullYear();
  const inYear = (y) => fin.filter((x) => new Date(x.last).getFullYear() === y).length;
  const thisYear = inYear(year);
  const lastYear = inYear(year - 1);
  if (lastYear > 0) add("Series and year", "beat", "📈", `Beat ${year - 1}`, `Books finished in ${year}`, thisYear, [lastYear + 1]);
  add("Series and year", "year", "🎯", `${year} readathon`, `Books finished in ${year}`, thisYear, [12, 24, 52, 100]);
}
