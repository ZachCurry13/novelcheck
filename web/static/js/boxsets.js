// 📦 Box sets: books whose titles say they hold several ("Books 1–3",
// "Trilogy", "Box Set"). A parent checks what's inside (✨ the AI can say),
// fixes the titles if needed, and splits it into its books, or says it
// isn't a box set. Split books use the box set's files and count as owned.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { openBook } from "./bookdialog.js";

// openBoxSets shows the box sets waiting (only focus's, when given).
export async function openBoxSets(onChange, focus = 0, state) {
  let d = document.getElementById("boxsets-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "boxsets-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  let list = (await attempt(() => get("/api/box-sets"))) || [];
  if (focus) list = list.filter((b) => b.book_id === focus);
  // What a title would become: a book the family already has (it joins it), or a new one.
  const badge = (e) => (!e.title ? "" : e.match_id ? `<span class="text-emerald-300">✓ already in your library as <button type="button" data-open="${e.match_id}" class="underline decoration-dotted">${esc(e.match_title)}</button>${e.match_owned ? "" : " (only on a list)"}: it joins that book</span>` : `<span class="text-sky-300">＋ new book, made from this box set</span>`);
  const asking = new Set(); // box sets with a question out to the AI
  const ASKING = "Asking… (a local AI can take a minute)";
  const row = (e = { title: "", number: 0 }) => `<li class="space-y-0.5" data-entry><div class="flex items-center gap-2">
    <input type="checkbox" data-on class="h-5 w-5 shrink-0" checked aria-label="Split this one out">
    <input data-n type="number" min="0" step="0.5" value="${e.number || ""}" class="input w-16 shrink-0 px-2 py-1 text-sm" placeholder="#" aria-label="Number in the series">
    <input data-t value="${esc(e.title)}" class="input min-w-0 flex-1 py-1 text-sm" placeholder="Title" aria-label="Title"></div>
    <p data-badge class="pl-7 text-xs">${badge(e)}</p></li>`;
  const card = (b) => b.state === "split"
    ? `<li class="card space-y-2" data-box="${b.book_id}"><p class="font-semibold">${esc(b.title)}</p>
        <p class="text-xs text-slate-400">Split into: ${b.members.map((m) => esc(m.title)).join(" · ")}</p>
        <button type="button" data-act="undo" class="btn-ghost py-1 text-sm">Undo the split</button></li>`
    : `<li class="card space-y-2" data-box="${b.book_id}">
        <p class="font-semibold leading-snug">📦 <button type="button" data-open="${b.book_id}" class="text-left underline decoration-dotted" title="Open this book to look at it">${esc(b.title)}</button></p>
        <p class="text-xs text-slate-400">${esc(b.author || "")}${b.series ? ` · ${esc(b.series)}` : ""}${b.formats ? ` · ${esc(b.formats)}` : ""}${b.added ? ` · added to the library ${esc(new Date(b.added.length > 10 ? b.added : b.added + "T00:00:00").toLocaleDateString())}` : ""}</p>
        <p class="text-xs text-slate-400">${b.proposal.length ? "Untick what isn't inside, fix titles, or add one:" : "What's inside? Press Find, or type the titles:"}</p>
        <ul data-entries class="space-y-1">${(b.proposal.length ? b.proposal : [{ title: "", number: 0 }]).map(row).join("")}</ul>
        <div class="flex flex-wrap gap-2">
          <button type="button" data-act="add" class="btn-ghost py-1 text-sm">＋ Add a book</button>
          <button type="button" data-act="ask" class="btn-secondary py-1 text-sm" ${asking.has(b.book_id) ? "disabled" : ""}>${asking.has(b.book_id) ? ASKING : "Find what's inside"}</button></div>
        <div class="grid grid-cols-2 gap-2">
          <button type="button" data-act="split" class="btn-primary">Split into these books</button>
          <button type="button" data-act="not" class="btn-ghost">Not a box set</button></div></li>`;
  const paint = () => {
    d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-semibold">Box sets</h2>
        <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">Once split, each book is its own card in the Library with its own rating, counts as yours, and opens the box set's file.</p>
      <ul class="space-y-3">${list.map(card).join("") || `<li class="text-sm text-slate-400">No box sets to check.</li>`}</ul></div>`;
  };
  paint();
  // Typing a title shows at once whether it is a book the family already has.
  let typing;
  d.oninput = (e) => {
    const t = e.target.closest("[data-t]");
    const li = e.target.closest("[data-box]");
    if (!t || !li) return;
    clearTimeout(typing);
    typing = setTimeout(async () => {
      const m = await get(`/api/box-sets/${li.dataset.box}/match?title=${encodeURIComponent(t.value.trim())}`).catch(() => null);
      const p = t.closest("[data-entry]")?.querySelector("[data-badge]");
      if (m && p) p.innerHTML = badge({ ...m, title: t.value.trim() });
    }, 400);
  };
  d.onclick = async (e) => {
    const open = e.target.closest("[data-open]");
    if (open) return openBook(Number(open.dataset.open), state, onChange);
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
      // One ask per set at a time; only this set's card is redrawn when the answer comes, so titles being
      // typed in the other cards stay, and a set decided in the meantime is left alone.
      if (asking.has(id)) return;
      asking.add(id);
      btn.textContent = ASKING;
      const r = await post(`/api/box-sets/${id}/ask`).catch((err) => ({ error: err.message || "The AI couldn't answer" }));
      asking.delete(id);
      const now = list.find((b) => b.book_id === id);
      const shown = d.querySelector(`[data-box="${id}"]`);
      if (!now || now.state !== "found" || !shown) return;
      if (r.error) toast(r.error, true);
      else {
        now.proposal = r.proposal;
        if (!r.proposal.length) toast("Neither the book's contents nor the AI list the books in this one: type the titles instead", true);
        else if (r.from === "contents") toast(`Found ${r.proposal.length} books in the box set's own table of contents`);
      }
      shown.outerHTML = card(now);
      return;
    }
    if (act === "split") {
      const books = [...li.querySelectorAll("[data-entry]")].filter((x) => x.querySelector("[data-on]").checked)
        .map((x) => ({ title: x.querySelector("[data-t]").value.trim(), number: Number(x.querySelector("[data-n]").value) || 0 }))
        .filter((b) => b.title);
      if (!books.length) {
        btn.disabled = false;
        return toast("Tick at least one book with a title", true);
      }
      const joins = [...li.querySelectorAll("[data-entry]")].filter((x) => x.querySelector("[data-on]").checked && x.querySelector("[data-t]").value.trim() && x.querySelector("[data-open]")).length;
      if (!confirm(`Split “${box.title}” into ${books.length} book${books.length === 1 ? "" : "s"}?

${joins} already in your library will be joined to the box set's file; ${books.length - joins} new book${books.length - joins === 1 ? "" : "s"} will be made. You can undo the split afterwards.`)) {
        btn.disabled = false;
        return;
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
