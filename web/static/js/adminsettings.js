// Admin settings, grouped by Admin tab into collapsible cards. Fields tied to
// a switch (for example the backup AI's details) stay hidden until it's on.
import { put } from "./api.js";
import { $, $$, esc, attempt } from "./ui.js";
import { initProviderPicker } from "./llmpresets.js";
import { on } from "./modules.js";
import { refreshUser } from "./app.js";

// [tab, title, fields]; a field is [key, label, placeholder/choices, type, shown only while this switch is on].
const SECTIONS = [
  ["ai", "LLM Analysis Engine", [
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
  ["ai", "Backup AI (optional)", [
    ["backup_llm_enabled", "Use a backup AI when the main one fails (a second Ollama, or a cloud AI)", "", "bool"],
    ["backup_llm_provider", "", "", "hidden"],
    ["backup_llm_base_url", "API base URL", "http://192.168.1.60:11434/v1 · https://api.openai.com/v1", "text", "backup_llm_enabled"],
    ["backup_llm_api_key", "API key", "Leave blank for Ollama", "password", "backup_llm_enabled"],
    ["backup_llm_model", "Model(s), in order", "e.g. llama3.2, or gpt-4o-mini; several? separate with commas", "text", "backup_llm_enabled"],
    ["backup_llm_json_mode", "JSON response mode", "", "bool", "backup_llm_enabled"],
    ["backup_price_input_per_million", "Input price per 1M tokens (USD)", "0 for Ollama", "number", "backup_llm_enabled"],
    ["backup_price_output_per_million", "Output price per 1M tokens (USD)", "0 for Ollama", "number", "backup_llm_enabled"],
  ]],
  ["ai", "Suggested Reads", [
    ["suggest_mode", "How suggestions under Up Next are picked", "AI picks + books you don't own|AI picks from your library|Free matching only (no AI)", "select"],
  ]],
  ["ai", "Rate Caps & Batching", [
    ["batch_size", "Books per batch (0 = all waiting books)", "20", "number"],
    ["tokens_per_hour_cap", "Max tokens per hour (0 = unlimited)", "25000", "number"],
    ["scan_delay_seconds", "Delay between scans (seconds)", "2", "number"],
    ["llm_timeout_seconds", "AI time limit per book (seconds)", "0 = automatic: 10 minutes for Ollama, 2 for cloud", "number"],
    ["price_input_per_million", "Input price per 1M tokens (USD)", "0.15", "number"],
    ["price_output_per_million", "Output price per 1M tokens (USD)", "0.60", "number"],
  ]],
  ["delivery", "SMTP / Send-to-Kindle", [
    ["smtp_host", "SMTP host", "smtp.gmail.com"],
    ["smtp_port", "SMTP port", "587 (STARTTLS) or 465 (TLS)", "number"],
    ["smtp_username", "SMTP username", "your full email address, e.g. you@gmail.com"],
    ["smtp_password", "SMTP password / app password", "", "password"],
    ["smtp_from", "Approved sender email", "must be on your Amazon approved list"],
  ]],
  ["delivery", "Calibre Library", [
    ["calibre_poll_hours", "Sync every N hours (0 = manual only)", "6", "number"],
    ["calibre_web_url", "Calibre-Web address (optional)", "http://192.168.1.10:8083 · adds \"Open in Calibre-Web\" to each book"],
  ]],
  ["system", "Features", [
    ["module_queue", "Up Next reading queue", "", "bool"],
    ["module_send_to_kindle", "Send-to-Kindle email delivery", "", "bool"],
    ["module_koreader", "KOReader catalog delivery", "", "bool"],
    ["module_import", "Import books (Kindle / drive scanner and lists)", "", "bool"],
    ["module_parents", "Parent tools (kids' accounts, age groups, parents' notes)", "", "bool"],
    ["module_suggestions", "💡 Suggested Reads under Up Next", "", "bool"],
    ["notify_routine", "🔔 Also show everyday events: new books from Calibre, rating finished, someone started a book", "", "bool"],
  ]],
  ["system", "Sign-in & Updates", [
    ["session_days", "Keep people signed in for (days, renewed while they use the app)", "30", "number"],
    ["check_updates", "Check GitHub for new NovelCheck versions (shows a banner to admins and editors)", "", "bool"],
  ]],
];

// Cards that only matter while a feature is on.
const SECTION_MODULE = { "SMTP / Send-to-Kindle": "send_to_kindle" };

const EXTRA = {
  "LLM Analysis Engine": `<div id="llm-preset-host"></div>`,
  "Backup AI (optional)": `<p class="text-xs text-slate-400" data-when="backup_llm_enabled">Tried only when the main AI fails on a book. If the main server is switched off, NovelCheck goes straight to the backup. You'll get a 🔔 notice when the backup is used.</p><div id="backup-preset-host" data-when="backup_llm_enabled"></div>`,
  "SMTP / Send-to-Kindle": `<button type="button" data-act="smtp-test" class="btn-secondary">Send test email</button>`,
  "Calibre Library": `<div id="calibre-picker"></div><div id="calibre-server"></div>`,
  Features: `<p class="text-xs text-slate-400">Turn off what your family doesn't use. It disappears for everyone; kids' content rules always keep applying.</p>`,
};

// renderSettingsTab puts one tab's settings (as collapsible cards) into host.
export function renderSettingsTab(host, tab, settings, user) {
  const sections = SECTIONS.filter(([t]) => t === tab);
  if (!sections.length) return;
  const form = document.createElement("form");
  form.className = "space-y-3";
  form.innerHTML = sections.map(([, title, fields], i) => `
    <details class="card min-w-0${on(user, SECTION_MODULE[title]) ? "" : " module-off"}" ${SECTION_MODULE[title] ? `data-module="${SECTION_MODULE[title]}"` : ""} ${i === 0 ? "open" : ""}>
      <summary class="cursor-pointer text-lg font-semibold">${esc(title)}</summary>
      <div class="mt-3 space-y-3">${title === "Features" ? EXTRA[title] : ""}
        ${fields.map(([k, label, ph, type, when]) => field(k, label, ph, type, settings[k], when)).join("")}
        ${title !== "Features" ? EXTRA[title] || "" : ""}</div>
    </details>`).join("") + `<button class="btn-primary">Save settings</button>`;
  host.append(form);

  const llmHost = $("#llm-preset-host", form);
  if (llmHost) {
    llmHost.parentElement.prepend(llmHost); // provider menu first
    initProviderPicker(llmHost, form, settings);
  }
  const backupHost = $("#backup-preset-host", form);
  if (backupHost) {
    // Under the on/off switch: the note, then the provider menu.
    $('[data-key="backup_llm_enabled"]', form).closest("label").after(backupHost.previousElementSibling, backupHost);
    initProviderPicker(backupHost, form, settings, "backup_");
  }
  const disclose = () => $$("[data-when]", form).forEach((el) => {
    el.classList.toggle("hidden", !$(`[data-key="${el.dataset.when}"]`, form)?.checked);
  });
  form.addEventListener("change", disclose);
  disclose();
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const body = {};
    $$("[data-key]", form).forEach((el) => (body[el.dataset.key] = el.type === "checkbox" ? String(el.checked) : el.value));
    if (await attempt(() => put("/api/admin/settings", body), "Settings saved")) await refreshUser(); // shows/hides features right away
  });
}

