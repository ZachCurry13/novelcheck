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
  const byId = new Map();
  for (const x of removals) if (!byId.has(String(x.calibre_id))) byId.set(String(x.calibre_id), x);
  const utc = (at) => (at.includes("T") ? at : `${at.replace(" ", "T")}Z`);
  const who = (x) => `removed through NovelCheck by ${esc(x.by || "someone")} (${esc(x.tool)})`;
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
  const FILTERS = { missing: "Missing from your library", kept: "Still in your library", empty: "Empty entries", all: "All" };
  let filter = books.some((b) => stateOf(b) === "missing") ? "missing" : "all";
  let busy = false;
  const stateLine = (b) => {
    const s = stateOf(b);
    if (s === "missing") return `<span class="block text-xs font-medium text-rose-300">Missing from your library: no copy of this book is in Calibre now</span>`;
    if (s === "kept") return `<span class="block text-xs text-emerald-300">Still in your library${b.library_at ? ` as Calibre #${esc(b.library_at)}` : ""}${b.library_title && b.library_title !== b.title ? ` (“${esc(b.library_title)}”)` : ""}: this was an extra copy</span>`;
    if (s === "back") return `<span class="block text-xs text-emerald-300">Put back in Calibre just now</span>`;
    return `<span class="block text-xs text-slate-500">An empty entry: nothing to put back</span>`;
  };
  let shown = 100;
  let q = "";
  const paint = () => {
    const counts = Object.fromEntries(Object.keys(FILTERS).map((k) => [k, k === "all" ? books.length : books.filter((b) => stateOf(b) === k).length]));
    const picks = missingPicks();
    const list = books.filter((b) => (filter === "all" || stateOf(b) === filter) && (!q || `${b.title} ${b.author}`.toLowerCase().includes(q)));
    host.innerHTML = `<p class="text-sm text-slate-300"><b>${fmtNum(books.length)}</b> book${books.length === 1 ? "" : "s"} in Calibre's recycle bin${empty ? ` (${fmtNum(empty)} of them empty entries with no book file)` : ""}.
        Calibre empties the bin by itself after a while, so put back what you want to keep soon.</p>
      <p class="text-xs text-slate-400">${[...days].map(([d, n]) => `${esc(d)}: ${fmtNum(n)}`).join(" · ")}</p>
      <p class="text-xs text-slate-500">${r.read_at ? `Read ${esc(when(r.read_at))}. ` : ""}<button type="button" data-again class="underline">Read the bin again</button></p>
      <div class="my-2 flex flex-wrap gap-2" role="group" aria-label="Show">${Object.entries(FILTERS).map(([k, l]) =>
        `<button type="button" data-filter="${k}" aria-pressed="${filter === k}" class="chip-filter">${l} · ${fmtNum(counts[k])}</button>`).join("")}</div>
      ${picks.length ? `<div class="card-outlined my-2 space-y-2 p-3"><p class="text-sm"><b>${fmtNum(picks.length)}</b> book${picks.length === 1 ? " is" : "s are"} in the bin with no copy left in your library.
          Put back adds each to Calibre again (one entry per book, the EPUB when there is one).</p>
        <button type="button" data-all class="btn-primary" ${busy ? "disabled" : ""}>Put back all ${fmtNum(picks.length)} missing</button>
        <p data-progress class="text-xs text-slate-400"></p></div>` : counts.missing === 0 && books.length ? `<p class="my-2 text-sm text-emerald-300">Every book in the bin still has a copy in your library.</p>` : ""}
      <input data-q class="input my-2" placeholder="Find a title or author" value="${esc(q)}" aria-label="Find in the recycle bin">
      <ul class="space-y-1">${list.slice(0, shown).map((b) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-2 text-sm" data-id="${esc(b.id)}">
        <span class="min-w-0 flex-1"><b class="break-words">${esc(b.title)}</b>
          <span class="block break-words text-xs text-slate-400">${esc(b.author || "Unknown author")} · was Calibre #${esc(b.id)} · removed ${esc(when(b.removed))}
            · ${(b.files || []).length ? (b.files || []).map((f) => `${esc(f.format)} ${size(f.size)}`).join(", ") : "no book file"}</span>
          ${stateLine(b)}
          <span class="block text-xs ${byId.has(b.id) ? "text-amber-300" : "text-slate-500"}">${byId.has(b.id) ? who(byId.get(b.id)) : "not removed through NovelCheck (Calibre, Calibre-Web, or before v1.71.1)"}</span></span>
        ${(b.files || []).length && stateOf(b) !== "back" ? `<button type="button" data-back class="btn-secondary shrink-0 py-1 text-sm">Put back</button>` : ""}</li>`).join("") || `<li class="text-sm text-slate-400">Nothing matches.</li>`}</ul>
      ${list.length > shown ? `<button type="button" data-more class="btn-ghost mt-2 py-1 text-sm">Show ${fmtNum(Math.min(100, list.length - shown))} more of ${fmtNum(list.length)}</button>` : ""}
      ${removals.length ? `<details class="mt-3 rounded-lg bg-slate-900/40 px-3 py-2"><summary class="cursor-pointer text-sm font-semibold">NovelCheck's removal log (${fmtNum(removals.length)})</summary>
        <p class="mt-1 text-xs text-slate-400">Every book NovelCheck removed from Calibre, newest first, kept for a year. File formats removed are under Removed formats.</p>
        <ul class="list mt-1 text-sm">${removals.slice(0, 300).map((x) => `<li class="list-row items-start"><div class="min-w-0 flex-1"><p class="break-words font-medium">${esc(x.title || `Calibre #${x.calibre_id}`)}</p>
          <p class="text-xs text-slate-400">${esc(when(utc(x.at)))} · ${esc(x.by || "someone")} · ${esc(x.tool)} · was Calibre #${x.calibre_id}</p></div></li>`).join("")}</ul></details>` : ""}
      <p class="mt-2 text-xs text-slate-400">“Put back” adds the book to Calibre again from its files, as a new entry with the title and authors it had. Its place in a series, tags and cover come back only as far as the file itself carries them.</p>`;
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
    const f = e.target.closest("[data-filter]");
    if (f) {
      filter = f.dataset.filter;
      shown = 100;
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
