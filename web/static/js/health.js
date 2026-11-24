// Health (admins, 2.0): one list that says whether NovelCheck is working, worst first, each problem with what to do about it.
import { get } from "./api.js";
import { esc, attempt } from "./ui.js";

const DOT = { ok: "text-emerald-300", warn: "text-amber-300", bad: "text-rose-300" };
const WORD = { ok: "OK", warn: "Look at", bad: "Fix now" };

export async function renderHealth(view) {
  view.innerHTML = `<a href="#/home" class="text-sm text-slate-400 underline">← Home</a><h1 class="page-title mb-1 mt-2">Health</h1>
    <p class="mb-4 max-w-3xl text-sm text-slate-400">Whether NovelCheck is working, from what it already knows (nothing is asked of outside services to draw this page). Fix the red ones first.</p><div id="health" class="space-y-2"><p class="text-slate-400">Looking…</p></div>
    <p class="mt-4 text-sm"><a href="#/system" class="underline">Connections and versions</a> · <a href="#/accuracy" class="underline">How accurate the ratings are</a></p>`;
  const d = await attempt(() => get("/api/admin/health"));
  if (!d) return;
  view.querySelector("#health").innerHTML = d.items.map((i) => `<div class="card flex items-start gap-3"><span class="mt-1 shrink-0 text-xs font-semibold ${DOT[i.level]}">${WORD[i.level]}</span>
    <div class="min-w-0"><p class="font-semibold">${esc(i.title)}</p><p class="text-sm text-slate-300">${esc(i.detail)}</p>
    ${i.fix ? `<p class="text-sm"><b>What to do:</b> ${esc(i.fix)} ${i.link ? `<a href="${esc(i.link)}" class="underline">Go</a>` : ""}</p>` : ""}</div></div>`).join("") || `<p class="text-slate-400">Nothing to report.</p>`;
}
