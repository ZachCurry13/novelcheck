// The admin area's tabs: the Admin page's sections plus Deep Scan, Usage and
// System checks, at the top of each of those pages (so they need no place in
// the main menu). Sections switch in place on the Admin page; the others are
// pages of their own. Editors see the first two. On phones a single button
// names the section and opens all of them as a sheet from the bottom, like
// More, instead of a strip to scroll sideways.
import { esc } from "./ui.js";

const TABS = [
  ["ai", "🤖", "AI & Scans"],
  ["users", "👪", "Users & Rules"],
  ["delivery", "📬", "Delivery & Services", true],
  ["system", "⚙️", "System & Toggles", true],
  ["@deepscan", "🧬", "Deep Scan", true],
  ["@usage", "📈", "Usage", true],
  ["@system", "🩺", "System checks", true],
];

const hrefOf = (k) => (k.startsWith("@") ? `#/${k.slice(1)}` : `#/admin?tab=${k}`);
const tabsFor = (isAdmin) => TABS.filter(([, , , adminOnly]) => isAdmin || !adminOnly);

// adminNavHTML: active is a section key ("users") or "@page".
export function adminNavHTML(active, isAdmin) {
  const cur = TABS.find(([k]) => k === active) || TABS[0];
  // After the page is drawn, the open tab is scrolled into view (not back to the first).
  setTimeout(() => document.querySelector("#admin-tabs .active")?.scrollIntoView({ block: "nearest", inline: "center" }), 0);
  return `<button type="button" data-admin-picker="${isAdmin ? 1 : 0}" data-active="${esc(cur[0])}"
      class="btn-secondary mb-4 w-full justify-between md:hidden">
      <span>${cur[1]} <b data-admin-current>${esc(cur[2])}</b></span><span class="text-xs font-normal text-slate-400">Admin sections ▾</span></button>
    <nav id="admin-tabs" class="-mx-4 mb-4 hidden gap-1 overflow-x-auto border-b border-slate-800 px-4 md:flex" aria-label="Admin">
    ${tabsFor(isAdmin).map(([k, icon, label]) => `<a href="${hrefOf(k)}" ${k.startsWith("@") ? "" : `data-tab="${k}"`}
      data-label="${esc(label)}" data-icon="${icon}" class="nav-link shrink-0 rounded-b-none${k === active ? " active" : ""}">${icon} ${esc(label)}</a>`).join("")}</nav>`;
}

// markAdminSection keeps the phone button's name in step when the Admin page
// switches sections in place.
export function markAdminSection(root, tab) {
  const t = TABS.find(([k]) => k === tab);
  const btn = root.querySelector("[data-admin-picker]");
  if (!t || !btn) return;
  btn.dataset.active = tab;
  btn.querySelector("span").innerHTML = `${t[1]} <b data-admin-current>${esc(t[2])}</b>`;
}

function openSheet(isAdmin, active) {
  let d = document.getElementById("admin-sheet");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "admin-sheet";
    d.className = "dialog";
    document.body.append(d);
    d.addEventListener("click", (e) => {
      if (e.target === d || e.target.closest("[data-close]") || e.target.closest("a")) d.close();
    });
  }
  d.innerHTML = `<div class="space-y-3 p-4">
    <div class="flex items-center justify-between"><h2 class="text-lg font-bold">Admin</h2>
      <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <div class="grid grid-cols-2 gap-2">${tabsFor(isAdmin).map(([k, icon, label]) => `<a href="${hrefOf(k)}"
      class="card flex min-h-[4.5rem] flex-col items-center justify-center gap-1 p-3 text-center text-sm${k === active ? " ring-2 ring-indigo-500" : ""}">
      <span class="text-2xl" aria-hidden="true">${icon}</span>${esc(label)}</a>`).join("")}</div></div>`;
  d.showModal();
}

document.addEventListener("click", (e) => {
  const b = e.target.closest("[data-admin-picker]");
  if (b) openSheet(b.dataset.adminPicker === "1", b.dataset.active);
});
