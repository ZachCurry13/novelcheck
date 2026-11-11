// Books finished somewhere other than KOReader (a paper book, an audiobook, another app): Hardcover's
// finished books and the "read" shelf of an imported list (Goodreads, StoryGraph…), each with the day it
// was finished. They join the reading model as finished books with no reading time, so the year's count,
// records, awards and trophies include them; hours, pages a day and streaks still come from KOReader alone.
const FINISHED = 0.97;
const key = (t) => (t || "").toLowerCase().replace(/[^a-z0-9]+/g, "");

// addElsewhere adds them to m.books and lists them in m.done as [{book, ms}] (noon on the day, in this
// device's time zone). A book KOReader already shows as finished is not counted twice.
export function addElsewhere(m, ...lists) {
  const have = new Set();
  for (const b of m.books) {
    if ((b.percent || 0) < FINISHED) continue;
    if (b.book_id) have.add(`id:${b.book_id}`);
    have.add(`t:${key(b.title)}`);
  }
  m.done = [];
  for (const h of lists.flat()) {
    const ms = new Date(`${h.finished}T12:00:00`).getTime();
    if (!h.finished || Number.isNaN(ms)) continue;
    const byId = h.book_id ? `id:${h.book_id}` : "";
    const byTitle = `t:${key(h.title)}`;
    if ((byId && have.has(byId)) || have.has(byTitle)) continue;
    if (byId) have.add(byId);
    have.add(byTitle);
    m.books.push({ title: h.title, author: h.author, book_id: h.book_id, pages: h.pages || 0, percent: 1, rating: h.rating || 0,
      read_seconds: 0, format: h.format, reads: h.reads || 1, kind: h.kind, genres: h.genres, series: h.series, elsewhere: true });
    m.done.push({ book: m.books.length - 1, ms });
  }
  return m;
}
