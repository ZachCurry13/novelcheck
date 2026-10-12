// The Reading stats report: one card per question, each with a plain sentence
// as its headline. What you read comes from the book details NovelCheck knows
// (fiction or not, genres, series, peppers), so books not in the library just
// leave those cards out.
import { esc } from "./ui.js";
import { PEPPERS } from "./peppers.js";
import { byDay, byHourOfDay, byWeekday, fmtDur, streaks } from "./statscalc.js";
import { count, pageBuckets, series, summary } from "./statsperiods.js";
import { areaChart, barRows, bigNum, card, columns, coverTile, donut } from "./statsparts.js";

const n = (x) => Number(x).toLocaleString();
const plural = (k, one, many = `${one}s`) => `${n(k)} ${k === 1 ? one : many}`;
const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const hourText = (h) => `${h % 12 || 12} ${h < 12 ? "am" : "pm"}`;
const partOfDay = (h) => (h >= 5 && h < 12 ? "morning" : h < 17 && h >= 12 ? "afternoon" : h >= 17 && h < 22 ? "evening" : "night");

export function reportHTML(m, s) {
  const sum = summary(m, s.period, s.offset);
  if (!sum.books.length) {
    return `<div class="card text-center text-slate-300"><p class="text-lg font-semibold">No reading in this period yet.</p>
      <p class="mt-1 text-sm text-slate-400">Pick another period, or read a little and sync your Kindle.</p></div>`;
  }
  const books = sum.books.map((x) => ({ ...m.books[x.book], idx: x.book, sec: x.sec, pagesRead: x.pages }));
  const all = [bigNum("📖", n(books.length), "Books read"), bigNum("📄", n(sum.pages), "Pages turned"), bigNum("⏱️", fmtDur(sum.sec), "Reading time"),
    bigNum("✍️", n(sum.authors), "Authors"), bigNum("📅", n(sum.days), plural(sum.days, "day", "days") + " reading"), bigNum("🏁", n(sum.finished), "Finished")];
  const parts = [`<div class="card grid grid-cols-2 gap-4 sm:grid-cols-3">${all.join("")}</div>`];

  parts.push(card("Books you read", `You read ${plural(books.length, "book")}${sum.finished ? `, and finished ${sum.finished}` : ""}`,
    `<div class="flex gap-2 overflow-x-auto pb-1">${books.slice(0, 40).map((b) => coverTile(b, b.idx)).join("")}</div>`));

  const pts = series(m, s.period, s.offset);
  const pageSeries = pts.map((p) => ({ label: p.label, value: p.pages }));
  const busiest = pts.reduce((a, p) => (p.pages > a.pages ? p : a), pts[0] || { pages: 0 });
  parts.push(card("Pages", `You turned ${plural(sum.pages, "page")}`, `${areaChart(pageSeries.map((p, i) => ({ ...p, label: i === 0 || i === pts.length - 1 ? new Date(pts[i].ms).toLocaleDateString(undefined, { month: "short", day: "numeric", year: s.period === "year" || s.period === "all" ? "numeric" : undefined }) : p.label })), n)}
    ${busiest.pages ? `<p class="text-sm text-slate-400">Busiest: ${esc(new Date(busiest.ms).toLocaleDateString(undefined, s.period === "year" || s.period === "all" ? { month: "long", year: "numeric" } : { weekday: "long", month: "short", day: "numeric" }))}, ${plural(busiest.pages, "page")}.</p>` : ""}`));

  if (s.period === "year" || s.period === "all") {
    parts.push(card("Reading time by month", fmtDur(sum.sec) + (s.period === "year" ? " this year" : " in all"),
      columns(pts.map((p) => ({ label: p.label, value: p.sec })), fmtDur, "fill-emerald-500")));
  } else if (s.period === "month") {
    parts.push(card("Reading time by day", `${fmtDur(sum.sec)} over ${plural(sum.days, "day")}`, columns(pts.map((p) => ({ label: p.label, value: p.sec })), fmtDur, "fill-emerald-500")));
  } else {
    parts.push(card("Reading time by day", `${fmtDur(sum.sec)} over ${plural(sum.days, "day")}`, barRows(pts.map((p) => ({
      label: new Date(p.ms).toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric" }), value: p.sec, text: p.sec ? fmtDur(p.sec) : "-", attrs: p.sec ? `data-day="${new Date(p.ms).getTime()}"` : "",
    })), { colour: false })));
  }

  parts.push(pace(m, sum, s));

  const kinds = count(books, (b) => b.kind || "unknown");
  const known = kinds.filter((k) => k.label !== "unknown");
  if (known.length) {
    const fic = kinds.find((k) => k.label === "fiction")?.n || 0, non = kinds.find((k) => k.label === "nonfiction")?.n || 0;
    const head = fic && non ? (fic >= non * 2 ? "You mostly read fiction." : non >= fic * 2 ? "You mostly read non-fiction." : "A good mix of fiction and non-fiction.") : fic ? "You read fiction." : "You read non-fiction.";
    parts.push(card("Fiction and non-fiction", head, donut([{ label: "Fiction", value: fic }, { label: "Non-fiction", value: non }, { label: "Not sure", value: books.length - fic - non }].filter((p) => p.value > 0))));
  }

  const genres = count(books, (b) => b.genres || []);
  if (genres.length) {
    const missing = books.filter((b) => !(b.genres || []).length).length;
    parts.push(card("Genres", `${esc(cap(genres[0].label))} is your top genre`, barRows(genres.slice(0, 8).map((g) => ({ label: cap(g.label), value: g.n, text: `${g.n} (${Math.round((g.n / books.length) * 100)}%)` })))
      + (missing ? `<p class="text-xs text-slate-500">Missing genres for ${plural(missing, "book")}.</p>` : "")));
  }

  const peppers = count(books.filter((b) => b.spice !== undefined && b.spice !== null), (b) => b.spice);
  if (peppers.length) {
    const top = peppers[0];
    parts.push(card("Content", `Most of what you read is level ${top.label}: ${esc(PEPPERS[top.label]?.name || "")}`,
      barRows(peppers.sort((a, b) => a.label - b.label).map((p) => ({ label: `Level ${p.label} · ${PEPPERS[p.label]?.name || ""}`, value: p.n, text: plural(p.n, "book") })), { colour: false })));
  }

  const buckets = pageBuckets(books);
  if (buckets.some((b) => b.n)) {
    const top = buckets.reduce((a, b) => (b.n > a.n ? b : a));
    const withPages = books.filter((b) => b.pages > 0);
    parts.push(card("Book length", `Most of your books are ${top.label} pages`, columns(buckets.map((b) => ({ label: b.label, value: b.n })), (v) => plural(v, "book"))
      + `<p class="text-sm text-slate-400">Average length: ${n(Math.round(withPages.reduce((a, b) => a + b.pages, 0) / withPages.length))} pages.</p>`));
  }

  parts.push(authorsCard(books), seriesCard(books));
  parts.push(habits(sum.rows));

  parts.push(card("Most read", `Your top books by time`, barRows(books.slice(0, 8).map((b) => ({
    label: b.title, value: b.sec, text: `${fmtDur(b.sec)} · ${plural(b.pagesRead, "page")}`, attrs: `data-book="${b.idx}"`,
  })))));
  return parts.filter(Boolean).join("");
}

