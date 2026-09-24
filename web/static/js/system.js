// Admin → System: NovelCheck's resource use, Ollama's loaded models, and a
// "Check everything" button that tests every connection NovelCheck uses.
import { get, post } from "./api.js";
import { $, esc, attempt } from "./ui.js";

const GB = 1 << 30;
const MB = 1 << 20;
// Sizes under 1 GB read better in MB.
const fmtGB = (b) => (b < GB ? `${Math.round(b / MB)} MB` : `${(b / GB).toFixed(b >= 10 * GB ? 0 : 1)} GB`);

// Meter fill turns amber then red as usage climbs; the label repeats the
// state in words so colour is never the only signal.
function meter(pct, label) {
  const level = pct >= 90 ? ["bg-rose-500", "bg-rose-950", "⛔ Very high"]
    : pct >= 75 ? ["bg-amber-400", "bg-amber-950", "⚠️ High"] : ["bg-indigo-500", "bg-indigo-950", "OK"];
  return `<div class="meter-track ${level[1]} mt-3" role="meter" aria-valuemin="0" aria-valuemax="100"
      aria-valuenow="${Math.round(pct)}" aria-label="${esc(label)}" title="${esc(label)}: ${Math.round(pct)}%">
      <div class="meter-fill ${level[0]}" data-w="${Math.max(0, Math.min(100, pct))}"></div></div>
    <p class="mt-1 text-xs text-slate-400">${level[2]}</p>`;
}

function tile(title, value, sub, pct) {
  return `<div class="card"><p class="label">${esc(title)}</p><p class="stat">${esc(value)}</p>
    <p class="text-xs text-slate-400">${esc(sub)}</p>${pct === undefined ? "" : meter(pct, title)}</div>`;
}

function uptime(s) {
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
  return d ? `${d}d ${h}h` : h ? `${h}h ${m}m` : `${m}m`;
}

function ollamaCard(o) {
  if (!o) {
    return `<p class="text-sm text-slate-400">Not using Ollama. Pick it as the AI provider in Admin to see its usage here.</p>`;
  }
  if (o.error && !o.version) return `<p class="text-sm text-rose-300">⛔ ${esc(o.error)}</p>`;
  const loaded = o.loaded || [];
  const rows = loaded.map((m) => {
    const gpu = m.vram_bytes, ram = Math.max(0, m.size_bytes - m.vram_bytes), total = m.size_bytes || 1;
    const where = gpu >= total ? "100% GPU" : gpu === 0 ? "100% CPU/RAM" : `${Math.round((gpu / total) * 100)}% GPU`;
    return `<li class="space-y-1">
      <div class="flex flex-wrap justify-between gap-2 text-sm"><code>${esc(m.name)}</code>
        <span class="text-slate-300">${fmtGB(gpu)} on GPU · ${fmtGB(ram)} in RAM · <b>${where}</b></span></div>
      <div class="flex h-3 w-full gap-[2px] overflow-hidden rounded-full bg-slate-800" title="${esc(m.name)}: ${fmtGB(gpu)} GPU, ${fmtGB(ram)} RAM">
        <div class="meter-fill rounded-none bg-indigo-500" data-w="${(gpu / total) * 100}"></div>
        <div class="meter-fill rounded-none bg-teal-600" data-w="${(ram / total) * 100}"></div>
      </div></li>`;
  }).join("");
  return `
    <p class="text-sm text-slate-300">Ollama ${esc(o.version || "")} at <code>${esc(o.url)}</code> · ${(o.downloaded || []).length} models downloaded</p>
    <div class="flex gap-4 text-xs text-slate-400">
      <span class="flex items-center gap-1"><span class="inline-block h-2 w-3 rounded-sm bg-indigo-500"></span> GPU memory</span>
      <span class="flex items-center gap-1"><span class="inline-block h-2 w-3 rounded-sm bg-teal-600"></span> System RAM</span>
    </div>
    <ul class="space-y-3">${rows || `<li class="text-sm text-slate-400">No model loaded right now. Ollama loads one when books are being rated and unloads it after a few idle minutes.</li>`}</ul>
    <p class="text-xs text-slate-500">Ollama doesn't report live CPU or GPU load. See the TrueNAS <b>Dashboard</b> (or <b>Apps → ollama</b>) for those.</p>`;
}

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
    <p class="mb-4 text-sm text-slate-400">How NovelCheck and its connections are doing. Refreshes every 5 seconds.</p>
    <section class="card mb-6">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-lg font-semibold">Connections</h2>
        <button id="run-checks" class="btn-primary">Check everything</button>
      </div>
      <div id="checks"><p class="text-sm text-slate-400">Tests your AI provider, Google Books, Open Library, Calibre, email, remote access, Ollama and updates. The AI check uses a few tokens (a tiny fraction of a cent).</p></div>
    </section>
    <h2 class="mb-2 text-lg font-semibold">NovelCheck</h2>
    <div id="nc-tiles" class="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"></div>
    <section class="card space-y-3"><h2 class="text-lg font-semibold">Ollama</h2><div id="ollama"></div></section>`;

  async function refresh() {
    const s = await get("/api/admin/system").catch(() => null);
    if (!s) return;
    const n = s.novelcheck;
    const memPct = n.mem_limit ? (n.mem_used / n.mem_limit) * 100 : 0;
    const diskPct = n.disk_total ? ((n.disk_total - n.disk_free) / n.disk_total) * 100 : 0;
    $("#nc-tiles", view).innerHTML = [
      tile("CPU", `${n.cpu_percent.toFixed(1)}%`, `of ${n.cpu_cores} cores · worker ${s.worker.state}`, n.cpu_percent),
      tile("Memory", fmtGB(n.mem_used), n.mem_is_container_limit ? `of ${fmtGB(n.mem_limit)} app limit` : `of ${fmtGB(n.mem_limit)} server RAM`, memPct),
      tile("Data disk", `${fmtGB(n.disk_free)} free`, `of ${fmtGB(n.disk_total)}`, diskPct),
      tile("Database", `${(n.db_size / (1 << 20)).toFixed(1)} MB`, `up ${uptime(n.uptime_seconds)}`),
    ].join("");
    $("#ollama", view).innerHTML = ollamaCard(s.ollama);
    // Widths are set through the DOM (the page's security policy blocks inline styles).
    view.querySelectorAll("[data-w]").forEach((el) => (el.style.width = `${el.dataset.w}%`));
  }

  $("#run-checks", view).addEventListener("click", async (e) => {
    e.target.disabled = true;
    e.target.textContent = "Checking…";
    const res = await attempt(() => post("/api/admin/health"));
    e.target.disabled = false;
    e.target.textContent = "Check everything";
    if (res) $("#checks", view).innerHTML = checksTable(res);
  });

  await refresh();
  const timer = setInterval(refresh, 5000);
  return () => clearInterval(timer);
}
