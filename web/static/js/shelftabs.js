// Collections and Series share one menu item: a switch at the top of each.
export function shelfTabsHTML(active) {
  const tabs = [["collections", "📚 Collections"], ["series", "🗂 Series"]];
  const tab = (key, label) => `<a href="#/${key}" class="rounded-lg px-2 py-2 text-center text-sm font-semibold ${key === active ? "bg-slate-800 text-white" : "text-slate-400 hover:text-white"}"
    ${key === active ? 'aria-current="page"' : ""}>${label}</a>`;
  return `<div class="mb-4 grid grid-cols-2 gap-1 rounded-xl bg-slate-900 p-1 ring-1 ring-slate-800">${tabs.map(([k, l]) => tab(k, l)).join("")}</div>`;
}
