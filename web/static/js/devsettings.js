// Settings → Developer: what to send when something doesn't work. This browser's
// record of recent taps and errors (sent by itself when something breaks, or by
// hand with a note); for admins also the logs the KOReader plugin sent, the
// server's errors, the secret link to all of it, and the server report.
import { get, post, del } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { clearNotes, report, autoSendOn, setAutoSend } from "./debugrec.js";
import { copyText } from "./copy.js";
import { renderLink, renderErrors, handleDevLink } from "./devlink.js";
import { activityCardHTML, bindActivityCard } from "./activitycard.js";

const when = (t) => {
  const d = new Date(/[zZ]$|[+-]\d\d:?\d\d$/.test(t) ? t : t.replace(" ", "T") + "Z");
  return isNaN(d) ? t : d.toLocaleString();
};
const appVersion = () => document.querySelector("[data-app-version]")?.textContent?.trim() || document.body.textContent.match(/Version\s+(\S+)/)?.[1] || "";

const browserCard = () => `<div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">🛠 This browser</h2>
  <p class="text-sm text-slate-400">NovelCheck keeps a short record of the pages you open, the buttons you press (their names only, never what you type) and any errors. If something doesn't work, repeat it, say what you were doing, and press Send.</p>
  <pre id="dev-web" class="max-h-56 overflow-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-3 text-xs text-slate-300"></pre>
  <textarea id="dev-note" class="input" rows="2" maxlength="500" placeholder="What were you doing? (optional)" aria-label="What were you doing"></textarea>
  <div class="flex flex-wrap gap-2"><button type="button" data-dev="send" class="btn-primary">📤 Send report to NovelCheck</button>
    <button type="button" data-dev="copy" class="btn-secondary">📋 Copy report</button>
    <button type="button" data-dev="refresh" class="btn-secondary">Refresh</button>
    <button type="button" data-dev="clear" class="btn-ghost">Clear</button></div>
  <label class="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" data-dev="auto" ${autoSendOn() ? "checked" : ""}> Send a report by itself when something breaks (at most one every few minutes)</label></div>`;

const kindleCard = (admin) => `<div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">📖 Kindle and KOReader</h2>
  <p class="text-sm text-slate-400">In KOReader open <b>NovelCheck → Developer tools</b>: <i>Run self-test</i> checks the connection and the plugin's files and sends the result here by itself, <i>View log</i> shows what the plugin did, and <i>Send log to NovelCheck</i> sends it with a note. A plugin error also sends the log on its own.
    If the plugin doesn't show up in KOReader at all, look for <code>LOAD_ERROR.txt</code> in its folder, or send KOReader's <code>crash.log</code>.</p>
  ${admin ? `<div class="flex items-center justify-between gap-2 pt-1"><h3 class="font-semibold">Bug reports</h3><button type="button" data-dev="rmall" class="btn-ghost py-1 text-sm">Delete all</button></div><div id="dev-logs" class="space-y-2"></div>` : `<p class="text-xs text-slate-500">Reports sent from a device are read by an admin.</p>`}</div>`;

const adminCards = () => `<div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">⚠️ Server errors</h2>
    <p class="text-sm text-slate-400">Every failed request with its real cause. When the app shows an error code like <b>E-3f2a</b>, it is in this list.</p><div id="dev-errors" class="space-y-2"></div></div>
  <div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">🔗 Link to recent bug reports</h2>
    <p class="text-sm text-slate-400">A secret link that shows the server report, the server errors and the latest reports from phones, browsers and e-readers as plain text, with no sign-in, so it can be pasted to whoever is fixing the problem. Reports are deleted once they are dealt with. It switches itself off after 7 days; anyone who has it can read the reports until then.</p>
    <div id="dev-link" class="space-y-2"></div></div>
  <div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">🩺 Server report</h2>
    <p class="text-sm text-slate-400">Version, settings (never keys or passwords), the rating worker, recent problems and the remote-access log.</p>
    <button type="button" data-dev="server" class="btn-secondary">📋 Copy server report</button></div>`;

export async function renderDevTab(host, user) {
  const admin = user.role === "admin";
  host.innerHTML = (admin ? activityCardHTML() : "") + browserCard() + kindleCard(admin) + (admin ? adminCards() : "");
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
        <span class="min-w-0"><b>#${l.id} ${esc(l.device || "device")}</b> · ${esc(l.username)} · ${l.device.startsWith("Browser") ? "app" : "plugin"} ${esc(l.version)}<br><span class="text-xs text-slate-400">${esc(when(l.created_at))} · ${Math.round(l.size / 1024) || 1} KB</span></span>
        <span class="flex gap-2"><button type="button" data-dev="log" data-id="${l.id}" class="btn-secondary py-1 text-sm">📋 Copy</button>
          <button type="button" data-dev="rm" data-id="${l.id}" class="btn-ghost py-1 text-sm">Delete</button></span></div>`).join("")
      : `<p class="text-sm text-slate-400">No reports. Anything the Kindle or a browser sends shows up here.</p>`;
  };
  logs();
  if (admin) {
    bindActivityCard(host);
    renderLink(host.querySelector("#dev-link"));
    renderErrors(host.querySelector("#dev-errors"));
  }
  host.onclick = async (e) => {
    if (await handleDevLink(e, host)) return;
    const b = e.target.closest("[data-dev]");
    if (!b) return;
    const k = b.dataset.dev;
    if (k === "copy") copyText(report(appVersion()));
    else if (k === "auto") setAutoSend(b.checked);
    else if (k === "send") {
      const note = host.querySelector("#dev-note").value.trim();
      const text = (note ? `NOTE: ${note}\n\n` : "") + report(appVersion());
      if (await attempt(() => post("/api/me/bug-report", { text }), "Sent")) logs();
    } else if (k === "refresh") show();
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
    } else if (k === "rmall") {
      if (confirm("Delete every bug report?") && (await attempt(() => del("/api/admin/device-logs"), "Deleted"))) logs();
    } else if (k === "rm" && (await attempt(() => del(`/api/admin/device-logs/${b.dataset.id}`), "Deleted"))) logs();
  };
}
