// Discover, the Wishlist and Events share one menu item: a tab strip at the top
// of each. With Discover turned off, the Wishlist has its own menu item and
// the strip keeps the other two.
import { on } from "./modules.js";
import { tabStrip } from "./tabs.js";

export function discoverTabsHTML(active, user) {
  const tabs = [["discover", "🧭 Discover", on(user, "discover")], ["wishlist", "⭐ Wishlist", true], ["events", "🎉 Events", true]]
    .filter(([, , shown]) => shown).map(([k, label]) => [k, label, `#/${k}`, false]);
  return tabStrip({ tabs, active, label: "Discover" });
}
