// Admin control panel: analysis dashboard, cost estimator, maintenance, settings.
import { get, post, put, qs } from "./api.js";
import { $, $$, esc, attempt, toast, fmtNum, fmtMoney } from "./ui.js";
import { renderUsers } from "./users.js";
import { renderCalibrePicker } from "./calibrepicker.js";
import { initProviderPicker } from "./llmpresets.js";
import { renderRemoteAccess } from "./remoteaccess.js";
import { renderCalibreServer } from "./calibreserver.js";

const SECTIONS = [
  ["LLM Analysis Engine", [
    ["llm_provider", "", "", "hidden"],
    ["llm_base_url", "API base URL", "https://api.openai.com/v1 · http://ollama:11434/v1"],
    ["llm_api_key", "API key", "Leave blank for local Ollama", "password"],
    ["llm_model", "Primary (small) model", "gpt-4o-mini · claude-haiku-4-5 · gemini-2.5-flash · sonar · llama3.2"],
    ["llm_fallback_model", "Fallback model(s)", "Only used when the main model fails. Several? Separate with commas, in order"],
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
    ["smtp_username", "SMTP username", "your full email address, e.g. you@gmail.com"],
    ["smtp_password", "SMTP password / app password", "", "password"],
    ["smtp_from", "Approved sender email", "must be on your Amazon approved list"],
  ]],
  ["Calibre Library", [
    ["calibre_poll_hours", "Sync every N hours (0 = manual only)", "6", "number"],
  ]],
  ["Sign-in & Updates", [
    ["session_days", "Keep people signed in for (days, renewed while they use the app)", "30", "number"],
    ["check_updates", "Check GitHub for new NovelCheck versions (shows a banner to admins and editors)", "", "bool"],
  ]],
];

// Admins see everything. Editors get the same dashboard and actions minus
// technical settings, secrets, backups and the destructive queue wipe.
export async function renderAdmin(view, state) {
  const isAdmin = state.user.role === "admin";
  const adminOnly = (html) => (isAdmin ? html : "");
  view.innerHTML = `
    <h1 class="mb-4 text-2xl font-bold">${isAdmin ? "Admin Control Panel" : "Manage NovelCheck"}</h1>
    <div id="stats" class="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"></div>
    <div class="card mb-6 flex flex-wrap items-end gap-3">
      <div><label class="label" for="batch-size">Batch size</label>
        <input id="batch-size" type="number" min="1" max="500" value="20" class="input w-28"></div>
      <button data-act="batch" class="btn-primary">Analyze batch</button>
      ${adminOnly(`<button data-act="wipe" class="btn-secondary">Wipe pending queue</button>`)}
      <button data-act="sync" class="btn-secondary">Sync Calibre now</button>
      ${adminOnly(`<a href="/api/admin/backup" class="btn-secondary" download>Download novelcheck.db</a>`)}
      <p id="worker" class="basis-full text-sm text-slate-400"></p>
      <div id="rerate" class="hidden basis-full rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    </div>
    ${adminOnly(`<form id="settings" class="mb-8 grid gap-4 lg:grid-cols-2"></form><div id="remote-access"></div>`)}
    <section id="users"></section>`;

  if (isAdmin) await renderSettings(view);

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
    } else if (act === "rerate") {
      const r = await attempt(() => post("/api/admin/rerate"));
      if (r) toast(`Re-rating ${r.queued} books on the pepper scale`);
    } else if (act === "sync") {
      await attempt(() => post("/api/admin/calibre-sync"), "Calibre sync started");
    } else if (act === "smtp-test") {
      const to = prompt("Send test email to:");
      // Test exactly what's on screen, even before Save.
      const val = (k) => $(`[data-key="${k}"]`, view)?.value ?? "";
      if (to) {
        await attempt(() => post("/api/admin/smtp-test", {
          to, smtp_host: val("smtp_host"), smtp_port: val("smtp_port"), smtp_username: val("smtp_username"),
          smtp_password: val("smtp_password"), smtp_from: val("smtp_from"),
        }), "Test email sent. If it looks right, click Save settings.");
      }
    }
    refresh();
  });

  async function refresh() {
    const s = await attempt(() => get("/api/admin/status"));
    if (s) renderStats(view, s);
  }
  await refresh();
  if (isAdmin) {
    renderCalibrePicker($("#calibre-picker", view), refresh);
    renderCalibreServer($("#calibre-server", view));
  }
  const timer = setInterval(refresh, 5000);
  const stopRemote = isAdmin ? await renderRemoteAccess($("#remote-access", view)) : null;
  await renderUsers($("#users", view), state.user);
  return () => {
    clearInterval(timer);
    stopRemote?.();
  };
}

