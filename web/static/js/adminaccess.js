// The main admin's controls on another admin's card (Admin → Users): which
// areas they may reach, and handing over the main admin. The server checks
// both (internal/api/areas_handlers.go).
import { put, post } from "./api.js";
import { esc, attempt } from "./ui.js";

export const AREA_NAMES = { ai: "🤖 AI settings", deep: "🧬 Deep Scans", calibre: "📚 Calibre & cleanup",
  services: "✉️ Email & Discover", system: "🖥️ System", users: "👥 Users" };

const AREA_HELP = {
  ai: "AI engine, keys, models, prices, limits, AI machines, custom AI filters",
  deep: "Start scans, Review, Deep Scan settings",
  calibre: "Library folder, Content server, removing books, formats, duplicates, delete requests",
  services: "Send-to-Kindle email, Discover lists and the NYT key",
  system: "Features, remote access, safe mode, backups, updates, system checks, Usage",
  users: "Other parents' accounts (every admin manages kids)",
};

// accessHTML: the areas as ticks, for the main admin, on another admin's card.
export function accessHTML(u, viewer) {
  if (u.role !== "admin" || u.owner || !viewer?.owner) return "";
  const have = (u.admin_areas || "").split(",");
  return `<fieldset data-access class="space-y-2 rounded-lg bg-slate-800/60 p-3">
    <legend class="px-1 text-sm font-semibold">What ${esc(u.username)} can reach</legend>
    ${Object.entries(AREA_NAMES).map(([k, name]) => `<label class="flex items-start gap-2 text-sm">
      <input type="checkbox" value="${k}" class="mt-1 h-4 w-4 shrink-0" ${have.includes(k) ? "checked" : ""}>
      <span>${esc(name)}<span class="block text-xs text-slate-400">${esc(AREA_HELP[k])}</span></span></label>`).join("")}
    <div class="flex flex-wrap gap-2 pt-1">
      <button type="button" data-uact="access" class="btn-secondary py-1 text-sm">Save access</button>
      <button type="button" data-uact="owner" class="btn-ghost py-1 text-sm" title="They decide what other admins can reach; you keep every area">🔑 Make main admin</button></div>
  </fieldset>`;
}

// accessAction saves the ticks ("access") or hands over the main admin ("owner").
export async function accessAction(act, id, cardEl, name, reload) {
  if (act === "access") {
    const areas = [...cardEl.querySelectorAll("[data-access] input:checked")].map((c) => c.value);
    if (await attempt(() => put(`/api/admin/users/${id}/access`, { areas }), `Access saved for ${name}`)) reload();
  } else if (act === "owner") {
    if (!confirm(`Make ${name} the main admin? They'll decide what the other admins can reach, you included. You keep every area as an ordinary admin.`)) return;
    if (await attempt(() => post(`/api/admin/users/${id}/owner`), `${name} is now the main admin`)) location.reload();
  }
}
