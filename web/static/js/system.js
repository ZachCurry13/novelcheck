// Admin → System: a "Check everything" button that tests every connection
// NovelCheck uses. Resource use lives on the Usage page.
import { post } from "./api.js";
import { $, esc, attempt } from "./ui.js";
import { renderDiagnose } from "./diagnose.js";
import { copyBar, bindCopy } from "./copy.js";

const STATUS = {
  ok: ["✓", "OK", "text-emerald-300"],
  warn: ["⚠️", "Warning", "text-amber-300"],
  error: ["⛔", "Problem", "text-rose-300"],
  skip: ["–", "Skipped", "text-slate-400"],
};

function checksTable(res) {
  return `<ul class="divide-y divide-slate-800">${res.results.map((r) => {
    const [icon, label, cls] = STATUS[r.status] || STATUS.skip;
    return `<li class="flex gap-3 py-3 text-sm"><span class="${cls} w-5" aria-hidden="true">${icon}</span>
      <div class="min-w-0 flex-1"><p><b>${esc(r.name)}</b> · <span class="${cls}">${label}</span>
        <span class="text-xs text-slate-500">(${r.ms} ms)</span></p>
        <p class="text-slate-300">${esc(r.message)}</p>
        ${r.fix ? `<p class="text-xs text-slate-400">💡 ${esc(r.fix)}${r.link ? ` <a href="${esc(r.link)}" class="underline">Go there</a>` : ""}</p>` : ""}
      </div></li>`;
  }).join("")}</ul><p class="mt-2 text-xs text-slate-500">Checked ${esc(new Date(res.checked_at).toLocaleString())}. Problems also appear under the 🔔 bell.</p>`;
}

export async function renderSystem(view) {
  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">System</h1>
    <p class="mb-4 text-sm text-slate-400">Check that every service NovelCheck talks to is working.</p>
    <div id="diagnose"></div>
    <section class="card mb-6">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-lg font-semibold">Connections</h2>
        <span class="flex flex-wrap gap-2"><span id="checks-copy" class="hidden">${copyBar("checks-text", "novelcheck-checks.txt")}</span>
        <button id="run-checks" class="btn-primary">Check everything</button></span>
      </div>
      <div id="checks"><p class="text-sm text-slate-400">Tests your AI provider, Google Books, Open Library, Calibre, email, remote access, Ollama and updates. The AI check uses a few tokens (a tiny fraction of a cent).</p></div>
    </section>
    <p class="text-sm text-slate-400">Looking for CPU, memory, network or AI token use? See <a href="#/usage" class="underline">Usage</a>.</p>`;

  renderDiagnose($("#diagnose", view));
  let lastChecks = "";
  bindCopy($("#checks-copy", view), { "checks-text": () => lastChecks });
  $("#run-checks", view).addEventListener("click", async (e) => {
    e.target.disabled = true;
    e.target.textContent = "Checking…";
    const res = await attempt(() => post("/api/admin/health"));
    e.target.disabled = false;
    e.target.textContent = "Check everything";
    if (res) {
      $("#checks", view).innerHTML = checksTable(res);
      lastChecks = res.results.map((r) => `${(STATUS[r.status] || STATUS.skip)[1]}: ${r.name}: ${r.message}${r.fix ? `\n   fix: ${r.fix}` : ""}`).join("\n");
      $("#checks-copy", view).classList.remove("hidden");
    }
  });

}
