// ☑ Select books in the Library, then act on all of them at once: add to
// Up Next, Deep Scan, or delete. Tap cards to tick them; with a mouse you
// can also drag across cards, or Shift-click to tick everything in between.
import { post } from "./api.js";
import { attempt, toast } from "./ui.js";
import { on } from "./modules.js";

const plural = (n, one, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

export function setupSelect(view, grid, state, reload) {
  const selected = new Set();
  let active = false;
  let pointer = "";
  let drag = null; // true = ticking, false = unticking while the mouse is down
  let last = null; // the last card clicked, for Shift-click
  const toggleBtn = view.querySelector("#select-toggle");
  const bar = document.createElement("div");
  bar.className = "fixed inset-x-0 bottom-0 z-30 hidden border-t border-slate-700 bg-slate-900/95 p-3 pb-safe backdrop-blur";
  document.body.append(bar);

  const cards = () => [...grid.querySelectorAll("[data-book]")];
  const paint = () => {
    for (const c of cards()) {
      c.classList.toggle("ring-2", selected.has(c.dataset.book));
      c.classList.toggle("ring-indigo-400", selected.has(c.dataset.book));
    }
    const n = selected.size;
    const admin = state.user.role === "admin";
    bar.innerHTML = `<div class="mx-auto flex max-w-7xl flex-wrap items-center gap-2 text-sm">
      <b>${n ? `${plural(n, "book")} selected` : "Tap books to select them"}</b>
      ${n && on(state.user, "queue") ? `<button data-bulk="queue" class="btn-primary py-1">＋ Up Next</button>` : ""}
      ${n ? `<button data-bulk="deep" class="btn-secondary py-1">🧬 ${admin ? "Deep Scan" : "Ask for Deep Scan"}</button>
        <button data-bulk="delete" class="btn-ghost py-1 text-rose-300">🗑 Delete…</button>` : ""}
      <span class="flex-1"></span>
      <button data-all class="btn-ghost py-1">Select all shown</button>
      ${n ? `<button data-clear class="btn-ghost py-1">Clear</button>` : ""}
      <button data-done class="btn-ghost py-1">Done</button></div>`;
  };
  const setActive = (on_) => {
    active = on_;
    if (!on_) selected.clear();
    grid.classList.toggle("selecting", on_);
    grid.classList.toggle("select-none", on_);
    bar.classList.toggle("hidden", !on_);
    toggleBtn.textContent = on_ ? "✓ Selecting" : "☑ Select";
    paint();
  };
  const set = (card, value) => {
    if (value) selected.add(card.dataset.book);
    else selected.delete(card.dataset.book);
  };

  toggleBtn.addEventListener("click", () => setActive(!active));
  // A mouse ticks on press and keeps ticking (or unticking) across the cards it's dragged over.
  grid.addEventListener("pointerdown", (e) => {
    pointer = e.pointerType;
    const card = e.target.closest("[data-book]");
    if (!active || !card || e.pointerType !== "mouse" || e.button !== 0) return;
    e.preventDefault();
    if (e.shiftKey && last) {
      const all = cards();
      const [a, b] = [all.indexOf(last), all.indexOf(card)].sort((x, y) => x - y);
      all.slice(a, b + 1).forEach((c) => set(c, true));
    } else {
      drag = !selected.has(card.dataset.book);
      set(card, drag);
    }
    last = card;
    paint();
  });
  grid.addEventListener("pointerover", (e) => {
    const card = e.target.closest("[data-book]");
    if (drag === null || !card) return;
    set(card, drag);
    paint();
  });
  window.addEventListener("pointerup", () => (drag = null));

  bar.addEventListener("click", async (e) => {
    if (e.target.closest("[data-done]")) return setActive(false);
    if (e.target.closest("[data-clear]") || e.target.closest("[data-all]")) {
      if (e.target.closest("[data-clear]")) selected.clear();
      else cards().forEach((c) => set(c, true));
      return paint();
    }
    const action = e.target.closest("[data-bulk]")?.dataset.bulk;
    if (!action) return;
    const ids = [...selected].map(Number);
    let reason = "";
    if (action === "delete") {
      reason = prompt(`Delete ${plural(ids.length, "book")}?\n\nBooks in your own libraries are taken out of them right away. For the rest, a delete request goes to the admins.\n\nWhy? (optional)`, "");
      if (reason === null) return;
    } else if (action === "deep") {
      const admin = state.user.role === "admin";
      if (!confirm(admin ? `Start a Deep Scan (the AI reads the whole book) for ${plural(ids.length, "book")}? Books without an EPUB file are skipped; you can cancel scans on the Deep Scan page.`
        : `Ask an admin to Deep Scan ${plural(ids.length, "book")}?`)) return;
    }
    const r = await attempt(() => post("/api/books/bulk", { ids, action, reason }));
    if (!r) return;
    const parts = [
      r.queued && `added ${plural(r.queued, "book")} to Up Next`, r.started && `started ${plural(r.started, "Deep Scan")}`,
      r.requested && (action === "delete" ? `asked the admins to delete ${plural(r.requested, "book")}` : `asked for ${plural(r.requested, "Deep Scan")}`),
      r.removed && `took ${plural(r.removed, "book")} out of your libraries`, r.already && `${r.already} already scanning or waiting`,
      r.no_epub && `${r.no_epub} without an EPUB file`, r.skipped && `${r.skipped} skipped`,
    ].filter(Boolean);
    toast(parts.length ? parts.join(" · ").replace(/^./, (c) => c.toUpperCase()) : "Nothing to do");
    setActive(false);
    if (action === "delete") reload();
  });

  return {
    // clicked handles a click on a card while selecting (a mouse already did it on press).
    clicked(card, e) {
      if (!active) return false;
      if (pointer !== "mouse") {
        set(card, !selected.has(card.dataset.book));
        last = card;
        paint();
      }
      return true;
    },
    repaint: paint, // after more cards load
    cleanup: () => bar.remove(),
  };
}
