// 📊 Reading stats: a report on your reading, in plain sentences. Pick a week,
// month, year or all time; see what you read, how much, when and what kind;
// then dig into any book, week, month or day. Everything is worked out on this
// device in its own time zone.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { ago, setBars } from "./progress.js";
import { dayKey, keyDate, prepare } from "./statscalc.js";
import { bounds } from "./statsperiods.js";
import { reportHTML } from "./statsreport.js";
import { bookPage, booksTab, dayPage, periodPage } from "./statsbooks.js";
import { calendarTab } from "./statscalendar.js";
import { tabStrip } from "./tabs.js";
import { openShareCard } from "./statscard.js";

const TABS = { report: "Report", books: "Books", calendar: "Calendar" };
const PERIODS = { week: "Week", month: "Month", year: "Year", all: "All time" };

// renderStats shows your own reading, or (source.url) another reader's who
// shares theirs: the same report, limited to what you may see.
export async function renderStats(view, state, source = {}) {
  const other = Boolean(source.url);
  const back = () => (source.back ? `<a href="${esc(source.back)}" class="btn-ghost mb-2 inline-flex px-3 py-1">‹ Readers</a>` : "");
  const heading = () => `<h1 class="mb-2 text-2xl font-bold">📊 Reading stats</h1>`;
  view.innerHTML = `${heading()}<p class="text-slate-400">Loading…</p>`;
  const data = await get(source.url || "/api/me/reading-stats").catch(() => null);
  if (!data) {
    view.innerHTML = `${back()}${heading()}<p class="text-slate-400">${other ? "That reading profile isn't shared with you." : "Couldn't load your stats just now."}</p>`;
    return;
  }
  const name = other ? data.name || "Reader" : state.user.username;
  const m = prepare(data);
  if (!m.rows.length && other) {
    view.innerHTML = `${back()}<h1 class="mb-2 text-2xl font-bold">📊 ${esc(name)}'s reading</h1><div class="card text-sm text-slate-300">Nothing to show yet.</div>`;
    return;
  }
  if (!m.rows.length) {
    const have = data.synced_at && m.books.length
      ? `<p>NovelCheck has your statistics file (${m.books.length} book${m.books.length === 1 ? "" : "s"}, synced ${esc(ago(data.synced_at))}) but it holds no page-by-page reading yet. Read a few pages in KOReader, then press <b>Reading statistics → Synchronize now</b> again.</p>` : "";
    view.innerHTML = `<h1 class="mb-2 text-2xl font-bold">📊 Reading stats</h1>
      <div class="card space-y-2 text-sm text-slate-300"><p>No reading recorded yet.</p>${have}
      <p>In KOReader, run <b>NovelCheck → Set up everything</b>, then <b>NovelCheck → Sync reading statistics now</b>. (Settings → Kindle &amp; KOReader has the steps.)</p>
      <a href="#/settings?s=kindle" class="btn-secondary inline-flex">Open Kindle settings</a></div>`;
    return;
  }
  const now = new Date();
  const s = { tab: "report", period: "month", offset: 0, kind: "week", month: new Date(now.getFullYear(), now.getMonth(), 1), page: null, stack: [], day: dayKey(Date.now()) };

  const title = () => {
    const b = bounds(s.period, s.offset);
    const who = `${name}'s`;
    return s.period === "all" ? `${who} reading, all time` : s.period === "week" && s.offset === 0 ? `${who} week in books` : `${who} ${b.title} in books`;
  };
  const picker = () => `<div class="mb-4 space-y-2"><div class="flex flex-wrap gap-1">${Object.entries(PERIODS).map(([k, v]) =>
      `<button type="button" data-period="${k}" class="${k === s.period ? "btn-primary" : "btn-secondary"} whitespace-nowrap px-2.5 py-1 text-sm">${v}</button>`).join("")}</div>
    ${s.period === "all" ? "" : `<div class="flex items-center gap-2"><button type="button" data-offset="-1" class="btn-ghost px-3 py-1" aria-label="Earlier">‹</button>
      <span class="min-w-0 flex-1 text-center text-sm text-slate-400">${esc(bounds(s.period, s.offset).title)}</span>
      <button type="button" data-offset="1" class="btn-ghost px-3 py-1" aria-label="Later" ${s.offset >= 0 ? "disabled" : ""}>›</button></div>`}</div>`;

  const body = () => {
    if (s.page?.type === "book") return bookPage(m, s.page.idx, other);
    if (s.page?.type === "period") return periodPage(m, s.page.kind, s.page.start);
    if (s.page?.type === "day") return dayPage(m, s.page.key);
    if (s.tab === "books") return booksTab(m, s);
    if (s.tab === "calendar") return calendarTab(m, s.month);
    return `<div class="mb-3 flex items-start justify-between gap-2"><h2 class="min-w-0 break-words text-2xl font-bold">${esc(title())}</h2>${other ? "" : `<button type="button" data-share class="btn-secondary shrink-0 px-3 py-1 text-sm">📤 Share</button>`}</div>${picker()}<div class="space-y-4">${reportHTML(m, s)}</div>`;
  };

  const draw = (top = false) => {
    view.innerHTML = `${s.page ? "" : back()}<h1 class="mb-3 text-2xl font-bold">${other ? `📊 ${esc(name)}'s reading` : "📊 Reading stats"}</h1>
      ${s.page ? `<button type="button" data-back class="btn-ghost mb-3 px-3 py-1">‹ Back</button>` : tabStrip({ id: "stats-tabs", label: "Reading stats", active: s.tab, tabs: Object.entries(TABS) })}
      ${body()}
      <p class="mt-6 text-xs text-slate-500">Days and hours use this device's time zone. Time is time with a page open (long pauses are cut short); pages are different pages seen. Books and details come from your library where NovelCheck knows the book.</p>`;
    setBars(view);
    if (top) window.scrollTo(0, 0);
  };
  const open = (page) => {
    s.stack.push(s.page);
    s.page = page;
    draw(true);
  };

  view.onclick = (e) => {
    const t = e.target.closest("[data-tab],[data-period],[data-offset],[data-kind],[data-period-open],[data-book],[data-day],[data-day-key],[data-shift],[data-month],[data-back],[data-open-lib],[data-share]");
    if (!t) return;
    const d = t.dataset;
    if (d.back !== undefined) {
      s.page = s.stack.pop() || null;
      return draw(true);
    }
    if (d.share !== undefined) return void openShareCard(m, s, title(), name);
    if (d.openLib) return void (location.hash = "#/library");
    if (d.book !== undefined) return open({ type: "book", idx: Number(d.book) });
    if (d.periodOpen) {
      const [kind, start] = d.periodOpen.split(":");
      return open({ type: "period", kind, start: Number(start) });
    }
    if (d.dayKey) return open({ type: "day", key: d.dayKey });
    if (d.day) return open({ type: "day", key: dayKey(Number(d.day)) });
    if (d.shift && s.page?.type === "day") {
      const x = keyDate(s.page.key);
      x.setDate(x.getDate() + Number(d.shift));
      s.page = { type: "day", key: dayKey(x.getTime()) };
      return draw();
    }
    if (d.tab) s.tab = d.tab;
    else if (d.period) [s.period, s.offset] = [d.period, 0];
    else if (d.offset) s.offset = Math.min(0, s.offset + Number(d.offset));
    else if (d.kind) s.kind = d.kind;
    else if (d.month) s.month = new Date(s.month.getFullYear(), s.month.getMonth() + Number(d.month), 1);
    draw();
  };
  draw();
}