const cap = (s) => s.replace(/\b\w/g, (c) => c.toUpperCase());

function pace(m, sum, s) {
  const avg = sum.days ? sum.sec / sum.days : 0;
  const pph = sum.sec ? (sum.pages / sum.sec) * 3600 : 0;
  const st = streaks(byDay(m.rows));
  const lines = [`<li>⏳ About <b>${fmtDur(avg)}</b> on a day you read.</li>`, pph ? `<li>📖 Around <b>${n(Math.round(pph))} pages an hour</b>.</li>` : "",
    `<li>🔥 Reading streak: <b>${plural(st.current, "day")}</b> (best ${plural(st.longest, "day")}).</li>`];
  if (s.period === "year" && s.offset === 0 && sum.finished) {
    const now = new Date();
    const dayOfYear = Math.ceil((now - new Date(now.getFullYear(), 0, 1)) / 864e5);
    lines.push(`<li>🔮 At this pace you'll finish about <b>${plural(Math.round((sum.finished / dayOfYear) * 365), "book")}</b> this year.</li>`);
  }
  return card("Reading pace", `You read ${fmtDur(avg)} a day when you read`, `<ul class="space-y-1 text-sm">${lines.join("")}</ul>`);
}

function authorsCard(books) {
  const by = new Map();
  for (const b of books) {
    if (!b.author) continue;
    const e = by.get(b.author) || { name: b.author, books: [], sec: 0 };
    e.books.push(b);
    e.sec += b.sec;
    by.set(b.author, e);
  }
  const top = [...by.values()].sort((a, b) => b.books.length - a.books.length || b.sec - a.sec).slice(0, 3);
  if (!top.length) return "";
  return card("Most read authors", `Your most read author is ${esc(top[0].name)}, with ${plural(top[0].books.length, "book")}`,
    top.map((a) => `<div class="rounded-lg bg-slate-800/40 p-3"><p class="font-semibold">${esc(a.name)}</p><p class="mb-2 text-xs text-slate-400">${plural(a.books.length, "book")} read · ${fmtDur(a.sec)}</p>
      <div class="flex gap-2 overflow-x-auto">${a.books.slice(0, 6).map((b) => coverTile(b, b.idx, "h-20 w-14")).join("")}</div></div>`).join(""));
}

function seriesCard(books) {
  const by = new Map();
  for (const b of books) {
    if (!b.series || !b.series_total) continue;
    const e = by.get(b.series) || { name: b.series, read: 0, total: b.series_total };
    e.read++;
    by.set(b.series, e);
  }
  const top = [...by.values()].sort((a, b) => b.read / b.total - a.read / a.total || b.read - a.read).slice(0, 4);
  if (!top.length) return "";
  return card("Series", `${esc(top[0].name)} is your top series`, barRows(top.map((x) => ({ label: x.name, value: x.read / x.total, text: `${x.read} of ${x.total} books` }))));
}

function habits(rows) {
  const hours = byHourOfDay(rows), days = byWeekday(rows);
  const peak = hours.indexOf(Math.max(...hours)), best = days.indexOf(Math.max(...days));
  const hourPts = hours.map((v, h) => ({ label: h % 6 === 0 ? hourText(h) : "", value: v }));
  return card("When you read", `You read most in the ${partOfDay(peak)}, around ${hourText(peak)}`,
    `${columns(hourPts, fmtDur)}<p class="text-sm text-slate-400">${DAYS[best]}s are your best day: ${fmtDur(days[best])} in total.</p>`);
}
