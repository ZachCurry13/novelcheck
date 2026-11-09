// Settings → Developer: what to send when something doesn't work. This browser's
// record of recent taps and errors; for admins also the logs the KOReader plugin
// sent and the server's own report. Everything is copied as plain text.
import { get, del } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { clearNotes, report } from "./debugrec.js";
import { copyText } from "./copy.js";

const appVersion = () => document.querySelector("[data-app-version]")?.textContent?.trim() || document.body.textContent.match(/Version\s+(\S+)/)?.[1] || "";

export async function renderDevTab(host, user) {
  const admin = user.role === "admin";
  host.innerHTML = `<div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">🛠 This browser</h2>
      <p class="text-sm text-slate-400">NovelCheck keeps a short record of the pages you open, the buttons you press (their names only, never what you type) and any errors. If something doesn't work, repeat it, then copy this and send it.</p>
      <pre id="dev-web" class="max-h-64 overflow-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-3 text-xs text-slate-300"></pre>
      <div class="flex flex-wrap gap-2"><button type="button" data-dev="copy" class="btn-primary">📋 Copy report</button>
        <button type="button" data-dev="refresh" class="btn-secondary">Refresh</button>
        <button type="button" data-dev="clear" class="btn-ghost">Clear</button></div></div>
    <div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">📖 Kindle and KOReader</h2>
      <p class="text-sm text-slate-400">In KOReader open <b>NovelCheck → Developer tools</b>: <i>Run self-test</i> checks the connection and the plugin's files, <i>View log</i> shows what the plugin did, and <i>Send log to NovelCheck</i> sends it here.
        If the plugin doesn't show up in KOReader at all, look for <code>LOAD_ERROR.txt</code> in its folder, or send KOReader's <code>crash.log</code>.</p>
      ${admin ? `<div id="dev-logs" class="space-y-2"></div>` : `<p class="text-xs text-slate-500">Logs sent from a device are read by an admin.</p>`}</div>
    ${admin ? `<div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">🩺 Server report</h2>
      <p class="text-sm text-slate-400">Version, settings (never keys or passwords), the rating worker, recent problems and the remote-access log.</p>
      <button type="button" data-dev="server" class="btn-secondary">📋 Copy server report</button></div>` : ""}`;
  const show = () => {
    host.querySelector("#dev-web").textContent = report(appVersion());
  };
  show();
  const logs = async () => {
    const box = host.querySelector("#dev-logs");
    if (!box) return;
    const r = await attempt(() => get("/api/admin/device-logs"));
    if (!r) return;
    box.innerHTML = r.logs.length ? r.logs.map((l) => `<div class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-2 text-sm">
        <span class="min-w-0"><b>${esc(l.device || "device")}</b> · ${esc(l.username)} · plugin ${esc(l.version)}<br><span class="text-xs text-slate-400">${esc(new Date(l.created_at.replace(" ", "T") + "Z").toLocaleString())} · ${Math.round(l.size / 1024) || 1} KB</span></span>
        <span class="flex gap-2"><button type="button" data-dev="log" data-id="${l.id}" class="btn-secondary py-1 text-sm">📋 Copy</button>
          <button type="button" data-dev="rm" data-id="${l.id}" class="btn-ghost py-1 text-sm">Delete</button></span></div>`).join("")
      : `<p class="text-sm text-slate-400">No logs yet. Send one from the plugin's Developer tools.</p>`;
  };
  logs();
  host.onclick = async (e) => {
    const b = e.target.closest("[data-dev]");
    if (!b) return;
    const k = b.dataset.dev;
    if (k === "copy") copyText(report(appVersion()));
    else if (k === "refresh") show();
    else if (k === "clear") {
      clearNotes();
      show();
      toast("Cleared");
    } else if (k === "server") {
      const r = await attempt(() => fetch("/api/admin/diagnostics").then((x) => x.text()));
      if (r) copyText(r);
    } else if (k === "log") {
      const r = await attempt(() => fetch(`/api/admin/device-logs/${b.dataset.id}`).then((x) => x.text()));
      if (r) copyText(r);
    } else if (k === "rm" && (await attempt(() => del(`/api/admin/device-logs/${b.dataset.id}`), "Deleted"))) logs();
  };
}
