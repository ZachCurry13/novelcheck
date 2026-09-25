// ⭐ The family wishlist: books someone would like to get (usually found with
// Check a book). Parents approve & track them, decline them, or mark them as
// got; books that turn up in the library are marked as got automatically.
import { get, post, del } from "./api.js";
import { esc, attempt } from "./ui.js";
import { pepperChip } from "./peppers.js";
import { openBook } from "./bookdialog.js";
import { when } from "./deepscan.js";
import { coverImg } from "./covers.js";

const STATUS = {
  wanted: ["⭐ Wanted", "chip-pending"],
  approved: ["🛒 To get", "chip-closed"],
  acquired: ["✓ Got it", "chip-none"],
  declined: ["Declined", "chip-pending"],
};

export async function renderWishlist(view, state) {
  const load = async () => {
    const data = (await attempt(() => get("/api/wishlist"))) || { items: [], manager: false };
    const open = data.items.filter((w) => w.status === "wanted" || w.status === "approved");
    const done = data.items.filter((w) => !open.includes(w));
    const row = (w) => {
      const [label, cls] = STATUS[w.status] || [w.status, "chip-pending"];
      const mine = w.username === state.user.username;
      const acts = [];
      if (data.manager && w.status === "wanted") acts.push(`<button data-act="approve" class="btn-primary py-1 text-sm">Approve & Track</button>`, `<button data-act="decline" class="btn-ghost py-1 text-sm">Decline</button>`);
      if (data.manager && w.status === "approved") acts.push(`<button data-act="acquired" class="btn-secondary py-1 text-sm">✓ Mark as got</button>`, `<button data-act="decline" class="btn-ghost py-1 text-sm">Decline</button>`);
      if (mine && w.status === "wanted") acts.push(`<button data-act="remove" class="btn-ghost py-1 text-sm">Remove</button>`);
      return `<li class="card space-y-2" data-wish="${w.id}" data-book="${w.book_id}">
        <div class="flex flex-wrap items-start justify-between gap-2">
          <button data-act="open" class="flex min-w-0 gap-3 text-left">${coverImg(w.book_id, "h-20 w-14")}<span class="min-w-0"><span class="block font-semibold">${esc(w.title)}</span>
            <span class="block text-sm text-slate-400">${esc(w.author || "")}</span></span></button>
          <span class="flex flex-wrap gap-1">${w.spice_level === null ? "" : pepperChip(w.spice_level)}<span class="${cls}">${label}</span></span>
        </div>
        <p class="text-xs text-slate-400">Wished for by ${esc(w.username || "someone")} · ${esc(when(w.created_at).toLocaleDateString())}${w.note ? ` · “${esc(w.note)}”` : ""}${w.decided_by && w.status !== "wanted" ? ` · ${esc(w.decided_by)}` : ""}</p>
        ${acts.length ? `<div class="flex flex-wrap gap-2">${acts.join("")}</div>` : ""}
      </li>`;
    };
    view.innerHTML = `
      <h1 class="mb-1 text-2xl font-bold">⭐ Wishlist</h1>
      <p class="mb-4 text-sm text-slate-400">${data.manager
        ? "Books the family would like to get. <b>Approve & Track</b> the ones you'll buy; they're marked as got by themselves once they show up in your library."
        : "Books you'd like to get. A parent decides; you'll see here when they're on the way."}
        Add books with <b>⭐ Add to Wishlist</b> on Check a book or in a book's window.</p>
      <ul class="space-y-2">${open.map(row).join("") || `<li class="text-sm text-slate-500">Nothing on the wishlist right now.</li>`}</ul>
      ${done.length ? `<h2 class="mb-2 mt-6 text-lg font-semibold">Done</h2><ul class="space-y-2">${done.map(row).join("")}</ul>` : ""}`;
  };
  view.onclick = async (e) => {
    const act = e.target.closest("[data-act]")?.dataset.act;
    const li = e.target.closest("[data-wish]");
    if (!act || !li) return;
    if (act === "open") return openBook(Number(li.dataset.book), state, load);
    const ok = act === "remove"
      ? await attempt(() => del(`/api/books/${li.dataset.book}/wish`), "Removed from your wishlist")
      : await attempt(() => post(`/api/wishlist/${li.dataset.wish}/${act}`));
    if (ok) load();
  };
  await load();
}
