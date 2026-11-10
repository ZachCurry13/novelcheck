// 🎁 Wrapped: your reading as a full-screen story you tap through, for any month or any year, plus your
// trophies and records. Everything is worked out in this browser from your reading statistics (so days and
// hours are in your own time zone), and nothing is sent anywhere. The last slide makes a picture to share.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { prepare, fmtDur } from "./statscalc.js";
import { bounds, summary } from "./statsperiods.js";
import { tabStrip } from "./tabs.js";
import { openShareCard } from "./statscard.js";
import { trophiesHTML, paintBars } from "./trophies.js";
import { slides } from "./wrappedslides.js";
import { openStory } from "./wrappedstory.js";

const TITLE = `<h1 class="mb-2 text-2xl font-bold">🎁 Wrapped &amp; trophies</h1>`;

export async function renderWrapped(view, state) {
  view.innerHTML = `${TITLE}<p class="text-slate-400">Loading…</p>`;
  const data = await get("/api/me/reading-stats").catch(() => null);
  if (!data) {
    view.innerHTML = `${TITLE}<p class="text-slate-400">Couldn't load your reading just now.</p>`;
    return;
  }
  const m = prepare(data);
  m.goals = data.hardcover_goals || [];
  m.hcRead = data.hardcover_read || []; // books finished according to Hardcover, with the day // reading challenge from Hardcover, when connected
  const s = { tab: new URLSearchParams(location.hash.split("?")[1] || "").get("tab") === "trophies" ? "trophies" : "wrapped", period: "month", offset: -1 };
  if (!m.rows.length && !m.books.length) {
    view.innerHTML = `${TITLE}<div class="card space-y-2 text-sm text-slate-300"><p>Nothing to look back on yet. Wrapped and trophies come from KOReader's reading statistics.</p>
      <a href="#/settings?s=kindle" class="btn-secondary inline-flex">Set up statistics sync</a></div>`;
    return;
  }
  if (!summary(m, "month", -1).sec) s.offset = 0; // no reading last month: start with this one
  const play = () => openStory(slides(m, s, state.user.username), {
    onShare: () => openShareCard(m, { period: s.period, offset: s.offset }, bounds(s.period, s.offset).title, state.user.username),
  });
  const draw = () => {
    const strip = tabStrip({ id: "wrap-tabs", label: "Wrapped", active: s.tab, tabs: [["wrapped", "🎁 Wrapped"], ["trophies", "🏆 Trophies"]] });
    if (s.tab === "trophies") {
      view.innerHTML = `${strip}<h1 class="mb-2 text-2xl font-bold">🏆 Trophies &amp; records</h1>${trophiesHTML(m)}`;
      paintBars(view);
      return;
    }
    const b = bounds(s.period, s.offset);
    const sum = summary(m, s.period, s.offset);
    const label = s.period === "year" ? b.title : new Date(b.from).toLocaleDateString(undefined, { month: "long", year: "numeric" });
    view.innerHTML = `${strip}<div class="mb-3 flex flex-wrap items-center gap-2">
      <button type="button" data-period="month" class="${s.period === "month" ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">By month</button>
      <button type="button" data-period="year" class="${s.period === "year" ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">By year</button>
      <span class="ml-2 flex items-center gap-1"><button type="button" data-off="-1" class="btn-ghost px-3 py-1" aria-label="Earlier">‹</button>
        <span class="min-w-[7rem] text-center text-sm text-slate-300">${esc(label)}</span>
        <button type="button" data-off="1" class="btn-ghost px-3 py-1" aria-label="Later" ${s.offset >= 0 ? "disabled" : ""}>›</button></span></div>
      <div class="wrap-hero card space-y-3 p-6">
        <p class="text-sm font-semibold uppercase tracking-wide text-indigo-200">Your ${esc(label)}</p>
        <p class="text-4xl font-extrabold">${sum.sec ? fmtDur(sum.sec) : "Nothing yet"}</p>
        <p class="text-slate-200">${sum.sec ? `${sum.days} reading day${sum.days === 1 ? "" : "s"} · ${sum.finished} finished · ${Number(sum.pages).toLocaleString()} pages` : "No reading in this period."}</p>
        <button type="button" data-play class="btn-primary px-6 py-3 text-base">▶ Play my Wrapped</button></div>`;
  };
  view.onclick = (e) => {
    const t = e.target.closest("[data-tab]")?.dataset.tab;
    if (t) {
      s.tab = t;
      return draw();
    }
    const p = e.target.closest("[data-period]")?.dataset.period;
    if (p) {
      Object.assign(s, { period: p, offset: summary(m, p, -1).sec ? -1 : 0 });
      return draw();
    }
    const off = e.target.closest("[data-off]")?.dataset.off;
    if (off) {
      s.offset = Math.min(0, s.offset + Number(off));
      return draw();
    }
    if (e.target.closest("[data-play]")) play();
  };
  view.onkeydown = null;
  draw();
}
