// The slides of Wrapped, in the big-number, bottom-left story style (played by wrappedstory.js).
import { esc } from "./ui.js";
import { GENRES } from "./genres.js";
import { byDay, byHourOfDay, fmtDur } from "./statscalc.js";
import { bounds, summary, count } from "./statsperiods.js";
import { coverImg } from "./covers.js";

const n = (x) => Number(x).toLocaleString();
const FINISHED = 0.97;

// awards: fun titles earned in a period, from how and what you read.
export function awards(m, sum, period) {
  const out = [];
  const hours = byHourOfDay(sum.rows);
  const total = hours.reduce((a, b) => a + b, 0) || 1;
  const share = (from, to) => hours.slice(from, to).reduce((a, b) => a + b, 0) / total;
  const days = byDay(sum.rows);
  const bestDay = [...days.values()].sort((a, b) => b.sec - a.sec)[0];
  const books = sum.books.map((b) => ({ ...m.books[b.book], sec: b.sec, pages: b.pages }));
  const bySeries = count(books.filter((b) => b.series), (b) => b.series);
  const genres = new Set(books.flatMap((b) => b.genres || []));
  const weekend = sum.rows.filter((r) => [0, 6].includes(new Date(r.ms).getDay())).reduce((t, r) => t + r.sec, 0) / total;
  if (share(22, 24) + share(0, 4) > 0.4) out.push(["🌙", "Night Owl", "Most of your reading happened after 10 pm."]);
  if (share(4, 9) > 0.35) out.push(["🌅", "Early Bird", "You read best before 9 in the morning."]);
  if (bestDay && bestDay.sec >= 4 * 3600) out.push(["🏃", "Marathon Reader", `One day you read for ${fmtDur(bestDay.sec)}.`]);
  if (bySeries[0]?.n >= 3) out.push(["🗂️", "Series Binger", `${bySeries[0].n} books of ${bySeries[0].label}.`]);
  if (genres.size >= 5) out.push(["🧭", "Genre Explorer", `${genres.size} different genres.`]);
  if (sum.pages >= (period === "year" ? 8000 : 1500)) out.push(["📄", "Page Turner", `${n(sum.pages)} pages.`]);
  if (sum.days >= (period === "year" ? 200 : 20)) out.push(["🔥", "Steady Reader", `You read on ${sum.days} different days.`]);
  if (weekend > 0.5) out.push(["🛋️", "Weekend Warrior", "More than half of your reading was on a weekend."]);
  if (sum.finished >= (period === "year" ? 25 : 4)) out.push(["📚", "Finisher", `${sum.finished} books read to the end.`]);
  return out;
}

const slide = (kicker, big, small, extra = "", cls = "s-big") => `${kicker ? `<p class="s-kicker">${kicker}</p>` : ""}<p class="${cls}">${big}</p>${small ? `<p class="s-small">${small}</p>` : ""}${extra}`;
const clock = (h) => new Date(2000, 0, 1, h).toLocaleTimeString(undefined, { hour: "numeric" });

// daySlide: a donut of the 24 hours, coloured by how much you read in each.
function daySlide(hours, peak) {
  const max = Math.max(...hours) || 1;
  const stops = hours.map((v, i) => `hsl(${265 - Math.round((v / max) * 60)} ${30 + Math.round((v / max) * 60)}% ${22 + Math.round((v / max) * 40)}%) ${(i / 24) * 100}% ${((i + 1) / 24) * 100}%`).join(",");
  return `<p class="s-kicker">A day in the life</p><div class="s-donut" data-donut="${stops}" role="img" aria-label="Reading by hour of the day"></div>
    <p class="s-title">Most of your reading: ${clock(peak)}</p><p class="s-small">Midnight is at the top and the clock runs clockwise. Brighter means more reading.</p>`;
}

