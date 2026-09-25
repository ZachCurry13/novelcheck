// Installed Ollama models with the disk space each takes, and a Delete button
// to free storage on the server. Models NovelCheck is set to use are marked
// and can't be deleted from here.
import { get, post, qs } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

const gb = (bytes) => `${(bytes / 1e9).toFixed(1)} GB`;

function dialog() {
  let d = document.getElementById("ollama-models-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "ollama-models-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

export async function openOllamaModels(url) {
  if (!url) return toast("Type the Ollama address first (or press Find Ollama)", true);
  const d = dialog();
  const load = async () => {
    const data = await attempt(() => get("/api/admin/ollama/models" + qs({ url })));
    if (!data) return false;
    d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🧹 Ollama models on ${esc(url)}</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">${data.models.length} model${data.models.length === 1 ? "" : "s"} using <b>${gb(data.total)}</b>. Delete the ones you don't use to free space on the server; you can always download them again.</p>
      <ul class="space-y-2">${data.models.map((m) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-3">
        <span class="min-w-0"><b class="break-all">${esc(m.name)}</b>
          <span class="block text-xs text-slate-400">${gb(m.size)}${m.params ? ` · ${esc(m.params)} parameters` : ""}${m.quant ? ` · ${esc(m.quant)}` : ""}</span></span>
        ${m.in_use ? `<span class="chip-none" title="NovelCheck is set to use this model">In use</span>`
          : `<button data-del="${esc(m.name)}" class="btn-ghost py-1 text-sm">🗑 Delete</button>`}</li>`).join("")
        || `<li class="text-sm text-slate-500">No models installed.</li>`}</ul></div>`;
    return true;
  };
  if (!(await load())) return;
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    const name = e.target.closest("[data-del]")?.dataset.del;
    if (!name || !confirm(`Delete ${name} from Ollama? It frees its disk space; you can download it again later.`)) return;
    if (await attempt(() => post("/api/admin/ollama/delete", { url, model: name }), `Deleted ${name}`)) load();
  };
  d.showModal();
}
