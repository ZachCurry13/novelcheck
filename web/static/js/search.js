// The search in the top bar: finds settings, admin tools, pages and actions (searchindex.js) and opens them.
// "/" or Ctrl+K opens it; arrows move, Enter opens, Esc closes.
import { esc } from "./ui.js";
import { entries, find } from "./searchindex.js";

let dialog, list, user, results = [], at = 0;

const rowHTML = (e, i) => `<li><a href="${esc(e.href || "#")}" data-i="${i}" role="option" aria-selected="${i === at}" class="list-row search-row px-4">
  <span class="ms" aria-hidden="true">${esc(e.icon)}</span>
  <span class="min-w-0 flex-1"><span class="block truncate font-medium">${esc(e.name)}</span><span class="block truncate text-sm text-slate-400">${esc(e.where)}</span></span></a></li>`;

function paint() {
  results = find(list, dialog.querySelector("input").value);
  at = Math.min(at, Math.max(0, results.length - 1));
  dialog.querySelector("[data-results]").innerHTML = results.length ? results.map(rowHTML).join("")
    : `<li class="px-4 py-6 text-sm text-slate-400">Nothing by that name. Search finds settings, admin tools, pages and actions; books are searched on the Library page.</li>`;
  dialog.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
}

function go(e) {
  dialog.close();
  if (e.click) document.querySelector(e.click)?.click();
  else if (location.hash === e.href) window.dispatchEvent(new HashChangeEvent("hashchange"));
  else location.hash = e.href;
}

function build() {
  dialog = document.createElement("dialog");
  dialog.id = "search-dialog";
  dialog.className = "dialog search-view";
  dialog.setAttribute("aria-label", "Search");
  dialog.innerHTML = `<div class="flex items-center gap-2 px-4 py-2"><span class="ms text-slate-400" aria-hidden="true">search</span>
      <input type="search" class="search-input" placeholder="Search settings, tools and pages" aria-label="Search settings, tools and pages" autocomplete="off" enterkeyhint="go">
      <button type="button" data-close class="icon-btn" aria-label="Close search"><span class="ms" aria-hidden="true">close</span></button></div>
    <ul data-results role="listbox" class="list search-results"></ul>`;
  document.body.append(dialog);
  dialog.querySelector("input").addEventListener("input", () => {
    at = 0;
    paint();
  });
  dialog.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      at = (at + (e.key === "ArrowDown" ? 1 : -1) + results.length) % Math.max(1, results.length);
      paint();
    } else if (e.key === "Enter" && results[at]) {
      e.preventDefault();
      go(results[at]);
    }
  });
  dialog.addEventListener("click", (e) => {
    const a = e.target.closest("[data-i]");
    if (a) {
      e.preventDefault();
      return go(results[Number(a.dataset.i)]);
    }
    if (e.target === dialog || e.target.closest("[data-close]")) dialog.close();
  });
}

export function openSearch() {
  if (!user || document.getElementById("app-view")?.classList.contains("hidden")) return;
  if (!dialog) build();
  if (dialog.open) return;
  list = entries(user); // built each time: what this person may open can change
  at = 0;
  dialog.querySelector("input").value = "";
  paint();
  dialog.showModal();
  dialog.querySelector("input").focus();
}

// initSearch wires the top-bar button and the keyboard shortcuts; setSearchUser says who is signed in.
export function initSearch() {
  document.getElementById("search-btn")?.addEventListener("click", openSearch);
  document.addEventListener("keydown", (e) => {
    const typing = /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement?.tagName) || document.activeElement?.isContentEditable;
    if ((e.key === "k" && (e.ctrlKey || e.metaKey)) || (e.key === "/" && !typing && !document.querySelector("dialog[open]"))) {
      e.preventDefault();
      openSearch();
    }
  });
}
export const setSearchUser = (u) => (user = u);
