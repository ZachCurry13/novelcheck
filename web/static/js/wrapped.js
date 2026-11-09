// 🎁 Wrapped: your reading as a short story you tap through, for any month or any year, plus your trophies and
// records. Everything is worked out in this browser from your reading statistics (so days and hours are in your
// own time zone), and nothing is sent anywhere. The last slide makes a picture to save or share.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { prepare, byDay, byHourOfDay, fmtDur } from "./statscalc.js";
import { bounds, summary, count } from "./statsperiods.js";
import { tabStrip } from "./tabs.js";
import { coverImg } from "./covers.js";
import { openShareCard } from "./statscard.js";
import { trophiesHTML, paintBars } from "./trophies.js";

const n = (x) => Number(x).toLocaleString();
const FINISHED = 0.97;

// awards: fun titles earned in a period, from how and what you read.
function awards(m, sum, period) {
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

const slide = (emoji, big, small, extra = "") => `<div class="flex min-h-[26rem] flex-col items-center justify-center gap-3 px-4 text-center">
  <p class="text-6xl" aria-hidden="true">${emoji}</p><p class="text-3xl font-extrabold leading-tight sm:text-4xl">${big}</p>
  ${small ? `<p class="max-w-md text-slate-300">${small}</p>` : ""}${extra}</div>`;

function slides(m, s, name) {
  const sum = summary(m, s.period, s.offset);
  const b = bounds(s.period, s.offset);
  const when = s.period === "year" ? `${b.title}` : new Date(b.from).toLocaleDateString(undefined, { month: "long", year: "numeric" });
  const books = sum.books.map((x) => ({ ...m.books[x.book], sec: x.sec, pages: x.pages }));
  if (!sum.sec) return [slide("📭", `Nothing read in ${esc(when)}`, "Pick another month or year, or sync your KOReader statistics.")];
  const fin = books.filter((x) => (x.percent || 0) >= FINISHED);
  const top = books[0];
  const authors = count(books, (x) => x.author || "");
  const genres = count(books, (x) => x.genres || []);
  const hours = byHourOfDay(sum.rows);
  const peak = hours.indexOf(Math.max(...hours));
  const part = peak >= 5 && peak < 12 ? "morning" : peak >= 12 && peak < 17 ? "afternoon" : peak >= 17 && peak < 22 ? "evening" : "night";
  const days = byDay(sum.rows);
  const best = [...days].sort((a, c) => c[1].sec - a[1].sec)[0];
  const pagesBook = books.slice().sort((a, c) => c.pages - a.pages)[0];
  const earned = awards(m, sum, s.period);
  const out = [
    slide("🎁", `${esc(name)}'s ${esc(when)} in books`, "Tap or press → to go on."),
    slide("⏱️", `${fmtDur(sum.sec)}`, `of reading across ${n(sum.days)} day${sum.days === 1 ? "" : "s"}, ${n(sum.pages)} pages.`),
    slide("📚", `${n(fin.length)} book${fin.length === 1 ? "" : "s"} finished`, `and ${n(books.length)} opened in all${authors[0]?.label ? `, by ${n(authors.filter((a) => a.label).length)} author${authors.length === 1 ? "" : "s"}` : ""}.`,
      fin.length ? `<ul class="mt-2 flex max-w-lg flex-wrap justify-center gap-2">${fin.slice(0, 8).map((x) => `<li>${x.book_id ? coverImg(x.book_id, "h-28 w-20") : ""}</li>`).join("")}</ul>` : ""),
  ];
  if (top) out.push(slide("🥇", esc(top.title), `your most-read book: ${fmtDur(top.sec)}${top.author ? ` · ${esc(top.author)}` : ""}.`, top.book_id ? `<div>${coverImg(top.book_id, "h-44 w-28", true)}</div>` : ""));
  if (authors[0]?.label) out.push(slide("✍️", esc(authors[0].label), `your top author: ${n(authors[0].n)} book${authors[0].n === 1 ? "" : "s"}${genres[0] ? `. Your favourite genre was ${esc(genres[0].label)}` : ""}.`));
  out.push(slide(peak < 5 || peak >= 22 ? "🌙" : "☀️", `You're ${part === "afternoon" ? "an" : "a"} ${part} reader`, `Your busiest hour was ${new Date(2000, 0, 1, peak).toLocaleTimeString(undefined, { hour: "numeric" })}.`));
  if (best) out.push(slide("📆", fmtDur(best[1].sec), `your longest day, ${new Date(best[0] + "T12:00:00").toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" })}, with ${n(best[1].pages)} pages.`));
  if (pagesBook?.pages) out.push(slide("📖", esc(pagesBook.title), `the book you read the most pages of: ${n(pagesBook.pages)}.`));
  out.push(slide("🏅", earned.length ? "Your awards" : "Keep reading", earned.length ? "" : "Awards are for reading a lot, late, early, long or wide. You'll earn some.",
    earned.length ? `<ul class="w-full max-w-md space-y-2 text-left">${earned.map(([e, t, d]) => `<li class="card flex items-center gap-3 p-3"><span class="text-3xl">${e}</span><span><b>${esc(t)}</b><span class="block text-sm text-slate-400">${esc(d)}</span></span></li>`).join("")}</ul>` : ""));
  out.push(slide("📤", "Share it", "Make a picture of this to save or send.", `<button type="button" data-share class="btn-primary">Make my card</button>`));
  return out;
}

export async function renderWrapped(view, state) {
  view.innerHTML = `<h1 class="mb-2 text-2xl font-bold">🎁 Wrapped &amp; trophies</h1><p class="text-slate-400">Loading…</p>`;
  const data = await get("/api/me/reading-stats").catch(() => null);
  if (!data) {
    view.innerHTML = `<h1 class="mb-2 text-2xl font-bold">🎁 Wrapped &amp; trophies</h1><p class="text-slate-400">Couldn't load your reading just now.</p>`;
    return;
  }
  const m = prepare(data);
  const s = { tab: new URLSearchParams(location.hash.split("?")[1] || "").get("tab") === "trophies" ? "trophies" : "wrapped", period: "month", offset: -1, at: 0 };
  if (!m.rows.length && !m.books.length) {
    view.innerHTML = `<h1 class="mb-2 text-2xl font-bold">🎁 Wrapped &amp; trophies</h1><div class="card space-y-2 text-sm text-slate-300"><p>Nothing to look back on yet. Wrapped and trophies come from KOReader's reading statistics.</p>
      <a href="#/settings?s=kindle" class="btn-secondary inline-flex">Set up statistics sync</a></div>`;
    return;
  }
  if (!summary(m, "month", -1).sec) s.offset = 0; // no reading last month: start with this one
  const draw = () => {
    const strip = tabStrip({ id: "wrap-tabs", label: "Wrapped", active: s.tab, tabs: [["wrapped", "🎁 Wrapped"], ["trophies", "🏆 Trophies"]] });
    if (s.tab === "trophies") {
      view.innerHTML = `${strip}<h1 class="mb-2 text-2xl font-bold">🏆 Trophies &amp; records</h1>${trophiesHTML(m)}`;
      paintBars(view);
      return;
    }
    const list = slides(m, s, state.user.username);
    s.at = Math.max(0, Math.min(s.at, list.length - 1));
    const b = bounds(s.period, s.offset);
    view.innerHTML = `${strip}<div class="mb-3 flex flex-wrap items-center gap-2">
      <button type="button" data-period="month" class="${s.period === "month" ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">By month</button>
      <button type="button" data-period="year" class="${s.period === "year" ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">By year</button>
      <span class="ml-2 flex items-center gap-1"><button type="button" data-off="-1" class="btn-ghost px-3 py-1" aria-label="Earlier">‹</button>
        <span class="min-w-[7rem] text-center text-sm text-slate-300">${esc(s.period === "year" ? b.title : new Date(b.from).toLocaleDateString(undefined, { month: "long", year: "numeric" }))}</span>
        <button type="button" data-off="1" class="btn-ghost px-3 py-1" aria-label="Later" ${s.offset >= 0 ? "disabled" : ""}>›</button></span></div>
      <div id="story" class="card relative select-none overflow-hidden" tabindex="0" aria-live="polite">${list[s.at]}</div>
      <div class="mt-3 flex items-center justify-between gap-3"><button type="button" data-go="-1" class="btn-secondary px-4" ${s.at === 0 ? "disabled" : ""}>‹ Back</button>
        <span class="flex gap-1" aria-hidden="true">${list.map((_, i) => `<span class="h-1.5 w-4 rounded-full ${i === s.at ? "bg-indigo-400" : "bg-slate-700"}"></span>`).join("")}</span>
        <button type="button" data-go="1" class="btn-primary px-4" ${s.at === list.length - 1 ? "disabled" : ""}>Next ›</button></div>`;
    view.querySelector("#story")?.focus({ preventScroll: true });
  };
  view.onclick = (e) => {
    const t = e.target.closest("[data-tab]")?.dataset.tab;
    if (t) {
      s.tab = t;
      return draw();
    }
    const p = e.target.closest("[data-period]")?.dataset.period;
    if (p) {
      Object.assign(s, { period: p, offset: p === "year" && summary(m, "year", -1).sec ? -1 : p === "month" && summary(m, "month", -1).sec ? -1 : 0, at: 0 });
      return draw();
    }
    const off = e.target.closest("[data-off]")?.dataset.off;
    if (off) {
      s.offset = Math.min(0, s.offset + Number(off));
      s.at = 0;
      return draw();
    }
    const go = e.target.closest("[data-go]")?.dataset.go;
    if (go) {
      s.at += Number(go);
      return draw();
    }
    if (e.target.closest("[data-share]")) return openShareCard(m, { period: s.period, offset: s.offset }, bounds(s.period, s.offset).title, state.user.username);
  };
  view.onkeydown = (e) => {
    if (s.tab !== "wrapped" || !["ArrowRight", "ArrowLeft"].includes(e.key)) return;
    s.at += e.key === "ArrowRight" ? 1 : -1;
    draw();
  };
  draw();
}

