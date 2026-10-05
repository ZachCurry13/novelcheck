// The reader in Deep Scan Review and in a scanned book's window (parents):
// the part a scan noted, highlighted in the
// book's own text with what comes before and after it, so a parent can read
// the scene and decide. ◀ Earlier / Later ▶ move through the book a part at a
// time. It follows each person's font and theme.
import { get } from "./api.js";
import { esc, attempt } from "./ui.js";

const paras = (text, cls = "") => text.split(/\n\n+/).filter(Boolean).map((p) => `<p class="${cls}">${esc(p)}</p>`).join("");

// openPassage shows words [from, to) of the scan's book; an older scan's part
// is found by its label.
export async function openPassage(scanID, title, part) {
  let d = document.getElementById("deep-reader");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "deep-reader";
    d.className = "dialog";
    document.body.append(d);
  }
  let { from, to, label } = part;
  const about = part.about || ""; // what the AI saw there (asked for, so spoilers are fine)
  let flagged = null; // the part the AI read, once the server has found it
  const show = async () => {
    const q = to > from ? `from=${from}&to=${to}` : `label=${encodeURIComponent(label)}`;
    const p = await attempt(() => get(`/api/admin/deep-scans/${scanID}/passage?${q}`));
    if (!p) return false;
    from = p.from;
    to = p.to;
    flagged ??= { from, to };
    const mark = from === flagged.from && to === flagged.to;
    const pct = p.total ? Math.round((100 * p.from) / p.total) : 0;
    d.innerHTML = `<div class="flex max-h-[92dvh] flex-col">
      <div class="flex items-start justify-between gap-3 border-b border-slate-800 p-4">
        <div class="min-w-0"><h2 class="font-bold leading-snug">📖 ${esc(title)}</h2>
          <p class="text-xs text-slate-400">${mark ? `${esc(label)}${label.includes("%") ? "" : ` · about ${pct}% into the book`} · the highlighted part is what the AI read` : `About ${pct}% into the book`}</p></div>
        <button type="button" data-close class="btn-ghost h-10 w-10 shrink-0 p-0 text-xl" aria-label="Close">✕</button></div>
      <div data-scroll class="space-y-3 overflow-y-auto p-4 text-[0.95rem] leading-relaxed">
        ${p.before ? `<div class="space-y-3 text-slate-400">${paras(p.before)}</div>` : ""}
        <div data-mark class="space-y-3 ${mark ? "rounded-lg border-l-4 border-amber-400 bg-amber-950/40 p-3" : ""} text-slate-100">${about && mark ? `<p class="rounded-md bg-slate-900/70 p-2 text-sm text-slate-300"><b>What the AI saw:</b> ${esc(about)}</p>` : ""}${paras(p.text)}</div>
        ${p.after ? `<div class="space-y-3 text-slate-400">${paras(p.after)}</div>` : ""}
      </div>
      <div class="grid grid-cols-2 gap-2 border-t border-slate-800 p-3">
        <button type="button" data-move="-1" class="btn-secondary" ${from <= 0 ? "disabled" : ""}>◀ Earlier</button>
        <button type="button" data-move="1" class="btn-secondary" ${to >= p.total ? "disabled" : ""}>Later ▶</button></div></div>`;
    if (!d.open) d.showModal();
    d.querySelector("[data-mark]").scrollIntoView({ block: "start" });
    return true;
  };
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    const move = e.target.closest("[data-move]");
    if (!move) return;
    const span = Math.max(to - from, 200);
    const step = Number(move.dataset.move) * span;
    from = Math.max(0, from + step);
    to = from + span;
    await show();
  };
  await show();
}
