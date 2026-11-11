// 🏅 Records and awards: the best of your reading, for all time or for one year, and that year's awards
// (book of the year, fastest read, author of the year…). Worked out in this browser from the same hourly
// reading statistics as the rest of Reading stats, so days and hours are in your own time zone.
import { esc } from "./ui.js";
import { byDay, dayKey, keyDate, fmtDur, streaks } from "./statscalc.js";
import { GENRES } from "./genres.js";

const FINISHED = 0.97;
const HOUR = 3600e3;
const n = (x) => Number(x).toLocaleString();
const plural = (k, one) => `${n(k)} ${one}${k === 1 ? "" : "s"}`;
const dayText = (ms) => new Date(ms).toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric", year: "numeric" });
const keyText = (k) => dayText(keyDate(k).getTime());
const hourText = (h) => new Date(2000, 0, 1, h).toLocaleTimeString(undefined, { hour: "numeric" });
const top = (map) => [...map].sort((a, b) => b[1] - a[1])[0];
const add = (map, k, v = 1) => map.set(k, (map.get(k) || 0) + v);

// yearsOf lists the years that have reading, newest first.
export const yearsOf = (m) => [...new Set(m.rows.map((r) => new Date(r.ms).getFullYear()))].sort((a, b) => b - a);

// facts gathers everything the records and awards are read from, for one year or (year = null) all time.
function facts(m, year) {
  const rows = m.rows.filter((r) => year == null || new Date(r.ms).getFullYear() === year).sort((a, b) => a.ms - b.ms);
  const span = new Map(); // book -> first and last hour read (all time: a book started last year counts whole)
  for (const r of m.rows) {
    const e = span.get(r.book) || { first: r.ms, last: r.ms, sec: 0 };
    e.first = Math.min(e.first, r.ms);
    e.last = Math.max(e.last, r.ms);
    e.sec += r.sec;
    span.set(r.book, e);
  }
  const fin = [...span].map(([i, e]) => ({ b: m.books[i], ...e, days: Math.max(1, Math.round((e.last - e.first) / 864e5) + 1) }))
    .filter((x) => x.b && (x.b.percent || 0) >= FINISHED && (year == null || new Date(x.last).getFullYear() === year));
  const days = byDay(rows);
  const weeks = new Map(), months = new Map(), finMonths = new Map(), hours = new Map(), weekdays = new Map();
  for (const r of rows) {
    const d = new Date(r.ms);
    const monday = new Date(d.getFullYear(), d.getMonth(), d.getDate() - ((d.getDay() + 6) % 7)).getTime();
    add(weeks, monday, r.sec);
    add(months, `${d.getFullYear()}-${d.getMonth()}`, r.sec);
    add(hours, d.getHours(), r.sec);
    add(weekdays, d.getDay(), r.sec);
  }
  for (const x of fin) add(finMonths, `${new Date(x.last).getFullYear()}-${new Date(x.last).getMonth()}`);
  // A sitting: hours in a row with reading in each (the statistics come by the hour).
  let sit = { sec: 0, at: 0 }, run = { sec: 0, at: 0, last: -Infinity };
  const perHour = new Map();
  for (const r of rows) add(perHour, r.ms, r.sec);
  for (const [ms, sec] of [...perHour].sort((a, b) => a[0] - b[0])) {
    run = ms - run.last <= HOUR ? { sec: run.sec + sec, at: run.at, last: ms } : { sec, at: ms, last: ms };
    if (run.sec > sit.sec) sit = { sec: run.sec, at: run.at };
  }
  const authors = new Map(), genres = new Map(), series = new Map();
  for (const x of fin) {
    if (x.b.author) add(authors, x.b.author);
    if (x.b.series) add(series, x.b.series);
    for (const g of x.b.genres || []) add(genres, g);
  }
  return { rows, fin, days, weeks, months, finMonths, hours, weekdays, sit, authors, genres, series,
    sec: rows.reduce((t, r) => t + r.sec, 0), pages: rows.reduce((t, r) => t + r.pages, 0) };
}

const monthText = (key) => {
  const [y, mo] = key.split("-").map(Number);
  return new Date(y, mo, 1).toLocaleDateString(undefined, { month: "long", year: "numeric" });
};

