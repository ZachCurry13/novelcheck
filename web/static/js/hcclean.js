// Admin → Library cleanup → "Fix Calibre details from Hardcover": a background scan compares every Calibre book
// with Hardcover and lists what differs (series and number, tidy title and author, ISBN, description, genres).
// Nothing is written until the ticked changes are applied; each book is read back afterwards to check Calibre
// kept them.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { once } from "./inflight.js";

const LABEL = { title: "Title", authors: "Author", series: "Series", series_index: "Number in series", identifiers: "ISBN", comments: "Description", tags: "Genres" };
const SHOW = 400;

export async function renderHCClean(host) {
  host.innerHTML = `<p class="text-sm text-slate-400">Compares every book in Calibre with Hardcover (your Hardcover, set under Settings → Connections) in the background; for a library of a few thousand books it can take up to an hour. It only fills gaps and tidies how things are written; it never swaps one book's details for another's. You choose what to apply.</p>
    <div class="my-2 flex flex-wrap items-center gap-2"><button type="button" data-hc="scan" class="btn-primary py-1 text-sm">🔎 Scan the library</button>
      <button type="button" data-hc="stop" class="btn-ghost hidden py-1 text-sm">Stop</button><span id="hc-status" class="text-sm text-slate-300"></span></div>
    <div id="hc-errors" class="hidden rounded-lg bg-amber-950/40 p-2 text-xs text-amber-200"></div>
    <div id="hc-list" class="space-y-2"></div>`;
  const status = host.querySelector("#hc-status");
  const list = host.querySelector("#hc-list");
  let timer = null;
  const draw = (s) => {
    const j = s.job || {};
    host.querySelector("[data-hc=scan]").disabled = j.running;
    host.querySelector("[data-hc=stop]").classList.toggle("hidden", !j.running);
    status.textContent = j.running ? `${j.kind === "apply" ? "Applying" : "Scanning"}: ${j.done} of ${j.total}${j.found ? ` · ${j.found} ${j.kind === "apply" ? "fixed" : "found"}` : ""}…` : j.note || "";
    const errs = host.querySelector("#hc-errors");
    errs.classList.toggle("hidden", !(j.errors || []).length);
    errs.innerHTML = (j.errors || []).map((e) => `<p>${esc(e)}</p>`).join("");
    if (!s.hardcover || !s.server) {
      list.innerHTML = `<p class="text-sm text-amber-200">${!s.hardcover ? "Connect your Hardcover under Settings → Connections first. " : ""}${!s.server ? "Set up the Calibre Content server under Admin → Calibre Library first." : ""}</p>`;
      return;
    }
    const items = s.suggestions || [];
    if (!items.length) {
      list.innerHTML = j.running ? "" : `<p class="text-sm text-slate-400">Nothing waiting. Press Scan to compare the library with Hardcover.</p>`;
      return;
    }
    const byBook = new Map();
    for (const x of items.slice(0, SHOW)) byBook.set(x.calibre_id, [...(byBook.get(x.calibre_id) || []), x]);
    list.innerHTML = `<div class="flex flex-wrap items-center gap-2"><label class="toggle"><input type="checkbox" data-hc="all"> Tick all shown</label>
      <button type="button" data-hc="apply" class="btn-primary py-1 text-sm">Apply ticked to Calibre</button>
      <button type="button" data-hc="skip" class="btn-ghost py-1 text-sm">Not these (don't suggest again)</button>
      <span class="text-xs text-slate-500">${items.length.toLocaleString()} suggestion${items.length === 1 ? "" : "s"}${items.length > SHOW ? `, the first ${SHOW} shown` : ""}</span></div>
      ${[...byBook.values()].map((g) => `<div class="card space-y-1 p-3"><p class="font-semibold">${esc(g[0].title)} <span class="text-xs font-normal text-slate-500">Calibre #${g[0].calibre_id} · matched by ${esc(g[0].confidence)}</span></p>
        ${g.map((x) => `<label class="flex items-start gap-2 text-sm"><input type="checkbox" data-hc-id="${x.id}">
          <span><b>${LABEL[x.field] || esc(x.field)}:</b> <span class="text-slate-400 line-through">${esc(x.current)}</span> → ${esc(x.shown)}</span></label>`).join("")}</div>`).join("")}`;
  };
  const refresh = once(async () => {
    const s = await get("/api/admin/hc-clean").catch(() => null);
    if (!s) return;
    const open = host.querySelector("#hc-list [data-hc-id]:checked");
    if (!(s.job || {}).running || !open) draw(s); // don't wipe the ticks while someone is choosing
    clearTimeout(timer);
    if ((s.job || {}).running && host.isConnected) timer = setTimeout(refresh, 2500);
  });
  const ticked = () => [...host.querySelectorAll("[data-hc-id]:checked")].map((e) => Number(e.dataset.hcId));
  host.addEventListener("click", async (e) => {
    const k = e.target.closest("[data-hc]")?.dataset.hc;
    if (k === "all") host.querySelectorAll("[data-hc-id]").forEach((c) => (c.checked = e.target.checked));
    else if (k === "scan" && (await attempt(() => post("/api/admin/hc-clean/scan", {}), "Scanning in the background"))) refresh();
    else if (k === "stop") attempt(() => post("/api/admin/hc-clean/stop", {}), "Stopping after this book");
    else if (k === "apply" || k === "skip") {
      const ids = ticked();
      if (!ids.length) return toast("Tick at least one change", true);
      if (k === "apply" && !confirm(`Write ${ids.length} change${ids.length === 1 ? "" : "s"} to Calibre? Calibre renames each book's files to match; this can take a while and runs in the background.`)) return;
      if (await attempt(() => post(`/api/admin/hc-clean/${k}`, { ids }), k === "apply" ? "Applying in the background" : "Skipped")) refresh();
    }
  });
  refresh();
}
