// Ollama "easy button": find the Ollama server, download a model with a
// progress bar, and switch NovelCheck to it, with no terminal needed.
import { get, post, qs } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";
import { initialOrder, orderHTML, bindOrder } from "./ollamaorder.js";
import { keyFor } from "./llmpresets.js";
import { openOllamaModels } from "./ollamamodels.js";

// How each model fits the measured GPU (see internal/ollama/gpu.go).
const FIT = {
  best: ["⭐ Best for your GPU", "text-emerald-300"],
  best_powerful: ["⭐ Best and most powerful for your GPU", "text-emerald-300"],
  powerful: ["💪 Most powerful that fits", "text-indigo-300"],
  fits: ["✓ Fits your GPU", "text-slate-300"],
  too_big: ["⚠️ Too big for your GPU (slow)", "text-amber-300"],
  cpu_ok: ["✓ OK without a GPU", "text-slate-300"],
  cpu_slow: ["⚠️ Slow without a GPU", "text-amber-300"],
};
const RANK = { best_powerful: 0, best: 1, cpu_ok: 2, powerful: 3, fits: 4, "": 5, cpu_slow: 6, too_big: 7 };
const GB = 1 << 30;
const vramKey = (url) => `nc:vram:${url}`;

function gpuText(g, msg) {
  const gb = (b) => (b / GB).toFixed(b >= 10 * GB ? 0 : 1);
  const text = {
    none: "⚠️ Ollama is running on the CPU only. Small models are recommended. If this server has a GPU, turn it on in the TrueNAS Ollama app's settings (GPU → allocate).",
    about: `🎮 About ${gb(g.vram_bytes)} GB of GPU memory (measured with ${g.basis}).`,
    at_least: `🎮 At least ${gb(g.vram_bytes)} GB of GPU memory (${g.basis} fits entirely). Bigger models may fit too; pick your GPU size to be sure.`,
    manual: `🎮 ${gb(g.vram_bytes)} GB of GPU memory (your choice).`,
  }[g.kind] || "GPU not measured yet. Click Check my GPU, or pick your GPU size.";
  return msg ? `${text} (${msg})` : text;
}


