// Join a person's Kindle libraries (admins, Integrations page): an Amazon import and the Kindle app's store books are the same
// Amazon account, so each person ends up with one "<name>'s Kindle". Nothing is joined until you press the button.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";

export async function kindleMergeCard(view) {
  const host = document.createElement("section");
  host.className = "card mb-4 space-y-2";
  view.append(host);
  const draw = (pairs) => {
    if (!pairs.length) { host.remove(); return; }
    host.innerHTML = `<h2 class="font-semibold">Join Kindle libraries</h2>
      <p class="text-sm text-slate-400">These people have more than one library for their Kindle books (an Amazon import and the Kindle app's store books). Joining moves every book into one library; no book, rating or Nightstand is lost, and a book both had is kept once.</p>
      <ul class="space-y-2">${pairs.map((p, i) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-2 text-sm">
        <span><b>${esc(p.owner)}</b>: “${esc(p.from_name)}” (${p.from_books} books) into “${esc(p.into_name)}” (${p.into_books}), called “${esc(p.new_name)}”</span>
        <button type="button" data-join="${i}" class="btn-secondary py-1 text-sm">Join</button></li>`).join("")}</ul>`;
    host.querySelectorAll("[data-join]").forEach((b) => b.onclick = async () => {
      const p = pairs[Number(b.dataset.join)];
      if (!confirm(`Join “${p.from_name}” into “${p.into_name}” and call it “${p.new_name}”? This can't be undone.`)) return;
      const r = await attempt(() => post("/api/admin/catalogs/merge", { from: p.from, into: p.into, name: p.new_name }), "Joined");
      if (r) draw(r.pairs || []);
    });
  };
  const d = await attempt(() => get("/api/admin/catalogs/kindle-pairs"));
  draw(d?.pairs || []);
}
