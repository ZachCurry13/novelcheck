// Admin → Library cleanup → Calibre's recycle bin: the books removed from Calibre lately (by NovelCheck, Calibre
// or Calibre-Web), read from the hidden .caltrash folder in the library, with a button to put one back.
import { get, post } from "./api.js";
import { esc, attempt, toast, fmtNum } from "./ui.js";

const MB = 1 << 20;
const size = (b) => (!b ? "" : b >= MB ? `${(b / MB).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`);
const when = (iso) => (iso ? new Date(iso).toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }) : "");
const dayOf = (iso) => (iso ? new Date(iso).toLocaleDateString([], { weekday: "long", month: "short", day: "numeric" }) : "Unknown day");

export async function renderCalTrash(host, fresh = false) {
  host.innerHTML = `<p class="text-sm text-slate-400">Looking in Calibre's recycle bin…</p>`;
  // The server reads the bin in the background (it can take a few minutes on a network share) and keeps the
  // result, so this asks every two seconds and shows how far it got. Leaving and coming back carries on.
  let r = await attempt(() => get(`/api/admin/calibre/trash${fresh ? "?fresh=1" : ""}`));
  while (r?.reading && host.isConnected) {
    host.innerHTML = `<p class="text-sm text-slate-400">Looking in Calibre's recycle bin…${r.total ? ` ${fmtNum(r.done)} of ${fmtNum(r.total)} books read.` : ""}</p>
      <p class="text-xs text-slate-500">On a network share this can take a few minutes. You can leave this page; it carries on and is kept for an hour.</p>`;
    await new Promise((ok) => setTimeout(ok, 2000));
    r = await get("/api/admin/calibre/trash").catch(() => null);
  }
  if (!r || !host.isConnected) return;
  if (!r.found) {
    host.innerHTML = `<p class="text-sm text-slate-400">NovelCheck found no recycle bin in the Calibre library folder. Either nothing has been removed lately, or this Calibre keeps none (it needs Calibre 6.13 or newer).</p>`;
    return;
  }
  const books = r.books || [];
  // NovelCheck's own removal log: which of these it removed, who asked and from which page.
  const removals = r.removals || [];
  const utc = (at) => (at.includes("T") ? at : `${at.replace(" ", "T")}Z`);
  const empty = books.filter((b) => !(b.files || []).length).length;
  const days = new Map();
  for (const b of books) {
    const k = dayOf(b.removed);
    days.set(k, (days.get(k) || 0) + 1);
  }
  // Each removed book against the library as it is now (the server matches title and author):
  // still there (another copy is in Calibre), missing (no copy left: the ones to look at), or an empty entry.
  const back = new Set(); // put back during this visit
  const keyOf = (b) => (b.library_id ? `id:${b.library_id}` : `t:${b.title}|${b.author}`.toLowerCase());
  const stateOf = (b) => (!(b.files || []).length ? "empty" : back.has(keyOf(b)) ? "back" : b.in_library ? "kept" : "missing");
  // For "Put back all missing": one entry per book, the one with an EPUB (or the biggest file), newest first.
  const missingPicks = () => {
    const pick = new Map();
    for (const b of books) {
      if (stateOf(b) !== "missing") continue;
      const k = keyOf(b), cur = pick.get(k);
      const score = (x) => ((x.files || []).some((f) => f.format === "EPUB") ? 1e12 : 0) + Math.max(0, ...(x.files || []).map((f) => f.size || 0));
      if (!cur || score(b) > score(cur)) pick.set(k, b);
    }
    return [...pick.values()];
  };
  let busy = false;
  let showKept = false;
  let shown = 100;
  let q = "";
  const day = (iso) => (iso ? new Date(iso).toLocaleDateString([], { month: "short", day: "numeric" }) : "");
  // One row: the title, the author, when it went, and (for a missing book) Put back. Nothing else.
  const row = (b) => {
    const s = stateOf(b);
    const note = s === "kept" ? "Still in your library" : s === "back" ? "Back in Calibre" : s === "empty" ? "Empty entry" : `Removed ${esc(day(b.removed))}`;
    return `<li class="list-row" data-id="${esc(b.id)}">
      <div class="min-w-0 flex-1"><p class="book-title break-words font-medium leading-snug">${esc(b.title)}</p>
        <p class="text-sm text-slate-400">${esc(b.author || "Unknown author")} · <span class="${s === "back" || s === "kept" ? "text-emerald-300" : ""}">${note}</span></p></div>
      ${s === "missing" ? `<button type="button" data-back class="btn-secondary shrink-0">Put back</button>` : ""}</li>`;
  };
  const paint = () => {
    const missing = books.filter((b) => stateOf(b) === "missing" && (!q || `${b.title} ${b.author}`.toLowerCase().includes(q)));
    const others = books.filter((b) => stateOf(b) !== "missing" && (!q || `${b.title} ${b.author}`.toLowerCase().includes(q)));
    const picks = missingPicks();
    host.innerHTML = `
      <div class="card-outlined space-y-3 p-4">
        ${picks.length
          ? `<p class="text-lg font-medium">${fmtNum(picks.length)} book${picks.length === 1 ? " is" : "s are"} missing from your library</p>
             <p class="text-sm text-slate-400">They are in Calibre's bin with no copy left in your library. Put them back before Calibre empties the bin.</p>
             <button type="button" data-all class="btn-primary" ${busy ? "disabled" : ""}>Put back all ${fmtNum(picks.length)}</button>
             <p data-progress class="text-sm text-slate-400"></p>`
          : `<p class="text-lg font-medium">Nothing is missing</p>
             <p class="text-sm text-slate-400">Every book in Calibre's bin still has a copy in your library.</p>`}
      </div>
      ${books.length > 8 ? `<input data-q class="input mt-3" placeholder="Find a book" value="${esc(q)}" aria-label="Find a book in the bin">` : ""}
      ${missing.length ? `<ul class="list mt-2">${missing.slice(0, shown).map(row).join("")}</ul>` : ""}
      ${missing.length > shown ? `<button type="button" data-more class="btn-text mt-1">Show ${fmtNum(Math.min(100, missing.length - shown))} more</button>` : ""}
      ${others.length ? `<button type="button" data-kept class="btn-text mt-2">${showKept ? "Hide" : "Show"} the ${fmtNum(others.length)} book${others.length === 1 ? "" : "s"} you still have</button>
        ${showKept ? `<ul class="list">${others.slice(0, 200).map(row).join("")}</ul>` : ""}` : ""}
      <details class="mt-4 text-sm text-slate-400"><summary class="cursor-pointer">Details</summary>
        <div class="mt-2 space-y-2">
          <p>${fmtNum(books.length)} entries in Calibre's bin${empty ? `, ${fmtNum(empty)} of them empty` : ""}. Removed: ${[...days].map(([d, n]) => `${esc(d)} (${fmtNum(n)})`).join(", ")}.</p>
          <p>${r.read_at ? `Read ${esc(when(r.read_at))}. ` : ""}<button type="button" data-again class="underline">Read the bin again</button></p>
          <p>“Put back” adds the book to Calibre again from its files, as a new entry with the title and authors it had. Calibre empties its bin by itself after a while.</p>
          ${removals.length ? `<p class="pt-1 font-medium text-slate-300">Removed through NovelCheck (${fmtNum(removals.length)})</p>
            <ul class="list">${removals.slice(0, 300).map((x) => `<li class="list-row"><div class="min-w-0 flex-1"><p class="break-words text-slate-200">${esc(x.title || `Calibre #${x.calibre_id}`)}</p>
              <p class="text-xs">${esc(when(utc(x.at)))} · ${esc(x.by || "someone")} · ${esc(x.tool)}</p></div></li>`).join("")}</ul>` : ""}
        </div></details>`;
  };
  host.oninput = (e) => {
    if (!e.target.matches("[data-q]")) return;
    q = e.target.value.trim().toLowerCase();
    const at = e.target.selectionStart;
    paint();
    const box = host.querySelector("[data-q]");
    box.focus();
    box.setSelectionRange(at, at);
  };
  // restore puts one bin entry back in Calibre; it says whether it worked.
  const restore = async (b) => {
    const res = await post("/api/admin/calibre/trash/restore", { id: b.id }).catch((err) => ({ error: err.message || "failed" }));
    if (res.error) return res.error;
    back.add(keyOf(b));
    return "";
  };
  host.onclick = async (e) => {
    if (e.target.closest("[data-again]")) return renderCalTrash(host, true);
    if (e.target.closest("[data-kept]")) {
      showKept = !showKept;
      return paint();
    }
    if (e.target.closest("[data-all]") && !busy) {
      const picks = missingPicks();
      if (!confirm(`Put back ${picks.length} book${picks.length === 1 ? "" : "s"} that have no copy left in your library? Each is added to Calibre again as a new entry.`)) return;
      busy = true;
      const failed = [];
      for (const [i, b] of picks.entries()) {
        const p = host.querySelector("[data-progress]");
        if (p) p.textContent = `Putting back ${i + 1} of ${picks.length}: ${b.title}`;
        const err = await restore(b);
        if (err) failed.push(`${b.title}: ${err}`);
      }
      busy = false;
      paint();
      toast(failed.length ? `Put back ${picks.length - failed.length} of ${picks.length}. Not put back: ${failed.slice(0, 3).join("; ")}${failed.length > 3 ? "…" : ""}` : `All ${picks.length} are back in Calibre`, failed.length > 0);
      return;
    }
    if (e.target.closest("[data-more]")) {
      shown += 100;
      return paint();
    }
    const btn = e.target.closest("[data-back]");
    if (!btn) return;
    const li = btn.closest("[data-id]");
    btn.disabled = true;
    btn.textContent = "Putting back…";
    const res = await attempt(() => post("/api/admin/calibre/trash/restore", { id: li.dataset.id }));
    if (!res) {
      btn.disabled = false;
      btn.textContent = "Put back";
      return;
    }
    toast(`“${res.title}” is back in Calibre${res.notes?.length ? ` (${res.notes.join("; ")})` : ""}`, Boolean(res.notes?.length));
    back.add(keyOf(books.find((b) => b.id === li.dataset.id) || {}));
    paint();
  };
  paint();
}
