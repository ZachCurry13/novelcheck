// Discover and the Wishlist share one menu item: a switch at the top of both.
import { on } from "./modules.js";

export function discoverTabsHTML(active, user) {
  if (!on(user, "discover")) return ""; // then the Wishlist has its own menu item
  const tab = (key, label) => `<a href="#/${key}" class="rounded-lg px-3 py-2 text-center text-sm font-semibold ${key === active ? "bg-slate-800 text-white" : "text-slate-400 hover:text-white"}"
    ${key === active ? 'aria-current="page"' : ""}>${label}</a>`;
  return `<div class="mb-4 grid grid-cols-2 gap-1 rounded-xl bg-slate-900 p-1 ring-1 ring-slate-800">${tab("discover", "🧭 Discover")}${tab("wishlist", "⭐ Wishlist")}</div>`;
}
