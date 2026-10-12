// The admin area's tabs: the Admin page's sections plus Deep Scan, Usage and
// System checks, at the top of each of those pages (so they need no place in
// the main menu). Sections switch in place on the Admin page; the others are
// pages of their own. Editors see the first two; other admins see the ones
// their areas cover (the main admin gives areas).
import { canHere } from "./ui.js";
import { tabStrip } from "./tabs.js";

// [key, icon, label, shown when]: no rule = every parent.
const TABS = [
  ["ai", "🤖", "AI & Scans"],
  ["users", "👪", "Users & Rules"],
  ["delivery", "📬", "Delivery & Services", () => canHere("calibre") || canHere("services") || canHere("system")],
  ["system", "⚙️", "System & Toggles", () => canHere("system")],
  ["@deepscan", "🧬", "Deep Scan", () => canHere("deep")],
  ["@usage", "📈", "Usage", () => canHere("system")],
  ["@system", "🩺", "System checks", () => canHere("system")],
];

const hrefOf = (k) => (k.startsWith("@") ? `#/${k.slice(1)}` : `#/admin?tab=${k}`);
// tabsFor lists the tabs this person may open.
export const tabsFor = () => TABS.filter(([, , , shown]) => !shown || shown());

// adminNavHTML: active is a section key ("users") or "@page". The same tab
// strip as Settings and the rest of the app.
export function adminNavHTML(active) {
  return tabStrip({
    id: "admin-tabs", label: "Admin", active,
    tabs: tabsFor().map(([k, icon, label]) => [k, `${icon} ${label}`, hrefOf(k), !k.startsWith("@")]),
  });
}

// markAdminSection keeps the strip in step when the Admin page switches
// sections in place.
export function markAdminSection(root, tab) {
  root.querySelectorAll("#admin-tabs [data-tab]").forEach((b) => b.setAttribute("aria-selected", String(b.dataset.tab === tab)));
}
