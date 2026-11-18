// Library cleanup → "Remembered ratings and covers" (api/memory_handlers.go): what NovelCheck keeps about books that
// left every library, list and event, so they are not rated or searched for again; a switch to stop, and buttons
// to forget ratings and to delete the covers of books that are gone.
import { get, put, post } from "./api.js";
import { attempt, toast, fmtNum } from "./ui.js";

const size = (b) => (b >= 1e6 ? `${(b / 1e6).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1e3))} KB`);

export async function renderMemory(host) {
  const draw = (r) => {
    if (!r) return;
    host.innerHTML = `<p class="text-sm text-slate-300">NovelCheck remembers the rating and cover of books that leave, so a returning book isn't rated twice. It takes little room:
      <b>${fmtNum(r.ratings)}</b> remembered rating${r.ratings === 1 ? "" : "s"}${r.ratings_oldest ? ` (the oldest from ${r.ratings_oldest})` : ""} and
      <b>${fmtNum(r.covers_orphans)}</b> cover${r.covers_orphans === 1 ? "" : "s"} of books that are gone (${size(r.covers_orphan_bytes)}; the ${fmtNum(r.covers_kept)} covers of books you have use ${size(r.covers_kept_bytes)}).</p>
      <label class="toggle mt-2"><input type="checkbox" data-mem-on ${r.remember ? "checked" : ""}> Remember ratings and covers of books that leave</label>
      <div class="mt-3 flex flex-wrap gap-2">
        <button type="button" data-mem="ratings:0" class="btn-secondary py-1 text-sm" ${r.ratings ? "" : "disabled"}>Forget all remembered ratings</button>
        <button type="button" data-mem="ratings:365" class="btn-secondary py-1 text-sm" ${r.ratings ? "" : "disabled"}>Forget ratings older than a year</button>
        <button type="button" data-mem="covers:0" class="btn-secondary py-1 text-sm" ${r.covers_orphans ? "" : "disabled"}>Delete covers of books that are gone</button></div>
      <p class="mt-2 text-xs text-slate-400">Forgetting can't be undone. Books you have are never touched.</p>`;
  };
  draw(await attempt(() => get("/api/admin/memory")));
  host.onchange = async (e) => {
    if (!e.target.matches("[data-mem-on]")) return;
    const r = await attempt(() => put("/api/admin/memory", { remember: e.target.checked }));
    if (r) draw(r);
    else e.target.checked = !e.target.checked;
  };
  host.onclick = async (e) => {
    const b = e.target.closest("[data-mem]");
    if (!b) return;
    const [what, days] = b.dataset.mem.split(":");
    const ask = what === "covers" ? "Delete the stored covers of books that are gone? This cannot be undone." : days === "0"
      ? "Forget every remembered rating? Books that come back will be rated again. This cannot be undone." : "Forget the ratings remembered for over a year? This cannot be undone.";
    if (!confirm(ask)) return;
    b.disabled = true;
    const r = await attempt(() => post("/api/admin/memory/purge", { what, days: Number(days) }));
    if (!r) return void (b.disabled = false);
    toast(what === "covers" ? `Deleted ${fmtNum(r.deleted)} cover${r.deleted === 1 ? "" : "s"} (${size(r.bytes)})` : `Forgot ${fmtNum(r.forgotten)} rating${r.forgotten === 1 ? "" : "s"}`);
    draw(r.state);
  };
}
