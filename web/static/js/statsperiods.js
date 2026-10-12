// Reading stats: periods (this week, a month, a year, all time), series for
// the charts, and everything about one book. Pure maths on the prepared data
// (see statscalc.js); days and hours are in this device's time zone.
import { byDay, dayKey, startOfDay, within } from "./statscalc.js";

const shortDate = (ms) => new Date(ms).toLocaleDateString(undefined, { month: "short", day: "numeric" });

export const weekStart = (ms) => {
  const d = new Date(ms);
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7)); // Monday
  return d.getTime();
};
export const monthStart = (ms) => {
  const d = new Date(ms);
  return new Date(d.getFullYear(), d.getMonth(), 1).getTime();
};

// bounds: [from, to) of a period, with a short title for headings.
export function bounds(period, offset = 0, now = Date.now()) {
  const d = new Date(now);
  if (period === "week") {
    const from = new Date(weekStart(now));
    from.setDate(from.getDate() + 7 * offset);
    const to = new Date(from);
    to.setDate(to.getDate() + 7);
    const last = new Date(to);
    last.setDate(last.getDate() - 1);
    return { from: from.getTime(), to: to.getTime(), title: offset === 0 ? "this week" : `${shortDate(from)} – ${shortDate(last)}`, unit: "week" };
  }
  if (period === "month") {
    const from = new Date(d.getFullYear(), d.getMonth() + offset, 1);
    const to = new Date(d.getFullYear(), d.getMonth() + offset + 1, 1);
    return { from: from.getTime(), to: to.getTime(), title: `${from.toLocaleDateString(undefined, { month: "short" })} '${String(from.getFullYear()).slice(2)}`, unit: "month" };
  }
  if (period === "year") {
    const y = d.getFullYear() + offset;
    return { from: new Date(y, 0, 1).getTime(), to: new Date(y + 1, 0, 1).getTime(), title: String(y), unit: "year" };
  }
  return { from: 0, to: Infinity, title: "all time", unit: "all" };
}

// summary: the numbers for a period.
export function summary(m, period, offset) {
  const b = bounds(period, offset);
  const w = within(m, b.from, b.to);
  const authors = new Set();
  for (const x of w.books) {
    const a = m.books[x.book]?.author;
    if (a) authors.add(a);
  }
  return { ...w, b, authors: authors.size, authorList: authors };
}

// series: the chart points: a day each for a week or month, a month each for a year or all time.
export function series(m, period, offset) {
  const b = bounds(period, offset);
  if (period === "week" || period === "month") {
    const days = byDay(m.rows);
    const out = [];
    for (let t = b.from; t < b.to;) {
      const e = days.get(dayKey(t));
      out.push({ ms: t, label: new Date(t).toLocaleDateString(undefined, period === "week" ? { weekday: "short" } : { day: "numeric" }), sec: e?.sec || 0, pages: e?.pages || 0 });
      const n = new Date(t);
      n.setDate(n.getDate() + 1);
      t = n.getTime();
    }
    return out;
  }
  const first = m.rows.length ? Math.min(...m.rows.map((r) => r.ms)) : Date.now();
  const from = period === "year" ? b.from : monthStart(first);
  const to = period === "year" ? b.to : monthStart(Date.now()) + 32 * 864e5;
  const months = new Map();
  for (const r of m.rows) {
    const k = monthStart(r.ms);
    const e = months.get(k) || { sec: 0, pages: 0 };
    e.sec += r.sec;
    e.pages += r.pages;
    months.set(k, e);
  }
  const out = [];
  for (let t = from; t < Math.min(to, period === "year" ? b.to : monthStart(Date.now()) + 1);) {
    const d = new Date(t);
    const e = months.get(t);
    out.push({ ms: t, label: d.toLocaleDateString(undefined, { month: "short" }) + (period === "all" ? ` '${String(d.getFullYear()).slice(2)}` : ""), sec: e?.sec || 0, pages: e?.pages || 0 });
    t = new Date(d.getFullYear(), d.getMonth() + 1, 1).getTime();
  }
  return out;
}

// listPeriods: weeks or months, newest first, with their reading (for the Books tab).
export function listPeriods(m, kind) {
  const map = new Map();
  for (const r of m.rows) {
    const start = kind === "week" ? weekStart(r.ms) : monthStart(r.ms);
    const e = map.get(start) || { start, sec: 0, pages: 0, books: new Set() };
    e.sec += r.sec;
    e.pages += r.pages;
    e.books.add(r.book);
    map.set(start, e);
  }
  return [...map.values()].sort((a, b) => b.start - a.start);
}

export function periodLabel(kind, start) {
  const d = new Date(start);
  if (kind === "month") return d.toLocaleDateString(undefined, { month: "long", year: "numeric" });
  const end = new Date(d);
  end.setDate(end.getDate() + 6);
  return `${shortDate(d)} – ${shortDate(end)}, ${end.getFullYear()}`;
}

export function booksIn(m, kind, start) {
  const end = new Date(start);
  if (kind === "month") end.setMonth(end.getMonth() + 1);
  else end.setDate(end.getDate() + 7);
  return within(m, start, end.getTime()).books;
}

const daysAgo = (ms) => Math.round((startOfDay(Date.now()) - startOfDay(ms)) / 864e5);
export const agoText = (ms) => {
  const n = daysAgo(ms);
  return n <= 0 ? "today" : n === 1 ? "yesterday" : `${n} days ago`;
};

// bookDetail: everything about one book, like KOReader's book page.
export function bookDetail(m, i) {
  const rows = m.rows.filter((r) => r.book === i);
  const days = byDay(rows);
  const sec = rows.reduce((a, r) => a + r.sec, 0);
  const pages = rows.reduce((a, r) => a + r.pages, 0);
  return {
    book: m.books[i],
    sec,
    pages,
    days: days.size,
    avgDay: days.size ? sec / days.size : 0,
    secPerPage: pages ? sec / pages : 0,
    first: rows.length ? Math.min(...rows.map((r) => r.ms)) : 0,
    last: rows.length ? Math.max(...rows.map((r) => r.ms)) : 0,
    perDay: [...days].sort((a, b) => (a[0] < b[0] ? 1 : -1)).map(([key, e]) => ({ key, sec: e.sec, pages: e.pages })),
  };
}

// count: tally labels, biggest first.
export function count(items, labelOf) {
  const t = new Map();
  for (const it of items) for (const l of [].concat(labelOf(it) || [])) t.set(l, (t.get(l) || 0) + 1);
  return [...t].map(([label, n]) => ({ label, n })).sort((a, b) => b.n - a.n || a.label.localeCompare(b.label));
}

const BUCKETS = [[1, 100], [101, 200], [201, 300], [301, 400], [401, 500], [501, 600], [601, 1000], [1001, Infinity]];
export function pageBuckets(books) {
  return BUCKETS.map(([lo, hi]) => ({ label: hi === Infinity ? "1000+" : `${lo}-${hi}`, n: books.filter((b) => b.pages >= lo && b.pages <= hi).length }));
}
