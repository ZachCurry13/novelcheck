// Discover, the Wishlist and Events share one menu item: a switch at the top
// of each. With Discover turned off, the Wishlist has its own menu item and
// the switch keeps the other two.
import { on } from "./modules.js";

export function discoverTabsHTML(active, user) {
  const tabs = [["discover", "🧭 Discover", on(user, "discover")], ["wishlist", "⭐ Wishlist", true], ["events", "🎉 Events", true]]
    .filter(([, , shown]) => shown);
  const tab = (key, label) => `<a href="#/${key}" class="rounded-lg px-2 py-2 text-center text-sm font-semibold ${key === active ? "bg-slate-800 text-white" : "text-slate-400 hover:text-white"}"
    ${key === active ? 'aria-current="page"' : ""}>${label}</a>`;
  return `<div class="mb-4 grid grid-cols-${tabs.length} gap-1 rounded-xl bg-slate-900 p-1 ring-1 ring-slate-800">${tabs.map(([k, l]) => tab(k, l)).join("")}</div>`;
}
