// Library cleanup → "Kindle tasks": ask a Kindle (in NovelCheck, from anywhere) to do something at its next sync:
// back up, tidy its books, delete crash logs, delete books NovelCheck holds a copy of, move books into a folder, or
// download library books. The plugin (js-free, plugin 1.59) asks for the waiting tasks, does them and reports back
// (api/kindletasks_handlers.go). Only these tasks exist.
import { get, post, del } from "./api.js";
import { esc, attempt } from "./ui.js";

const KIND = { backup: "Back up", tidy: "Tidy the books", crashlogs: "Delete crash logs", delete_books: "Delete books", download_books: "Send books", move_books: "Move books",
  report: "Report its books", highlights: "Send highlights", stats: "Sync reading statistics", download_nightstand: "Download the Nightstand", tidy_names: "Tidy book names",
  selftest: "Self-test and send the log", update_plugin: "Update the plugin", stop: "Stop what it's doing", resume_auto: "Resume the automatic jobs", set_auto: "Automatic switch" };
// the plugin's automatic switches (NovelCheck > Automatic on the Kindle)
const SWITCH = { auto_report: "Report books when online", auto_download: "Download Nightstand when online", auto_stats: "Sync reading statistics when online",
  auto_highlights: "Send highlights when online", auto_finish: "Ask for stars at the end of a book", auto_backup: "Back up the whole Kindle (once a day, on the charger)",
  auto_kindle: "Copy the Kindle's books when online", auto_crashlogs: "Delete crash logs when online" };
const mb = (b) => (b >= 1e6 ? `${(b / 1e6).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1e3))} KB`);

