// Change cover (a book's page): pick one of the covers found for the book, or upload your own. Only your own view
// changes; an admin can tick "Use it for everyone" (it then wins over Calibre's cover). api/mycover.go.
import { get, del } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

export async function openCoverPicker(b, done, { everyone: forAll = false } = {}) {
  const dlg = document.createElement("dialog");
  dlg.className = "dialog max-w-lg";
  dlg.innerHTML = `<div class="space-y-3 p-4"><h3 class="text-lg font-semibold">Change cover</h3>
    <p class="text-sm text-slate-400">${esc(b.title)}. Your choice changes only what you see.</p>
    <div data-cands class="grid grid-cols-4 gap-2 text-sm text-slate-400">Looking for covers…</div>
    <label class="btn-secondary inline-flex cursor-pointer">Upload my own<input type="file" accept="image/jpeg,image/png" class="hidden" data-file></label>
    <label data-all class="toggle hidden text-sm"><input type="checkbox" data-everyone> Use it for everyone (admins)</label>
    <div class="flex flex-wrap gap-2"><button type="button" data-reset class="btn-ghost">Back to the usual cover</button><button type="button" data-close class="btn-ghost">Close</button></div></div>`;
  document.body.appendChild(dlg);
  dlg.showModal();
  dlg.addEventListener("close", () => dlg.remove());
  const everyone = () => dlg.querySelector("[data-everyone]").checked;
  const finish = (msg) => { toast(msg); dlg.close(); done?.(); };
  const send = async (init) => {
    const res = await fetch(`/api/books/${b.id}/cover`, { method: "POST", credentials: "same-origin", ...init, headers: { "X-NovelCheck": "1", ...(init.headers || {}) } });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || "That cover couldn't be saved");
    return data;
  };
  const r = await get(`/api/books/${b.id}/cover-options`).catch(() => null);
  if (r?.can_set_for_everyone) {
    dlg.querySelector("[data-all]").classList.remove("hidden");
    dlg.querySelector("[data-everyone]").checked = forAll; // Fix titles and covers: for everyone by default
  }
  const cands = r?.candidates || [];
  dlg.querySelector("[data-cands]").innerHTML = cands.length ? cands.map((c, i) => `<button type="button" data-pick="${i}" class="overflow-hidden rounded border border-slate-700 hover:border-indigo-400">
      <img src="/api/cover-preview?u=${encodeURIComponent(c.url)}" alt="" loading="lazy" class="aspect-[2/3] w-full object-cover"></button>`).join("")
    : `<p class="col-span-4">No covers found online. You can upload your own.</p>`;
  dlg.onclick = async (e) => {
    if (e.target === dlg || e.target.closest("[data-close]")) return dlg.close();
    const pick = e.target.closest("[data-pick]");
    if (pick && await attempt(() => send({ headers: { "Content-Type": "application/json" }, body: JSON.stringify({ url: cands[Number(pick.dataset.pick)].url, everyone: everyone() }) }))) finish("Cover changed");
    if (e.target.closest("[data-reset]") && await attempt(() => del(`/api/books/${b.id}/cover${everyone() ? "?everyone=1" : ""}`))) finish("Back to the usual cover");
  };
  dlg.querySelector("[data-file]").onchange = async (e) => {
    const f = e.target.files[0];
    if (!f) return;
    const form = new FormData();
    form.append("image", f);
    form.append("everyone", String(everyone()));
    if (await attempt(() => send({ body: form }))) finish("Your cover is in");
  };
}
