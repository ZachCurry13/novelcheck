// 📦 Box sets: books whose titles say they hold several ("Books 1–3",
// "Trilogy", "Box Set"). A parent checks what's inside (✨ the AI can say),
// fixes the titles if needed, and splits it into its books, or says it
// isn't a box set. Split books use the box set's files and count as owned.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

// openBoxSets shows the box sets waiting (only focus's, when given).
export async function openBoxSets(onChange, focus = 0) {
  let d = document.getElementById("boxsets-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "boxsets-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  let list = (await attempt(() => get("/api/box-sets"))) || [];
  if (focus) list = list.filter((b) => b.book_id === focus);
  const row = (e = { title: "", number: 0 }) => `<li class="flex items-center gap-2" data-entry>
    <input type="checkbox" data-on class="h-5 w-5 shrink-0" checked aria-label="Split this one out">
    <input data-n type="number" min="0" step="0.5" value="${e.number || ""}" class="input w-16 shrink-0 px-2 py-1 text-sm" placeholder="#" aria-label="Number in the series">
    <input data-t value="${esc(e.title)}" class="input min-w-0 flex-1 py-1 text-sm" placeholder="Title" aria-label="Title"></li>`;
  const card = (b) => b.state === "split"
    ? `<li class="card space-y-2" data-box="${b.book_id}"><p class="font-semibold">📦 ${esc(b.title)}</p>
        <p class="text-xs text-slate-400">Split into: ${b.members.map((m) => esc(m.title)).join(" · ")}</p>
        <button type="button" data-act="undo" class="btn-ghost py-1 text-sm">Undo the split</button></li>`
    : `<li class="card space-y-2" data-box="${b.book_id}">
        <p class="font-semibold leading-snug">📦 ${esc(b.title)}</p>
        <p class="text-xs text-slate-400">${esc(b.author || "")}${b.series ? ` · ${esc(b.series)}` : ""}</p>
        <p class="text-xs text-slate-400">${b.proposal.length ? "Untick what isn't inside, fix titles, or add one:" : "What's inside? Ask the AI, or type the titles:"}</p>
        <ul data-entries class="space-y-1">${(b.proposal.length ? b.proposal : [{ title: "", number: 0 }]).map(row).join("")}</ul>
        <div class="flex flex-wrap gap-2">
          <button type="button" data-act="add" class="btn-ghost py-1 text-sm">＋ Add a book</button>
          <button type="button" data-act="ask" class="btn-secondary py-1 text-sm">✨ Ask the AI what's inside</button></div>
        <div class="grid grid-cols-2 gap-2">
          <button type="button" data-act="split" class="btn-primary">Split into these books</button>
          <button type="button" data-act="not" class="btn-ghost">Not a box set</button></div></li>`;
  const paint = () => {
    d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-bold">📦 Box sets</h2>
        <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">Once split, each book is its own card in the Library with its own rating, counts as yours, and opens the box set's file.</p>
      <ul class="space-y-3">${list.map(card).join("") || `<li class="text-sm text-slate-400">No box sets to check.</li>`}</ul></div>`;
  };
  paint();
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    const act = e.target.closest("[data-act]")?.dataset.act;
    const li = e.target.closest("[data-box]");
    if (!act || !li) return;
    const id = Number(li.dataset.box);
    const box = list.find((b) => b.book_id === id);
    const btn = e.target.closest("[data-act]");
    if (act === "add") return li.querySelector("[data-entries]").insertAdjacentHTML("beforeend", row());
    btn.disabled = true;
    if (act === "ask") {
      btn.textContent = "Asking… (a local AI can take a minute)";
      const r = await attempt(() => post(`/api/box-sets/${id}/ask`));
      if (r) {
        box.proposal = r.proposal;
        if (!r.proposal.length) toast("The AI doesn't know this one: type the titles instead", true);
      }
      return paint();
    }
    if (act === "split") {
      const books = [...li.querySelectorAll("[data-entry]")].filter((x) => x.querySelector("[data-on]").checked)
        .map((x) => ({ title: x.querySelector("[data-t]").value.trim(), number: Number(x.querySelector("[data-n]").value) || 0 }))
        .filter((b) => b.title);
      if (!books.length) {
        btn.disabled = false;
        return toast("Tick at least one book with a title", true);
      }
      const r = await attempt(() => post(`/api/box-sets/${id}/split`, { books }), `Split into ${books.length} book${books.length === 1 ? "" : "s"}`);
      if (!r) return (btn.disabled = false);
      box.state = "split";
      box.members = books;
    } else if (act === "not" || act === "undo") {
      if (!(await attempt(() => post(`/api/box-sets/${id}/${act}`), act === "not" ? "Noted: not a box set" : "Put back together"))) return (btn.disabled = false);
      if (act === "not") list = list.filter((b) => b.book_id !== id);
      else box.state = "found";
    }
    paint();
    onChange?.();
  };
  if (!d.open) d.showModal();
}
