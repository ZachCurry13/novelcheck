// The Library's audiobook filters: length and narrator (shown once there are audiobooks).
import { esc } from "./ui.js";
import { LENGTHS } from "./audioui.js";

export const hasAudio = (facets) => (facets.narrators || []).length > 0;

export function audioFilterHTML(facets) {
  if (!hasAudio(facets)) return "";
  return `<select name="length" class="input filter-more" title="How long the audiobook is">
      ${LENGTHS.map(([l, lo, hi]) => `<option value="${lo}-${hi}">${l}</option>`).join("").replace('value="0-0"', 'value=""')}</select>
    <input name="narrator" list="narrator-list" placeholder="Narrator" class="input filter-more" autocomplete="off">
    <datalist id="narrator-list">${facets.narrators.map((n) => `<option value="${esc(n)}"></option>`).join("")}</datalist>`;
}

export const audioSortHTML = (facets) => hasAudio(facets)
  ? `<option value="long">Sort: Longest audiobooks</option><option value="short">Sort: Shortest audiobooks</option>` : "";

// audioParams turns the filter form into the query's audiobook parts.
export function audioParams(fd) {
  const [lo, hi] = String(fd.get("length") || "0-0").split("-").map(Number);
  return { narrator: fd.get("narrator"), min_hours: lo || "", max_hours: hi || "" };
}