async function renderSettings(view) {
  const settings = (await attempt(() => get("/api/admin/settings"))) || {};
  $("#batch-size", view).value = settings.batch_size || 20;
  $("#settings", view).innerHTML = SECTIONS.map(([title, fields]) => `
    <fieldset class="card min-w-0 space-y-3">
      <legend class="px-1 text-lg font-semibold">${esc(title)}</legend>
      ${fields.map(([k, label, ph, type]) => field(k, label, ph, type, settings[k])).join("")}
      ${title.startsWith("SMTP") ? `<button type="button" data-act="smtp-test" class="btn-secondary">Send test email</button>` : ""}
      ${title.startsWith("Calibre") ? `<div id="calibre-picker"></div><div id="calibre-server"></div>` : ""}
      ${title.startsWith("LLM") ? `<div id="llm-preset-host"></div>` : ""}
    </fieldset>`).join("") +
    `<div class="lg:col-span-2"><button class="btn-primary">Save settings</button></div>`;

  // Move the provider menu to the top of the LLM box and wire it up.
  const llmHost = $("#llm-preset-host", view);
  llmHost.parentElement.insertBefore(llmHost, llmHost.parentElement.children[1]);
  initProviderPicker(llmHost, $("#settings", view), settings);

  $("#settings", view).addEventListener("submit", async (e) => {
    e.preventDefault();
    const body = {};
    $$("[data-key]", e.target).forEach((el) => {
      body[el.dataset.key] = el.type === "checkbox" ? String(el.checked) : el.value;
    });
    await attempt(() => put("/api/admin/settings", body), "Settings saved");
  });
}

function field(key, label, placeholder, type, value) {
  if (type === "hidden") return `<input type="hidden" data-key="${key}" value="${esc(value ?? "")}">`;
  if (type === "bool") {
    return `<label class="toggle"><input type="checkbox" data-key="${key}" ${value === "true" ? "checked" : ""}> ${esc(label)}</label>`;
  }
  const t = type === "password" ? "password" : type === "number" ? "number" : "text";
  return `<div><label class="label" for="s-${key}">${esc(label)}</label>
    <input id="s-${key}" data-key="${key}" type="${t}" ${t === "number" ? 'step="any" min="0"' : ""}
      value="${esc(value ?? "")}" placeholder="${esc(placeholder)}" class="input"
      autocomplete="${t === "password" ? "new-password" : "off"}" data-1p-ignore data-lpignore="true">${HELP[key] || ""}</div>`;
}

// Extra help under a few fields.
const HELP = {
  smtp_password: `<p class="mt-1 text-xs text-slate-400">Gmail: your normal Google password won't work. Turn on 2-Step Verification,
    then create an <a href="https://myaccount.google.com/apppasswords" target="_blank" rel="noopener noreferrer" class="underline">App Password</a>
    and paste its 16 letters here (spaces are fine). Use your full Gmail address as the username.</p>`,
};

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
  const box = $("#rerate", view);
  box.classList.toggle("hidden", !s.rerate_candidates);
  if (s.rerate_candidates) {
    // Rough cost from the running average (free with Ollama).
    const est = s.cost_spent && s.usage.total_calls ? (s.cost_spent / s.usage.total_calls) * s.rerate_candidates : 0;
    box.innerHTML = `🌶️ <b>${fmtNum(s.rerate_candidates)}</b> book${s.rerate_candidates === 1 ? " was" : "s were"} rated before the pepper scale.
      <button data-act="rerate" class="btn-secondary ml-2 py-1">Re-rate them on the pepper scale</button>
      <span class="block text-xs text-slate-400">They stay in the library with their old rating until the new one arrives. Hand-rated books are left alone.${est ? ` Estimated cost ≈ ${fmtMoney(est)}.` : ""}</span>`;
  }
  const w = s.worker;
  const sync = s.calibre_last_sync;
  $("#worker", view).innerHTML = `Worker: <b>${esc(w.state)}</b>${w.current_title ? ` — ${esc(w.current_title)}` : ""}
    · ${fmtNum(w.queue_length)} waiting${w.last_error ? ` · <span class="text-rose-400">last error: ${esc(w.last_error)}</span>` : ""}
    <br>Calibre: ${s.calibre_available ? "library found" : "<span class='text-amber-400'>no library selected (see Calibre Library below)</span>"}${sync
      ? ` · last sync ${esc(new Date(sync.at).toLocaleString())} (${fmtNum(sync.result?.books)} books${sync.error ? `, <span class="text-rose-400">${esc(sync.error)}</span>` : ""})` : ""}`;
}
