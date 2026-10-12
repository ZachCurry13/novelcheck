// Reading stats maths. Every day and hour is worked out in this device's own
// time zone: the server only keeps the reading by the hour (UTC).
const pad = (n) => String(n).padStart(2, "0");
export const dayKey = (ms) => {
  const d = new Date(ms);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
export const keyDate = (k) => {
  const [y, m, d] = k.split("-").map(Number);
  return new Date(y, m - 1, d);
};
export const startOfDay = (ms) => {
  const d = new Date(ms);
  d.setHours(0, 0, 0, 0);
  return d.getTime();
};

// prepare turns the server's [hour, book, seconds, pages] rows into objects.
export function prepare(data) {
  const rows = data.hours.map(([h, b, s, p]) => ({ ms: h * 1000, book: b, sec: s, pages: p }));
  return { books: data.books, rows };
}

export function byDay(rows) {
  const days = new Map();
  for (const r of rows) {
    const k = dayKey(r.ms);
    const e = days.get(k) || { sec: 0, pages: 0, books: new Set() };
    e.sec += r.sec;
    e.pages += r.pages;
    e.books.add(r.book);
    days.set(k, e);
  }
  return days;
}

const READING_DAY = 60; // seconds that make a day count for a streak

export function streaks(days) {
  const keys = [...days.keys()].filter((k) => days.get(k).sec >= READING_DAY).sort();
  let longest = 0, run = 0, prev = null;
  for (const k of keys) {
    const d = keyDate(k);
    run = prev && Math.round((d - prev) / 864e5) === 1 ? run + 1 : 1;
    prev = d;
    longest = Math.max(longest, run);
  }
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  if (!(days.get(dayKey(d.getTime()))?.sec >= READING_DAY)) d.setDate(d.getDate() - 1); // today may not be read yet
  let current = 0;
  while (days.get(dayKey(d.getTime()))?.sec >= READING_DAY) {
    current++;
    d.setDate(d.getDate() - 1);
  }
  return { current, longest };
}

// within sums rows from `from` up to (not including) `to` (ms).
export function within(m, from, to = Infinity) {
  const rows = m.rows.filter((r) => r.ms >= from && r.ms < to);
  const perBook = new Map();
  let sec = 0, pages = 0;
  for (const r of rows) {
    sec += r.sec;
    pages += r.pages;
    const b = perBook.get(r.book) || { book: r.book, sec: 0, pages: 0, last: 0 };
    b.sec += r.sec;
    b.pages += r.pages;
    b.last = Math.max(b.last, r.ms);
    perBook.set(r.book, b);
  }
  const books = [...perBook.values()].sort((a, b) => b.sec - a.sec);
  const finished = books.filter((b) => (m.books[b.book]?.percent || 0) >= 0.97).length;
  return { sec, pages, books, finished, days: byDay(rows).size, rows };
}

export function byHourOfDay(rows) {
  const out = new Array(24).fill(0);
  for (const r of rows) out[new Date(r.ms).getHours()] += r.sec;
  return out;
}

export function byWeekday(rows) {
  const out = new Array(7).fill(0);
  for (const r of rows) out[new Date(r.ms).getDay()] += r.sec;
  return out;
}

// timeline is one day's seconds per hour, and the books read that day.
export function timeline(m, key) {
  const hours = new Array(24).fill(0);
  const perBook = new Map();
  for (const r of m.rows) {
    if (dayKey(r.ms) !== key) continue;
    hours[new Date(r.ms).getHours()] += r.sec;
    perBook.set(r.book, (perBook.get(r.book) || 0) + r.sec);
  }
  return { hours, books: [...perBook].map(([book, sec]) => ({ book, sec })).sort((a, b) => b.sec - a.sec) };
}

export function fmtDur(sec) {
  if (sec > 0 && sec < 60) return `${Math.round(sec)} sec`;
  const m = Math.round(sec / 60);
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  return m % 60 ? `${h} h ${pad(m % 60)} min` : `${h} h`;
}
