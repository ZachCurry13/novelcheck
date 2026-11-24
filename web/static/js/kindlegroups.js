// Settings → Kindle & KOReader → Your e-readers: the KOReader plugin and the Kindle app on one Kindle are listed
// together. Each report (the plugin's, the app's) says which Kindle it belongs to ("Same Kindle as…"); names that
// match ("Zach Paperwhite" and "Zach Paperwhite (Amazon reader)") are linked by NovelCheck by itself.
import { put } from "./api.js";
import { esc, attempt } from "./ui.js";
import { ago } from "./progress.js";

const isApp = (d) => String(d.plugin_version || "").startsWith("app ");
const groupOf = (d, links) => links[d.name] || d.name;

// devicesHTML lists the Kindles; badge(d) is the version chip (out of date or not).
export function devicesHTML(devices, links, badge) {
  if (!devices.length) return `<p class="text-xs text-slate-500">None yet. After installing, open <b>NovelCheck → Sync now</b> in KOReader, or <b>Sync now</b> in the Kindle app.</p>`;
  const groups = new Map();
  for (const d of devices) {
    const g = groupOf(d, links);
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g).push(d);
  }
  const names = [...groups.keys()];
  const part = (d, g) => `<li class="flex flex-wrap items-center justify-between gap-2 py-1"><span class="min-w-0">${isApp(d) ? "📱 Kindle app" : "📖 KOReader plugin"} ${badge(d)}<br>
      <span class="text-xs text-slate-400">${esc(d.name)} · ${d.books} book${d.books === 1 ? "" : "s"}, ${d.in_library} in your library · reported ${esc(ago(d.reported_at))}</span></span>
    ${names.length > 1 || groups.get(g).length > 1 ? `<label class="text-xs text-slate-400">Same Kindle as <select data-link="${esc(d.name)}" class="input mt-1 py-0.5 text-xs">
      <option value="">(its own Kindle)</option>${names.filter((n) => n !== g || groups.get(g).length > 1).map((n) => `<option value="${esc(n)}" ${n === g && groups.get(g).length > 1 ? "selected" : ""}>${esc(n)}</option>`).join("")}</select></label>` : ""}</li>`;
  // a plugin and an app that are not linked yet: offer the link
  const loose = [...groups.entries()].filter(([, ds]) => ds.length === 1).map(([, ds]) => ds[0]);
  const plugin = loose.find((d) => !isApp(d)), app = loose.find(isApp);
  const hint = plugin && app ? `<p class="rounded bg-slate-800/60 p-2 text-xs text-slate-300">Are <b>${esc(plugin.name)}</b> (KOReader) and <b>${esc(app.name)}</b> (the app) the same Kindle?
      <button type="button" data-link-pair="${esc(plugin.name)}|${esc(app.name)}" class="btn-secondary ml-1 py-0.5 text-xs">Yes, same Kindle</button></p>` : "";
  return `${hint}<ul class="space-y-3">${[...groups.entries()].map(([g, ds]) => `<li class="rounded-lg border border-slate-800 p-2"><p class="font-semibold">${esc(g)}${ds.length > 1 ? ` <span class="text-xs font-normal text-slate-400">· plugin and app</span>` : ""}</p>
      <ul class="divide-y divide-slate-800">${ds.map((d) => part(d, g)).join("")}</ul></li>`).join("")}</ul>`;
}

// bindDevices wires the selects and the pair button; done() redraws.
export function bindDevices(host, devices, links, done) {
  const set = (device, kindle) => put("/api/me/device-link", { device, kindle });
  host.onchange = async (e) => {
    const sel = e.target.closest("[data-link]");
    if (!sel) return;
    const device = sel.dataset.link, target = sel.value;
    await attempt(async () => {
      if (target) { // join that Kindle: every report that is part of it is named by it
        for (const d of devices) if (groupOf(d, links) === target && !links[d.name]) await set(d.name, target);
      }
      await set(device, target);
    }, target ? `Linked: one Kindle, ${target}` : "Unlinked");
    done();
  };
  host.onclick = async (e) => {
    const b = e.target.closest("[data-link-pair]");
    if (!b) return;
    const [plugin, app] = b.dataset.linkPair.split("|");
    await attempt(async () => { await set(plugin, plugin); await set(app, plugin); }, `One Kindle: ${plugin}`);
    done();
  };
}
