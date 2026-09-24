// Admin control panel: analysis dashboard, cost estimator, maintenance, settings.
import { get, post, put, qs } from "./api.js";
import { $, $$, esc, attempt, toast, fmtNum, fmtMoney } from "./ui.js";
import { renderUsers } from "./users.js";
import { renderCalibrePicker } from "./calibrepicker.js";

const SECTIONS = [
  ["LLM Analysis Engine", [
    ["llm_base_url", "OpenAI-compatible base URL", "https://api.openai.com/v1 · http://ollama:11434/v1"],
    ["llm_api_key", "API key", "Leave blank for local Ollama", "password"],
    ["llm_model", "Primary (small) model", "gpt-4o-mini · gemini-1.5-flash · llama3.2"],
    ["llm_fallback_model", "Fallback (large) model", "Only used when the small model fails"],
    ["llm_json_mode", "JSON response mode", "true / false", "bool"],
    ["google_books_api_key", "Google Books API key (optional)", "", "password"],
  ]],
  ["Rate Caps & Batching", [
    ["batch_size", "Books per batch", "20", "number"],
    ["tokens_per_hour_cap", "Max tokens per hour (0 = unlimited)", "25000", "number"],
    ["scan_delay_seconds", "Delay between scans (seconds)", "2", "number"],
    ["price_input_per_million", "Input price per 1M tokens (USD)", "0.15", "number"],
    ["price_output_per_million", "Output price per 1M tokens (USD)", "0.60", "number"],
  ]],
  ["SMTP / Send-to-Kindle", [
    ["smtp_host", "SMTP host", "smtp.gmail.com"],
    ["smtp_port", "SMTP port", "587 (STARTTLS) or 465 (TLS)", "number"],
    ["smtp_username", "SMTP username", ""],
    ["smtp_password", "SMTP password / app password", "", "password"],
    ["smtp_from", "Approved sender email", "must be on your Amazon approved list"],
  ]],
  ["Calibre Library", [
    ["calibre_poll_hours", "Sync every N hours (0 = manual only)", "6", "number"],
  ]],
];

export async function renderAdmin(view) {
  view.innerHTML = `
    <h1 class="mb-4 text-2xl font-bold">Admin Control Panel</h1>
    <div id="stats" class="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"></div>
    <div class="card mb-6 flex flex-wrap items-end gap-3">
      <div><label class="label" for="batch-size">Batch size</label>
        <input id="batch-size" type="number" min="1" max="500" class="input w-28"></div>
      <button data-act="batch" class="btn-primary">Analyze batch</button>
      <button data-act="wipe" class="btn-secondary">Wipe pending queue</button>
      <button data-act="sync" class="btn-secondary">Sync Calibre now</button>
      <a href="/api/admin/backup" class="btn-secondary" download>Download novelcheck.db</a>
      <p id="worker" class="basis-full text-sm text-slate-400"></p>
    </div>
    <form id="settings" class="mb-8 grid gap-4 lg:grid-cols-2"></form>
    <section id="users"></section>`;

  const settings = (await attempt(() => get("/api/admin/settings"))) || {};
  $("#batch-size", view).value = settings.batch_size || 20;
  $("#settings", view).innerHTML = SECTIONS.map(([title, fields]) => `
    <fieldset class="card space-y-3">
      <legend class="px-1 text-lg font-semibold">${esc(title)}</legend>
      ${fields.map(([k, label, ph, type]) => field(k, label, ph, type, settings[k])).join("")}
      ${title.startsWith("SMTP") ? `<button type="button" data-act="smtp-test" class="btn-secondary">Send test email</button>` : ""}
      ${title.startsWith("Calibre") ? `<div id="calibre-picker"></div>` : ""}
    </fieldset>`).join("") +
    `<div class="lg:col-span-2"><button class="btn-primary">Save settings</button></div>`;

  $("#settings", view).addEventListener("submit", async (e) => {
    e.preventDefault();
    const body = {};
    $$("[data-key]", e.target).forEach((el) => {
      body[el.dataset.key] = el.type === "checkbox" ? String(el.checked) : el.value;
    });
    await attempt(() => put("/api/admin/settings", body), "Settings saved");
  });

  view.addEventListener("click", async (e) => {
    const act = e.target.closest("[data-act]")?.dataset.act;
    if (!act) return;
    if (act === "batch") {
      const r = await attempt(() => post("/api/admin/analyze-batch" + qs({ size: $("#batch-size", view).value })));
      if (r) toast(`Queued ${r.queued} books`);
    } else if (act === "wipe") {
      if (!confirm("Return all queued books to Pending Analysis?")) return;
      const r = await attempt(() => post("/api/admin/wipe-queue"));
      if (r) toast(`Reset ${r.reset} books to pending`);
    } else if (act === "sync") {
      await attempt(() => post("/api/admin/calibre-sync"), "Calibre sync started");
    } else if (act === "smtp-test") {
      const to = prompt("Send test email to:");
      if (to) await attempt(() => post("/api/admin/smtp-test", { to }), "Test email sent");
    }
    refresh();
  });

  async function refresh() {
    const s = await attempt(() => get("/api/admin/status"));
    if (s) renderStats(view, s);
  }
  await refresh();
  renderCalibrePicker($("#calibre-picker", view), refresh);
  const timer = setInterval(refresh, 5000);
  await renderUsers($("#users", view));
  return () => clearInterval(timer);
}

