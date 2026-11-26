// "Ask about this book" (a book's window): a clarifying question about a rated book, such as "is the LGBTQ+ character a main
// character or only mentioned?". The answer comes from the rating and, when the book was read in full, from the notes Deep Scan
// kept for each part; it says which, so a best guess is not taken for a fact.
import { post } from "./api.js";
import { esc, attempt } from "./ui.js";

export function askBox(host, book, user) {
  if (!host || book.status !== "analyzed" || user.role === "friend" || (user.role === "restricted" && !user.ai_features)) return;
  host.innerHTML = `<details class="rounded-lg border border-slate-700/60 p-3"><summary class="cursor-pointer text-sm font-semibold">Ask a question about this book</summary>
    <div class="mt-2 space-y-2"><div class="flex flex-wrap gap-2"><input data-q class="input min-w-0 flex-1" maxlength="300" placeholder="Is the LGBTQ+ character a main character or just mentioned?">
    <button type="button" data-ask class="btn-secondary py-1 text-sm">Ask</button></div><div data-answer class="text-sm"></div></div></details>`;
  const q = host.querySelector("[data-q]");
  const out = host.querySelector("[data-answer]");
  const go = async () => {
    const text = q.value.trim();
    if (!text) return;
    out.innerHTML = `<p class="text-slate-400">Thinking…</p>`;
    const r = await attempt(() => post(`/api/books/${book.id}/ask`, { question: text }));
    out.innerHTML = r ? `<p>${esc(r.answer)}</p><p class="mt-1 text-xs text-slate-400">${r.basis === "full read"
      ? "From the notes kept when the whole book was read."
      : "A best guess from the rating and summary: the whole book has not been read for this. A Deep Scan would let this answer from the book itself."}</p>` : "";
  };
  host.querySelector("[data-ask]").onclick = go;
  q.onkeydown = (e) => { if (e.key === "Enter") go(); };
}
