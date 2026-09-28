// Live status on the Library's cards (parents): "⚡ Rating now…" on the book
// the AI is rating, "🧬 Part 4 of 12" on the one being Deep Scanned, and each
// card's rating in place as soon as it's saved, without reloading the page.
// Driven by the "nc:activity" event (js/activity.js).
import { get } from "./api.js";

// liveCards watches grid; render(book) returns a card's HTML and after()
// runs once cards were replaced (e.g. to repaint the selection).
export function liveCards(grid, render, after) {
  let lastBook = -1;
  let lastWaiting = -1; // no update seen yet

  async function refreshWaiting() {
    const cards = [...grid.querySelectorAll("[data-book]")].filter((c) => c.querySelector("[data-status]"));
    if (!cards.length) return;
    const ids = cards.map((c) => c.dataset.book).slice(0, 100).join(",");
    const d = await get(`/api/books/states?ids=${ids}`).catch(() => null);
    let changed = false;
    for (const b of d?.books || []) {
      const el = grid.querySelector(`[data-book="${b.id}"]`);
      if (el && b.status !== el.dataset.state) {
        el.outerHTML = render(b);
        changed = true;
      }
    }
    if (changed) after?.();
  }

  function mark(a) {
    grid.querySelectorAll("[data-live]").forEach((x) => x.remove());
    grid.querySelectorAll("[data-live-hidden]").forEach((x) => {
      x.classList.remove("hidden");
      x.removeAttribute("data-live-hidden");
    });
    const chip = a.book_id && a.state !== "idle" && grid.querySelector(`[data-book="${a.book_id}"] [data-status]`);
    if (chip) {
      chip.insertAdjacentHTML("beforebegin", `<span class="chip-busy" data-live>⚡ Rating now…</span>`);
      chip.classList.add("hidden");
      chip.setAttribute("data-live-hidden", "");
    }
    const deep = a.deep && grid.querySelector(`[data-book="${a.deep.book_id}"] [data-chips]`);
    if (deep) deep.insertAdjacentHTML("afterbegin", `<span class="chip-deep" data-live title="Deep Scan: the AI is reading the whole book">🧬 Part ${a.deep.part} of ${a.deep.parts}</span>`);
  }

  const onActivity = async (e) => {
    if (!grid.isConnected) return window.removeEventListener("nc:activity", onActivity);
    const a = e.detail;
    // First look, or something finished since: fetch the cards still waiting.
    if (a.waiting !== lastWaiting || a.book_id !== lastBook) await refreshWaiting();
    lastWaiting = a.waiting;
    lastBook = a.book_id;
    mark(a);
  };
  window.addEventListener("nc:activity", onActivity);
  return () => window.removeEventListener("nc:activity", onActivity);
}
