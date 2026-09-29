// "Same book as…": a parent picks the other title of this book (a TV tie-in
// edition, a reissue under another name). NovelCheck keeps the better-rated
// one, moves everything over, and remembers the other title for the future.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";
import { coverImg } from "./covers.js";

export function openSameAs(book, onDone) {
  let d = document.getElementById("sameas-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "sameas-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
    <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-bold">🔗 Same book as…</h2>
      <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <p class="text-sm text-slate-400">Find the other title of <b>${esc(book.title)}</b>. The two become one book: the better-rated one stays, and its libraries, Up Next places, notes and wishes all come together.</p>
    <input type="search" data-q class="input" placeholder="Title or author" value="${esc(book.title)}" autocomplete="off">
    <ul data-found class="space-y-2"></ul></div>`;
  const found = d.querySelector("[data-found]");
  let timer;
  const search = async () => {
    const q = d.querySelector("[data-q]").value.trim();
    if (q.length < 2) return (found.innerHTML = "");
    const r = await get(`/api/books?q=${encodeURIComponent(q)}&limit=12`).catch(() => null);
    const books = (r?.books || []).filter((b) => b.id !== book.id);
    found.innerHTML = books.map((b) => `<li><button type="button" data-pick="${b.id}" class="card flex w-full items-center gap-3 p-2 text-left hover:ring-indigo-600">
      ${coverImg(b.id, "h-14 w-10")}<span class="min-w-0"><span class="block font-semibold leading-snug">${esc(b.title)}</span>
      <span class="block truncate text-xs text-slate-400">${esc(b.author || "")}${b.catalogs ? ` · ${esc(b.catalogs)}` : ""}</span></span></button></li>`).join("")
      || `<li class="text-sm text-slate-400">No other books match.</li>`;
  };
  d.oninput = () => {
    clearTimeout(timer);
    timer = setTimeout(search, 250);
  };
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    const pick = e.target.closest("[data-pick]");
    if (!pick) return;
    const title = pick.querySelector(".font-semibold")?.textContent || "that book";
    if (!confirm(`Make “${book.title}” and “${title}” one book? This can't be undone from NovelCheck.`)) return;
    const r = await attempt(() => post(`/api/books/${book.id}/same-as`, { other_id: Number(pick.dataset.pick) }), "They're one book now");
    if (r) {
      d.close();
      onDone(r.kept);
    }
  };
  d.showModal();
  search();
}
