// Ollama "easy button": find the Ollama server, download a model with a
// progress bar, and switch NovelCheck to it, with no terminal needed.
import { get, post, qs } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";

const MODELS = [
  ["qwen2.5:7b", "Qwen 2.5 7B · 4.7 GB · best, needs GPU"],
  ["llama3.1:8b", "Llama 3.1 8B · 4.9 GB · needs GPU"],
  ["llama3.2", "Llama 3.2 3B · 2 GB · fast, no GPU needed"],
];

export function renderOllamaHelper(host, form) {
  host.innerHTML = `
    <div class="space-y-3 rounded-lg bg-slate-800/60 p-4 text-sm">
      <p class="font-semibold">Ollama easy setup</p>
      <p class="text-slate-400">First install the <b>Ollama</b> app from TrueNAS <b>Apps → Discover Apps</b> (turn on your GPU there if you have one). Then:</p>
      <div class="flex flex-wrap gap-2">
        <button type="button" data-ol="find" class="btn-primary py-1">1. Find Ollama</button>
        <input id="ol-url" class="input w-64 max-w-full py-1" placeholder="or type its address, e.g. 192.168.1.50:11434">
      </div>
      <div id="ol-servers" class="space-y-3"></div>
    </div>`;
  const list = $("#ol-servers", host);
  // Start from the address already saved, if it isn't the placeholder.
  const saved = $('[data-key="llm_base_url"]', form)?.value || "";
  if (saved && !saved.includes("YOUR-TRUENAS-IP")) $("#ol-url", host).value = saved.replace(/\/v1\/?$/, "");
  let pollTimer = null;

  async function find() {
    list.innerHTML = `<p class="text-slate-400">Looking for Ollama…</p>`;
    const data = await attempt(() => get("/api/admin/ollama/find" + qs({ url: $("#ol-url", host).value.trim() })));
    if (!data) return (list.innerHTML = "");
    if (!data.servers.length) {
      list.innerHTML = `<p class="text-amber-300">Ollama wasn't found. Check the Ollama app is <b>Running</b> in TrueNAS,
        then type its address above (your TrueNAS IP and the port shown on the app, usually 11434) and try again.</p>`;
      return;
    }
    list.innerHTML = data.servers.map(serverCard).join("");
  }

  function serverCard(s) {
    const installed = s.models.length
      ? s.models.map((m) => `<li class="flex flex-wrap items-center justify-between gap-2"><code class="break-all">${esc(m)}</code>
          <button type="button" data-ol="use" data-url="${esc(s.url)}" data-model="${esc(m)}" class="btn-primary py-0.5 text-xs">3. Use this model</button></li>`).join("")
      : `<li class="text-slate-400">No models downloaded yet.</li>`;
    return `
      <div class="min-w-0 space-y-2 rounded-lg ring-1 ring-slate-700 p-3" data-server="${esc(s.url)}">
        <p>✓ Found Ollama ${esc(s.version)} at <code>${esc(s.url)}</code></p>
        <p class="label mb-0">Downloaded models</p>
        <ul class="space-y-1">${installed}</ul>
        <p class="label mb-0">2. Download a model</p>
        <div class="flex flex-wrap gap-2">
          <select class="input w-auto max-w-full py-1" data-ol-model>${MODELS.map(([v, l]) => `<option value="${v}">${esc(l)}</option>`).join("")}</select>
          <button type="button" data-ol="pull" data-url="${esc(s.url)}" class="btn-secondary py-1">Download</button>
        </div>
        <div data-ol-progress class="hidden space-y-1">
          <progress max="100" value="0" class="w-full"></progress>
          <p class="text-xs text-slate-400"></p>
        </div>
      </div>`;
  }

  async function pull(url, card) {
    const model = $("[data-ol-model]", card).value;
    const ok = await attempt(() => post("/api/admin/ollama/pull", { url, model }));
    if (!ok) return;
    const box = $("[data-ol-progress]", card);
    box.classList.remove("hidden");
    clearInterval(pollTimer);
    pollTimer = setInterval(async () => {
      const st = await get("/api/admin/ollama/pull").catch(() => null);
      if (!st) return;
      $("progress", box).value = Math.round(st.percent || 0);
      $("p", box).textContent = st.error ? `Problem: ${st.error}` : `${st.status || "working"} · ${Math.round(st.percent || 0)}%`;
      if (!st.active) {
        clearInterval(pollTimer);
        if (st.done) {
          toast(`${st.model} downloaded`);
          find();
        }
      }
    }, 1000);
  }

  async function use(url, model) {
    const r = await attempt(() => post("/api/admin/ollama/use", { url, model }));
    if (!r) return;
    // Reflect the saved settings in the form without reloading the page.
    const set = (k, v) => {
      const el = $(`[data-key="${k}"]`, form);
      if (el) el.type === "checkbox" ? (el.checked = v) : (el.value = v);
    };
    set("llm_provider", "openai");
    set("llm_base_url", r.base_url);
    set("llm_model", r.model);
    set("llm_fallback_model", "");
    set("llm_json_mode", true);
    set("price_input_per_million", "0");
    set("price_output_per_million", "0");
    toast(`NovelCheck will now use ${r.model} on your Ollama server. Try a small batch!`);
  }

  host.addEventListener("click", (e) => {
    const b = e.target.closest("[data-ol]");
    if (!b) return;
    if (b.dataset.ol === "find") find();
    else if (b.dataset.ol === "pull") pull(b.dataset.url, b.closest("[data-server]"));
    else if (b.dataset.ol === "use") use(b.dataset.url, b.dataset.model);
  });
  return () => clearInterval(pollTimer);
}

