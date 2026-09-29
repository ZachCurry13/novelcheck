// AI machines (Admin → AI & Scans): whether the Deep Scan machine answers,
// newer versions of the Ollama models in use (Update downloads it again), and
// the speed test that times each machine's models on a made-up sample book.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

const box = (title, body) => `<div class="space-y-2 rounded-lg bg-slate-800/60 p-3"><p class="font-semibold">${title}</p>${body}</div>`;
const ago = (t) => (t ? new Date(t).toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }) : "");
const secs = (s) => (s >= 90 ? `${Math.round(s / 60)} min` : `${s.toFixed(s < 10 ? 1 : 0)} s`);
const hours = (s) => (s >= 5400 ? `about ${(s / 3600).toFixed(1)} hours` : `about ${Math.max(1, Math.round(s / 60))} minutes`);

export async function renderAITools(host) {
  let timer;
  const paint = async () => {
    const st = await get("/api/admin/aitools").catch(() => null);
    if (!st || !host.isConnected) return clearInterval(timer);
    const ups = st.updates || [];
    const results = Object.values(st.results || {}).sort((a, b) => a.ai.localeCompare(b.ai) || a.model.localeCompare(b.model));
    host.innerHTML = `
      ${st.deep_machine ? box("🧬 Deep Scan machine", st.deep_away
        ? `<p class="text-sm text-amber-300">⏸ Not answering right now (switched off or asleep).${st.deep_waiting ? " Deep Scans are waiting and carry on when it's back." : " Deep Scans will wait for it."}</p>`
        : `<p class="text-sm text-emerald-300">✓ Answering: Deep Scans run there.</p>`) : ""}
      ${box("⬆️ Model updates", `${ups.length ? `<ul class="space-y-2">${ups.map((u, i) => `<li class="flex flex-wrap items-center gap-2 text-sm">
          <span class="min-w-0 flex-1">A newer <b>${esc(u.model)}</b> is out <span class="text-xs text-slate-400">(on ${esc(u.server)})</span></span>
          <button type="button" data-update="${i}" class="btn-secondary py-1 text-sm">Update</button></li>`).join("")}</ul>`
        : `<p class="text-sm text-slate-400">${st.checked_at ? `Your Ollama models are up to date (checked ${ago(st.checked_at)}).` : "NovelCheck checks your Ollama models for newer versions once a day."}</p>`}
        ${st.check_error ? `<p class="text-xs text-amber-300">Couldn't check: ${esc(st.check_error)}</p>` : ""}
        <div data-progress class="hidden text-sm"><progress max="100" class="w-full"></progress><p class="text-xs text-slate-400"></p></div>
        <button type="button" data-check class="btn-ghost py-1 text-sm">Check now</button>`)}
      ${box("⏱ Speed test", `<p class="text-xs text-slate-400">Times each model on a made-up sample book: one rating, and one Deep Scan part of about 600 words. Deep Scan estimates then say how long a book takes (and its electricity).</p>
        <div class="flex flex-wrap gap-2">${[["main", "Main AI"], ["backup", "Backup AI"], ["deep", "Deep Scans"]].map(([k, l]) =>
          `<button type="button" data-bench="${k}" class="btn-secondary py-1 text-sm" ${st.benching ? "disabled" : ""}>Test ${l}</button>`).join("")}</div>
        ${st.benching ? `<p class="text-sm text-slate-300">Testing… a local model can take a few minutes.</p>` : ""}
        ${results.length ? `<div class="overflow-x-auto"><table class="w-full text-left text-xs"><thead class="text-slate-400"><tr><th class="py-1 pr-2">Model</th><th class="pr-2">A rating</th><th class="pr-2">A 100,000-word Deep Scan</th><th>Tested</th></tr></thead>
          <tbody>${results.map((b) => `<tr class="border-t border-slate-700"><td class="py-1 pr-2">${esc(b.model)} <span class="text-slate-500">${esc(b.ai)}</span></td>
            ${b.error ? `<td colspan="2" class="pr-2 text-amber-300">${esc(b.error)}</td>`
              : `<td class="pr-2">${secs(b.rating_s)}</td><td class="pr-2">${b.part_words ? hours((100000 / b.part_words) * b.part_s * 1.15) : "–"}${b.tok_s ? ` <span class="text-slate-500">(${Math.round(b.tok_s)} words/s written)</span>` : ""}</td>`}
            <td class="text-slate-400">${ago(b.at)}</td></tr>`).join("")}</tbody></table></div>` : ""}`)}`;
    host._ups = ups;
    if (!st.benching) clearInterval(timer);
  };
  const poll = () => {
    clearInterval(timer);
    timer = setInterval(paint, 3000);
  };
  host.onclick = async (e) => {
    const b = e.target.closest("button");
    if (!b) return;
    if (b.dataset.check !== undefined && (await attempt(() => post("/api/admin/aitools/check-updates"), "Checking… this takes a minute"))) setTimeout(paint, 20000);
    else if (b.dataset.bench && (await attempt(() => post("/api/admin/aitools/bench", { which: b.dataset.bench }), "Speed test started"))) paint().then(poll);
    else if (b.dataset.update !== undefined) update(host._ups[Number(b.dataset.update)], host, paint);
  };
  await paint();
  if (host.querySelector("[data-bench][disabled]")) poll();
}

// update downloads the model again on its Ollama, showing progress.
async function update(u, host, done) {
  if (!u || !(await attempt(() => post("/api/admin/ollama/pull", { url: u.server, model: u.model })))) return;
  const prog = host.querySelector("[data-progress]");
  prog.classList.remove("hidden");
  const t = setInterval(async () => {
    const st = await get("/api/admin/ollama/pull").catch(() => null);
    if (!st || !host.isConnected) return;
    prog.querySelector("progress").value = Math.round(st.percent || 0);
    prog.querySelector("p").textContent = st.error ? `Problem: ${st.error}` : `${u.model}: ${st.status || "working"} · ${Math.round(st.percent || 0)}%`;
    if (st.active) return;
    clearInterval(t);
    if (st.done && !st.error) {
      await post("/api/admin/aitools/updated", { server: u.server, model: u.model }).catch(() => null);
      toast(`${u.model} is up to date`);
      done();
    }
  }, 1000);
}