export async function renderKindleTasks(host, device = "") {
  host.textContent = "Loading…";
  const r = await get(`/api/me/kindle-tasks${device ? `?device=${encodeURIComponent(device)}` : ""}`).catch(() => null);
  if (!r) { host.textContent = "Couldn't load that just now."; return; }
  if (!r.devices.length) { host.textContent = "No Kindle yet. On the Kindle, run NovelCheck > Report books on this device."; return; }
  const dev = r.device;
  const task = (t) => `<li class="flex items-center justify-between gap-2 py-1"><span class="min-w-0"><b>${esc(KIND[t.kind] || t.kind)}</b> <span class="text-slate-400">${esc(t.device || "any Kindle")}</span>
    ${t.note ? `<span class="block text-xs text-slate-400">${esc(t.note)}</span>` : ""}</span>
    ${t.status === "waiting" ? `<button type="button" data-kt-cancel="${t.id}" class="btn-ghost shrink-0 py-0.5 text-xs">Take back</button>` : `<span class="shrink-0 text-xs ${t.status === "done" ? "text-emerald-400" : "text-slate-400"}">${esc(t.status)}</span>`}</li>`;
  host.innerHTML = `<p class="mb-2">Your Kindle does these at its next sync. Only backed-up books can be deleted.</p>
    <label class="mb-2 block text-xs text-slate-400">Kindle <select id="kt-dev" class="input mt-1">${r.devices.map((d) => `<option ${d.name === dev ? "selected" : ""}>${esc(d.name)}</option>`).join("")}</select></label>
    <p class="mb-2 flex flex-wrap gap-2">${["backup", "tidy", "crashlogs", "report", "highlights", "stats", "download_nightstand", "tidy_names", "selftest", "update_plugin", "stop", "resume_auto"]
      .map((k) => `<button type="button" data-kt="${k}" class="btn-secondary py-1 text-sm">${esc(k === "tidy" ? "Tidy its books into Books/NovelCheck" : KIND[k])}</button>`).join("")}</p>
    <details class="mb-3"><summary class="cursor-pointer text-sm text-slate-300">Its automatic switches</summary>
      <ul class="mt-1 divide-y divide-slate-800 text-sm">${Object.entries(SWITCH).map(([k, l]) => `<li class="flex items-center justify-between gap-2 py-1"><span class="min-w-0">${esc(l)}</span>
        <span class="flex shrink-0 gap-1"><button type="button" data-kt-sw="${k}" data-on="1" class="btn-ghost py-0.5 text-xs">Turn on</button><button type="button" data-kt-sw="${k}" data-on="0" class="btn-ghost py-0.5 text-xs">Turn off</button></span></li>`).join("")}</ul>
      <p class="mt-1 text-xs text-slate-500">Changes apply at the next sync. A plugin update needs a KOReader restart.</p></details>
    ${(r.duplicates || []).length ? `<h4 class="label">The same book more than once (${r.duplicates.length})</h4>
      <p class="mb-1 text-xs text-slate-400">★ is the copy you read furthest. The others are deleted at the next sync.</p>
      <ul class="mb-3 divide-y divide-slate-800 text-sm">${r.duplicates.map((g, gi) => `<li class="py-1.5"><b>${esc(g[0].title || g[0].filename)}</b>
        <ul class="mt-1 space-y-0.5 text-xs text-slate-400">${g.map((f, i) => `<li>${i === 0 ? "★ " : ""}${esc(f.filename)} · ${mb(f.size)} · ${Math.round((f.percent || 0) * 100)}% read${f.backed_up ? "" : " · not backed up"}</li>`).join("")}</ul>
        <button type="button" data-kt-keep="${gi}" class="btn-ghost mt-1 py-0.5 text-xs">Keep the ★ copy, delete the others</button></li>`).join("")}</ul>` : ""}
    <h4 class="label">Books on this Kindle (${r.files.length})</h4>
    <ul class="max-h-80 divide-y divide-slate-800 overflow-auto text-sm">${r.files.map((f) => `<li class="flex items-center gap-2 py-1"><input type="checkbox" data-kt-file="${esc(f.filename)}">
      <span class="min-w-0 flex-1"><span class="block break-words">${esc(f.title || f.filename)}${f.author ? ` <span class="text-slate-400">· ${esc(f.author)}</span>` : ""}</span>
      <span class="block break-words text-xs text-slate-500">${esc(f.filename)} · ${mb(f.size)}</span></span>
      <span class="shrink-0 text-xs ${f.backed_up ? "text-emerald-400" : "text-amber-400"}">${f.backed_up ? "backed up ✓" : "not backed up"}</span></li>`).join("") || `<li class="py-2 text-slate-500">No books reported yet.</li>`}</ul>
    <p class="mt-2 flex flex-wrap items-center gap-2"><button type="button" data-kt="delete_books" class="btn-secondary py-1 text-sm">Delete the ticked books from the Kindle</button>
      <input id="kt-folder" class="input w-44" placeholder="folder under Books" maxlength="60"><button type="button" data-kt="move_books" class="btn-secondary py-1 text-sm">Move the ticked books there</button></p>
    <h4 class="label mt-4">Send library books to this Kindle</h4>
    <p class="flex gap-2"><input id="kt-q" class="input flex-1" placeholder="Search title or author"><button type="button" id="kt-search" class="btn-secondary py-1 text-sm">Search</button></p>
    <ul id="kt-hits" class="mt-1 divide-y divide-slate-800 text-sm"></ul>
    <h4 class="label mt-4">Tasks</h4>
    <ul class="divide-y divide-slate-800 text-sm">${r.tasks.map(task).join("") || `<li class="py-1 text-slate-500">None yet.</li>`}</ul>`;
  const again = () => renderKindleTasks(host, host.querySelector("#kt-dev")?.value || dev);
  const make = async (kind, extra = {}, msg = "Queued: it happens at the Kindle's next sync") => {
    const res = await attempt(() => post("/api/me/kindle-tasks", { device: host.querySelector("#kt-dev").value, kind, ...extra }), msg);
    if (res) again();
  };
  host.querySelector("#kt-dev").onchange = again;
  host.onclick = async (e) => {
    const keep = e.target.closest("[data-kt-keep]");
    if (keep) {
      const g = r.duplicates[Number(keep.dataset.ktKeep)];
      const others = g.slice(1).map((f) => f.filename);
      if (confirm(`Keep "${g[0].filename}" and delete ${others.length} other cop${others.length === 1 ? "y" : "ies"}?`)) make("delete_books", { filenames: others });
      return;
    }
    const sw = e.target.closest("[data-kt-sw]");
    if (sw) return make("set_auto", { switch: sw.dataset.ktSw, on: sw.dataset.on === "1" }, `Queued: ${SWITCH[sw.dataset.ktSw]} ${sw.dataset.on === "1" ? "on" : "off"} at the next check-in`);
    const k = e.target.closest("[data-kt]")?.dataset.kt, c = e.target.closest("[data-kt-cancel]")?.dataset.ktCancel, s = e.target.closest("#kt-search"), q = e.target.closest("[data-kt-send]")?.dataset.ktSend;
    const ticked = () => [...host.querySelectorAll("[data-kt-file]:checked")].map((x) => x.dataset.ktFile);
    if (c) { await attempt(() => del(`/api/me/kindle-tasks/${c}`), "Taken back"); again(); }
    else if (q) make("download_books", { book_ids: [Number(q)] });
    else if (s) {
      const list = await get(`/api/books?q=${encodeURIComponent(host.querySelector("#kt-q").value)}&limit=8`).catch(() => null);
      const books = Array.isArray(list) ? list : list?.books || [];
      host.querySelector("#kt-hits").innerHTML = books.map((b) => `<li class="flex items-center justify-between gap-2 py-1"><span class="min-w-0 break-words">${esc(b.title)} <span class="text-slate-400">· ${esc(b.author || "")}</span></span>
        <button type="button" data-kt-send="${b.id}" class="btn-ghost shrink-0 py-0.5 text-xs">Send</button></li>`).join("") || `<li class="py-1 text-slate-500">Nothing found.</li>`;
    } else if (k && !["delete_books", "move_books"].includes(k)) make(k);
    else if (k === "delete_books") { const f = ticked(); if (f.length && confirm(`Delete ${f.length} book(s) from the Kindle? NovelCheck keeps a backup.`)) make(k, { filenames: f }); }
    else if (k === "move_books") { const f = ticked(); if (f.length) make(k, { filenames: f, folder: host.querySelector("#kt-folder").value }); }
  };
}
