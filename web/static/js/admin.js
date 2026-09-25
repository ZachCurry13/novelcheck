// Admin control panel: analysis dashboard, cost estimator, maintenance, settings.
import { get, post, put, qs } from "./api.js";
import { $, $$, esc, attempt, toast, fmtNum, fmtMoney } from "./ui.js";
import { renderUsers } from "./users.js";
import { renderCalibrePicker } from "./calibrepicker.js";
import { initProviderPicker } from "./llmpresets.js";
import { renderRemoteAccess } from "./remoteaccess.js";
import { renderCalibreServer } from "./calibreserver.js";
import { renderErrors } from "./errors.js";
import { on } from "./modules.js";
import { renderFlagsAdmin } from "./customflags.js";
import { refreshUser } from "./app.js";

const SECTIONS = [
  ["LLM Analysis Engine", [
    ["llm_provider", "", "", "hidden"],
    ["llm_base_url", "API base URL", "https://api.openai.com/v1 · http://ollama:11434/v1"],
    ["llm_api_key", "API key", "Leave blank for local Ollama", "password"],
    ["llm_model", "Primary (small) model", "gpt-4o-mini · claude-haiku-4-5 · gemini-2.5-flash · sonar · llama3.2"],
    ["llm_fallback_model", "Fallback model(s)", "Only used when the main model fails. Several? Separate with commas, in order"],
    ["llm_json_mode", "JSON response mode", "true / false", "bool"],
    ["deep_read_model", "Deep Scan model (optional)", "Blank = the models above · e.g. a cheaper model with a big context window"],
    ["google_books_api_key", "Google Books API key (optional)", "", "password"],
    ["language", "Language for book summaries", "English (US)|English (UK)|Spanish|French|German|Portuguese|Italian|Dutch", "select"],
  ]],
  ["Backup AI (optional)", [
    ["backup_llm_enabled", "Use a backup AI when the main one fails (a second Ollama, or a cloud AI)", "", "bool"],
    ["backup_llm_provider", "", "", "hidden"],
    ["backup_llm_base_url", "API base URL", "http://192.168.1.60:11434/v1 · https://api.openai.com/v1"],
    ["backup_llm_api_key", "API key", "Leave blank for Ollama", "password"],
    ["backup_llm_model", "Model(s), in order", "e.g. llama3.2, or gpt-4o-mini; several? separate with commas"],
    ["backup_llm_json_mode", "JSON response mode", "", "bool"],
    ["backup_price_input_per_million", "Input price per 1M tokens (USD)", "0 for Ollama", "number"],
    ["backup_price_output_per_million", "Output price per 1M tokens (USD)", "0 for Ollama", "number"],
  ]],
  ["Rate Caps & Batching", [
    ["batch_size", "Books per batch (0 = all waiting books)", "20", "number"],
    ["tokens_per_hour_cap", "Max tokens per hour (0 = unlimited)", "25000", "number"],
    ["scan_delay_seconds", "Delay between scans (seconds)", "2", "number"],
    ["llm_timeout_seconds", "AI time limit per book (seconds)", "0 = automatic: 10 minutes for Ollama, 2 for cloud", "number"],
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
    ["calibre_web_url", "Calibre-Web address (optional)", "http://192.168.1.10:8083 · adds \"Open in Calibre-Web\" to each book"],
  ]],
  ["Sign-in & Updates", [
    ["session_days", "Keep people signed in for (days, renewed while they use the app)", "30", "number"],
    ["check_updates", "Check GitHub for new NovelCheck versions (shows a banner to admins and editors)", "", "bool"],
  ]],
  ["Features", [
    ["module_queue", "Up Next reading queue", "", "bool"],
    ["module_send_to_kindle", "Send-to-Kindle email delivery", "", "bool"],
    ["module_koreader", "KOReader sync delivery", "", "bool"],
    ["module_import", "Import books (Kindle / drive scanner and lists)", "", "bool"],
    ["module_parents", "Parent tools (kids' accounts, age groups, parents' notes)", "", "bool"],
    ["notify_routine", "🔔 Also show everyday events: new books from Calibre, rating finished, someone started a book", "", "bool"],
  ]],
];

// Settings boxes that only matter while a feature is on.
const SECTION_MODULE = { "SMTP / Send-to-Kindle": "send_to_kindle" };

