// Health (admins, 2.0): one list that says whether NovelCheck is working, worst first, each problem with what to do about it.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

const DOT = { ok: "text-emerald-300", info: "text-slate-400", warn: "text-amber-300", bad: "text-rose-300" };
const WORD = { ok: "OK", info: "FYI", warn: "Look at", bad: "Fix now" };

export async function renderHealth(view) {
  view.innerHTML = `<a href="#/home" class="text-sm text-slate-400 underline">← Home</a><h1 class="page-title mb-1 mt-2">Health</h1>
    <p class="mb-4 max-w-3xl text-sm text-slate-400">Whether NovelCheck is working, from what it already knows (nothing is asked of outside services to draw this page). Fix the red ones first.</p><div id="health" class="space-y-2"><p class="text-slate-400">Looking…</p></div>
    <p class="mt-4 text-sm"><a href="#/system" class="underline">Connections and versions</a> · <a href="#/accuracy" class="underline">How accurate the ratings are</a></p>`;
  const d = await attempt(() => get("/api/admin/health"));
  if (!d) return;
  backups(view);
  const host = view.querySelector("#health");
  host.onclick = async (e) => {
    const rm = e.target.closest("[data-remind]");
    if (rm) {
      rm.disabled = true;
      const r = await attempt(() => post("/api/admin/devices/remind", JSON.parse(rm.dataset.remind)));
      if (r) toast(r.sent ? "Reminder sent to their phone" : "They have no phone notifications on, so nothing was sent. Ask them to open the app on their Kindle.");
      rm.disabled = false;
      return;
    }
    const f = e.target.closest("[data-forget]");
    if (!f || !confirm("Forget this Kindle? NovelCheck stops listing it. Its backup files stay.")) return;
    if (await attempt(() => post("/api/admin/devices/forget", JSON.parse(f.dataset.forget)), "Forgotten")) renderHealth(view);
  };
  view.querySelector("#health").innerHTML = d.items.map((i) => `<div class="card flex items-start gap-3"><span class="mt-1 shrink-0 text-xs font-semibold ${DOT[i.level]}">${WORD[i.level]}</span>
    <div class="min-w-0"><p class="font-semibold">${esc(i.title)}</p><p class="text-sm text-slate-300">${esc(i.detail)}</p>
    ${i.fix ? `<p class="text-sm"><b>What to do:</b> ${esc(i.fix)} ${i.link ? `<a href="${esc(i.link)}" class="underline">Go</a>` : ""}</p>` : ""}
    ${i.remind ? `<button type="button" data-remind='${esc(JSON.stringify(i.remind))}' class="btn-secondary mt-1 py-1 text-xs">Send a reminder</button>` : ""}
    ${i.forget ? `<button type="button" data-forget='${esc(JSON.stringify(i.forget))}' class="btn-ghost mt-1 py-1 text-xs">Forget this Kindle</button>` : ""}</div></div>`).join("") || `<p class="text-slate-400">Nothing to report.</p>`;
}

// backups lists the daily copies of NovelCheck's database with "Back up now"; the main admin can download one to keep elsewhere.
async function backups(view) {
  const host = document.createElement("section");
  host.className = "card mt-4 space-y-2";
  view.append(host);
  const draw = (r) => {
    const mb = (n) => `${(n / (1 << 20)).toFixed(1)} MB`;
    host.innerHTML = `<div class="flex items-center justify-between gap-2"><h2 class="font-semibold">Backups of NovelCheck's data</h2><button type="button" data-now class="btn-secondary py-1 text-sm">Back up now</button></div>
      <p class="text-xs text-slate-400">A copy is made every day and the last week is kept. To restore: stop NovelCheck, put a copy in the data folder as <b>novelcheck.db</b>, start it. Keep a copy somewhere else too.</p>
      ${r.files.length ? `<ul class="space-y-1 text-sm">${r.files.map((f) => `<li class="flex items-center justify-between gap-2"><span>${esc(f.name)} <span class="text-slate-400">${mb(f.size)} · ${esc(f.at)} UTC</span></span>
        ${r.can_download ? `<a href="/api/admin/backups/${esc(f.name)}" class="underline">Download</a>` : ""}</li>`).join("")}</ul>` : `<p class="text-sm text-slate-400">None yet. The first one is made a few minutes after NovelCheck starts.</p>`}`;
    const tidy = document.createElement("div");
    tidy.className = "space-y-2 text-sm";
    tidy.innerHTML = `<button type="button" data-tidy class="btn-ghost py-1 text-sm">Tidy backups…</button><div data-tidy-out></div>`;
    host.append(tidy);
    tidy.querySelector("[data-tidy]").onclick = async (e) => {
      e.target.disabled = true;
      const t = await attempt(() => get("/api/admin/backups/tidy"));
      e.target.disabled = false;
      if (!t) return;
      const mb = (n) => `${(n / (1 << 20)).toFixed(1)} MB`;
      const out = tidy.querySelector("[data-tidy-out]");
      out.innerHTML = t.count ? `<p>This would remove <b>${t.count}</b> file${t.count === 1 ? "" : "s"} (${mb(t.bytes)}): Amazon's dictionary files, and files from a Kindle app backup whose identical copy is in the same Kindle's plugin backup. Nothing else is touched.</p>
        <ul class="max-h-48 space-y-1 overflow-y-auto text-xs text-slate-400">${t.items.map((i) => `<li>${esc(i.device)} · ${esc(i.path)} · ${mb(i.size)} · ${esc(i.why)}</li>`).join("")}</ul>
        ${t.can_delete ? `<button type="button" data-go class="btn-primary py-1 text-sm">Remove these ${t.count} files</button>` : `<p class="text-xs text-slate-400">Only the main admin can remove them.</p>`}` : `<p>Nothing to tidy.</p>`;
      if (t.unlinked.length) out.innerHTML += `<p class="text-xs text-amber-300">Not checked for copies: ${t.unlinked.map(esc).join(", ")}. If it is the same Kindle as one with the KOReader plugin, link them (Settings → Kindle: "Same Kindle as…") and tidy again.</p>`;
      out.querySelector("[data-go]")?.addEventListener("click", async (ev) => {
        ev.target.disabled = true;
        const r = await attempt(() => post("/api/admin/backups/tidy"));
        out.innerHTML = r ? `<p class="text-emerald-300">Removed ${r.removed} file${r.removed === 1 ? "" : "s"} (${mb(r.bytes)}).</p>` : "";
      });
    };
    host.querySelector("[data-now]").onclick = async (e) => {
      e.target.disabled = true;
      const n = await attempt(() => post("/api/admin/backups/now"), "Backed up");
      if (n) draw({ ...r, files: n.files });
      else e.target.disabled = false;
    };
  };
  const r = await attempt(() => get("/api/admin/backups"));
  if (r) draw(r);
}
