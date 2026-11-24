// Settings → Kindle & KOReader, for admins with the library area: Kindle books NovelCheck doesn't know (add them or
// leave them out) and the Kindle backups (download as ZIP, or file by file). Moved here from Library cleanup so every
// Kindle tool is in one place.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";

export const kindleAdminHTML = `<div class="card space-y-2"><details id="ka-unknown"><summary class="cursor-pointer text-lg font-semibold">Kindle books NovelCheck doesn't know</summary>
      <div id="ka-unknown-body" class="mt-2 text-sm text-slate-300"></div></details></div>
    <div class="card space-y-2"><details id="ka-copies"><summary class="cursor-pointer text-lg font-semibold">Kindle backups</summary>
      <div id="ka-copies-body" class="mt-2 text-sm text-slate-300"></div></details></div>`;

export function bindKindleAdmin(host) {
  host.querySelector("#ka-unknown").addEventListener("toggle", async (e) => {
    if (!e.target.open) return;
    const body = host.querySelector("#ka-unknown-body");
    const load = async () => {
      const r = await get("/api/admin/kindle-unknown").catch(() => null);
      if (!r) { body.textContent = "Couldn't load that just now."; return; }
      if (!r.books.length) { body.textContent = "Nothing waiting. Kindle books NovelCheck doesn't know will show up here."; return; }
      body.innerHTML = `<p class="mb-2">Kindle books NovelCheck doesn't know. Tick the ones to add; skip any you already have under another name.</p>
        <ul class="max-h-96 divide-y divide-slate-800 overflow-auto">${r.books.map((b) => `<li class="flex items-center gap-2 py-1.5"><input type="checkbox" data-ku="${b.id}">
          <span class="min-w-0"><span class="block break-words">${esc(b.title || b.filename)}${b.author ? ` <span class="text-slate-400">· ${esc(b.author)}</span>` : ""}</span>
          <span class="block break-words text-xs text-slate-500">${esc(b.path)}</span></span></li>`).join("")}</ul>
        <p class="mt-2 flex flex-wrap gap-2"><button type="button" data-ku-add class="btn-secondary py-1 text-sm">Add the ticked books</button>
        <button type="button" data-ku-drop class="btn-ghost py-1 text-sm">Leave the ticked ones out (don't ask again)</button></p>`;
      body.onclick = async (ev) => {
        const add = ev.target.closest("[data-ku-add]"), drop = ev.target.closest("[data-ku-drop]");
        if (!add && !drop) return;
        const ids = [...body.querySelectorAll("[data-ku]:checked")].map((c) => Number(c.dataset.ku));
        if (!ids.length) return;
        await attempt(() => post("/api/admin/kindle-unknown", add ? { add: ids } : { drop: ids }), add ? "Added" : "Left out");
        load();
      };
    };
    load();
  });
  host.querySelector("#ka-copies").addEventListener("toggle", async (e) => {
    if (!e.target.open) return;
    const body = host.querySelector("#ka-copies-body");
    const r = await get("/api/admin/kindle-backup").catch(() => null);
    const mb = (b) => (b >= 1e9 ? `${(b / 1e9).toFixed(1)} GB` : `${Math.max(1, Math.round(b / 1e6))} MB`);
    const q = (d) => `user=${encodeURIComponent(d.user)}&device=${encodeURIComponent(d.name)}`;
    body.innerHTML = !r ? "Couldn't load that just now." : !r.devices.length ? "Nothing copied yet. Back up a Kindle from the Kindle remote."
      : `${r.devices.map((d, i) => `<div class="mb-3"><p><b>${esc(d.name)}</b>: ${d.files} book${d.files === 1 ? "" : "s"}, ${mb(d.bytes)}.</p>
          <p class="mt-1 flex flex-wrap gap-2">${[["all", "Everything"], ["apps", "Apps and settings"], ["books", "All books"], ["drm", "DRM books"]].map(([p, l]) =>
            `<a class="${p === "all" ? "btn-secondary" : "btn-ghost"} py-1 text-sm" href="/api/admin/kindle-backup/zip?${q(d)}&part=${p}">⬇ ${l} (ZIP)</a>`).join("")}
          <button type="button" data-kb-list="${i}" class="btn-ghost py-1 text-sm">Show the files</button></p><div data-kb-files="${i}" class="mt-2"></div></div>`).join("")}
        <p class="mt-2 text-xs text-slate-400">Saved as-is in the <b>${esc(r.folder)}</b>. 🔒 books open only on a device with your Kindle account.</p>`;
    body.onclick = async (ev) => {
      const b = ev.target.closest("[data-kb-list]");
      if (!b) return;
      const d = r.devices[Number(b.dataset.kbList)], box = body.querySelector(`[data-kb-files="${b.dataset.kbList}"]`);
      box.textContent = "Loading…";
      const l = await get(`/api/admin/kindle-backup/files?${q(d)}`).catch(() => null);
      box.innerHTML = !l ? "Couldn't load that just now." : `<ul class="max-h-80 divide-y divide-slate-800 overflow-auto text-xs">${l.files.map((f) =>
        `<li class="flex items-center justify-between gap-2 py-1"><span class="min-w-0 break-words">${f.locked ? "🔒 " : ""}${esc(f.path)}</span>
          <a class="shrink-0 underline" href="/api/admin/kindle-backup/file?${q(d)}&path=${encodeURIComponent(f.path)}">${mb(f.size)} ⬇</a></li>`).join("")}</ul>`;
    };
  });
}
