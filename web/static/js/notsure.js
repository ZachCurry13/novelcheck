// Not sure ratings (Admin → AI & Scans): every rating the small AI was unsure of, and what to do with them: look at all of them with the
// bigger model in one batch, this many a night, or none. The bigger model gives one second opinion per rating (per rules version).
import { get, post, put } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

export async function notSureCard(host) {
  const card = document.createElement("section");
  card.className = "card mb-4 space-y-2";
  host.append(card);
  let timer;
  const paint = async () => {
    const d = await get("/api/admin/not-sure").catch(() => null);
    if (!d || !card.isConnected) return clearInterval(timer);
    const ready = d.machine && d.local;
    const pep = (b) => (b.level == null ? "?" : b.level);
    card.innerHTML = `<h2 class="font-semibold">Not sure ratings</h2>
      <p class="text-sm text-slate-400">${d.books.length ? `${d.books.length} rating${d.books.length === 1 ? "" : "s"} the AI wasn't sure of; <b>${d.waiting}</b> still waiting for the bigger model's second opinion.` : "No rating is marked Not sure."}
        ${ready ? "" : ` <span class="text-amber-300">The bigger model needs a Deep Scan machine on your home network (set one up above); until then nothing here can run.</span>`}${d.away ? ` <span class="text-amber-300">The machine isn't answering right now.</span>` : ""}</p>
      ${d.run.running ? `<p class="text-sm text-emerald-300">Looking at them now: ${d.run.done} of ${d.run.total} done. You can leave this page.</p>` : ""}
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" data-run="0" class="btn-primary py-1 text-sm" ${ready && !d.run.running && d.waiting ? "" : "disabled"}>Look at all ${d.waiting} now</button>
        <button type="button" data-run="10" class="btn-secondary py-1 text-sm" ${ready && !d.run.running && d.waiting ? "" : "disabled"}>Look at 10 now</button>
      </div>
      <label class="flex flex-wrap items-center gap-2 text-sm">Or each night, inside the hours automatic rating is allowed:
        <input data-nightly type="number" min="0" max="1000" class="input w-24" value="${d.nightly}"> books <button type="button" data-save class="btn-ghost py-1 text-sm">Save</button>
        <span class="text-xs text-slate-400">0 = not by night${d.nightly ? `; ${d.done_tonight} done tonight` : ""}</span></label>
      ${d.books.length ? `<details class="text-sm"><summary class="cursor-pointer text-slate-300">Show the ${d.books.length} ratings</summary>
        <ul class="mt-2 max-h-72 space-y-1 overflow-y-auto">${d.books.map((b) => `<li class="flex gap-2"><span class="w-5 shrink-0 text-center">${pep(b)}</span>
          <span class="min-w-0 flex-1 truncate">${esc(b.title)} <span class="text-slate-400">${esc(b.author)}</span></span>
          <span class="shrink-0 text-xs ${b.checked ? "text-slate-500" : "text-amber-300"}">${b.checked ? "checked" : "waiting"}</span></li>`).join("")}</ul></details>` : ""}`;
    card.querySelectorAll("[data-run]").forEach((b) => b.onclick = async () => {
      const n = Number(b.dataset.run);
      if (!confirm(n ? `Look at ${n} of them now with the bigger model?` : `Look at all ${d.waiting} with the bigger model now? It can take a long while.`)) return;
      if (await attempt(() => post("/api/admin/not-sure/run", { limit: n }), "Started")) { clearInterval(timer); timer = setInterval(paint, 5000); paint(); }
    });
    card.querySelector("[data-save]").onclick = async () => {
      if (await attempt(() => put("/api/admin/not-sure", { nightly: Number(card.querySelector("[data-nightly]").value) || 0 }), "Saved")) paint();
    };
    if (!d.run.running) clearInterval(timer);
    else if (!timer) timer = setInterval(paint, 5000);
  };
  paint();
}
