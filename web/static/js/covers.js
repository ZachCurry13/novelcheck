// Book covers (from Calibre, or a drawn placeholder), "Wrong cover?"
// reports, and the admin's list to review them.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";

// coverImg is a lazy-loaded cover; cls sets its size (keep a 2:3 shape).
export const coverImg = (id, cls = "h-24 w-16", large = false, bust = "") =>
  `<img src="/api/books/${Number(id)}/cover${large ? "?size=l" : ""}${bust ? `${large ? "&" : "?"}v=${bust}` : ""}" alt="" loading="lazy" decoding="async"
    class="${cls} shrink-0 rounded bg-slate-800 object-cover shadow">`;

// A stand-in for books that aren't in the library yet.
export const noCover = (cls = "h-20 w-14") =>
  `<div class="${cls} flex shrink-0 items-center justify-center rounded bg-slate-800 text-2xl" aria-hidden="true">📖</div>`;

// reportCover asks what's wrong and tells the admins.
export async function reportCover(b) {
  const note = prompt(`What's wrong with the cover of "${b.title}"? (optional)\nAn admin will fix it.`, "");
  if (note === null) return false;
  return !!(await attempt(() => post(`/api/books/${b.id}/cover-report`, { note }), "Thanks! An admin will fix the cover."));
}

function dialog() {
  let d = document.getElementById("cover-reports-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "cover-reports-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

// openCoverReports lists covers people flagged, for an admin to fix in Calibre.
export async function openCoverReports(onDone) {
  const d = dialog();
  const load = async () => {
    const data = await attempt(() => get("/api/admin/cover-reports"));
    if (!data) return false;
    const cw = data.calibre_web_url;
    const now = Date.now(); // show the cover as it is now, not a cached one
    d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🖼️ Covers reported as wrong</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">Change the cover in Calibre (Edit metadata → Download cover, or choose an image), then press <b>Fixed</b>. NovelCheck shows the new cover by itself.</p>
      <ul class="space-y-2">${data.reports.map((r) => `<li class="flex gap-3 rounded-lg bg-slate-800/60 p-3">
        ${coverImg(r.book_id, "h-24 w-16", false, now)}
        <div class="min-w-0 flex-1 space-y-1">
          <p><b>${esc(r.title)}</b> <span class="text-sm text-slate-400">${esc(r.author || "")}</span></p>
          <p class="text-xs text-slate-400">Reported by ${esc(r.username)}${r.note ? `: “${esc(r.note)}”` : ""}</p>
          <div class="flex flex-wrap items-center gap-2 pt-1">
            ${cw && r.calibre_id ? `<a href="${esc(cw)}/book/${r.calibre_id}" target="_blank" rel="noopener noreferrer" class="text-sm text-sky-300 underline">Open in Calibre-Web ↗</a>` : ""}
            <button data-fix="${r.id}" class="btn-primary py-1 text-sm">✓ Fixed</button>
            <button data-dismiss="${r.id}" class="btn-ghost py-1 text-sm">Dismiss</button>
          </div></div></li>`).join("") || `<li class="text-sm text-slate-500">No cover reports. 🎉</li>`}</ul></div>`;
    return true;
  };
  if (!(await load())) return;
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) {
      d.close();
      return onDone?.();
    }
    const fix = e.target.closest("[data-fix]")?.dataset.fix;
    const dismiss = e.target.closest("[data-dismiss]")?.dataset.dismiss;
    const id = fix || dismiss;
    if (id && (await attempt(() => post(`/api/admin/cover-reports/${id}/${fix ? "fixed" : "dismiss"}`), fix ? "Marked fixed" : "Dismissed"))) load();
  };
  d.showModal();
}