// prefix "" sets up the main AI; "backup_" the backup AI.
export function renderOllamaHelper(host, form, prefix = "") {
  const field = (key) => { const k = keyFor(prefix, key); return k && $(`[data-key="${k}"]`, form); };
  host.innerHTML = `
    <div class="space-y-3 rounded-lg bg-slate-800/60 p-4 text-sm">
      <p class="font-semibold">Ollama easy setup${prefix ? " (backup)" : ""}</p>
      ${prefix ? `<p class="text-slate-400">For a second Ollama on your network, type its address (e.g. <code>192.168.1.60:11434</code>) and click Find.</p>` : ""}
      <p class="text-slate-400">First install the <b>Ollama</b> app from TrueNAS <b>Apps → Discover Apps</b> (turn on your GPU there if you have one). Then:</p>
      <div class="flex flex-wrap gap-2">
        <button type="button" data-ol="find" class="btn-primary py-1">1. Find Ollama</button>
        <button type="button" data-ol="models" class="btn-ghost py-1" title="See installed models and free disk space">🧹 Installed models</button>
        <input data-ol-url class="input w-64 max-w-full py-1" placeholder="or type its address, e.g. 192.168.1.50:11434">
      </div>
      <div data-ol-servers class="space-y-3"></div>
    </div>`;
  const list = $("[data-ol-servers]", host);
  // Start from the address already saved, if it isn't the placeholder.
  const saved = field("llm_base_url")?.value || "";
  if (saved && !saved.includes("YOUR-TRUENAS-IP")) $("[data-ol-url]", host).value = saved.replace(/\/v1\/?$/, "");
  let pollTimer = null;

  async function find() {
    list.innerHTML = `<p class="text-slate-400">Looking for Ollama…</p>`;
    const data = await attempt(() => get("/api/admin/ollama/find" + qs({ url: $("[data-ol-url]", host).value.trim() })));
    if (!data) return (list.innerHTML = "");
    if (!data.servers.length) {
      list.innerHTML = `<p class="text-amber-300">Ollama wasn't found. Check the Ollama app is <b>Running</b> in TrueNAS,
        then type its address above (your TrueNAS IP and the port shown on the app, usually 11434) and try again.</p>`;
      return;
    }
    list.innerHTML = data.servers.map(serverCard).join("");
    list.querySelectorAll("[data-server]").forEach((card) => {
      bindOrder($("[data-order]", card), orders[card.dataset.server]);
      let saved = "";
      try {
        saved = localStorage.getItem(vramKey(card.dataset.server)) || "";
      } catch {
        /* private mode */
      }
      $("[data-gpu-manual]", card).value = saved;
      loadGPU(card, saved === "" ? {} : { vram_gb: saved });
    });
  }

  const orders = {}; // server url -> { list }
  function serverCard(s) {
    const current = [...(field("llm_model")?.value || "").split(","),
      ...(field("llm_fallback_model")?.value || "").split(",")].map((m) => m.trim()).filter(Boolean);
    orders[s.url] = { list: initialOrder(s.models, current) };
    if (!orders[s.url].list.some((m) => m.on) && orders[s.url].list.length) orders[s.url].list[0].on = true;
    return `
      <div class="min-w-0 space-y-2 rounded-lg ring-1 ring-slate-700 p-3" data-server="${esc(s.url)}">
        <p>✓ Found Ollama ${esc(s.version)} at <code>${esc(s.url)}</code></p>
        <p class="label mb-0">2. Tick the models to use and put them in order</p>
        <p class="text-xs text-slate-400">#1 rates every book. If it fails on a book, #2 tries, then #3, and so on.</p>
        <ul class="space-y-1" data-order>${orderHTML(orders[s.url].list)}</ul>
        ${s.models.length ? `<button type="button" data-ol="use" data-url="${esc(s.url)}" class="btn-primary py-1">Use these models in this order</button>` : ""}
        <p class="label mb-0">3. Download another model (optional)</p>
        <div class="space-y-2 rounded-lg bg-slate-900/60 p-2">
          <p data-gpu-text class="text-xs text-slate-300">Checking what your GPU can hold…</p>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" data-ol="gpu" data-url="${esc(s.url)}" class="btn-ghost px-2 py-0.5 text-xs" title="Loads your biggest model for a moment to see how much fits on the GPU">🎮 Check my GPU</button>
            <select data-gpu-manual class="input w-auto py-0.5 text-xs" aria-label="GPU memory">
              <option value="">…or pick your GPU memory</option><option value="0">No GPU</option>
              ${[4, 6, 8, 10, 12, 16, 20, 24, 32, 48].map((n) => `<option value="${n}">${n} GB</option>`).join("")}
            </select>
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <select class="input w-auto max-w-full py-1" data-ol-model></select>
          <button type="button" data-ol="pull" data-url="${esc(s.url)}" class="btn-secondary py-1">Download</button>
        </div>
        <p data-model-note class="text-xs text-slate-400"></p>
        <div data-ol-progress class="hidden space-y-1">
          <progress max="100" value="0" class="w-full"></progress>
          <p class="text-xs text-slate-400"></p>
        </div>
      </div>`;
  }

  // loadGPU fetches GPU info and relabels the model menu and the downloaded list.
  async function loadGPU(card, params) {
    const url = card.dataset.server;
    const res = await get("/api/admin/ollama/gpu" + qs({ url, ...params })).catch((e) => ({ error: e.message }));
    if (res.error) return ($("[data-gpu-text]", card).textContent = `Couldn't check the GPU: ${res.error}`);
    $("[data-gpu-text]", card).textContent = gpuText(res.gpu, res.message);
    const models = [...res.models].sort((a, b) => RANK[a.fit] - RANK[b.fit] || a.size_gb - b.size_gb);
    const sel = $("[data-ol-model]", card);
    sel.innerHTML = models.map((m) => `<option value="${esc(m.name)}" data-fit="${m.fit}" data-note="${esc(m.note)}">${esc(m.label)} · ${m.size_gb} GB${
      m.fit ? " · " + FIT[m.fit][0] : ""}</option>`).join("");
    const note = () => {
      const o = sel.selectedOptions[0];
      const [label, cls] = FIT[o?.dataset.fit] || ["", "text-slate-400"];
      $("[data-model-note]", card).className = `text-xs ${cls}`;
      $("[data-model-note]", card).textContent = o ? `${o.dataset.note}${label ? ` · ${label}` : ""}` : "";
    };
    sel.onchange = note;
    note();
    // Flag downloaded models that are too big.
    const state = orders[url];
    state.fits = Object.fromEntries(res.models.map((m) => [m.name, m.fit]));
    $("[data-order]", card).innerHTML = orderHTML(state.list, state.fits);
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

  async function use(url) {
    const models = (orders[url]?.list || []).filter((m) => m.on).map((m) => m.name);
    if (!models.length) return toast("Tick at least one model", true);
    const r = await attempt(() => post("/api/admin/ollama/use", { url, models, target: prefix ? "backup" : "" }));
    if (!r) return;
    // Reflect the saved settings in the form without reloading the page.
    const set = (k, v) => {
      const el = field(k);
      if (el) el.type === "checkbox" ? (el.checked = v) : (el.value = v);
    };
    if (prefix) {
      set("llm_enabled", true);
      set("llm_provider", "openai");
      set("llm_base_url", r.base_url);
      set("llm_model", r.model);
      set("llm_json_mode", true);
      set("price_input_per_million", "0");
      set("price_output_per_million", "0");
      return toast(`Backup AI set: ${r.model.split(",").join(" then ")} at ${r.base_url}. It's used when the main AI fails.`);
    }
    set("llm_provider", "openai");
    set("llm_base_url", r.base_url);
    set("llm_model", r.model);
    set("llm_fallback_model", r.fallback || "");
    set("llm_json_mode", true);
    set("price_input_per_million", "0");
    set("price_output_per_million", "0");
    const backups = r.fallback ? `, with ${r.fallback.split(",").join(" then ")} as backup` : "";
    toast(`NovelCheck will now use ${r.model}${backups}. Try a small batch!`);
  }

  host.addEventListener("change", (e) => {
    const m = e.target.closest("[data-gpu-manual]");
    if (!m) return;
    const card = m.closest("[data-server]");
    try {
      localStorage.setItem(vramKey(card.dataset.server), m.value);
    } catch {
      /* private mode: just this visit */
    }
    loadGPU(card, m.value === "" ? {} : { vram_gb: m.value });
  });
  host.addEventListener("click", (e) => {
    const b = e.target.closest("[data-ol]");
    if (!b) return;
    if (b.dataset.ol === "find") find();
    else if (b.dataset.ol === "models") openOllamaModels($("[data-ol-url]", host).value.trim());
    else if (b.dataset.ol === "pull") pull(b.dataset.url, b.closest("[data-server]"));
    else if (b.dataset.ol === "use") use(b.dataset.url);
    else if (b.dataset.ol === "gpu") {
      const card = b.closest("[data-server]");
      $("[data-gpu-text]", card).textContent = "Checking… this loads your biggest model for a moment (up to a minute).";
      $("[data-gpu-manual]", card).value = "";
      try {
        localStorage.removeItem(vramKey(card.dataset.server));
      } catch {
        /* ignore */
      }
      loadGPU(card, { probe: "1" });
    }
  });
  return () => clearInterval(pollTimer);
}

