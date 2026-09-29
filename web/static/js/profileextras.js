// Profile: the page NovelCheck opens on, and (parents) where books are added:
// Import books (Kindle, drive, lists) and Paper books.
import { put } from "./api.js";
import { $, attempt, canManage } from "./ui.js";
import { on } from "./modules.js";
import { refreshUser } from "./app.js";

const PAGES = [
  ["check", "📷 Check a book", (u) => canManage(u)],
  ["library", "📚 Library", () => true],
  ["discover", "🧭 Discover", (u) => on(u, "discover")],
  ["queue", "▶️ Up Next", (u) => on(u, "queue")],
  ["collections", "📚 Collections", () => true],
  ["wishlist", "⭐ Wishlist", () => true],
];

export function profileExtrasHTML(u) {
  const def = canManage(u) ? "Check a book" : "Library";
  return `
    ${canManage(u) ? `<div class="card space-y-2">
      <h2 class="text-lg font-semibold">➕ Add books</h2>
      <div class="grid gap-2 sm:grid-cols-2">
        <a href="#/import" class="btn-secondary${on(u, "import") ? "" : " module-off"}" data-module="import">💾 Import books<span class="sr-only">: a Kindle, a drive or a list</span></a>
        <a href="#/shelf" class="btn-secondary${on(u, "import") ? "" : " module-off"}" data-module="import">📕 Paper books</a>
      </div>
      <p class="text-xs text-slate-400">Import a Kindle, a drive or a list from Goodreads or Amazon; or scan the barcodes of printed books.</p></div>` : ""}
    <div class="card space-y-2">
      <h2 class="text-lg font-semibold">🏠 Start page</h2>
      <label class="label" for="start-page">NovelCheck opens on</label>
      <select id="start-page" class="input">
        <option value="">Default (${def})</option>
        ${PAGES.filter(([, , ok]) => ok(u)).map(([k, l]) => `<option value="${k}" ${u.start_page === k ? "selected" : ""}>${l}</option>`).join("")}
      </select></div>`;
}

export function bindProfileExtras(view) {
  $("#start-page", view)?.addEventListener("change", async (e) => {
    if (await attempt(() => put("/api/me/start-page", { page: e.target.value }), "Start page saved")) await refreshUser();
  });
}
