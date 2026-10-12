// Profile: the KOReader plugin and your e-readers. Download the plugin already
// connected to your account, see each device that reported, which plugin
// version it runs, and when the plugin there is out of date.
import { get, put } from "./api.js";
import { esc, attempt } from "./ui.js";
import { ago } from "./progress.js";
import { openKindleBooks } from "./kindlebooks.js";

// semverLess says whether version a is older than b ("1.2.0" < "1.10.0").
export function older(a, b) {
  const x = String(a || "0").split(".").map(Number), y = String(b || "0").split(".").map(Number);
  for (let i = 0; i < 3; i++) {
    if ((x[i] || 0) !== (y[i] || 0)) return (x[i] || 0) < (y[i] || 0);
  }
  return false;
}

// pluginStepsHTML is the install steps with a download button for zipUrl.
export function pluginStepsHTML(zipUrl, latest) {
  return `<a href="${esc(zipUrl)}" download class="btn-primary inline-flex">⬇ Download the plugin${latest ? ` (${esc(latest)})` : ""}</a>
    <p class="text-xs text-slate-500">It is already connected to this account: your server, username and sync code are inside, so there is nothing to type on the e-reader. Keep the file private.</p>
    <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
      <li>Unzip it, and connect the e-reader to the computer by USB.</li>
      <li>Copy the <b>novelcheck.koplugin</b> folder into <b>koreader/plugins/</b> on the e-reader (replace the old one when updating).</li>
      <li>Restart KOReader. It offers <b>Set up NovelCheck sync?</b>: tap <b>Set up</b>. That connects progress sync and reading statistics for you (no WebDAV or catalog to add by hand) and asks to restart once more. The same button is always in the top menu under <b>🛠 Tools → NovelCheck → Set up everything</b>.</li>
    </ol>`;
}

export async function renderPluginCard(host) {
  const s = await get("/api/me/reading-sync").catch(() => null);
  if (!s) return;
  const devices = s.devices || [];
  const badge = (d) => (!d.plugin_version ? `<span class="chip-pending">plugin before 1.38</span>`
    : older(d.plugin_version, s.plugin_latest) ? `<span class="chip-wait">plugin ${esc(d.plugin_version)}: update to ${esc(s.plugin_latest)}</span>`
      : `<span class="chip-none">plugin ${esc(d.plugin_version)}</span>`);
  host.innerHTML = `<h2 class="text-lg font-semibold">📱 KOReader plugin</h2>
    <p class="text-sm text-slate-400">Up Next on your e-reader, one-tap downloads, a list of what's on it with delete, and reports back to NovelCheck.</p>
    ${pluginStepsHTML("/api/me/koplugin.zip", s.plugin_latest)}
    <label class="toggle min-h-[2.5rem] text-sm"><input type="checkbox" data-autoadd ${s.kindle_autoadd ? "checked" : ""}> Add the books on my e-reader to Up Next (started ones show as Currently reading)</label>
    <button type="button" data-match class="btn-secondary w-full text-sm">🔗 Match books on my e-readers${s.kindle_attention ? ` (${s.kindle_attention} to look at)` : ""}</button>
    <div class="space-y-1 border-t border-slate-800 pt-2 text-sm"><p class="label">Your e-readers</p>
      ${devices.length ? `<ul class="space-y-2">${devices.map((d) => `<li><b>${esc(d.name)}</b> ${badge(d)}<br>
        <span class="text-xs text-slate-400">${d.books} book${d.books === 1 ? "" : "s"}, ${d.in_library} in your library · reported ${esc(ago(d.reported_at))}</span></li>`).join("")}</ul>`
      : `<p class="text-xs text-slate-500">None yet. After installing, open <b>NovelCheck → Report books on this device</b> in KOReader.</p>`}</div>`;
  host.querySelector("[data-match]").onclick = () => openKindleBooks(() => renderPluginCard(host));
  host.querySelector("[data-autoadd]").onchange = (e) => attempt(() => put("/api/me/kindle-autoadd", { enabled: e.target.checked }), e.target.checked ? "Books on your e-reader will join Up Next" : "Switched off");
}
