// The admin area's tabs: the Admin page's sections plus Deep Scan, Usage and
// System checks, at the top of each of those pages (so they need no place in
// the main menu). Sections switch in place on the Admin page; the others are
// pages of their own. Editors see the first two.
const TABS = [
  ["ai", "🤖 AI & Scans"],
  ["users", "👪 Users & Rules"],
  ["delivery", "📬 Delivery & Services", true],
  ["system", "⚙️ System & Toggles", true],
  ["@deepscan", "🧬 Deep Scan", true],
  ["@usage", "📈 Usage", true],
  ["@system", "🩺 System checks", true],
];

// adminNavHTML: active is a section key ("users") or "@page".
export function adminNavHTML(active, isAdmin) {
  return `<nav id="admin-tabs" class="-mx-4 mb-4 flex gap-1 overflow-x-auto border-b border-slate-800 px-4" aria-label="Admin">
    ${TABS.filter(([, , adminOnly]) => isAdmin || !adminOnly).map(([k, label]) => {
      const page = k.startsWith("@");
      const href = page ? `#/${k.slice(1)}` : `#/admin?tab=${k}`;
      return `<a href="${href}" ${page ? "" : `data-tab="${k}"`} class="nav-link shrink-0 rounded-b-none${k === active ? " active" : ""}">${label}</a>`;
    }).join("")}</nav>`;
}
