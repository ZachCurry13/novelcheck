// Profile: the KOReader plugin, the Kindle app and your e-readers. Download the plugin already
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
    <p class="text-xs text-slate-500">Already connected to your account; nothing to type on the e-reader. Keep the file private.</p>
    <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
      <li>Unzip it, and connect the e-reader to the computer by USB.</li>
      <li>Copy the <b>novelcheck.koplugin</b> folder into <b>koreader/plugins/</b> on the e-reader (replace the old one when updating).</li>
      <li>Restart KOReader. It offers <b>Set up NovelCheck sync?</b>: tap <b>Set up</b>. That connects progress sync and reading statistics for you (no WebDAV or catalog to add by hand) and asks to restart once more. The same button is always in the top menu under <b>Tools → NovelCheck → Set up everything</b>.</li>
    </ol>`;
}

// kindleAppHTML offers the Kindle app: NovelCheck on a jailbroken Kindle that has no KOReader (kindle-app/).
const kindleAppHTML = `<details class="space-y-2 border-t border-slate-800 pt-2 text-sm"><summary class="label cursor-pointer">No KOReader? The Kindle app</summary>
    <p class="text-slate-400">For a jailbroken Kindle with KindleForge. A NovelCheck "book" on the Kindle's home screen opens your library: search, see the spice level, download (as AZW3, so Amazon's reader opens it) and report the Kindle's books, including the ones in the Amazon cloud.</p>
    <a href="/api/me/kindle-app.zip" download class="btn-primary inline-flex">⬇ Download the Kindle app</a>
    <p class="text-xs text-slate-500">Connected to your account. Keep the file private.</p>
    <ol class="list-decimal space-y-1 pl-5 text-slate-300">
      <li>Connect the Kindle to the computer by USB and unzip the file onto it, merging the <b>documents</b> and <b>extensions</b> folders.</li>
      <li>Eject the Kindle. <b>NovelCheck</b> shows in its library: tap it.</li>
      <li>If downloads say they need KindleForge's helper, open KindleForge once, then NovelCheck again.</li>
      <li>Something not working? Tap <b>NovelCheck Check</b> in the Kindle's library. It tests the Kindle and sends the result to NovelCheck's bug log.</li>
    </ol></details>`;

export async function renderPluginCard(host) {
  const s = await get("/api/me/reading-sync").catch(() => null);
  if (!s) return;
  const devices = s.devices || [];
  const badge = (d) => (String(d.plugin_version).startsWith("app ") ? `<span class="chip-none">Kindle ${esc(d.plugin_version)}</span>`
    : !d.plugin_version ? `<span class="chip-pending">plugin before 1.38</span>`
    : older(d.plugin_version, s.plugin_latest) ? `<span class="chip-wait">plugin ${esc(d.plugin_version)}: update to ${esc(s.plugin_latest)}</span>`
      : `<span class="chip-none">plugin ${esc(d.plugin_version)}</span>`);
  host.innerHTML = `<h2 class="text-lg font-semibold">KOReader plugin</h2>
    <p class="text-sm text-slate-400">Nightstand on your e-reader, one-tap downloads, a list of what's on it with delete, and reports back to NovelCheck.</p>
    ${pluginStepsHTML("/api/me/koplugin.zip", s.plugin_latest)}
    ${kindleAppHTML}
    <label class="toggle min-h-[2.5rem] text-sm"><input type="checkbox" data-autoadd ${s.kindle_autoadd ? "checked" : ""}> Add the books on my e-reader to your Nightstand (started ones show as Currently reading)</label>
    <button type="button" data-match class="btn-secondary w-full text-sm">Match books on my e-readers${s.kindle_attention ? ` (${s.kindle_attention} to look at)` : ""}</button>
    <div class="space-y-1 border-t border-slate-800 pt-2 text-sm"><p class="label">Your e-readers</p>
      ${devices.length ? `<ul class="space-y-2">${devices.map((d) => `<li><b>${esc(d.name)}</b> ${badge(d)}<br>
        <span class="text-xs text-slate-400">${d.books} book${d.books === 1 ? "" : "s"}, ${d.in_library} in your library · reported ${esc(ago(d.reported_at))}</span></li>`).join("")}</ul>`
      : `<p class="text-xs text-slate-500">None yet. After installing, open <b>NovelCheck → Report books on this device</b> in KOReader.</p>`}</div>`;
  host.querySelector("[data-match]").onclick = () => openKindleBooks(() => renderPluginCard(host));
  host.querySelector("[data-autoadd]").onchange = (e) => attempt(() => put("/api/me/kindle-autoadd", { enabled: e.target.checked }), e.target.checked ? "Books on your e-reader will join your Nightstand" : "Switched off");
}
