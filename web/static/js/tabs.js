// One tab strip for the whole app (Admin, Settings, Reading stats, Discover):
// an underlined row that scrolls sideways on a narrow screen and keeps the open
// tab in view. A tab is [key, text, href?, inPlace?]: with an href it is a link
// (a page of its own); without, a button the page handles itself. In-place
// tabs carry the attribute named by attr (data-tab by default).
import { esc } from "./ui.js";

export function tabStrip({ tabs, active, label = "Sections", id = "", attr = "data-tab" }) {
  setTimeout(() => document.querySelector(`${id ? `#${id}` : "[data-tabs]"} .active`)?.scrollIntoView({ block: "nearest", inline: "center" }), 0);
  const item = ([key, text, href, inPlace = true]) => {
    const on = key === active;
    const common = `${inPlace ? `${attr}="${esc(key)}"` : ""} role="tab" aria-selected="${on}" class="nav-link shrink-0 rounded-b-none${on ? " active" : ""}"`;
    return href ? `<a href="${esc(href)}" ${common}>${esc(text)}</a>` : `<button type="button" ${common}>${esc(text)}</button>`;
  };
  return `<nav ${id ? `id="${id}"` : ""} data-tabs role="tablist" aria-label="${esc(label)}" class="-mx-4 mb-4 flex gap-1 overflow-x-auto border-b border-slate-800 px-4">${tabs.map(item).join("")}</nav>`;
}
