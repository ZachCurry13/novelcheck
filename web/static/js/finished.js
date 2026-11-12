// "I read this" in a book's window: the day you finished it, so the book counts in Reading stats for that
// year even when neither KOReader nor an imported list knows it (a paper book, a library loan, a list with
// no "Date Read"). Clearing the day takes the book out of your finished books again.
import { put } from "./api.js";
import { attempt, esc, toast } from "./ui.js";

const today = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};
const nice = (day) => new Date(`${day}T12:00:00`).toLocaleDateString(undefined, { month: "long", day: "numeric", year: "numeric" });

export const finishedHTML = (day = "") => `<div data-finished="${esc(day)}" class="flex flex-wrap items-center gap-2 text-sm"></div>`;

export function bindFinished(host, bookId, onChange) {
  const box = host.querySelector("[data-finished]");
  if (!box) return;
  let day = box.dataset.finished || "";
  const paint = (editing = false) => {
    box.innerHTML = editing
      ? `<label class="text-slate-400" for="fin-day">Finished on</label>
        <input id="fin-day" type="date" class="input w-auto py-1.5" value="${esc(day || today())}" max="${today()}">
        <button type="button" data-fin-save class="btn-tonal px-4 py-1.5">Save</button>
        <button type="button" data-fin-cancel class="btn-text py-1.5">Cancel</button>`
      : day
        ? `<span class="text-emerald-300">✓ You finished this on ${esc(nice(day))}</span>
          <button type="button" data-fin-edit class="btn-text py-1">Change</button>
          <button type="button" data-fin-clear class="btn-text py-1">Not read</button>`
        : `<button type="button" data-fin-edit class="btn-outlined px-4 py-1.5">✓ I read this</button>
          <span class="text-xs text-slate-400">Counts it in your Reading stats</span>`;
  };
  const save = async (value) => {
    const r = await attempt(() => put(`/api/books/${bookId}/finished`, { day: value }));
    if (!r) return;
    day = r.day || "";
    toast(day ? `Counted as read on ${nice(day)}` : "Taken out of your finished books");
    paint();
    onChange?.();
  };
  box.onclick = (e) => {
    if (e.target.closest("[data-fin-edit]")) return paint(true);
    if (e.target.closest("[data-fin-cancel]")) return paint();
    if (e.target.closest("[data-fin-clear]")) return save("");
    if (e.target.closest("[data-fin-save]")) return save(box.querySelector("#fin-day").value);
  };
  paint();
}
