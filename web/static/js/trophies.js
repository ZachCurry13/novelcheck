// 🏆 Trophies (drawn by trophyview.js; records and awards are in records.js), worked out in this browser from the same reading statistics as the report (so the
// time zone is yours). A book counts as read when KOReader's statistics show you finished it (97% or more).
// Each trophy has levels (bronze, silver, gold, platinum, diamond) and shows how far to the next.
import { esc } from "./ui.js";
import { byDay, dayKey, fmtDur, streaks } from "./statscalc.js";
import { moreTrophies } from "./trophies2.js";
import { trophies3 } from "./trophies3.js";

export const LEVELS = ["Bronze", "Silver", "Gold", "Platinum", "Diamond"];
const FINISHED = 0.97;
const n = (x) => Number(x).toLocaleString();

// The genres that always get a trophy, so there is something to aim for; any other genre you read gets one too.
const GENRES = ["Young Adult", "Romance", "Science Fiction", "Fantasy", "Mystery", "Thriller", "Historical", "Horror"];

export const finished = (m) => m.books.filter((b) => (b.percent || 0) >= FINISHED);

function facts(m) {
  const fin = finished(m);
  const days = byDay(m.rows);
  const sec = m.rows.reduce((t, r) => t + r.sec, 0);
  const pages = m.rows.reduce((t, r) => t + r.pages, 0);
  const night = new Set();
  const early = new Set();
  for (const r of m.rows) {
    const h = new Date(r.ms).getHours();
    if (h < 4) night.add(dayKey(r.ms));
    else if (h >= 4 && h < 7) early.add(dayKey(r.ms));
  }
  const byGenre = new Map();
  const byAuthor = new Map();
  const bySeries = new Map();
  for (const b of fin) {
    for (const g of b.genres || []) byGenre.set(g, (byGenre.get(g) || 0) + 1);
    if (b.author) byAuthor.set(b.author, (byAuthor.get(b.author) || 0) + 1);
    if (b.series) {
      const s = bySeries.get(b.series) || { done: 0, total: b.series_total || 0 };
      s.done++;
      bySeries.set(b.series, s);
    }
  }
  const bestDay = [...days].sort((a, b) => b[1].sec - a[1].sec)[0];
  return {
    fin, days, sec, pages, night: night.size, early: early.size, byGenre, byAuthor,
    seriesDone: [...bySeries.values()].filter((s) => s.total >= 3 && s.done >= s.total).length,
    streak: streaks(days).longest, bestDayHours: bestDay ? bestDay[1].sec / 3600 : 0, bestDay,
    bigBook: Math.max(0, ...fin.map((b) => b.pages || 0)),
    topAuthor: Math.max(0, ...byAuthor.values()),
    kinds: byGenre.size,
    highlights: m.books.reduce((t, b) => t + (b.highlights || 0), 0),
  };
}

// achievements lists every trophy with where you are: [{id, group, icon, title, what, value, tiers, level, next, pct}].
export function achievements(m) {
  const f = facts(m);
  const a = [];
  const add = (group, id, icon, title, what, value, tiers, unit = "") => {
    const level = tiers.filter((t) => value >= t).length;
    const next = tiers[level] ?? null;
    const prev = level ? tiers[level - 1] : 0;
    a.push({ id, group, icon, title, what, value, tiers, level, next, unit, pct: next ? Math.max(0, Math.min(100, Math.round(((value - prev) / (next - prev)) * 100))) : 100 });
  };
  add("Reading", "books", "📚", "Bookworm", "Books finished", f.fin.length, [1, 5, 10, 25, 50, 100, 250]);
  add("Reading", "pages", "📄", "Page turner", "Pages read", f.pages, [1000, 5000, 10000, 25000, 50000, 100000]);
  add("Reading", "hours", "⏱️", "Time well spent", "Hours of reading", Math.floor(f.sec / 3600), [10, 50, 100, 250, 500, 1000]);
  add("Habits", "streak", "🔥", "On a roll", "Longest streak, in days", f.streak, [3, 7, 14, 30, 100]);
  add("Habits", "marathon", "🏃", "Marathon reader", "Most hours in one day", Math.floor(f.bestDayHours), [2, 4, 8]);
  add("Habits", "night", "🌙", "Night owl", "Days you read after midnight", f.night, [5, 25, 100]);
  add("Habits", "early", "🌅", "Early bird", "Days you read before 7", f.early, [5, 25, 100]);
  add("Variety", "variety", "🧭", "Explorer", "Different genres finished", f.kinds, [3, 5, 10]);
  add("Variety", "series", "🗂️", "Series finisher", "Whole series finished", f.seriesDone, [1, 3, 5, 10]);
  add("Variety", "author", "✍️", "Loyal fan", "Books finished by one author", f.topAuthor, [5, 10, 25]);
  add("Variety", "big", "🧱", "Heavyweight", "Pages in the longest book you finished", f.bigBook, [500, 800, 1200]);
  add("Variety", "notes", "🖍️", "Marginalia", "Highlights made", f.highlights, [10, 100, 500]);
  const have = [...f.byGenre].sort((x, y) => y[1] - x[1]);
  const wanted = new Set([...GENRES.filter((g) => [...f.byGenre.keys()].some((k) => k.toLowerCase() === g.toLowerCase()) || true), ...have.slice(0, 6).map((x) => x[0])]);
  for (const g of wanted) {
    const key = [...f.byGenre.keys()].find((k) => k.toLowerCase() === g.toLowerCase());
    add("Genres", `genre:${g}`, "🏷️", `${g} reader`, `${g} books finished`, key ? f.byGenre.get(key) : 0, [10, 25, 50, 100]);
  }
  moreTrophies(m, f, add);
  trophies3(m, f, add);
  return a;
}

// paintBars sets the progress widths (the page's security rules allow no inline styles in the HTML).
export const paintBars = (root) => root.querySelectorAll("[data-pct]").forEach((el) => (el.style.width = `${el.dataset.pct}%`));
