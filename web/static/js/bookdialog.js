// Book detail modal: verdict, blurb, catalog copies and actions.
import { get, post } from "./api.js";
import { $, esc, attempt, classChip, flagChips, canManage } from "./ui.js";
import { verdictFormHTML, bindVerdictForm } from "./verdictform.js";

export async function openBook(id, state, onChange) {
  const dlg = $("#book-dialog");
  const data = await attempt(() => get(`/api/books/${id}`));
  if (!data) return;
  const b = data.book;
  const isAdmin = state.user.role === "admin";
  const manager = canManage(state.user);
  const copies = data.copies.map((c) => `
    <li class="flex justify-between gap-3 text-sm">
      <span class="chip-cat">${esc(c.catalog_name)}</span>
      <span class="truncate text-slate-400">${esc(c.format.toUpperCase())}${isAdmin && c.path ? " · " + esc(c.path) : ""}</span>
    </li>`).join("");
  dlg.innerHTML = `
    <div class="max-h-[85vh] overflow-y-auto p-6 space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h2 class="text-xl font-bold">${esc(b.title)}</h2>
          <p class="text-slate-400">${esc(b.author || "Unknown author")}${b.isbn ? " · ISBN " + esc(b.isbn) : ""}</p>
        </div>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button>
      </div>
      <div class="flex flex-wrap gap-1">${classChip(b)} ${flagChips(b)}</div>
      ${b.summary_verdict ? `<p class="rounded-lg bg-slate-800 p-3 text-slate-200">${esc(b.summary_verdict)}</p>` : ""}
      ${b.status === "error" && manager ? `<p class="text-sm text-rose-400">Last error: ${esc(b.analysis_error)}</p>` : ""}
      ${(b.blurb || b.description) ? `<div><span class="label">Blurb</span>
        <p class="text-sm leading-relaxed text-slate-300 whitespace-pre-line">${esc(b.blurb || b.description)}</p></div>` : ""}
      <div><span class="label">In catalogs</span><ul class="space-y-1">${copies || "<li class='text-sm text-slate-500'>None</li>"}</ul></div>
      ${b.analysis_model ? `<p class="text-xs text-slate-500">${b.analysis_model.startsWith("manual: ")
        ? "Rated by hand by " + esc(b.analysis_model.slice(8)) : "Analyzed by " + esc(b.analysis_model)}${b.analyzed_at ? " · " + esc(new Date(b.analyzed_at).toLocaleDateString()) : ""}</p>` : ""}
      ${manager ? verdictFormHTML(b) : ""}
      <div class="flex flex-wrap gap-2 pt-2">
        <button data-act="queue" class="btn-primary">Add to Up Next</button>
        ${data.downloadable ? `<a href="/api/books/${b.id}/download" class="btn-secondary">Download</a>` : ""}
        ${manager ? `<button data-act="analyze" class="btn-secondary">${b.classification ? "Re-analyze" : "Analyze now"}</button>
          <button data-act="edit-verdict" class="btn-secondary">Edit rating</button>` : ""}
      </div>
    </div>`;
  const refresh = () => {
    dlg.close();
    onChange?.();
  };
  if (manager) bindVerdictForm(dlg, b.id, refresh);
  dlg.onclick = async (e) => {
    if (e.target === dlg || e.target.closest("[data-close]")) return dlg.close();
    const act = e.target.closest("[data-act]")?.dataset.act;
    if (act === "edit-verdict" || act === "cancel-verdict") {
      $("#verdict-form", dlg).classList.toggle("hidden", act === "cancel-verdict");
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
