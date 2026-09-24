// Admin → LLM Analysis Engine: a provider menu that fills in sensible settings.
// Everything except Claude uses the OpenAI-compatible API; Claude uses
// Anthropic's native API (llm_provider = "anthropic").
import { $, esc } from "./ui.js";

// Defaults follow the "small model first" rule; prices are USD per 1M tokens.
export const PRESETS = {
  openai: {
    label: "OpenAI", provider: "openai", base: "https://api.openai.com/v1",
    model: "gpt-4o-mini", fallback: "", json: true, pin: "0.15", pout: "0.60",
    hint: "Get a key at platform.openai.com → API keys.",
  },
  claude: {
    label: "Anthropic Claude", provider: "anthropic", base: "",
    model: "claude-haiku-4-5", fallback: "claude-sonnet-5", json: false, pin: "1.00", pout: "5.00",
    hint: "Get a key at console.anthropic.com → API Keys. Claude Haiku 4.5 rates books; Claude Sonnet 5 is only used when Haiku can't.",
  },
  gemini: {
    label: "Google Gemini", provider: "openai", base: "https://generativelanguage.googleapis.com/v1beta/openai",
    model: "gemini-2.5-flash", fallback: "", json: true, pin: "0.30", pout: "2.50",
    hint: "Get a key at aistudio.google.com → Get API key. Check the current Flash model name and price there.",
  },
  perplexity: {
    label: "Perplexity", provider: "openai", base: "https://api.perplexity.ai",
    model: "sonar", fallback: "", json: false, pin: "1.00", pout: "1.00",
    hint: "Get a key at perplexity.ai → Settings → API. Sonar searches the web, which helps with lesser-known books. Perplexity also charges a small per-request fee that the cost estimate doesn't include.",
  },
  ollama: {
    label: "Ollama (on your own server, free)", provider: "openai", base: "http://YOUR-TRUENAS-IP:11434/v1",
    model: "llama3.2", fallback: "", json: true, pin: "0", pout: "0",
    hint: "Runs on your own hardware; no API key needed. Replace YOUR-TRUENAS-IP with your server's address.",
  },
  other: { label: "Other (OpenAI-compatible)", hint: "Any service with an OpenAI-style /chat/completions endpoint, such as vLLM or LM Studio." },
};

// Guess which preset matches the saved settings.
function detect(s) {
  if (s.llm_provider === "anthropic") return "claude";
  const url = s.llm_base_url || "";
  if (url.includes("api.openai.com")) return "openai";
  if (url.includes("generativelanguage.googleapis.com")) return "gemini";
  if (url.includes("api.perplexity.ai")) return "perplexity";
  if (url.includes(":11434")) return "ollama";
  return "other";
}

const row = (form, key) => $(`[data-key="${key}"]`, form)?.closest("div, label");

export function initProviderPicker(host, form, settings) {
  host.innerHTML = `
    <label class="label" for="llm-preset">AI provider</label>
    <select id="llm-preset" class="input">
      ${Object.entries(PRESETS).map(([k, p]) => `<option value="${k}">${esc(p.label)}</option>`).join("")}
    </select>
    <p id="llm-hint" class="mt-1 text-xs text-slate-400"></p>`;
  const select = $("#llm-preset", host);
  const set = (key, value) => {
    const el = $(`[data-key="${key}"]`, form);
    if (!el || value === undefined) return;
    if (el.type === "checkbox") el.checked = value;
    else el.value = value;
  };
  const showFor = (name) => {
    const claude = name === "claude";
    row(form, "llm_base_url")?.classList.toggle("hidden", claude);
    row(form, "llm_json_mode")?.classList.toggle("hidden", claude);
    $("#llm-hint", host).textContent = PRESETS[name].hint;
  };
  select.value = detect(settings);
  showFor(select.value);
  select.addEventListener("change", () => {
    const p = PRESETS[select.value];
    set("llm_provider", p.provider || "openai");
    if (select.value !== "other") {
      set("llm_base_url", p.base);
      set("llm_model", p.model);
      set("llm_fallback_model", p.fallback);
      set("llm_json_mode", p.json);
      set("price_input_per_million", p.pin);
      set("price_output_per_million", p.pout);
    }
    showFor(select.value);
  });
}
