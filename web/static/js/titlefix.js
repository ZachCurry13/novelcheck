// Tidy titles in Calibre. NovelCheck shows "01 - Dune" as "Dune" (book 1)
// by itself; these dialogs let an admin make Calibre match, through its
// Content server, or open the book in Calibre-Web to change it there.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

const num = (n) => String(Number(n)); // 2, 2.5

// seriesText is "Discworld #8", or "Book 2" when only the number is known.
export function seriesText(b) {
  if (b.series) return b.series + (b.series_index ? ` #${num(b.series_index)}` : "");
  return b.series_index ? `Book ${num(b.series_index)}` : "";
}

export const seriesLine = (b) => (seriesText(b) ? `<p class="text-sm text-sky-300">📚 ${esc(seriesText(b))}</p>` : "");

function dialog(id) {
  let d = document.getElementById(id);
  if (!d) {
    d = document.createElement("dialog");
    d.id = id;
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

const cwLink = (cw, id) => (cw && id ? `<a href="${esc(cw)}/book/${id}" target="_blank" rel="noopener noreferrer" class="text-sky-300 underline">Open in Calibre-Web ↗</a>` : "");
const noServer = (cw) => `<p class="rounded-lg bg-amber-950/40 p-3 text-sm text-amber-200">To save from here, connect the Calibre Content server under <a href="#/admin?tab=delivery" data-close class="underline">Admin → Delivery &amp; Services → Calibre Library</a>.${cw ? " Or change it in Calibre-Web." : ""}</p>`;

// openTitleEdit edits one book's title and series in Calibre.
export async function openTitleEdit(b, calibreIds, cw, onSaved) {
  const srv = await attempt(() => get("/api/admin/calibre/server"));
  if (!srv) return;
  const d = dialog("title-edit-dialog");
  d.innerHTML = `<form class="space-y-3 p-5">
    <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">✏️ Title in Calibre</h2>
      <button type="button" data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    <p class="text-sm text-slate-400">${b.title_fix ? `Calibre calls it <b class="text-slate-200">“${esc(b.title_fix)}”</b>. NovelCheck already shows it tidied; save to make Calibre match.`
      : "Changes the title in your Calibre library."} Calibre renames the book's folder to match.</p>
    <label class="block"><span class="label">Title</span><input name="title" class="input" required maxlength="300" value="${esc(b.title)}"></label>
    <div class="flex gap-2">
      <label class="block flex-1"><span class="label">Series (optional)</span><input name="series" class="input" maxlength="200" value="${esc(b.series || "")}" placeholder="e.g. Discworld"></label>
      <label class="block w-24"><span class="label">Number</span><input name="series_index" type="number" min="0" max="9999" step="any" class="input" value="${b.series_index ? num(b.series_index) : ""}"></label>
    </div>
    <p class="text-xs text-slate-500">Calibre keeps the number only with a series name.</p>
    ${srv.url ? `<button class="btn-primary">Save to Calibre</button>` : noServer(cw)}
    <div class="flex flex-wrap gap-3 text-sm">${calibreIds.map((id) => cwLink(cw, id)).join("")}</div>
  </form>`;
  const form = d.querySelector("form");
  d.onclick = (e) => {
    if (e.target === d || e.target.closest("[data-close]")) d.close();
  };
  form.onsubmit = async (e) => {
    e.preventDefault();
    const f = new FormData(form);
    const body = { title: f.get("title"), series: f.get("series"), series_index: Number(f.get("series_index")) || 0 };
    if (await attempt(() => post(`/api/books/${b.id}/calibre-title`, body), "Saved in Calibre")) {
      d.close();
      onSaved?.();
    }
  };
  d.showModal();
}

// openTitleFixes lists every numbered title, to tidy them in Calibre at once.
export async function openTitleFixes(onDone) {
  const d = dialog("title-fixes-dialog");
  const load = async () => {
    const data = await attempt(() => get("/api/admin/title-fixes"));
    if (!data) return false;
    const { fixes, server, calibre_web_url: cw } = data;
    d.innerHTML = `<form class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🏷️ Tidy titles in Calibre</h2>
        <button type="button" data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">${fixes.length ? `These titles carry track or series numbers in Calibre. NovelCheck already shows them tidied${server ? "; tick the ones to change in Calibre too." : "."}` : "Every title in Calibre is tidy. 🎉"}</p>
      ${!server && fixes.length ? noServer(cw) : ""}
      ${server && fixes.length > 1 ? `<label class="flex items-center gap-2 text-sm"><input type="checkbox" data-all checked> All ${fixes.length}</label>` : ""}
      <ul class="space-y-2">${fixes.map((f) => `<li><label class="flex items-start gap-2 rounded-lg bg-slate-800/60 p-2 text-sm">
        ${server ? `<input type="checkbox" name="id" value="${f.id}" checked class="mt-1">` : ""}
        <span class="min-w-0"><s class="text-slate-500">${esc(f.calibre_title)}</s> → <b>${esc(f.title)}</b>
          <span class="block text-xs text-slate-400">${esc(f.author || "Unknown author")}${seriesText(f) ? ` · 📚 ${esc(seriesText(f))}` : ""}${cw && f.calibre_id ? ` · ${cwLink(cw, f.calibre_id)}` : ""}</span></span></label></li>`).join("")}</ul>
      ${server && fixes.length ? `<button class="btn-primary">Tidy selected in Calibre</button>
        <p class="text-xs text-slate-500">Series numbers found in a title are kept in NovelCheck; to store them in Calibre too, add the series name from the book's own page.</p>` : ""}
    </form>`;
    const form = d.querySelector("form");
    form.onchange = (e) => {
      if (e.target.matches("[data-all]")) form.querySelectorAll("[name=id]").forEach((c) => (c.checked = e.target.checked));
    };
    form.onsubmit = async (e) => {
      e.preventDefault();
      const ids = [...form.querySelectorAll("[name=id]:checked")].map((c) => Number(c.value));
      if (!ids.length) return toast("Tick at least one title", true);
      if (!confirm(`Change ${ids.length} title${ids.length === 1 ? "" : "s"} in Calibre? Calibre renames their folders to match.`)) return;
      const r = await attempt(() => post("/api/admin/title-fixes", { ids }));
      if (r) {
        toast(`Tidied ${r.fixed} title${r.fixed === 1 ? "" : "s"} in Calibre`);
        onDone?.();
        load();
      }
    };
    return true;
  };
  if (!(await load())) return;
  d.onclick = (e) => {
    if (e.target === d || e.target.closest("[data-close]")) d.close();
  };
  d.showModal();
}
