// Collection dialogs: a new (or renamed) collection made by hand, and one the
// AI fills from a theme. The AI searches in the background (a local AI can
// take a minute), then shows its picks with reasons; untick any, then save.
// "Find more" reuses it for an existing collection.
import { get, post, api } from "./api.js";
import { $$, esc, attempt, toast } from "./ui.js";
import { coverImg } from "./covers.js";

const ICONS = ["📚", "✨", "🐉", "🧙", "🕵️", "🚀", "🐴", "🏰", "🌊", "⚽", "🎨", "🦸", "👑", "🧸", "🌙", "🎃", "🍂", "🕯️", "🎄", "❄️", "💝", "✝️", "🌷", "☀️", "🎒", "👼"];

function dialog() {
  let d = document.getElementById("collection-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "collection-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  d.onclick = (e) => {
    if (e.target === d || e.target.closest("[data-close]")) d.close();
  };
  return d;
}

const head = (title) => `<div class="flex items-center justify-between gap-3"><h2 class="text-lg font-bold">${title}</h2>
  <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>`;
const iconSelect = (sel) => `<select name="icon" class="input w-auto text-xl" aria-label="Icon">${ICONS.map((i) => `<option ${i === sel ? "selected" : ""}>${i}</option>`).join("")}</select>`;

// openNewCollection makes a collection by hand, or edits existing.
export function openNewCollection(onDone, existing = null) {
  const d = dialog();
  d.innerHTML = `<form class="max-h-[85vh] space-y-3 overflow-y-auto p-5">${head(existing ? "Edit collection" : "New collection")}
    <div class="flex gap-2">${iconSelect(existing?.icon || "📚")}
      <input name="name" required maxlength="80" class="input min-w-0 flex-1" placeholder="e.g. Summer reading" value="${esc(existing?.name || "")}"></div>
    <textarea name="description" rows="2" maxlength="300" class="input" placeholder="What's it for? (optional)">${esc(existing?.description || "")}</textarea>
    ${existing ? "" : `<p class="text-xs text-slate-400">Then add books from each book's window: <b>＋ Add to a collection</b>.</p>`}
    <button class="btn-primary w-full">${existing ? "Save" : "Create"}</button></form>`;
  d.querySelector("form").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    const body = { name: f.name.value, icon: f.icon.value, description: f.description.value };
    const r = await attempt(() => (existing ? api(`/api/collections/${existing.id}`, { method: "PATCH", body }) : post("/api/collections", body)),
      existing ? "Saved" : `Created ${body.name}`);
    if (r) {
      d.close();
      onDone?.(r);
    }
  };
  d.showModal();
}

// openAICollection: opts {theme, name, icon, season} prefill a new one (a
// seasonal shelf), or {collection} finds more books for an existing one.
export function openAICollection(opts = {}, onDone) {
  const d = dialog();
  const into = opts.collection;
  d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">${head(into ? `✨ Find more for ${esc(into.name)}` : "✨ Describe a collection")}
    <form data-ask class="space-y-2">
      <textarea name="theme" rows="2" required maxlength="500" class="input" placeholder="e.g. Dragon adventures for ages 8-10, not too scary">${esc(opts.theme || into?.theme || "")}</textarea>
      ${into ? "" : `<div class="flex gap-2">${iconSelect(opts.icon || "✨")}
        <input name="name" maxlength="80" class="input min-w-0 flex-1" placeholder="Name, e.g. Dragon adventures" value="${esc(opts.name || "")}"></div>`}
      <p class="text-xs text-slate-400">The AI looks through your libraries, picks the books that are clearly about this, and checks each pick a second time
        (about 17,000 tokens; free with a local AI). Nothing is saved until you press Save.</p>
      <button class="btn-primary w-full">Find books</button>
    </form>
    <div data-out class="space-y-2"></div></div>`;
  const out = d.querySelector("[data-out]");
  const form = d.querySelector("[data-ask]");
  let picks = [];

  const paint = () => {
    const n = $$("[data-pick]:checked", out).length;
    const save = out.querySelector("[data-save]");
    if (save) save.textContent = `Save ${n} book${n === 1 ? "" : "s"}`;
  };
  form.onsubmit = async (e) => {
    e.preventDefault();
    const theme = form.theme.value.trim();
    const start = await attempt(() => post("/api/collections/ai", { theme, collection_id: into?.id || 0, season: opts.season || "" }));
    if (!start) return;
    form.querySelector("button").disabled = true;
    const t0 = Date.now();
    let job = null;
    while (Date.now() - t0 < 6 * 60 * 1000 && d.open) {
      out.innerHTML = `<p class="rounded-lg bg-slate-800 p-3 text-sm">✨ Looking through your libraries… ${Math.round((Date.now() - t0) / 1000)} s
        <span class="block text-xs text-slate-400">A local AI can take a minute or two.</span></p>`;
      await new Promise((r) => setTimeout(r, 2000));
      job = await get(`/api/collections/ai/${start.job}`).catch(() => null);
      if (job && job.status !== "running") break;
    }
    form.querySelector("button").disabled = false;
    if (!job || job.status === "running") return (out.innerHTML = `<p class="text-sm text-rose-300">The AI is taking too long. Try again later.</p>`);
    if (job.status === "error") return (out.innerHTML = `<p class="text-sm text-rose-300">${esc(job.error)}</p>`);
    picks = job.picks;
    if (!picks.length) return (out.innerHTML = `<p class="text-sm text-slate-400">No books in your libraries fit that. Try describing it differently.</p>`);
    out.innerHTML = `<p class="text-sm text-slate-300">Untick any you don't want:</p>
      ${picks.map((p) => `<label class="flex items-center gap-3 rounded-lg bg-slate-800/60 p-2">
        <input type="checkbox" checked data-pick value="${p.book.id}" class="h-5 w-5 shrink-0">
        ${coverImg(p.book.id, "h-14 w-10")}
        <span class="min-w-0 flex-1"><span class="font-semibold leading-snug line-clamp-2">${esc(p.book.title)}</span>
          <span class="block truncate text-xs text-slate-400">${esc(p.book.author || "")}</span>
          <span class="block text-xs text-slate-300">✨ ${esc(p.reason)}</span></span></label>`).join("")}
      <button type="button" data-save class="btn-primary sticky bottom-0 w-full"></button>`;
    paint();
  };
  out.onchange = paint;
  out.onclick = async (e) => {
    if (!e.target.closest("[data-save]")) return;
    const ids = $$("[data-pick]:checked", out).map((cb) => Number(cb.value));
    if (!ids.length) return toast("Tick at least one book", true);
    const reasons = Object.fromEntries(picks.filter((p) => ids.includes(p.book.id)).map((p) => [p.book.id, p.reason]));
    const theme = form.theme.value.trim();
    const r = into
      ? await attempt(() => post(`/api/collections/${into.id}/books`, { ids, reasons }))
      : await attempt(() => post("/api/collections", { name: form.name.value.trim() || theme.slice(0, 60), icon: form.icon.value, theme, season: opts.season || "", ids, reasons }));
    if (!r) return;
    toast(into ? `Added ${r.added} book${r.added === 1 ? "" : "s"}` : `Saved with ${ids.length} book${ids.length === 1 ? "" : "s"}`);
    d.close();
    onDone?.(r);
  };
  d.showModal();
}