function field(key, label, placeholder, type, value, when) {
  const wrap = (html) => (when ? `<div data-when="${when}">${html}</div>` : html);
  if (type === "hidden") return `<input type="hidden" data-key="${key}" value="${esc(value ?? "")}">`;
  if (type === "bool") {
    return wrap(`<label class="toggle"><input type="checkbox" data-key="${key}" ${value === "true" ? "checked" : ""}> ${esc(label)}</label>`);
  }
  if (type === "select") {
    const opts = placeholder.split("|"); // the choices; the first is the default
    return wrap(`<div><label class="label" for="s-${key}">${esc(label)}</label>
      <select id="s-${key}" data-key="${key}" class="input">${opts.map((o) => `<option ${o === (value || opts[0]) ? "selected" : ""}>${esc(o)}</option>`).join("")}</select>${HELP[key] || ""}</div>`);
  }
  const t = type === "password" ? "password" : type === "number" ? "number" : "text";
  return wrap(`<div><label class="label" for="s-${key}">${esc(label)}</label>
    <input id="s-${key}" data-key="${key}" type="${t}" ${t === "number" ? 'step="any" min="0"' : ""}
      value="${esc(value ?? "")}" placeholder="${esc(placeholder)}" class="input"
      autocomplete="${t === "password" ? "new-password" : "off"}" data-1p-ignore data-lpignore="true">${HELP[key] || ""}</div>`);
}

// Extra help under a few fields.
const HELP = {
  suggest_mode: `<p class="mt-1 text-xs text-slate-400">Free matching looks for the next book in a series, the same authors and similar descriptions. With AI, your AI then picks the best 10 with a reason, at most once a day per person and within the hourly token cap. Kids' accounts (and anyone hiding unrated books) never get books you don't own.</p>`,
  smtp_password: `<p class="mt-1 text-xs text-slate-400">Gmail: your normal Google password won't work. Turn on 2-Step Verification,
    then create an <a href="https://myaccount.google.com/apppasswords" target="_blank" rel="noopener noreferrer" class="underline">App Password</a>
    and paste its 16 letters here (spaces are fine). Use your full Gmail address as the username.</p>`,
};
