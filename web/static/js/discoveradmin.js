// Admin → Delivery & Services → Discover: how the lists are doing, a test for
// the New York Times key, and a refresh button.
import { get, post } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";

const GUIDE_URL = "https://github.com/ZachCurry13/novelcheck/blob/main/docs/DISCOVER.md";

export async function renderDiscoverAdmin(host, form) {
  const s = await attempt(() => get("/api/admin/discover"));
  if (!s) return;
  const when = s.updated ? new Date(s.updated).toLocaleString() : "not yet";
  host.innerHTML = `
    <p class="text-xs text-slate-400">Without a key, the Popular, teen and kids' rows come from Open Library and lean toward older books.
      A New York Times Books key is free and takes about five minutes: <a href="${GUIDE_URL}" target="_blank" rel="noopener noreferrer" class="underline">how to get one</a>.</p>
    <p class="text-sm">Lists last refreshed: <b>${esc(when)}</b>${s.refreshing ? " (refreshing now…)" : ""}.
      ${s.counts.listed} books listed, ${s.counts.rated} rated, ${s.counts.waiting} waiting.</p>
    ${s.result ? `<p class="text-xs text-slate-400">${esc(s.result)}</p>` : ""}
    <div class="flex flex-wrap gap-2">
      <button type="button" data-dact="test" class="btn-secondary">Test key</button>
      <button type="button" data-dact="refresh" class="btn-secondary" ${s.refreshing ? "disabled" : ""}>Refresh lists now</button>
    </div>`;
  host.onclick = async (e) => {
    const act = e.target.closest("[data-dact]")?.dataset.dact;
    if (act === "test") {
      const key = $('[data-key="nyt_api_key"]', form)?.value || "";
      const r = await attempt(() => post("/api/admin/discover/test", { key }));
      if (r) toast(`The key works: its teen list has ${r.books} books. Remember to save.`);
    } else if (act === "refresh") {
      if (await attempt(() => post("/api/admin/discover/refresh"), "Refreshing the lists; this takes about a minute")) {
        setTimeout(() => renderDiscoverAdmin(host, form), 90_000);
      }
    }
  };
}