function field(key, label, placeholder, type, value) {
  if (type === "bool") {
    return `<label class="toggle"><input type="checkbox" data-key="${key}" ${value === "true" ? "checked" : ""}> ${esc(label)}</label>`;
  }
  const t = type === "password" ? "password" : type === "number" ? "number" : "text";
  return `<div><label class="label" for="s-${key}">${esc(label)}</label>
    <input id="s-${key}" data-key="${key}" type="${t}" ${t === "number" ? 'step="any" min="0"' : ""}
      value="${esc(value ?? "")}" placeholder="${esc(placeholder)}" class="input" autocomplete="off"></div>`;
}

function renderStats(view, s) {
  const c = s.counts;
  const cap = s.tokens_per_hour || 0;
  const pct = cap ? Math.min(100, Math.round((s.usage.last_hour_tokens / cap) * 100)) : 0;
  const tile = (label, value, sub = "") => `<div class="card"><p class="label">${esc(label)}</p>
    <p class="stat">${value}</p>${sub ? `<p class="text-xs text-slate-500">${sub}</p>` : ""}</div>`;
  $("#stats", view).innerHTML = [
    tile("Analyzed", fmtNum(c.analyzed), `${fmtNum(c.pending)} pending · ${fmtNum(c.queued + c.processing)} queued · ${fmtNum(c.error)} errors`),
    tile("Tokens this hour", fmtNum(s.usage.last_hour_tokens), cap ? `${pct}% of ${fmtNum(cap)} cap` : "No hourly cap"),
    tile("Spent to date", fmtMoney(s.cost_spent), `${fmtNum(s.usage.total_prompt_tokens + s.usage.total_completion_tokens)} tokens · ${fmtNum(s.usage.total_calls)} calls`),
    tile("Est. to finish library", fmtMoney(s.cost_projected), `≈ ${fmtNum(s.tokens_projected)} tokens remaining`),
  ].join("");
  const w = s.worker;
  const sync = s.calibre_last_sync;
  $("#worker", view).innerHTML = `Worker: <b>${esc(w.state)}</b>${w.current_title ? ` — ${esc(w.current_title)}` : ""}
    · ${fmtNum(w.queue_length)} waiting${w.last_error ? ` · <span class="text-rose-400">last error: ${esc(w.last_error)}</span>` : ""}
    <br>Calibre: ${s.calibre_available ? "library found" : "<span class='text-amber-400'>no library selected (see Calibre Library below)</span>"}${sync
      ? ` · last sync ${esc(new Date(sync.at).toLocaleString())} (${fmtNum(sync.result?.books)} books${sync.error ? `, <span class="text-rose-400">${esc(sync.error)}</span>` : ""})` : ""}`;
}
