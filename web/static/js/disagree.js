// "I disagree" on a book's page (parents and admins, 2.0): pick the level you would give it. It does not change the book;
// it is kept next to the AI's level so the Accuracy page can score the AI.
import { post } from "./api.js";
import { attempt, esc } from "./ui.js";

export const disagreeHTML = () => `<button type="button" data-disagree class="btn-ghost" title="Give the level you think this book should have">I disagree</button>
  <div data-disagree-panel class="hidden space-y-2 rounded-lg border border-slate-700 p-3 text-sm"><p class="font-semibold">What level would you give it?</p>
    <div class="flex flex-wrap gap-2">${[0, 1, 2, 3, 4, 5].map((n) => `<button type="button" data-level="${n}" class="btn-secondary py-1">${n}</button>`).join("")}</div>
    <input data-note class="input" placeholder="Why? (optional)" aria-label="Why" maxlength="500"><p class="text-xs text-slate-400">It doesn't change the book. An admin can use your level from the Accuracy page.</p></div>`;

export function bindDisagree(root, b) {
  const panel = root.querySelector("[data-disagree-panel]");
  root.querySelector("[data-disagree]").onclick = () => panel.classList.toggle("hidden");
  panel.onclick = async (e) => {
    const l = e.target.closest("[data-level]");
    if (l && await attempt(() => post(`/api/books/${b.id}/disagree`, { level: Number(l.dataset.level), note: panel.querySelector("[data-note]").value }), `Thanks: you gave “${esc(b.title)}” a ${l.dataset.level}`)) panel.classList.add("hidden");
  };
}
