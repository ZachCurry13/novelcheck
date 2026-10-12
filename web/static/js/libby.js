// 📚 Libby: on a book, whether the person's library lends it (e-book copies,
// holds, wait) with a link to it at the library, where Libby's Borrow or
// Place hold is one tap away. And in Profile, which library that is.
import { get, put } from "./api.js";
import { esc, attempt } from "./ui.js";

const num = (n) => Number(n || 0).toLocaleString();

// renderBookLibby fills host in a book's window.
export async function renderBookLibby(host, bookId) {
  const r = await get(`/api/books/${bookId}/libby`).catch(() => null);
  if (!r) return;
  if (!r.configured) {
    host.innerHTML = `<p class="text-xs text-slate-500">📚 Borrow from your library? <a href="#/profile" data-close class="underline">Set your Libby library in Profile</a></p>`;
    return;
  }
  const a = r.availability;
  let line, link = "";
  if (r.error) line = `Couldn't check ${esc(r.library)} just now.`;
  else if (!a.found) line = `Not an e-book at ${esc(r.library)}.`;
  else {
    link = `<a href="${esc(a.url)}" target="_blank" rel="noopener noreferrer" class="btn-secondary mt-1 inline-flex py-1 text-sm">${a.available ? "Borrow in Libby ↗" : a.holdable ? "Place a hold in Libby ↗" : "Open at the library ↗"}</a>`;
    line = a.available ? `<b class="text-emerald-300">Available now</b> (${num(a.free)} of ${num(a.copies)} copies)`
      : `<b class="text-amber-300">${num(a.holds)} hold${a.holds === 1 ? "" : "s"}</b> on ${num(a.copies)} cop${a.copies === 1 ? "y" : "ies"}${a.wait_days ? `, about ${num(a.wait_days)} day${a.wait_days === 1 ? "" : "s"}` : ""}`;
    if (a.kindle) line += " · Kindle supported";
  }
  host.innerHTML = `<div class="space-y-1 rounded-lg bg-slate-800/60 p-3 text-sm"><p class="label">📚 ${esc(r.library)} (Libby)</p><p>${line}</p>${link}</div>`;
}

// renderLibbyCard fills Profile's library card.
export async function renderLibbyCard(host) {
  const cur = await get("/api/me/libby").catch(() => null);
  if (!cur) return;
  const draw = (c) => {
    host.innerHTML = `<h2 class="text-lg font-semibold">📚 Libby library</h2>
      ${c.library ? `<p class="text-sm">Your library: <b>${esc(c.name)}</b> <span class="text-xs text-slate-500">(${esc(c.library)})</span>. Books show whether it lends them.</p>
        <button type="button" data-lb="off" class="btn-ghost">Change or remove</button>`
      : `<p class="text-sm text-slate-400">Free. See whether your library lends a book, how long the hold is, and open it in Libby. Libby does the borrowing; NovelCheck can't sign in for you.</p>
        <p class="text-xs text-slate-500">Type your library's key: the first part of its OverDrive address (lapl for lapl.overdrive.com). You can paste the whole address.</p>
        <form data-lb-form class="flex flex-wrap gap-2"><input name="library" required class="input min-w-0 flex-1" placeholder="e.g. lapl" autocomplete="off" aria-label="Library key">
          <button class="btn-primary">Save</button></form>`}`;
  };
  draw(cur);
  host.onclick = async (e) => {
    if (!e.target.closest("[data-lb=off]")) return;
    const r = await attempt(() => put("/api/me/libby", { library: "" }), "Library removed");
    if (r) draw(r);
  };
  host.onsubmit = async (e) => {
    e.preventDefault();
    const r = await attempt(() => put("/api/me/libby", { library: e.target.library.value }), "Library saved");
    if (r) draw(r);
  };
}