// records: [{icon, title, value, note}], the ones there is data for.
export function records(m, year = null) {
  const f = facts(m, year);
  const out = [];
  const put = (icon, title, value, note = "") => out.push({ icon, title, value, note });
  if (!f.rows.length) return out;
  put("⏱️", "Time read", fmtDur(f.sec), `${plural(f.days.size, "reading day")} · ${n(f.pages)} pages · ${plural(f.fin.length, "book")} finished`);
  const dSec = top(new Map([...f.days].map(([k, v]) => [k, v.sec])));
  const dPages = top(new Map([...f.days].map(([k, v]) => [k, v.pages])));
  put("📆", "Longest day of reading", fmtDur(dSec[1]), keyText(dSec[0]));
  if (dPages[1]) put("📖", "Most pages in one day", `${n(dPages[1])} pages`, keyText(dPages[0]));
  if (f.sit.sec) put("🛋️", "Longest sitting", fmtDur(f.sit.sec), `hours in a row, starting ${dayText(f.sit.at)}`);
  const st = streaks(f.days);
  if (st.longest) put("🔥", "Longest streak", `${plural(st.longest, "day")} in a row`, year == null && st.current > 1 ? `You are on ${plural(st.current, "day")} now` : "");
  const wk = top(f.weeks), mo = top(f.months), fm = top(f.finMonths);
  if (wk) put("🗓️", "Biggest week", fmtDur(wk[1]), `week of ${dayText(wk[0])}`);
  if (mo) put("📅", "Biggest month", fmtDur(mo[1]), monthText(mo[0]));
  if (fm && fm[1] > 1) put("🏁", "Most books finished in a month", plural(fm[1], "book"), monthText(fm[0]));
  const paced = [...f.days].filter(([, v]) => v.sec >= 3600 && v.pages).map(([k, v]) => [k, Math.round(v.pages / (v.sec / 3600))]);
  const pace = top(new Map(paced));
  if (pace) put("💨", "Fastest pace", `${n(pace[1])} pages an hour`, `${keyText(pace[0])} (days with an hour or more)`);
  const real = f.fin.filter((x) => (x.b.pages || 0) >= 100);
  const fast = real.slice().sort((a, b) => a.days - b.days || (b.b.pages || 0) - (a.b.pages || 0))[0];
  const slow = real.slice().sort((a, b) => b.days - a.days)[0];
  if (fast) put("⚡", "Fastest finish", plural(fast.days, "day"), `${fast.b.title} · ${n(fast.b.pages)} pages`);
  if (slow && slow !== fast) put("🐢", "Longest haul", plural(slow.days, "day"), `${slow.b.title}, from first page to last`);
  const big = f.fin.slice().sort((a, b) => (b.b.pages || 0) - (a.b.pages || 0))[0];
  if (big?.b.pages) put("🧱", "Longest book finished", `${n(big.b.pages)} pages`, big.b.title);
  const most = f.fin.slice().sort((a, b) => b.sec - a.sec)[0];
  if (most) put("⌛", "Book you spent the most time on", fmtDur(most.sec), most.b.title);
  const night = f.rows.filter((r) => new Date(r.ms).getHours() < 5).sort((a, b) => new Date(b.ms).getHours() - new Date(a.ms).getHours())[0];
  const early = f.rows.filter((r) => { const h = new Date(r.ms).getHours(); return h >= 4 && h < 8; }).sort((a, b) => new Date(a.ms).getHours() - new Date(b.ms).getHours())[0];
  if (night) put("🌙", "Latest night", `still reading after ${hourText(new Date(night.ms).getHours())}`, dayText(night.ms));
  if (early) put("🌅", "Earliest start", `reading by ${hourText(new Date(early.ms).getHours() + 1)}`, dayText(early.ms));
  const hr = top(f.hours), wd = top(f.weekdays);
  if (hr) put("🕰️", "Your reading hour", hourText(hr[0]), `${fmtDur(hr[1])} read in that hour of the day`);
  if (wd) put("📌", "Your reading day", new Date(2024, 0, 7 + wd[0]).toLocaleDateString(undefined, { weekday: "long" }), `${fmtDur(wd[1])} read on that weekday`);
  const au = top(f.authors), ge = top(f.genres), se = top(f.series);
  if (au) put("✍️", "Most-read author", au[0], `${plural(au[1], "book")} finished`);
  if (ge) put("🏷️", "Most-read genre", GENRES[ge[0]] || ge[0], `${plural(ge[1], "book")} finished`);
  if (se && se[1] > 1) put("🗂️", "Most-read series", se[0], `${plural(se[1], "book")} finished`);
  if (year == null) put("🌱", "Reading since", dayText(f.rows[0].ms), "the first day in your statistics");
  return out;
}

