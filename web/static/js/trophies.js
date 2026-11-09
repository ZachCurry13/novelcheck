// 🏆 Trophies and records, worked out in this browser from the same reading statistics as the report (so the
// time zone is yours). A book counts as read when KOReader's statistics show you finished it (97% or more).
// Each trophy has levels (bronze, silver, gold, platinum, diamond) and shows how far to the next.
import { esc } from "./ui.js";
import { byDay, dayKey, fmtDur, streaks } from "./statscalc.js";

const LEVELS = ["Bronze", "Silver", "Gold", "Platinum", "Diamond"];
const MEDAL = ["🥉", "🥈", "🥇", "💎", "👑"];
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
  return a;
}

// records: the best of you, all time.
export function records(m) {
  const f = facts(m);
  const long = (ms) => new Date(ms).toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric", year: "numeric" });
  const out = [];
  if (f.bestDay) {
    out.push({ icon: "📆", title: "Longest day of reading", value: fmtDur(f.bestDay[1].sec), note: long(new Date(f.bestDay[0] + "T12:00:00").getTime()) });
    const pagesDay = [...f.days].sort((a, b) => b[1].pages - a[1].pages)[0];
    if (pagesDay?.[1].pages) out.push({ icon: "📖", title: "Most pages in a day", value: `${n(pagesDay[1].pages)} pages`, note: long(new Date(pagesDay[0] + "T12:00:00").getTime()) });
  }
  if (f.streak) out.push({ icon: "🔥", title: "Longest streak", value: `${n(f.streak)} day${f.streak === 1 ? "" : "s"} in a row`, note: "" });
  const per = new Map();
  for (const r of m.rows) per.set(r.book, (per.get(r.book) || 0) + r.pages);
  const most = [...per].sort((a, b) => b[1] - a[1])[0];
  if (most && most[1]) out.push({ icon: "🏆", title: "Book with the most pages read", value: `${n(most[1])} pages`, note: m.books[most[0]]?.title || "" });
  const big = f.fin.slice().sort((a, b) => (b.pages || 0) - (a.pages || 0))[0];
  if (big?.pages) out.push({ icon: "🧱", title: "Longest book finished", value: `${n(big.pages)} pages`, note: big.title });
  const slow = f.fin.filter((b) => b.read_seconds > 0).sort((a, b) => b.read_seconds - a.read_seconds)[0];
  if (slow) out.push({ icon: "🐢", title: "Book you spent the most time on", value: fmtDur(slow.read_seconds), note: slow.title });
  return out;
}

const medals = (a) => a.tiers.map((t, i) => `<span title="${LEVELS[i] || `Level ${i + 1}`}: ${n(t)}" class="${i < a.level ? "" : "opacity-25 grayscale"}">${MEDAL[Math.min(i, MEDAL.length - 1)]}</span>`).join("");

export function trophiesHTML(m) {
  const list = achievements(m);
  const groups = [...new Set(list.map((a) => a.group))];
  const earned = list.reduce((t, a) => t + a.level, 0);
  const total = list.reduce((t, a) => t + a.tiers.length, 0);
  const card = (a) => `<li class="card space-y-1 p-3" data-trophy="${esc(a.id)}">
    <div class="flex items-start gap-3"><span class="text-3xl" aria-hidden="true">${a.icon}</span>
      <div class="min-w-0 flex-1"><p class="font-semibold leading-snug">${esc(a.title)}${a.level ? ` <span class="text-xs font-normal text-amber-300">${LEVELS[Math.min(a.level - 1, LEVELS.length - 1)]}</span>` : ""}</p>
        <p class="text-xs text-slate-400">${esc(a.what)}: <b class="text-slate-200">${n(a.value)}</b>${a.next ? ` · next at ${n(a.next)}` : " · every level earned"}</p></div></div>
    <div class="flex gap-1 text-lg">${medals(a)}</div>
    <div class="h-1.5 overflow-hidden rounded-full bg-slate-800"><div class="h-full rounded-full bg-amber-400" data-pct="${a.pct}"></div></div></li>`;
  return `<p class="mb-3 text-sm text-slate-400"><b class="text-slate-200">${n(earned)}</b> of ${n(total)} levels earned. A book counts when KOReader's statistics show you finished it.</p>
    ${groups.map((g) => `<section class="mb-5"><h2 class="mb-2 text-lg font-semibold">${esc(g)}</h2>
      <ul class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">${list.filter((a) => a.group === g).map(card).join("")}</ul></section>`).join("")}
    <section class="mb-5"><h2 class="mb-2 text-lg font-semibold">🏅 Records</h2>
      <ul class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">${records(m).map((r) => `<li class="card p-3"><p class="text-2xl">${r.icon}</p><p class="text-xs uppercase tracking-wide text-slate-400">${esc(r.title)}</p>
        <p class="text-xl font-bold">${esc(r.value)}</p><p class="text-xs text-slate-400">${esc(r.note)}</p></li>`).join("") || `<li class="text-sm text-slate-400">Your records show up once KOReader's statistics are in.</li>`}</ul></section>`;
}

// paintBars sets the progress widths (the page's security rules allow no inline styles in the HTML).
export const paintBars = (root) => root.querySelectorAll("[data-pct]").forEach((el) => (el.style.width = `${el.dataset.pct}%`));