// Admins see everything. Editors get the same dashboard and actions minus
// technical settings, secrets, backups and the destructive queue wipe.
export async function renderAdmin(view, state) {
  const isAdmin = state.user.role === "admin";
  const adminOnly = (html) => (isAdmin ? html : "");
  view.innerHTML = `
    <h1 class="mb-4 text-2xl font-bold">${isAdmin ? "Admin Control Panel" : "Manage NovelCheck"}</h1>
    <div id="stats" class="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"></div>
    <div id="errors"></div>
    <div class="card mb-6 flex flex-wrap items-end gap-3">
      <div><label class="label" for="batch-size">Batch size</label>
        <input id="batch-size" type="number" min="0" max="500" value="20" class="input w-28" title="0 = all waiting books (up to 500)"></div>
      <button data-act="batch" class="btn-primary">Analyze batch</button>
      ${adminOnly(`<button data-act="wipe" class="btn-secondary">Wipe pending queue</button>`)}
      <button data-act="sync" class="btn-secondary">Sync Calibre now</button>
      ${adminOnly(`<a href="#/deepscan" class="btn-secondary">🧬 Deep Scan…</a>`)}
      <button data-act="rerate-all" class="btn-ghost" title="Rate every AI-rated book again, e.g. after changing the AI or its rules">Re-rate whole library…</button>
      ${adminOnly(`<a href="/api/admin/backup" class="btn-secondary" download>Download novelcheck.db</a>`)}
      <p id="worker" class="basis-full text-sm text-slate-400"></p>
      <div id="rerate" class="hidden basis-full rounded-lg bg-slate-800/60 p-3 text-sm"></div>
      <div id="del-banner" class="hidden basis-full rounded-lg bg-rose-950/50 p-3 text-sm"></div>
      <div id="deep-banner" class="hidden basis-full rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    </div>
    ${adminOnly(`<div id="custom-flags"></div><form id="settings" class="mb-8 grid gap-4 lg:grid-cols-2"></form><div id="remote-access"></div>`)}
    <section id="users"></section>`;

  if (isAdmin) await renderSettings(view, state);

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
    } else if (act === "rerate-language") {
      const r = await attempt(() => post("/api/admin/rerate", { which: "language" }));
      if (r) toast(`Re-rating ${r.queued} books so their summaries are in English`);
    } else if (act === "rerate") {
      const r = await attempt(() => post("/api/admin/rerate"));
      if (r) toast(`Re-rating ${r.queued} books with the current pepper rules`);
    } else if (act === "rerate-all") {
      const s = await attempt(() => get("/api/admin/status"));
      if (!s) return;
      const est = perBookCost(s) * s.ai_rated;
      if (!confirm(`Re-rate all ${fmtNum(s.ai_rated)} AI-rated books?${est ? ` Estimated cost ≈ ${fmtMoney(est)}.` : ""}\n\nThey stay in the library with their current rating until the new one arrives. Hand-rated books are left alone.`)) return;
      const r = await attempt(() => post("/api/admin/rerate", { which: "all" }));
      if (r) toast(`Re-rating ${r.queued} books`);
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
  renderErrors($("#errors", view), isAdmin, refresh);
  if (isAdmin) {
    renderFlagsAdmin($("#custom-flags", view), refresh);
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

async function renderSettings(view, state) {
  const settings = (await attempt(() => get("/api/admin/settings"))) || {};
  $("#batch-size", view).value = settings.batch_size || 20;
  $("#settings", view).innerHTML = SECTIONS.map(([title, fields]) => `
    <fieldset class="card min-w-0 space-y-3${on(state.user, SECTION_MODULE[title]) ? "" : " module-off"}" ${SECTION_MODULE[title] ? `data-module="${SECTION_MODULE[title]}"` : ""}>
      <legend class="px-1 text-lg font-semibold">${esc(title)}</legend>
      ${title === "Features" ? `<p class="text-xs text-slate-400">Turn off what your family doesn't use. It disappears for everyone; kids' content rules always keep applying.</p>` : ""}
      ${fields.map(([k, label, ph, type]) => field(k, label, ph, type, settings[k])).join("")}
      ${title.startsWith("SMTP") ? `<button type="button" data-act="smtp-test" class="btn-secondary">Send test email</button>` : ""}
      ${title.startsWith("Calibre") ? `<div id="calibre-picker"></div><div id="calibre-server"></div>` : ""}
      ${title.startsWith("LLM") ? `<div id="llm-preset-host"></div>` : ""}
      ${title.startsWith("Backup") ? `<p id="backup-note" class="text-xs text-slate-400">Tried only when the main AI fails on a book. If the main server is switched off, NovelCheck goes straight to the backup. You'll get a 🔔 notice when the backup is used.</p><div id="backup-preset-host"></div>` : ""}
    </fieldset>`).join("") +
    `<div class="lg:col-span-2"><button class="btn-primary">Save settings</button></div>`;

  // Move the provider menu to the top of the LLM box and wire it up.
  const llmHost = $("#llm-preset-host", view);
  llmHost.parentElement.insertBefore(llmHost, llmHost.parentElement.children[1]);
  initProviderPicker(llmHost, $("#settings", view), settings);
  // Same menu for the backup AI, placed under its on/off switch.
  const backupHost = $("#backup-preset-host", view);
  backupHost.parentElement.insertBefore(backupHost, backupHost.parentElement.children[2]);
  backupHost.parentElement.insertBefore($("#backup-note", view), backupHost);
  initProviderPicker(backupHost, $("#settings", view), settings, "backup_");

  $("#settings", view).addEventListener("submit", async (e) => {
    e.preventDefault();
    const body = {};
    $$("[data-key]", e.target).forEach((el) => {
      body[el.dataset.key] = el.type === "checkbox" ? String(el.checked) : el.value;
    });
    if (await attempt(() => put("/api/admin/settings", body), "Settings saved")) await refreshUser(); // shows/hides features right away
  });
}

function field(key, label, placeholder, type, value) {
  if (type === "hidden") return `<input type="hidden" data-key="${key}" value="${esc(value ?? "")}">`;
  if (type === "bool") {
    return `<label class="toggle"><input type="checkbox" data-key="${key}" ${value === "true" ? "checked" : ""}> ${esc(label)}</label>`;
  }
  if (type === "select") {
    // placeholder holds the choices, separated by "|"; the first is the default.
    const opts = placeholder.split("|");
    return `<div><label class="label" for="s-${key}">${esc(label)}</label>
      <select id="s-${key}" data-key="${key}" class="input">${opts.map((o) => `<option ${o === (value || opts[0]) ? "selected" : ""}>${esc(o)}</option>`).join("")}</select></div>`;
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

// Rough cost of rating one book, from the running average (free with Ollama).
const perBookCost = (s) => (s.cost_spent && s.usage.total_calls ? s.cost_spent / s.usage.total_calls : 0);

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
  const deepBox = $("#deep-banner", view);
  deepBox.classList.toggle("hidden", !(s.pending_deep && document.body.dataset.role === "admin"));
  if (s.pending_deep) {
    deepBox.innerHTML = `🧬 <b>${fmtNum(s.pending_deep)}</b> Deep Scan request${s.pending_deep === 1 ? " is" : "s are"} waiting for your approval. <a href="#/deepscan" class="ml-2 underline">Review</a>`;
  }
  const delBox = $("#del-banner", view);
  delBox.classList.toggle("hidden", !(s.pending_deletes && document.body.dataset.role === "admin"));
  if (s.pending_deletes) {
    delBox.innerHTML = `🗑 <b>${fmtNum(s.pending_deletes)}</b> book${s.pending_deletes === 1 ? " is" : "s are"} waiting for your delete review.
      <a href="#/deletions" class="ml-2 underline">Review</a>`;
  }
  const box = $("#rerate", view);
  box.classList.toggle("hidden", !s.rerate_candidates && !s.non_english);
  box.innerHTML = "";
  if (s.non_english) {
    box.innerHTML = `🌐 <b>${fmtNum(s.non_english)}</b> book summar${s.non_english === 1 ? "y isn't" : "ies aren't"} in English.
      <button data-act="rerate-language" class="btn-secondary ml-2 py-1">Re-rate them in English</button>
      <span class="block text-xs text-slate-400">The AI rewrites them in the language chosen under LLM Analysis Engine. They stay in the library meanwhile.</span>`;
  }
  if (s.rerate_candidates) {
    const est = perBookCost(s) * s.rerate_candidates;
    box.innerHTML += `${s.non_english ? `<hr class="my-2 border-slate-700">` : ""}🌶️ <b>${fmtNum(s.rerate_candidates)}</b> book${s.rerate_candidates === 1 ? " was" : "s were"} ${s.changed_books ? `changed in Calibre since they were rated (${fmtNum(s.changed_books)}), or were ` : ""}rated before your latest rule changes (pepper wording or custom filters).
      <button data-act="rerate" class="btn-secondary ml-2 py-1">Re-rate with the current rules</button>
      <span class="block text-xs text-slate-400">They stay in the library with their old rating until the new one arrives. Hand-rated books are left alone.${est ? ` Estimated cost ≈ ${fmtMoney(est)}.` : ""}</span>`;
  }
  const w = s.worker;
  const sync = s.calibre_last_sync;
  $("#worker", view).innerHTML = `Worker: <b>${esc(w.state)}</b>${w.current_title ? ` — ${esc(w.current_title)}` : ""}
    · ${fmtNum(w.queue_length)} waiting${w.last_error ? ` · <span class="text-rose-400">last error: ${esc(w.last_error)}</span>` : ""}
    <br>Calibre: ${s.calibre_available ? "library found" : "<span class='text-amber-400'>no library selected (see Calibre Library below)</span>"}${sync
      ? ` · last sync ${esc(new Date(sync.at).toLocaleString())} (${fmtNum(sync.result?.books)} books${sync.error ? `, <span class="text-rose-400">${esc(sync.error)}</span>` : ""})` : ""}`;
}
