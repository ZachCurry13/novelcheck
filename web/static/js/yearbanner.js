// The "Read this year" banner at the top of the Library: books finished this year (KOReader, an imported list,
// Hardcover and "I read this" all count), against last year by this date. Hidden for someone with no reading
// recorded. Its colours come from the theme (web/static/themes).
import { get } from "./api.js";
import { prepare } from "./statscalc.js";
import { addElsewhere } from "./statselsewhere.js";
import { summary } from "./statsperiods.js";

const plural = (n, one, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

export async function renderYearBanner(host) {
  if (!host) return;
  const data = await get("/api/me/reading-stats").catch(() => null);
  if (!data || !host.isConnected) return;
  const m = addElsewhere(prepare(data), data.hardcover_read || [], data.imported_read || []);
  const now = summary(m, "year", 0).finished;
  // Last year up to the same day, so early in the year the comparison is fair.
  const lastAll = summary(m, "year", -1);
  const sameDay = Date.now() - 365 * 864e5;
  const lastByNow = lastAll.finishedList.filter((d) => d.ms <= sameDay).length;
  const lastTotal = lastAll.finished;
  if (!now && !lastTotal) return;
  const goal = Math.max(lastTotal, now, 1);
  const line = lastTotal === 0 ? "Your first year in NovelCheck"
    : now > lastTotal ? `${plural(now - lastTotal, "book")} more than all of last year`
    : now === lastTotal ? "As many as all of last year"
    : `${plural(lastTotal - now, "more book")} to beat last year${lastByNow ? ` · ${lastByNow} by this date last year` : ""}`;
  host.innerHTML = `<a href="#/stats" class="year-banner mb-4 block" aria-label="Reading stats">
    <span class="text-sm opacity-90">Read this year</span>
    <b>${plural(now, "book")}</b>
    <span class="text-sm opacity-90">${line}</span>
    <span class="bar" aria-hidden="true"><i data-pct="${Math.min(100, Math.round((now / goal) * 100))}"></i></span></a>`;
  host.querySelectorAll("[data-pct]").forEach((el) => (el.style.width = `${el.dataset.pct}%`));
}
