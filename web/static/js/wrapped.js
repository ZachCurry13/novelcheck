// Wrapped, records and trophies moved onto the Reading stats page (stats.js) in v1.71. Old links to
// #/wrapped still work: they open that page on the matching tab.
import { renderStats } from "./stats.js";

export function renderWrapped(view, state) {
  const asked = new URLSearchParams(location.hash.split("?")[1] || "").get("tab");
  return renderStats(view, state, { tab: asked === "trophies" ? "trophies" : "records" });
}