export function slides(m, s, name) {
  const sum = summary(m, s.period, s.offset);
  const b = bounds(s.period, s.offset);
  const when = s.period === "year" ? `${b.title}` : new Date(b.from).toLocaleDateString(undefined, { month: "long", year: "numeric" });
  const books = sum.books.map((x) => ({ ...m.books[x.book], sec: x.sec, pages: x.pages }));
  if (!sum.sec) return [slide("", `Nothing read in ${esc(when)}`, "Pick another month or year, or sync your KOReader statistics.", "", "s-title")];
  const fin = books.filter((x) => (x.percent || 0) >= FINISHED);
  // Books finished elsewhere (paper, audio, another reader) that Hardcover has a finish day for, in this period
  // and not already counted from KOReader.
  const seen = new Set(fin.flatMap((x) => [x.book_id ? `id:${x.book_id}` : "", `t:${(x.title || "").toLowerCase()}`]));
  const extra = (m.hcRead || []).filter((h) => {
    const t = new Date(`${h.finished}T12:00:00`).getTime();
    return t >= b.from && t < b.to && !seen.has(`id:${h.book_id}`) && !seen.has(`t:${(h.title || "").toLowerCase()}`);
  });
  const finCount = fin.length + extra.length;
  const top = books[0];
  const authors = count(books, (x) => x.author || "");
  const genres = count(books, (x) => x.genres || []);
  const hours = byHourOfDay(sum.rows);
  const peak = hours.indexOf(Math.max(...hours));
  const part = peak >= 5 && peak < 12 ? "morning" : peak >= 12 && peak < 17 ? "afternoon" : peak >= 17 && peak < 22 ? "evening" : "night";
  const best = [...byDay(sum.rows)].sort((a, c) => c[1].sec - a[1].sec)[0];
  const pagesBook = books.slice().sort((a, c) => c.pages - a.pages)[0];
  const earned = awards(m, sum, s.period);
  const whole = sum.books.map((x) => m.books[x.book]).filter(Boolean); // the books themselves (their full page counts)
  const doneBooks = whole.filter((x) => (x.percent || 0) >= FINISHED && x.pages);
  const longest = doneBooks.slice().sort((a, c) => c.pages - a.pages)[0];
  const shortest = doneBooks.slice().sort((a, c) => a.pages - c.pages)[0];
  const rated = whole.filter((x) => x.rating > 0);
  const avgRating = rated.length ? rated.reduce((t, x) => t + x.rating, 0) / rated.length : 0;
  const lastRead = new Map();
  for (const r of sum.rows) lastRead.set(String(r.book), Math.max(lastRead.get(String(r.book)) || 0, r.ms));
  const current = sum.books.map((x) => ({ ...m.books[x.book], last: lastRead.get(String(x.book)) || 0 })).filter((x) => x.title && (x.percent || 0) > 0 && (x.percent || 0) < FINISHED).sort((a, c) => c.last - a.last)[0];
  const out = [
    slide("🎁 Your Wrapped", `${esc(when)}`, `${esc(name)}'s reading, one tap at a time.`),
    slide("You read for", fmtDur(sum.sec), `across ${n(sum.days)} day${sum.days === 1 ? "" : "s"} and ${n(sum.pages)} pages. That is about ${n(Math.round(sum.sec / 60 / Math.max(1, sum.days)))} minutes on a day you read.`),
    slide("Finished", `${n(finCount)} book${finCount === 1 ? "" : "s"}`, `${n(books.length)} opened on your e-reader${extra.length ? `; ${n(extra.length)} more finished according to Hardcover` : ""}.`,
      finCount ? `<div class="s-covers">${[...fin, ...extra].slice(0, 8).map((x) => (x.book_id ? coverImg(x.book_id, "h-28 w-20") : "")).join("")}</div>` : ""),
  ];
  const year = new Date(b.from).getFullYear();
  const goal = s.period === "year" ? (m.goals || []).find((g) => g.goal > 0 && String(g.start_date || "").slice(0, 4) === String(year)) : null;
  if (goal) {
    const unit = goal.metric === "page" ? "pages" : "books";
    out.push(slide(`Your ${year} reading goal`, `${n(Math.round(goal.progress))} of ${n(goal.goal)}`, `${unit} on Hardcover: ${Math.min(100, Math.round((goal.progress / goal.goal) * 100))}% of the way there.`));
  }
  if (top) out.push(slide("Your most-read book", esc(top.title), `${fmtDur(top.sec)}${top.author ? ` · ${esc(top.author)}` : ""}`, top.book_id ? `<div>${coverImg(top.book_id, "h-44 w-28", true)}</div>` : "", "s-title"));
  if (authors[0]?.label) out.push(slide("Your top author", esc(authors[0].label), `${n(authors[0].n)} book${authors[0].n === 1 ? "" : "s"}${genres[0] ? `. Favourite genre: ${esc(GENRES[genres[0].label] || genres[0].label)}` : ""}${authors[1]?.label ? `. Runner-up: ${esc(authors[1].label)}` : ""}.`, "", "s-title"));
  if (rated.length) {
    const star = (lo, hi) => rated.filter((x) => x.rating >= lo && x.rating < hi).length;
    const rows = [["⭐ 5 stars", star(5, 6)], ["4¼ to 4¾", star(4.25, 5)], ["4 stars", star(4, 4.25)], ["3 to 3¾", star(3, 4)], ["Under 3", star(0, 3)]].filter((r) => r[1]);
    out.push(`<p class="s-kicker">Your ratings</p><p class="s-big">${avgRating.toFixed(2)}</p><p class="s-small">average over ${n(rated.length)} rated book${rated.length === 1 ? "" : "s"}</p>
      <ul class="s-list">${rows.map((r) => `<li><span class="em">⭐</span><span><b>${n(r[1])}</b> · ${r[0]}</span></li>`).join("")}</ul>`);
  }
  if (authors.filter((a) => a.label).length > 1) {
    out.push(`<p class="s-kicker">Top authors</p><ul class="s-list">${authors.filter((a) => a.label).slice(0, 4).map((a, i) => `<li><span class="em">${["🥇", "🥈", "🥉", "4️⃣"][i]}</span><span><b>${esc(a.label)}</b><br>${n(a.n)} book${a.n === 1 ? "" : "s"}</span></li>`).join("")}</ul>`);
  }
  out.push(slide("Your reading personality", `${/^[aeiou]/.test(part) ? "An" : "A"} ${part} reader`, `Your busiest hour was ${clock(peak)}.`, "", "s-title"), daySlide(hours, peak));
  if (best) out.push(slide("Your longest day", fmtDur(best[1].sec), `${new Date(best[0] + "T12:00:00").toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" })}, with ${n(best[1].pages)} pages.`));
  if (pagesBook?.pages) out.push(slide("Most pages from one book", n(pagesBook.pages), `${esc(pagesBook.title)}`));
  if (longest && shortest && longest !== shortest) {
    out.push(`<p class="s-kicker">Book extremes</p><ul class="s-list">
      <li><span class="em">🧱</span><span><b>Longest:</b> ${esc(longest.title)}<br>${n(longest.pages)} pages</span></li>
      <li><span class="em">🍬</span><span><b>Shortest:</b> ${esc(shortest.title)}<br>${n(shortest.pages)} pages</span></li></ul>`);
  }
  if (current) out.push(slide("Currently reading", esc(current.title), `${Math.round((current.percent || 0) * 100)}% done${current.author ? ` · ${esc(current.author)}` : ""}`, current.book_id ? `<div>${coverImg(current.book_id, "h-40 w-28", true)}</div>` : "", "s-title"));
  out.push(earned.length
    ? `<p class="s-kicker">Achievements</p><div class="s-grid">${earned.map(([e, t]) => `<div class="s-tile"><b>${e}</b>${esc(t)}</div>`).join("")}</div><ul class="s-list">${earned.slice(0, 3).map(([e, t, d]) => `<li><span class="em">${e}</span><span><b>${esc(t)}</b><br>${esc(d)}</span></li>`).join("")}</ul>`
    : slide("Achievements", "Keep reading", "Awards are for reading a lot, late, early, long or wide. You will earn some.", "", "s-title"));
  out.push(`<p class="s-kicker">${esc(when)}</p><div class="s-grid"><div class="s-tile"><b>${n(finCount)}</b>Books read</div><div class="s-tile"><b>${n(sum.pages)}</b>Pages</div><div class="s-tile"><b>${avgRating ? avgRating.toFixed(2) : fmtDur(sum.sec)}</b>${avgRating ? "Avg. rating" : "Reading time"}</div></div>
    ${authors[0]?.label ? `<p class="s-small">Top author: ${esc(authors[0].label)} (${n(authors[0].n)})${longest ? ` · Longest: ${esc(longest.title)} (${n(longest.pages)} pages)` : ""}</p>` : ""}
    <div class="s-actions"><button type="button" data-share>Share picture</button><button type="button" class="alt" data-sdone>Done</button></div>`);
  return out;
}
