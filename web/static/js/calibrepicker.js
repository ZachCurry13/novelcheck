// Admin → Calibre Library: browse the mounted folder and pick the exact
// Calibre library (the folder containing metadata.db).
import { get, put, qs } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";

export async function renderCalibrePicker(host, onChange) {
  const box = document.createElement("div");
  host.replaceChildren(box);
  box.innerHTML = `
    <div class="space-y-3">
      <div>
        <span class="label">Library folder</span>
        <p id="cp-current" class="break-all text-sm"></p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" data-cact="browse" class="btn-secondary">Browse folders…</button>
        <button type="button" data-cact="find" class="btn-secondary">Find libraries automatically</button>
      </div>
      <div id="cp-panel" class="hidden rounded-lg ring-1 ring-slate-700"></div>
    </div>`;
  const panel = $("#cp-panel", box);

  let currentSel = "";
  async function showCurrent() {
    const data = await attempt(() => get("/api/admin/calibre/browse"));
    if (!data) return;
    currentSel = data.selected || "";
    $("#cp-current", box).innerHTML = `<code>${esc(data.library)}</code> ${data.library_is_valid
      ? `<span class="chip-none">Calibre library found</span>`
      : `<span class="chip-open">No metadata.db here, so pick your library folder</span>`}`;
  }

  async function browse(rel) {
    const data = await attempt(() => get("/api/admin/calibre/browse" + qs({ path: rel })));
    if (!data) return;
    const l = data.listing;
    const crumbs = [`<button type="button" data-open="" class="underline">${esc(data.mount)}</button>`];
    let acc = "";
    for (const part of (l.path ? l.path.split("/") : [])) {
      acc = acc ? acc + "/" + part : part;
      crumbs.push(`<button type="button" data-open="${esc(acc)}" class="underline">${esc(part)}</button>`);
    }
    panel.innerHTML = `
      <div class="flex flex-wrap items-center gap-1 border-b border-slate-700 p-3 text-sm">
        ${crumbs.join('<span class="text-slate-500">/</span>')}
        ${l.has_library ? `<button type="button" data-use="${esc(l.path)}" class="btn-primary ml-auto py-1">Use this folder</button>` : ""}
      </div>
      <ul class="max-h-80 overflow-y-auto divide-y divide-slate-800">
        ${l.parent !== null ? `<li><button type="button" data-open="${esc(l.parent)}" class="w-full p-3 text-left text-sm text-slate-400 hover:bg-slate-800">⬆ Up one level</button></li>` : ""}
        ${l.folders.map(folderRow).join("") || `<li class="p-3 text-sm text-slate-500">No subfolders.</li>`}
      </ul>
      ${l.truncated ? `<p class="p-3 text-xs text-slate-500">Showing the first 500 folders.</p>` : ""}`;
    panel.classList.remove("hidden");
  }

  async function find() {
    panel.classList.remove("hidden");
    panel.innerHTML = `<p class="p-3 text-sm text-slate-400">Searching for Calibre libraries…</p>`;
    const data = await attempt(() => get("/api/admin/calibre/find"));
    if (!data) return panel.classList.add("hidden");
    panel.innerHTML = data.libraries.length
      ? `<ul class="divide-y divide-slate-800">${data.libraries.map((f) => `
          <li class="flex items-center gap-3 p-3 text-sm">
            <span class="min-w-0 flex-1 break-all">📚 <code>${esc(data.mount + (f.path ? "/" + f.path : ""))}</code></span>
            <button type="button" data-use="${esc(f.path)}" class="btn-primary py-1">Use this</button>
          </li>`).join("")}</ul>`
      : `<p class="p-3 text-sm text-slate-400">No Calibre libraries found. Use <b>Browse folders…</b>, or check the Calibre volume in your TrueNAS app settings.</p>`;
  }

  async function use(rel) {
    const r = await attempt(() => put("/api/admin/calibre/library", { path: rel }));
    if (!r) return;
    toast("Library folder saved. Syncing now…");
    panel.classList.add("hidden");
    await showCurrent();
    onChange?.();
  }

  box.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t) return;
    if (t.dataset.cact === "browse") browse(currentSel);
    else if (t.dataset.cact === "find") find();
    else if (t.dataset.open !== undefined) browse(t.dataset.open);
    else if (t.dataset.use !== undefined) use(t.dataset.use);
  });

  await showCurrent();
}

function folderRow(f) {
  return `
    <li class="flex items-center gap-2 hover:bg-slate-800">
      <button type="button" data-open="${esc(f.path)}" class="min-w-0 flex-1 truncate p-3 text-left text-sm">
        ${f.has_library ? "📚" : "📁"} ${esc(f.name)}
        ${f.has_library ? `<span class="chip-none ml-2">Calibre library</span>` : ""}
      </button>
      ${f.has_library ? `<button type="button" data-use="${esc(f.path)}" class="btn-primary mr-3 py-1">Use this</button>` : ""}
    </li>`;
}
