// Collections and Series share one place in the menu: the same tab strip as every other page (tabs.js).
import { tabStrip } from "./tabs.js";

export function shelfTabsHTML(active) {
  return tabStrip({ id: "shelf-tabs", label: "Shelves", active, tabs: [["collections", "Collections", "#/collections", false], ["series", "Series", "#/series", false]] });
}