// awards: one year's prizes, each with its winner.
export function awards(m, year) {
  const f = facts(m, year);
  const out = [];
  const put = (icon, title, winner, why) => winner && out.push({ icon, title, winner, why });
  const by = (fn) => f.fin.slice().sort(fn)[0];
  const real = f.fin.filter((x) => (x.b.pages || 0) >= 100);
  const book = by((a, b) => b.sec - a.sec);
  put("📕", "Book of the year", book?.b.title, book && `${fmtDur(book.sec)} with it${book.b.author ? ` · ${book.b.author}` : ""}`);
  const fav = f.fin.filter((x) => x.b.rating > 0).sort((a, b) => b.b.rating - a.b.rating || b.sec - a.sec)[0];
  put("⭐", "Your favourite", fav?.b.title, fav && `you gave it ${fav.b.rating} star${fav.b.rating === 1 ? "" : "s"}`);
  const fast = real.slice().sort((a, b) => a.days - b.days)[0];
  put("⚡", "Couldn't put it down", fast?.b.title, fast && `finished in ${plural(fast.days, "day")}`);
  const big = by((a, b) => (b.b.pages || 0) - (a.b.pages || 0));
  put("🧱", "The doorstop", big?.b.pages ? big.b.title : "", big && `${n(big.b.pages)} pages`);
  const au = top(f.authors), ge = top(f.genres), se = top(f.series), mo = top(f.months);
  put("✍️", "Author of the year", au?.[0], au && `${plural(au[1], "book")} finished`);
  put("🏷️", "Genre of the year", ge && (GENRES[ge[0]] || ge[0]), ge && `${plural(ge[1], "book")} finished`);
  put("🗂️", "Series of the year", se && se[1] > 1 ? se[0] : "", se && `${plural(se[1], "book")} finished`);
  put("📅", "Month of the year", mo && monthText(mo[0]), mo && `${fmtDur(mo[1])} of reading`);
  const nights = new Set(f.rows.filter((r) => new Date(r.ms).getHours() < 4).map((r) => dayKey(r.ms))).size;
  const dawns = new Set(f.rows.filter((r) => { const h = new Date(r.ms).getHours(); return h >= 4 && h < 7; }).map((r) => dayKey(r.ms))).size;
  if (nights || dawns) put(nights >= dawns ? "🌙" : "🌅", nights >= dawns ? "Night owl award" : "Early bird award", plural(Math.max(nights, dawns), "day"), nights >= dawns ? "you read past midnight" : "you read before 7");
  return out;
}

// recordsHTML is the whole tab: a year picker, that year's awards, then the records.
export function recordsHTML(m, year) {
  const years = yearsOf(m);
  const pick = `<div class="mb-4 flex flex-wrap gap-1"><button type="button" data-year="" class="${year == null ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">All time</button>
    ${years.map((y) => `<button type="button" data-year="${y}" class="${y === year ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">${y}</button>`).join("")}</div>`;
  const aw = year == null ? [] : awards(m, year);
  const rec = records(m, year);
  const tile = (r) => `<li class="card p-3"><p class="text-2xl" aria-hidden="true">${r.icon}</p><p class="text-xs uppercase tracking-wide text-slate-400">${esc(r.title)}</p>
    <p class="break-words text-xl font-bold">${esc(r.value)}</p><p class="break-words text-xs text-slate-400">${esc(r.note)}</p></li>`;
  return `${pick}
    ${aw.length ? `<section class="mb-6"><h2 class="mb-2 text-lg font-semibold">🏆 Your ${year} awards</h2>
      <ul class="grid grid-cols-2 gap-3 lg:grid-cols-3">${aw.map((a) => `<li class="card award p-3"><p class="text-2xl" aria-hidden="true">${a.icon}</p>
        <p class="text-xs uppercase tracking-wide text-amber-300">${esc(a.title)}</p><p class="break-words text-lg font-bold">${esc(a.winner)}</p>
        <p class="break-words text-xs text-slate-400">${esc(a.why || "")}</p></li>`).join("")}</ul></section>` : ""}
    <section class="mb-5"><h2 class="mb-2 text-lg font-semibold">🏅 ${year == null ? "All-time records" : `${year} records`}</h2>
      ${rec.length ? `<ul class="grid grid-cols-2 gap-3 lg:grid-cols-3">${rec.map(tile).join("")}</ul>` : `<p class="text-sm text-slate-400">No reading recorded ${year == null ? "yet" : `in ${year}`}.</p>`}</section>
    <p class="text-xs text-slate-500">A book counts as finished when KOReader's statistics show 97% or more. Sittings and hours are counted by the hour, as KOReader records them.</p>`;
}
