// Admin → Delete requests: books people asked to delete. Delete them from
// Calibre (to its recycle bin), dismiss them, or mark them done.
import { get, post } from "./api.js";
import { $, $$, esc, attempt } from "./ui.js";
import { copyText } from "./copy.js";

const STATUS = { deleted: ["🗑 Deleted", "text-rose-300"], dismissed: ["↩ Kept", "text-slate-300"], done: ["✓ Done", "text-emerald-300"] };
const when = (s) => {
  const d = new Date(s && !s.includes("T") ? s.replace(" ", "T") + "Z" : s);
  return isNaN(d) ? "" : d.toLocaleDateString();
};

export async function renderDeletions(view) {
  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">Delete requests</h1>
    <p class="mb-4 max-w-3xl text-sm text-slate-400">Books someone asked to delete. <b>Delete from Calibre</b> moves them to Calibre's
      recycle bin (you can restore them from Calibre's Trash). <b>Keep</b> dismisses the request. For books that aren't in Calibre
      (for example only on a Kindle), delete them on the device and click <b>Mark done</b>.</p>
    <div id="del-result"></div>
    <div id="del-body"><p class="text-slate-400">Loading…</p></div>`;
  const body = $("#del-body", view);
  const result = $("#del-result", view);

  async function load() {
    const data = await attempt(() => get("/api/admin/delete-requests"));
    if (!data) return;
    const pending = data.pending.length ? `
      <div class="card mb-4 flex flex-wrap items-center gap-2">
        <label class="toggle mr-auto"><input type="checkbox" id="del-all" checked> <b>${data.pending.length}</b>&nbsp;book${data.pending.length === 1 ? "" : "s"} waiting</label>
        <button data-do="copy" class="btn-ghost py-1 text-xs">📋 Copy Calibre search</button>
        <button data-do="done" class="btn-ghost py-1 text-xs">✓ Mark done</button>
        <button data-do="dismiss" class="btn-secondary py-1">↩ Keep</button>
        ${data.can_remove ? `<button data-do="delete" class="btn-danger py-1">🗑 Delete from Calibre</button>`
          : `<span class="text-xs text-slate-400">Turn on <a href="#/admin" class="underline">One-click removal</a> to delete from here.</span>`}
      </div>
      <div class="mb-6 space-y-2">${data.pending.map((g) => `
        <label class="card flex cursor-pointer gap-3">
          <input type="checkbox" data-book="${g.book_id}" data-cal="${esc(g.calibre_ids.join(","))}" checked class="mt-1">
          <div class="min-w-0 flex-1 space-y-1 text-sm">
            <p><b>${esc(g.title)}</b> <span class="text-slate-400">· ${esc(g.author || "Unknown author")}</span></p>
            <p class="flex flex-wrap items-center gap-1 text-xs text-slate-400">${esc(g.catalogs)}
              ${g.formats ? g.formats.split(",").map((f) => `<span class="chip-fmt">${esc(f)}</span>`).join(" ") : ""}
              ${g.calibre_ids.length ? `· Calibre #${g.calibre_ids.map(esc).join(", #")}` : `· <span class="text-amber-300">not in Calibre</span>`}</p>
            ${g.requests.map((r) => `<p class="text-slate-300">🗑 <b>${esc(r.username || "someone")}</b>, ${esc(when(r.created_at))}${r.reason ? `: “${esc(r.reason)}”` : ""}</p>`).join("")}
          </div>
        </label>`).join("")}</div>`
      : `<div class="card mb-6 text-slate-300">✓ No books waiting. People can ask for a book to be deleted with <b>🗑 Request to delete</b> in the book's window.</div>`;
    const recent = data.recent.length ? `
      <h2 class="mb-2 text-lg font-semibold">Recently handled</h2>
      <ul class="card divide-y divide-slate-800 text-sm">${data.recent.map((r) => {
        const [label, cls] = STATUS[r.status] || [r.status, ""];
        return `<li class="flex flex-wrap gap-x-2 py-2"><span class="${cls}">${label}</span> <b>${esc(r.title)}</b>
          <span class="text-slate-400">asked by ${esc(r.username || "someone")}${r.decided_by ? `, handled by ${esc(r.decided_by)}` : ""} ${esc(when(r.decided_at))}</span></li>`;
      }).join("")}</ul>` : "";
    body.innerHTML = pending + recent;
  }

  const picked = () => $$("[data-book]:checked", body);
  body.onchange = (e) => {
    if (e.target.id === "del-all") $$("[data-book]", body).forEach((c) => (c.checked = e.target.checked));
  };
  body.onclick = async (e) => {
    const b = e.target.closest("[data-do]");
    if (!b) return;
    const boxes = picked();
    if (!boxes.length) return result.innerHTML = `<div class="card mb-4 text-amber-200">Tick at least one book.</div>`;
    const ids = boxes.map((c) => Number(c.dataset.book));
    if (b.dataset.do === "copy") {
      const cal = boxes.flatMap((c) => c.dataset.cal.split(",").filter(Boolean));
      return copyText(cal.length ? cal.map((id) => `id:${id}`).join(" or ") : "(none of these are in Calibre)");
    }
    const n = `${ids.length} book${ids.length === 1 ? "" : "s"}`;
    if (b.dataset.do === "delete" && !confirm(`Delete ${n} from Calibre? They go to Calibre's recycle bin.`)) return;
    b.disabled = true;
    const r = await attempt(() => post("/api/admin/delete-requests/decide", { book_ids: ids, action: b.dataset.do }));
    b.disabled = false;
    if (!r) return;
    result.innerHTML = `<div class="card mb-4 text-emerald-200">${{
      delete: `🗑 Deleted ${r.removed} from Calibre (in its recycle bin)${r.skipped ? `; ${r.skipped} were already gone` : ""}.${
        r.not_in_calibre ? ` ${r.not_in_calibre} aren't in Calibre: delete those on the device, then Mark done.` : ""}`,
      dismiss: `↩ Kept ${n}; the requests are closed.`,
      done: `✓ Marked ${n} done.`,
    }[b.dataset.do]}</div>`;
    await load();
  };
  await load();
}
