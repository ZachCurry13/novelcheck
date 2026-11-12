// The Admin settings, card by card (adminsettings.js draws them).
// [tab, title, fields]; a field is [key, label, placeholder/choices, type, shown only while this switch is on,
// "adv" = under the card's Advanced line].
export const SECTIONS = [
  ["ai", "Your AI", [
    ["llm_provider", "", "", "hidden"],
    ["llm_base_url", "Address", "https://api.openai.com/v1 · http://ollama:11434/v1"],
    ["llm_api_key", "API key", "Leave blank for local Ollama", "password"],
    ["llm_model", "Model that rates books", "gpt-4o-mini · claude-haiku-4-5 · gemini-3.5-flash-lite · sonar · llama3.2"],
    ["vision_model", "Photo-reading model (cover photos in Check a book)", "e.g. gemma3:4b · blank = picked by itself"],
    ["llm_fallback_model", "Fallback model(s)", "Only used when the main model fails. Several? Separate with commas, in order", "text", "", "adv"],
    ["llm_json_mode", "JSON response mode", "true / false", "bool", "", "adv"],
    ["llm_timeout_seconds", "AI time limit per book (seconds)", "0 = automatic: 10 minutes for Ollama, 2 for cloud", "number", "", "adv"],
    ["local_context_tokens", "Context size of your local AI (tokens)", "4096", "number", "", "adv"],
  ]],
  ["ai", "Backup AI", [
    ["backup_llm_enabled", "Use a backup AI when the main one fails (a second Ollama, or a cloud AI)", "", "bool"],
    ["backup_llm_provider", "", "", "hidden"],
    ["backup_llm_base_url", "Address", "http://192.168.1.60:11434/v1 · https://api.openai.com/v1", "text", "backup_llm_enabled"],
    ["backup_llm_api_key", "API key", "Leave blank for Ollama", "password", "backup_llm_enabled"],
    ["backup_llm_model", "Model(s), in order", "e.g. llama3.2, or gpt-4o-mini; several? separate with commas", "text", "backup_llm_enabled"],
    ["backup_llm_json_mode", "JSON response mode", "", "bool", "backup_llm_enabled", "adv"],
  ]],
  ["ai", "Deep Scans", [
    ["deep_llm_enabled", "Run Deep Scans on a separate, bigger machine (they wait while it's switched off)", "", "bool"],
    ["deep_llm_provider", "", "", "hidden"],
    ["deep_llm_base_url", "Address", "http://<the machine's address>:11434/v1", "text", "deep_llm_enabled"],
    ["deep_llm_api_key", "API key", "Leave blank for Ollama", "password", "deep_llm_enabled"],
    ["deep_llm_model", "Model(s), in order", "e.g. qwen2.5:14b; several? separate with commas", "text", "deep_llm_enabled"],
    ["deep_read_model", "Deep Scan model when there is no separate machine (optional)", "Blank = the models of Your AI · e.g. a cheaper model with a big context window"],
    ["deep_llm_json_mode", "JSON response mode", "", "bool", "deep_llm_enabled", "adv"],
  ]],
  ["ai", "When it runs", [
    ["tokens_per_hour_cap", "Most tokens per hour (0 = no limit)", "25000", "number"],
    ["batch_size", "Books per “Rate waiting books now” (0 = all waiting books)", "20", "number"],
    ["scan_delay_seconds", "Pause between books (seconds)", "2", "number", "", "adv"],
  ]],
  ["ai", "Extras", [
    ["suggest_mode", "How suggestions under Nightstand are picked", "AI picks + books you don't own|AI picks from your library|Free matching only (no AI)", "select"],
    ["language", "Language for book summaries", "English (US)|English (UK)|Spanish|French|German|Portuguese|Italian|Dutch", "select"],
    ["google_books_api_key", "Google Books API key (optional)", "", "password"],
  ]],
  ["ai", "Machines & costs", [
    ["power_price_kwh", "Electricity price per kWh (USD)", "0 = don't count electricity", "number"],
    ["main_ai_watts", "Main AI machine: extra watts while it works", "e.g. 150 · 0 for a cloud AI", "number"],
    ["backup_ai_watts", "Backup AI machine: extra watts while it works", "0 for a cloud AI", "number", "backup_llm_enabled"],
    ["deep_ai_watts", "Deep Scan machine: extra watts while it works", "e.g. 350", "number", "deep_llm_enabled"],
    ["price_input_per_million", "Your AI: input price per 1M tokens (USD)", "0.15", "number", "", "adv"],
    ["price_output_per_million", "Your AI: output price per 1M tokens (USD)", "0.60", "number", "", "adv"],
    ["backup_price_input_per_million", "Backup AI: input price per 1M tokens (USD)", "0 for Ollama", "number", "backup_llm_enabled", "adv"],
    ["backup_price_output_per_million", "Backup AI: output price per 1M tokens (USD)", "0 for Ollama", "number", "backup_llm_enabled", "adv"],
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
  ["delivery", "Audiobookshelf", [
    ["abs_url", "Audiobookshelf address", "http://192.168.1.10:13378"],
    ["abs_api_token", "API token", "Paste the token from Audiobookshelf", "password"],
    ["abs_poll_hours", "Read the library every N hours (0 = only when you press the button)", "12", "number"],
  ]],
  ["delivery", "Discover", [
    ["nyt_api_key", "New York Times Books API key (optional, free)", "Paste the key from developer.nytimes.com", "password"],
    ["discover_daily_ratings", "Discover books the AI rates per day (0 = none)", "30", "number"],
  ]],
  ["system", "Features", [
    ["module_queue", "Nightstand reading queue", "", "bool"],
    ["module_send_to_kindle", "Send-to-Kindle email delivery", "", "bool"],
    ["module_koreader", "KOReader catalog delivery", "", "bool"],
    ["module_import", "Import books (Kindle / drive scanner and lists)", "", "bool"],
    ["module_parents", "Parent tools (kids' accounts, age groups, parents' notes)", "", "bool"],
    ["module_suggestions", "Suggested Reads under Nightstand", "", "bool"],
    ["module_taste", "Taste profile: people can rate books they know to improve their suggestions (uses no AI)", "", "bool"],
    ["module_discover", "Discover tab: best sellers, classics, and the family's new and popular books", "", "bool"],
    ["notify_routine", "Also show everyday events: new books from Calibre, rating finished, someone started a book", "", "bool"],
  ]],
  ["system", "Sign-in & Updates", [
    ["session_days", "Keep people signed in for (days, renewed while they use the app)", "30", "number"],
    ["check_updates", "Check GitHub for new NovelCheck versions (shows a banner to admins and editors)", "", "bool"],
  ]],
];

// Cards that only matter while a feature is on.
export const SECTION_MODULE = { "SMTP / Send-to-Kindle": "send_to_kindle", Discover: "discover" };

// The area each card belongs to; an admin sees the cards of the areas the
// main admin gave them (the server sorts each setting the same way: settingArea).
export const SECTION_AREA = {
  "Your AI": "ai", "Backup AI": "ai", "Deep Scans": "ai", "When it runs": "ai", "Extras": "ai", "Machines & costs": "ai",
  "SMTP / Send-to-Kindle": "services", "Calibre Library": "calibre", Audiobookshelf: "services", Discover: "services",
  Features: "system", "Sign-in & Updates": "system",
};

// One line under a card's title: what the card is for.
export const NOTE = {
  "Backup AI": "A second AI, tried only when the main one fails.",
  "Deep Scans": "Reading a whole book, on this AI or a bigger machine.",
  "When it runs": "Automatic rating, its hours and its limits.",
  "Extras": "Suggestions, collection ideas, summary language, your own filters.",
  "Machines & costs": "Model updates, the speed test, electricity and token prices.",
};

const note = (when, text) => `<p class="text-xs text-slate-400"${when ? ` data-when="${when}"` : ""}>${text}</p>`;

// What a card holds besides its fields: before them (TOP) or after them.
export const TOP = {
  "Your AI": `<div id="llm-preset-host"></div>`,
  "When it runs": `<div id="auto-rate-host" class="space-y-3"></div>`,
  "Machines & costs": `<div id="aitools-host" class="space-y-3"></div>`,
  Features: note("", "Turn off what your family doesn't use. It disappears for everyone; kids' content rules always keep applying."),
};
export const EXTRA = {
  "Backup AI": note("backup_llm_enabled", "Tried only when the main AI fails on a book. If the main server is switched off, NovelCheck goes straight to the backup. You'll get a 🔔 notice when the backup is used.") + `<div id="backup-preset-host" data-when="backup_llm_enabled"></div>`,
  "Deep Scans": note("deep_llm_enabled", "Deep Scans read whole books, so a bigger model on a stronger machine rates them better. Ratings stay on the main AI. While this machine is off or asleep, Deep Scans wait (🧬⏸) and carry on when it answers.") + `<div id="deep-preset-host" data-when="deep_llm_enabled"></div><div id="deep-status-host"></div>`,
  "Extras": `<div id="ideas-host"></div><div id="custom-flags"></div>`,
  "SMTP / Send-to-Kindle": `<button type="button" data-act="smtp-test" class="btn-secondary">Send test email</button>`,
  "Calibre Library": `<div id="calibre-picker"></div><div id="calibre-server"></div>`,
  Discover: `<div id="discover-admin" class="space-y-2"></div>`,
  Audiobookshelf: `<div id="abs-admin" class="space-y-2"></div>`,
};

// Extra help under a few fields.
const help = (text) => `<p class="mt-1 text-xs text-slate-400">${text}</p>`;
export const HELP = {
  vision_model: help("A model on your local AI that can read pictures. Pick Ollama above, press Find Ollama and download Gemma 3 4B; NovelCheck then uses it for photos by itself."),
  local_context_tokens: help("How much text your local AI takes at once. Ollama's default is 4096; Deep Scan cuts books into parts that fit it. If you raised it on the server (e.g. OLLAMA_CONTEXT_LENGTH=8192), enter the new size so parts can be bigger and scans faster."),
  power_price_kwh: help("With the watts below, System → Usage counts what your own AI machines cost in electricity, and Deep Scan estimates show it. A plug-in power meter tells you a machine's watts while the AI works; count only the extra over idle."),
  deep_read_model: help("Deep Scan reads whole books, so it needs a model that can tell romance from a tense or violent scene: 7B or bigger is recommended (e.g. qwen2.5:7b or llama3.1:8b on Ollama, or a cloud model). Small 1–3B models often over-rate books. Not used when a separate machine is switched on above."),
  suggest_mode: help("Free matching looks for the next book in a series, the same authors and similar descriptions. With AI, your AI then picks the best 10 with a reason, at most once a day per person and within the hourly token limit. Kids' accounts (and anyone hiding unrated books) never get books you don't own."),
  smtp_password: help(`Gmail: your normal Google password won't work. Turn on 2-Step Verification,
    then create an <a href="https://myaccount.google.com/apppasswords" target="_blank" rel="noopener noreferrer" class="underline">App Password</a>
    and paste its 16 letters here (spaces are fine). Use your full Gmail address as the username.`),
};
