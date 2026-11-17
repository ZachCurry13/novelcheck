// Settings → "Reading summaries": which of the weekly, monthly and yearly notices this person wants on their phone
// (api/summaries.go). They go to the phones where notifications are on (the card above turns them on).
import { get, put } from "./api.js";
import { attempt, toast } from "./ui.js";

const ROWS = [["week", "Every week", "Mondays at 9:00, with your hours and days"], ["month", "Every month", "On the 1st at 9:00: your Wrapped is ready"], ["year", "Every year", "On 1 January at 9:00: your year's Wrapped"]];

export async function renderSummaryPrefs(host) {
  if (!host) return;
  const prefs = await attempt(() => get("/api/me/summaries"));
  if (!prefs) return;
  host.innerHTML = `<h2 class="text-lg font-semibold">Reading summaries</h2>
    <p class="text-sm text-slate-400">A notice on your phone when a week, a month or a year of reading is ready. Nothing is sent for a period you didn't read, and only to phones that have notifications on.</p>
    <div class="space-y-1">${ROWS.map(([k, name, note]) => `<label class="toggle min-h-[2.75rem]"><input type="checkbox" data-sum="${k}" ${prefs[k] ? "checked" : ""}>
      <span>${name}<span class="block text-xs text-slate-400">${note}</span></span></label>`).join("")}</div>`;
  host.onchange = async (e) => {
    const box = e.target.closest("[data-sum]");
    if (!box) return;
    const next = Object.fromEntries(ROWS.map(([k]) => [k, host.querySelector(`[data-sum="${k}"]`).checked]));
    const saved = await attempt(() => put("/api/me/summaries", next));
    if (saved) toast("Saved");
    else box.checked = !box.checked;
  };
}
