// Profile: is reading sync really working? When KOReader last reported its
// place (progress sync) and when its statistics last arrived, with a nudge
// when nothing has.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { ago } from "./progress.js";

export async function renderSyncStatus(host) {
  const s = await get("/api/me/reading-sync").catch(() => null);
  if (!s) return;
  const row = (label, at, extra, hint) => `<li class="flex items-start gap-2"><span aria-hidden="true">${at ? "✅" : "⚪"}</span>
    <span><b>${label}:</b> ${at ? `${esc(ago(at))}${extra ? ` · ${esc(extra)}` : ""}` : `never yet. ${hint}`}</span></li>`;
  host.innerHTML = `<h2 class="text-lg font-semibold">🔄 Reading sync</h2>
    <ul class="space-y-1 text-sm">
      ${row("Progress from KOReader", s.progress_at, s.progress_device, "Open a book from NovelCheck in KOReader with Progress sync on (KOReader setup has the steps).")}
      ${row("Reading statistics", s.stats_at, s.stats_books ? `${s.stats_books} book${s.stats_books === 1 ? "" : "s"}` : "", "In KOReader: Reading statistics → Synchronize now.")}
    </ul>
    ${(s.devices || []).length ? `<div class="space-y-1 border-t border-slate-800 pt-2 text-sm"><p class="label">📱 Your e-readers</p>
      <ul class="space-y-1">${s.devices.map((d) => `<li><b>${esc(d.name)}</b>: ${d.books} book${d.books === 1 ? "" : "s"}, ${d.in_library} in your library · reported ${esc(ago(d.reported_at))}</li>`).join("")}</ul>
      <p class="text-xs text-slate-500">From the KOReader plugin's <b>Report books on this device</b>.</p></div>` : ""}
    ${(s.webdav || []).length ? `<div class="space-y-1 border-t border-slate-800 pt-2 text-sm"><p class="label">🗂️ Latest Cloud storage (WebDAV) requests</p>
      <ul class="space-y-1">${s.webdav.map((w) => `<li>${w.status < 300 ? "✅" : "⚠️"} ${esc(ago(w.at))}: ${esc(w.method)} <code>${esc(w.path)}</code>${w.name ? ` as <b>${esc(w.name)}</b>` : ""}: ${esc(w.note)}</li>`).join("")}</ul></div>` : `<p class="text-xs text-slate-500">🗂️ No Cloud storage (WebDAV) request has reached NovelCheck since it started. If KOReader says it can't fetch the folder, the address isn't getting here.</p>`}
    <p class="text-xs text-slate-500">Progress syncs as you read (when KOReader's Auto sync is on). Statistics sync when you press Synchronize now.</p>`;
}
