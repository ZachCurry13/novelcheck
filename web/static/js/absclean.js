// Library cleanup → Audiobookshelf (api/absclean.go): every audiobook matched against Hardcover. Certain
// matches are fixed by themselves (a switch turns that off); the rest wait here with what would change,
// to Apply or Skip one by one.
import { get, put, post } from "./api.js";
import { esc, attempt } from "./ui.js";

const FIELDS = [["title", "Title"], ["author", "Author"], ["series", "Series"], ["isbn", "ISBN"], ["asin", "ASIN"],
  ["narrators", "Narrator"], ["genres", "Genres"], ["description", "Description"], ["cover", "Cover"]];

const change = (f) => FIELDS.filter(([k]) => f[k] && (!Array.isArray(f[k]) || f[k].length)).map(([k, l]) => {
  let v = Array.isArray(f[k]) ? f[k].join(", ") : f[k];
  if (k === "series" && f.sequence) v += ` #${f.sequence}`;
  if (k === "description") v = v.length > 120 ? v.slice(0, 120) + "…" : v;
  if (k === "cover") v = "Hardcover's cover";
  return `<li><span class="text-slate-400">${l}:</span> ${esc(v)}</li>`;
}).join("");

export async function renderABSClean(host) {
  const paint = (r) => {
    if (!r) return;
    host.innerHTML = `<p class="text-sm text-slate-300">Each audiobook is matched with Hardcover by ISBN, by Audible ASIN, then by exact title and author.
      When the match is certain, Audiobookshelf gets Hardcover's title, author and series, and whatever it is missing (ISBN, ASIN, narrator, genres,
      description, cover). Anything less certain waits below for you. Every audiobook is looked at again after a month.</p>
      ${r.hardcover ? "" : `<p class="mt-2 text-sm text-amber-300">Connect Hardcover first (Profile → Integrations): the cleanup uses that connection.</p>`}
      <label class="toggle mt-2"><input type="checkbox" data-abs-auto ${r.auto ? "checked" : ""}> Fix certain matches by themselves</label>
      <p class="mt-2 text-xs text-slate-400">${esc(r.result || "Not run yet: it runs with the Hardcover ratings, every 6 hours.")}</p>
      <button type="button" data-abs-run class="btn-secondary mt-2 py-1 text-sm" ${r.busy ? "disabled" : ""}>${r.busy ? "Looking now…" : "Look now"}</button>
      <h3 class="mt-4 font-semibold">Waiting for you (${r.fixes.length})</h3>
      ${r.fixes.length > 1 ? `<button type="button" data-abs-all class="btn-secondary mt-2 py-1 text-sm">Apply all ${r.fixes.length}</button>` : ""}
      ${r.fixes.length ? `<ul class="mt-2 space-y-2">${r.fixes.map((x) => `<li class="rounded-lg bg-slate-800/60 p-3 text-sm">
        <p class="font-semibold">${esc(x.title)} <span class="font-normal text-slate-400">${esc(x.author)}</span></p>
        <p class="text-xs text-amber-200">${esc(x.why)}</p><ul class="mt-1 text-xs">${change(x.fix)}</ul>
        <div class="mt-2 flex gap-2"><button type="button" data-abs-fix="${esc(x.item_id)}" data-do="apply" class="btn-primary py-1 text-sm">Apply</button>
          <button type="button" data-abs-fix="${esc(x.item_id)}" data-do="skip" class="btn-ghost py-1 text-sm">Skip</button></div></li>`).join("")}</ul>`
        : `<p class="text-sm text-emerald-400">✓ Nothing waiting</p>`}`;
  };
  paint(await attempt(() => get("/api/admin/audiobookshelf/cleanup")));
  host.onchange = async (e) => {
    if (e.target.matches("[data-abs-auto]")) paint(await attempt(() => put("/api/admin/audiobookshelf/cleanup", { auto: e.target.checked })));
  };
  host.onclick = async (e) => {
    if (e.target.closest("[data-abs-run]")) {
      if (await attempt(() => post("/api/admin/audiobookshelf/cleanup/run"), "Looking at the audiobooks in the background")) {
        setTimeout(async () => paint(await attempt(() => get("/api/admin/audiobookshelf/cleanup"))), 1500);
      }
      return;
    }
    if (e.target.closest("[data-abs-all]")) {
      if (!confirm("Apply every change listed here to Audiobookshelf?")) return;
      const r = await attempt(() => post("/api/admin/audiobookshelf/fixes/all"), "Fixed in Audiobookshelf");
      if (r) paint(r);
      return;
    }
    const b = e.target.closest("[data-abs-fix]");
    if (!b) return;
    b.disabled = true;
    const r = await attempt(() => post(`/api/admin/audiobookshelf/fixes/${encodeURIComponent(b.dataset.absFix)}`, { action: b.dataset.do }),
      b.dataset.do === "apply" ? "Fixed in Audiobookshelf" : "Skipped");
    if (r) paint(r);
    else b.disabled = false;
  };
}
