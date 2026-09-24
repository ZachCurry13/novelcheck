// Book detail modal: verdict, blurb, catalog copies and actions.
import { PEPPERS, openPepperGuide, grayAreaChip } from "./peppers.js";
import { copyText } from "./copy.js";
import { diagnoseLater } from "./diagnose.js";
import { get, post, put, del } from "./api.js";
import { $, esc, attempt, classChip, flagChips, ageChip, canManage } from "./ui.js";
import { verdictFormHTML, bindVerdictForm } from "./verdictform.js";
import { ageAndNotesHTML, bindAgeAndNotes } from "./booknotes.js";
import { on } from "./modules.js";

export async function openBook(id, state, onChange) {
  const dlg = $("#book-dialog");
  const data = await attempt(() => get(`/api/books/${id}`));
  if (!data) return;
  const b = data.book;
  const isAdmin = state.user.role === "admin";
  const manager = canManage(state.user);
  const parents = on(state.user, "parents"); // age groups and parents' notes
  // One row per catalog entry (a Calibre book id, or a Kindle/drive catalog), listing its formats.
  const entries = new Map();
  for (const c of data.copies) {
    const key = c.source === "calibre" ? `${c.catalog_id}#${c.external_id}` : String(c.catalog_id);
    if (!entries.has(key)) entries.set(key, { ...c, formats: [], paths: [] });
    const e = entries.get(key);
    if (c.format && c.format !== "list") e.formats.push(c.format.toUpperCase());
    if (c.path && !c.path.startsWith("list:") && !c.path.startsWith("calibre-entry:")) e.paths.push(c.path);
  }
  const calibreCount = [...entries.values()].filter((e) => e.source === "calibre").length;
  const cw = data.calibre_web_url; // set by an admin; only sent to admins and editors
  const copies = [...entries.values()].map((e) => `
    <li class="text-sm">
      <div class="flex flex-wrap items-center gap-2">
        <span class="chip-cat">${esc(e.catalog_name)}</span>
        ${e.source === "calibre" && e.external_id ? `<span class="text-xs text-slate-500">Calibre #${esc(e.external_id)}</span>` : ""}
        ${cw && e.source === "calibre" && /^\d+$/.test(e.external_id) ? `<a href="${esc(cw)}/book/${e.external_id}" target="_blank" rel="noopener noreferrer" class="text-xs text-sky-300 underline">Open in Calibre-Web ↗</a>` : ""}
        ${e.formats.length ? e.formats.map((f) => `<span class="chip-fmt">${esc(f)}</span>`).join(" ") : `<span class="text-xs text-slate-500">no file</span>`}
      </div>
      ${isAdmin && e.paths.length ? `<p class="mt-0.5 break-all text-xs text-slate-500">${e.paths.map(esc).join("<br>")}</p>` : ""}
    </li>`).join("");
  const dupNote = calibreCount > 1
    ? `<p class="mt-2 text-sm text-orange-300">⚠ This book is in Calibre ${calibreCount} times.${manager ? ` <a href="#/duplicates" data-close class="underline">Review duplicates</a>` : ""}</p>` : "";
  dlg.innerHTML = `
    <div class="max-h-[85vh] overflow-y-auto p-6 space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h2 class="text-xl font-bold">${esc(b.title)}</h2>
          <p class="text-slate-400">${esc(b.author || "Unknown author")}${b.isbn ? " · ISBN " + esc(b.isbn) : ""}</p>
        </div>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button>
      </div>
      <div class="flex flex-wrap gap-1">${classChip(b)} ${grayAreaChip(b)} ${ageChip(b)} ${flagChips(b)}</div>
      ${b.spice_level !== null && b.spice_level !== undefined
        ? `<p class="text-xs text-slate-400">${b.spice_reason ? `<b class="text-slate-200">Why ${b.spice_level} 🌶️:</b> ${esc(b.spice_reason)}. ` : ""}${esc(PEPPERS[b.spice_level].desc)} <button type="button" data-peppers class="underline">About peppers</button></p>` : ""}
      ${b.summary_verdict ? `<p class="rounded-lg bg-slate-800 p-3 text-slate-200">${esc(b.summary_verdict)}</p>` : ""}
      ${b.status === "error" && manager ? `<p class="text-sm text-rose-400">Last error: ${esc(b.analysis_error)}
        <button type="button" data-copy-err class="ml-1 text-xs underline">📋 Copy</button>${isAdmin ? ` <button type="button" data-dx-err class="text-xs underline">🩺 Diagnose</button>` : ""}</p>` : ""}
      ${(b.blurb || b.description) ? `<div><span class="label">Blurb</span>
        <p class="text-sm leading-relaxed text-slate-300 whitespace-pre-line">${esc(b.blurb || b.description)}</p></div>` : ""}
      <div><span class="label">In catalogs</span><ul class="space-y-2">${copies || "<li class='text-sm text-slate-500'>None</li>"}</ul>${dupNote}</div>
      ${b.analysis_model ? `<p class="text-xs text-slate-500">${b.analysis_model.startsWith("manual: ")
        ? "Rated by hand by " + esc(b.analysis_model.slice(8)) : "Analyzed by " + esc(b.analysis_model)}${b.analyzed_at ? " · " + esc(new Date(b.analyzed_at).toLocaleDateString()) : ""}</p>` : ""}
      ${b.approved ? `<p class="text-xs text-emerald-400">✓ Marked OK by ${esc(b.approved_by)}: shown to everyone, even if it matches their hide filters or content rules.</p>` : ""}
      ${manager ? verdictFormHTML(b) : ""}
      ${parents ? ageAndNotesHTML(b, data.notes || [], manager, state.user) : ""}
      <div class="flex flex-wrap gap-2 pt-2">
        ${on(state.user, "queue") ? `<button data-act="queue" class="btn-primary">Add to Up Next</button>` : ""}
        ${data.downloadable ? `<a href="/api/books/${b.id}/download" class="btn-secondary">Download</a>` : ""}
        ${manager ? `<button data-act="analyze" class="btn-secondary">${b.classification ? "Re-analyze" : "Analyze now"}</button>
          <button data-act="edit-verdict" class="btn-secondary">Edit rating</button>
          <button data-act="approve" class="btn-secondary" title="Show this book even when it matches someone's hide filters or content rules">${b.approved ? "Remove OK mark" : "✓ Mark as OK"}</button>` : ""}
        ${data.my_delete_request
          ? `<button data-act="cancel-delete" class="btn-ghost" title="${esc(data.my_delete_request.reason || "")}">🗑 Delete requested · Cancel</button>`
          : `<button data-act="request-delete" class="btn-ghost" title="Ask an admin to delete this book">🗑 Request to delete</button>`}
        ${isAdmin && b.delete_requests ? `<a href="#/deletions" data-close class="btn-ghost">Review delete requests (${b.delete_requests})</a>` : ""}
      </div>
    </div>`;
  const refresh = () => {
    dlg.close();
    onChange?.();
  };
  if (manager) bindVerdictForm(dlg, b.id, refresh);
  if (parents) bindAgeAndNotes(dlg, b, data.notes || [], state.user, onChange);
  dlg.onclick = async (e) => {
    if (e.target === dlg || e.target.closest("[data-close]")) return dlg.close();
    if (e.target.closest("[data-peppers]")) return openPepperGuide();
    if (e.target.closest("[data-copy-err]")) return copyText(`${b.title}: ${b.analysis_error}`);
    if (e.target.closest("[data-dx-err]")) {
      dlg.close();
      return diagnoseLater(`Rating "${b.title}" failed: ${b.analysis_error}`);
    }
    const act = e.target.closest("[data-act]")?.dataset.act;
    if (act === "edit-verdict" || act === "cancel-verdict") {
      $("#verdict-form", dlg).classList.toggle("hidden", act === "cancel-verdict");
    } else if (act === "approve") {
      const ok = await attempt(() => put(`/api/books/${b.id}/approval`, { approved: !b.approved }),
        b.approved ? "OK mark removed" : "Marked OK: it will show even when filters would hide it");
      if (ok) refresh();
    } else if (act === "request-delete") {
      const reason = prompt("Why should this book be deleted? (optional)\nAn admin will review it.", "");
      if (reason === null) return;
      const ok = await attempt(() => post(`/api/books/${b.id}/delete-request`, { reason }), "Delete requested. An admin will review it.");
      if (ok) refresh();
    } else if (act === "cancel-delete") {
      const ok = await attempt(() => del(`/api/books/${b.id}/delete-request`), "Delete request cancelled");
      if (ok) refresh();
    } else if (act === "queue") {
      await attempt(() => post("/api/queue", { book_id: b.id }), "Added to Up Next");
    } else if (act === "analyze") {
      const ok = await attempt(() => post(`/api/books/${b.id}/analyze`), "Queued for analysis");
      if (ok) {
        dlg.close();
        onChange?.();
      }
    }
  };
  dlg.showModal();
}
