// Your own star rating of a book, in the book's window: tap a star to rate (tap the same one again to clear it),
// and the ¼ buttons fine-tune it in quarter steps. It feeds Wrapped's rating breakdown and your average.
import { put } from "./api.js";
import { attempt } from "./ui.js";

const label = (v) => (v ? `${v} of 5` : "Not rated");

export const starsHTML = (v = 0) => `<div data-stars="${v}" class="flex flex-wrap items-center gap-1" role="group" aria-label="Your rating">
  <span class="text-sm text-slate-400">Your rating</span>
  ${[1, 2, 3, 4, 5].map((i) => `<button type="button" data-star="${i}" class="text-2xl leading-none" aria-label="${i} star${i === 1 ? "" : "s"}">☆</button>`).join("")}
  <button type="button" data-fine="-0.25" class="btn-ghost px-2 py-0 text-xs" aria-label="A quarter star less">−¼</button>
  <button type="button" data-fine="0.25" class="btn-ghost px-2 py-0 text-xs" aria-label="A quarter star more">+¼</button>
  <span data-stars-text class="text-xs text-slate-400"></span></div>`;

export function bindStars(host, bookId) {
  const box = host.querySelector("[data-stars]");
  if (!box) return;
  let value = Number(box.dataset.stars) || 0;
  const paint = () => {
    box.querySelectorAll("[data-star]").forEach((b) => {
      const i = Number(b.dataset.star);
      b.textContent = value > i - 1 ? "★" : "☆";
      b.classList.toggle("opacity-60", value > i - 1 && value < i - 0.125); // a part star is dimmer
      b.classList.toggle("text-amber-300", value > i - 1);
    });
    box.querySelector("[data-stars-text]").textContent = label(value);
  };
  const save = async (v) => {
    v = Math.max(0, Math.min(5, Math.round(v * 4) / 4));
    const r = await attempt(() => put(`/api/books/${bookId}/rating`, { stars: v }));
    if (r) {
      value = r.stars;
      paint();
    }
  };
  box.addEventListener("click", (e) => {
    const star = e.target.closest("[data-star]");
    if (star) return save(Number(star.dataset.star) === value ? 0 : Number(star.dataset.star));
    const fine = e.target.closest("[data-fine]");
    if (fine) save((value || 0) + Number(fine.dataset.fine));
  });
  paint();
}
