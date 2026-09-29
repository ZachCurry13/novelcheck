// 🔍 Check these books: the AI takes a second look at a shelf (a collection
// or a seasonal shelf) and lists the books that don't seem to be about it,
// with its reason. The ticked ones come off the shelf for good.
import { get, post } from "./api.js";
import { $$, esc, attempt, toast } from "./ui.js";
import { coverImg } from "./covers.js";

function dialog() {
  let d = document.getElementById("shelfcheck-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "shelfcheck-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

const books = (n) => `${n} book${n === 1 ? "" : "s"}`;

// openShelfCheck: shelf is {collection_id, season}; name is what it's called.
export async function openShelfCheck(shelf, name, onChange) {
  if (!confirm(`Have the AI check every book on “${name}” and list the ones that don't fit? It reads each book's details (about 140 tokens a book; free with a local AI).`)) return;
  const start = await attempt(() => post("/api/shelves/check", shelf));
  if (!start) return;
  const d = dialog();
  d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
    <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-bold">🔍 Check these books</h2>
      <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <div data-out class="space-y-2"></div></div>`;
  const out = d.querySelector("[data-out]");
  d.onclick = (e) => {
    if (e.target === d || e.target.closest("[data-close]")) d.close();
  };
  d.showModal();

  let job = null;
  while (d.open) {
    job = await get(`/api/collections/ai/${start.job}`).catch(() => null);
    if (!job || job.status !== "running") break;
    out.innerHTML = `<p class="rounded-lg bg-slate-800 p-3 text-sm">🔍 Checked ${job.done} of ${books(job.total)}…
      <span class="block text-xs text-slate-400">A local AI can take a minute or two.</span></p>`;
    await new Promise((r) => setTimeout(r, 2000));
  }
  if (!d.open) return;
  if (!job || job.status === "error") return (out.innerHTML = `<p class="text-sm text-rose-300">${esc(job?.error || "The check stopped. Try again later.")}</p>`);
  const more = start.more ? `<p class="text-xs text-slate-400">Only the first ${books(start.total)} (by title) were checked.</p>` : "";
  const partial = job.error ? `<p class="text-xs text-amber-300">${esc(job.error)}</p>` : "";
  if (!job.picks.length) {
    out.innerHTML = `<p class="text-sm">Every book checked fits “${esc(name)}”. 🎉</p>${partial}${more}`;
    return;
  }
  out.innerHTML = `<p class="text-sm text-slate-300">These ${books(job.picks.length)} don't seem to be about “${esc(name)}”. Untick any to keep:</p>
    ${partial}${more}
    ${job.picks.map((p) => `<label class="flex items-center gap-3 rounded-lg bg-slate-800/60 p-2">
      <input type="checkbox" checked data-pick value="${p.book.id}" class="h-5 w-5 shrink-0">
      ${coverImg(p.book.id, "h-14 w-10")}
      <span class="min-w-0 flex-1"><span class="font-semibold leading-snug line-clamp-2">${esc(p.book.title)}</span>
        <span class="block truncate text-xs text-slate-400">${esc(p.book.author || "")}</span>
        <span class="block text-xs text-slate-300">🔍 ${esc(p.reason)}</span></span></label>`).join("")}
    <button type="button" data-take class="btn-primary sticky bottom-0 w-full"></button>`;
  const paint = () => {
    const n = $$("[data-pick]:checked", out).length;
    out.querySelector("[data-take]").textContent = `Take ${books(n)} off this shelf`;
  };
  paint();
  out.onchange = paint;
  out.querySelector("[data-take]").onclick = async () => {
    const ids = $$("[data-pick]:checked", out).map((c) => Number(c.value));
    if (!ids.length) return toast("Tick at least one book", true);
    const r = await attempt(() => post("/api/shelves/reject", { ...shelf, ids }));
    if (!r) return;
    toast(`Took ${books(r.removed)} off this shelf`);
    d.close();
    onChange?.();
  };
}
