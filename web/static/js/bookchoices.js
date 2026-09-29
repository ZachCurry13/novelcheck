// When a typed title could be several books (Check a book, Paper books),
// ask which one was meant instead of guessing.
import { esc } from "./ui.js";

// pickBook resolves with the chosen {title, author, isbn}, {title: query} for
// "as typed", or null when closed.
export function pickBook(choices, query) {
  let d = document.getElementById("choice-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "choice-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
    <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-bold">Which book did you mean?</h2>
      <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <p class="text-sm text-slate-400">For “${esc(query)}”:</p>
    <ul class="space-y-2">${choices.map((c, i) => `<li><button type="button" data-i="${i}" class="w-full rounded-lg bg-slate-800/70 p-3 text-left hover:bg-slate-700">
      <span class="block font-semibold leading-snug">${esc(c.title)}</span>
      <span class="block text-sm text-slate-400">${esc(c.author || "Unknown author")}${c.year ? ` · ${c.year}` : ""}</span></button></li>`).join("")}</ul>
    <button type="button" data-typed class="btn-ghost w-full text-sm">None of these: use “${esc(query)}” as typed</button></div>`;
  return new Promise((resolve) => {
    let done = false;
    const finish = (v) => {
      if (done) return;
      done = true;
      if (d.open) d.close();
      resolve(v);
    };
    d.onclick = (e) => {
      const i = e.target.closest("[data-i]")?.dataset.i;
      if (i !== undefined) finish(choices[Number(i)]);
      else if (e.target.closest("[data-typed]")) finish({ title: query });
      else if (e.target === d || e.target.closest("[data-close]")) finish(null);
    };
    d.onclose = () => finish(null);
    d.showModal();
  });
}
