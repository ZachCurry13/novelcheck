// Admin → Usage: NovelCheck's CPU, memory, network and disk, recent history
// charts, AI token use, library progress, and what Ollama has loaded.
import { get } from "./api.js";
import { $, esc } from "./ui.js";
import { lineChart, columnChart } from "./charts.js";

const GB = 1 << 30;
const MB = 1 << 20;
// Sizes under 1 GB read better in MB.
const fmtGB = (b) => (b < GB ? `${Math.round(b / MB)} MB` : `${(b / GB).toFixed(b >= 10 * GB ? 0 : 1)} GB`);

// Meter fill turns amber then red as usage climbs; the label repeats the
// state in words so colour is never the only signal.
function meter(pct, label, progress) {
  const level = progress ? ["bg-indigo-500", "bg-indigo-950", `${Math.round(pct)}% done`] : pct >= 90 ? ["bg-rose-500", "bg-rose-950", "⛔ Very high"]
    : pct >= 75 ? ["bg-amber-400", "bg-amber-950", "⚠️ High"] : ["bg-indigo-500", "bg-indigo-950", "OK"];
  return `<div class="meter-track ${level[1]} mt-3" role="meter" aria-valuemin="0" aria-valuemax="100"
      aria-valuenow="${Math.round(pct)}" aria-label="${esc(label)}" title="${esc(label)}: ${Math.round(pct)}%">
      <div class="meter-fill ${level[0]}" data-w="${Math.max(0, Math.min(100, pct))}"></div></div>
    <p class="mt-1 text-xs text-slate-400">${level[2]}</p>`;
}

function tile(title, value, sub, pct, progress) {
  return `<div class="card"><p class="label">${esc(title)}</p><p class="stat">${esc(value)}</p>
    <p class="text-xs text-slate-400">${esc(sub)}</p>${pct === undefined ? "" : meter(pct, title, progress)}</div>`;
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

const fmtRate = (b) => (b >= MB ? `${(b / MB).toFixed(1)} MB/s` : b >= 1024 ? `${Math.round(b / 1024)} KB/s` : `${Math.round(b)} B/s`);
const fmtNum = (n) => Number(n || 0).toLocaleString();
const clock = (iso) => new Date(iso).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });

function libraryTiles(s) {
  const c = s.counts || {};
  const total = Object.values(c).reduce((a, b) => a + b, 0);
  const done = c.analyzed || 0;
  return [
    tile("Books rated", fmtNum(done), `of ${fmtNum(total)} in the library`, total ? (done / total) * 100 : 0, true),
    tile("Waiting", fmtNum((c.pending || 0) + (c.queued || 0) + (c.processing || 0)), `worker ${s.worker.state}`),
    tile("Errors", fmtNum(c.error || 0), c.error ? "Retry from Admin → Analysis" : "None"),
    tile("AI spend to date", `$${(s.cost_spent || 0).toFixed(2)}`, `${fmtNum(s.usage?.total_calls)} AI calls · ${fmtNum(s.usage?.today_tokens)} tokens today`),
  ].join("");
}

export async function renderUsage(view) {
  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">Usage</h1>
    <p class="mb-4 text-sm text-slate-400">What NovelCheck is using right now. Refreshes every 5 seconds; charts show the last 15 minutes.</p>
    <div id="nc-tiles" class="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"></div>
    <div class="mb-6 grid gap-3 lg:grid-cols-2">
      <section class="card"><h2 class="label">CPU</h2><div id="cpu-chart"></div></section>
      <section class="card"><h2 class="label">Network</h2><div id="net-chart"></div></section>
    </div>
    <h2 class="mb-2 text-lg font-semibold">Library &amp; AI</h2>
    <div id="lib-tiles" class="mb-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"></div>
    <section class="card mb-6"><h2 class="label">AI tokens per day (last 14 days)</h2><div id="token-chart"></div></section>
    <section class="card space-y-3"><h2 class="text-lg font-semibold">Ollama</h2><div id="ollama"></div></section>`;

  async function refresh() {
    const s = await get("/api/admin/system").catch(() => null);
    if (!s) return;
    const n = s.novelcheck;
    const memPct = n.mem_limit ? (n.mem_used / n.mem_limit) * 100 : 0;
    const diskPct = n.disk_total ? ((n.disk_total - n.disk_free) / n.disk_total) * 100 : 0;
    $("#nc-tiles", view).innerHTML = [
      tile("CPU", `${n.cpu_percent.toFixed(1)}%`, `of ${n.cpu_cores} cores · up ${uptime(n.uptime_seconds)}`, n.cpu_percent),
      tile("Memory", fmtGB(n.mem_used), n.mem_is_container_limit ? `of ${fmtGB(n.mem_limit)} app limit` : `of ${fmtGB(n.mem_limit)} server RAM`, memPct),
      tile("Network", `↓ ${fmtRate(n.rx_per_sec)}`, `↑ ${fmtRate(n.tx_per_sec)} upload`),
      tile("Data disk", `${fmtGB(n.disk_free)} free`, `of ${fmtGB(n.disk_total)} · database ${(n.db_size / MB).toFixed(1)} MB`, diskPct),
    ].join("");
    const h = s.history || [];
    const labels = h.map((p) => clock(p.at));
    lineChart($("#cpu-chart", view), {
      labels, max: 10, fmt: (v) => `${v.toFixed(1)}%`,
      series: [{ name: "CPU", cls: "stroke-indigo-500", values: h.map((p) => p.cpu_percent) }],
    });
    lineChart($("#net-chart", view), {
      labels, fmt: fmtRate,
      series: [
        { name: "Download", cls: "stroke-indigo-500", values: h.map((p) => p.rx_per_sec) },
        { name: "Upload", cls: "stroke-teal-600", values: h.map((p) => p.tx_per_sec) },
      ],
    });
    $("#lib-tiles", view).innerHTML = libraryTiles(s);
    const days = s.daily_tokens || [];
    columnChart($("#token-chart", view), days.map((d) => {
      const label = new Date(`${d.day}T12:00:00Z`).toLocaleDateString([], { month: "short", day: "numeric" });
      return { label, value: d.tokens, tip: `${label}: ${fmtNum(d.tokens)} tokens · ${fmtNum(d.calls)} AI calls` };
    }));
    $("#ollama", view).innerHTML = ollamaCard(s.ollama);
    // Widths are set through the DOM (the page's security policy blocks inline styles).
    view.querySelectorAll("[data-w]").forEach((el) => (el.style.width = `${el.dataset.w}%`));
  }

  await refresh();
  const timer = setInterval(refresh, 5000);
  return () => clearInterval(timer);
}
