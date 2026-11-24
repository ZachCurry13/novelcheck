// The admin area's tabs: the Admin page's sections plus Deep Scan, Usage and
// Integrations (the page that took over from System checks), at the top of each of those pages (so they need no place in
// the main menu). Sections switch in place on the Admin page; the others are
// pages of their own. Editors see the first two; other admins see the ones
// their areas cover (the main admin gives areas).
import { canHere } from "./ui.js";
import { tabStrip } from "./tabs.js";

// [key, icon, label, shown when]: no rule = every parent. Five tabs; the pages of their own (Deep Scan, Integrations,
// Usage) sit under the tab they belong to, in a second strip.
const TABS = [
  ["ai", "🤖", "AI & Scans"],
  ["users", "👪", "People & Rules"],
  ["cleanup", "🧹", "Library", () => canHere("calibre")],
  ["delivery", "📬", "E-readers & Delivery", () => canHere("calibre") || canHere("services") || canHere("system")],
  ["system", "⚙️", "System", () => canHere("system")],
];

// SUBS are each tab's parts: [key, label, address, switches in place on the Admin page, shown when].
const SUBS = {
  ai: [["ai", "AI settings", "#/admin?tab=ai", true], ["@deepscan", "Deep Scan", "#/deepscan", false, () => canHere("deep")]],
  system: [["system", "Toggles", "#/admin?tab=system", true], ["@system", "Integrations and server check", "#/system", false], ["@usage", "Usage", "#/usage", false]],
};
const parentOf = { "@deepscan": "ai", "@system": "system", "@usage": "system" };

const hrefOf = (k) => `#/admin?tab=${k}`;
// tabsFor lists the tabs this person may open.
export const tabsFor = () => TABS.filter(([, , , shown]) => !shown || shown());

// adminPages are the pages of their own under the tabs (for search): [key, label, address].
export const adminPages = () => Object.values(SUBS).flat().filter(([k, , , , shown]) => k.startsWith("@") && (!shown || shown())).map(([k, l, href]) => [k, l, href]);

const subStrip = (active) => {
  const list = (SUBS[parentOf[active] || active] || []).filter(([, , , , shown]) => !shown || shown());
  return list.length < 2 ? "" : tabStrip({ id: "admin-subtabs-strip", label: "Part", active, tabs: list.map(([k, l, href, inPlace]) => [k, l, href, inPlace]) });
};

// adminNavHTML: active is a section key ("users") or "@page". The same tab strip as Settings and the rest of the
// app, plus the second strip for a tab with parts.
export function adminNavHTML(active) {
  return tabStrip({
    id: "admin-tabs", label: "Admin", active: parentOf[active] || active,
    tabs: tabsFor().map(([k, , label]) => [k, label, hrefOf(k), true]),
  }) + `<div id="admin-subtabs">${subStrip(active)}</div>`;
}

// markAdminSection keeps the strip in step when the Admin page switches
// sections in place.
export function markAdminSection(root, tab) {
  root.querySelectorAll("#admin-tabs [data-tab]").forEach((b) => {
    b.setAttribute("aria-selected", String(b.dataset.tab === tab));
    b.classList.toggle("active", b.dataset.tab === tab);
  });
  const sub = root.querySelector("#admin-subtabs");
  if (sub) sub.innerHTML = subStrip(tab);
}
