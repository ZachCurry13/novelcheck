// "Want to read" (Check a book): the books on the person's Hardcover Want to read shelf, each against the family library: ready to
// open, free to add (public domain), or neither (check it to request it). Nothing is added unless the person taps.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";
import { openBook } from "./bookdialog.js";

export async function wantCard(host, state) {
  const d = await get("/api/me/want-to-read").catch(() => null);
  if (!host || !d?.connected || !d.books?.length) return;
  const row = (b, i) => {
    const what = b.in_library ? (b.downloadable ? "In the library · ready to download" : "In the library")
      : b.free ? "Free (public domain)" : "Not in the library";
    const act = b.in_library ? `<button type="button" data-want-open="${b.id}" class="btn-secondary py-1 text-sm">Open</button>`
      : b.free ? `<button type="button" data-want-free="${i}" class="btn-primary py-1 text-sm">Add the free copy</button>`
      : `<button type="button" data-want-check="${i}" class="btn-ghost py-1 text-sm">Check it</button>`;
    return `<li class="flex items-center gap-2 rounded-lg bg-slate-800/60 p-2 text-sm"><span class="min-w-0 flex-1"><b class="block truncate">${esc(b.title)}</b>
      <span class="block truncate text-xs text-slate-400">${esc(b.author || "")} · ${what}</span></span>${act}</li>`;
  };
  host.innerHTML = `<h2 class="mb-2 mt-4 text-sm font-medium text-slate-400">Your Hardcover Want to read</h2>
    <ul class="space-y-2">${d.books.map(row).join("")}</ul>`;
  host.onclick = async (e) => {
    const open = e.target.closest("[data-want-open]")?.dataset.wantOpen;
    if (open) return openBook(Number(open), state);
    const free = e.target.closest("[data-want-free]");
    if (free) {
      const b = d.books[Number(free.dataset.wantFree)];
      free.disabled = true;
      free.textContent = "Adding…";
      const r = await attempt(() => post("/api/free/add", { title: b.title, author: b.author }), "Added to the library; it shows up in a minute");
      free.textContent = r ? "Added" : "Add the free copy";
      free.disabled = !!r;
      return;
    }
    const chk = e.target.closest("[data-want-check]");
    if (chk) {
      const b = d.books[Number(chk.dataset.wantCheck)];
      location.hash = `#/check?q=${encodeURIComponent(b.title + (b.author ? " " + b.author : ""))}`;
    }
  };
}
