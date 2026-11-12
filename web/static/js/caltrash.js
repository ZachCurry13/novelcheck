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
  const empty = books.filter((b) => !b.files.length).length;
  const days = new Map();
  for (const b of books) {
    const k = dayOf(b.removed);
    days.set(k, (days.get(k) || 0) + 1);
  }
  let shown = 100;
  let q = "";
  const paint = () => {
    const list = books.filter((b) => !q || `${b.title} ${b.author}`.toLowerCase().includes(q));
    host.innerHTML = `<p class="text-sm text-slate-300"><b>${fmtNum(books.length)}</b> book${books.length === 1 ? "" : "s"} in Calibre's recycle bin${empty ? ` (${fmtNum(empty)} of them empty entries with no book file)` : ""}.
        Calibre empties the bin by itself after a while, so put back what you want to keep soon.</p>
      <p class="text-xs text-slate-400">${[...days].map(([d, n]) => `${esc(d)}: ${fmtNum(n)}`).join(" · ")}</p>
      <p class="text-xs text-slate-500">${r.read_at ? `Read ${esc(when(r.read_at))}. ` : ""}<button type="button" data-again class="underline">Read the bin again</button></p>
      <input data-q class="input my-2" placeholder="Find a title or author" value="${esc(q)}" aria-label="Find in the recycle bin">
      <ul class="space-y-1">${list.slice(0, shown).map((b) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-2 text-sm" data-id="${esc(b.id)}">
        <span class="min-w-0 flex-1"><b class="break-words">${esc(b.title)}</b>
          <span class="block break-words text-xs text-slate-400">${esc(b.author || "Unknown author")} · was Calibre #${esc(b.id)} · removed ${esc(when(b.removed))}
            · ${b.files.length ? b.files.map((f) => `${esc(f.format)} ${size(f.size)}`).join(", ") : "no book file"}</span>
          <span class="block text-xs ${byId.has(b.id) ? "text-amber-300" : "text-slate-500"}">${byId.has(b.id) ? who(byId.get(b.id)) : "not removed through NovelCheck (Calibre, Calibre-Web, or before v1.71.1)"}</span></span>
        ${b.files.length ? `<button type="button" data-back class="btn-secondary shrink-0 py-1 text-sm">Put back</button>` : ""}</li>`).join("") || `<li class="text-sm text-slate-400">Nothing matches.</li>`}</ul>
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
  host.onclick = async (e) => {
    if (e.target.closest("[data-again]")) return renderCalTrash(host, true);
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
    btn.textContent = "✓ Back in Calibre";
  };
  paint();
}
