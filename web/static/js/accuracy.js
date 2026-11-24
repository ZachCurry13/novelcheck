// Accuracy (parents and admins, 2.0): how the AI's spice levels compare with the levels people gave ("I disagree" on a book),
// per model, and the newest disagreements, each with a button to use the person's level for the book.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";

const pct = (x) => `${Math.round(x * 100)}%`;

export async function renderAccuracy(view, state) {
  view.innerHTML = `<a href="#/home" class="text-sm text-slate-400 underline">← Home</a><h1 class="page-title mb-1 mt-2">How accurate are the ratings?</h1>
    <p class="mb-4 max-w-3xl text-sm text-slate-400">Whenever a parent opens a book and says “I disagree” and gives their own level, it is kept next to what the AI said. This page scores the AI against those levels. The more books you check, the more it means: start with books you know well.</p><div id="acc"><p class="text-slate-400">Counting…</p></div>`;
  const draw = async () => {
    const d = await attempt(() => get("/api/admin/accuracy"));
    if (!d) return;
    const rows = d.models.length ? `<div class="card overflow-x-auto"><table class="w-full text-sm"><thead><tr class="text-left text-slate-400"><th class="py-1 pr-3">Model</th><th class="pr-3">Books</th><th class="pr-3">Exactly right</th><th class="pr-3">Within one level</th><th class="pr-3">Said more spice</th><th>Said less</th></tr></thead><tbody>
      ${d.models.map((m) => `<tr class="border-t border-slate-800"><td class="py-1 pr-3 font-semibold">${esc(m.model)}</td><td class="pr-3">${m.n}</td><td class="pr-3">${pct(m.exact)}</td><td class="pr-3">${pct(m.within1)}</td><td class="pr-3">${m.higher}</td><td>${m.lower}</td></tr>`).join("")}</tbody></table></div>`
      : `<div class="card text-sm text-slate-300">No levels yet. Open a book that has been rated, tap <b>I disagree</b>, and pick the level you would give it.</div>`;
    const recent = d.recent.length ? `<h2 class="mb-2 mt-5 font-semibold">Newest answers</h2><div class="space-y-2">${d.recent.map((f) => `<div class="card flex items-center justify-between gap-3"><div class="min-w-0">
      <p><a href="#/library?book=${f.book_id}" class="font-semibold underline">${esc(f.title)}</a> <span class="text-slate-400">${esc(f.author)}</span></p>
      <p class="text-sm">${esc(f.user)} says <b>${f.level}</b>; the AI said <b>${f.ai_level ?? "nothing"}</b>${f.now_level != null && f.now_level !== f.ai_level ? `, now <b>${f.now_level}</b>` : ""}${f.note ? ` · “${esc(f.note)}”` : ""}</p></div>
      ${f.now_level === f.level ? `<span class="shrink-0 text-xs text-emerald-300">In use</span>` : state.user.role === "admin" ? `<button type="button" data-use="${f.id}" class="btn-secondary shrink-0 py-1 text-sm">Use ${f.level}</button>` : ""}</div>`).join("")}</div>` : "";
    const host = view.querySelector("#acc");
    host.innerHTML = rows + recent;
    host.onclick = async (e) => {
      const b = e.target.closest("[data-use]");
      if (b && await attempt(() => post(`/api/admin/accuracy/${b.dataset.use}/use`), "The book has that level now")) draw();
    };
  };
  await draw();
}
