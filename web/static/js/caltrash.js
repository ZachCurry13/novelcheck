// Admin → Library cleanup → Calibre's recycle bin: the books removed from Calibre lately (by NovelCheck, Calibre
// or Calibre-Web), read from the hidden .caltrash folder in the library, with a button to put one back.
import { get, post } from "./api.js";
import { esc, attempt, toast, fmtNum } from "./ui.js";

const MB = 1 << 20;
const size = (b) => (!b ? "" : b >= MB ? `${(b / MB).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`);
const when = (iso) => (iso ? new Date(iso).toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }) : "");
const dayOf = (iso) => (iso ? new Date(iso).toLocaleDateString([], { weekday: "long", month: "short", day: "numeric" }) : "Unknown day");

export async function renderCalTrash(host) {
  host.innerHTML = `<p class="text-sm text-slate-400">Looking in Calibre's recycle bin…</p>`;
  const r = await attempt(() => get("/api/admin/calibre/trash"));
  if (!r) return;
  if (!r.found) {
    host.innerHTML = `<p class="text-sm text-slate-400">NovelCheck found no recycle bin in the Calibre library folder. Either nothing has been removed lately, or this Calibre keeps none (it needs Calibre 6.13 or newer).</p>`;
    return;
  }
  const books = r.books || [];
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
      <input data-q class="input my-2" placeholder="Find a title or author" value="${esc(q)}" aria-label="Find in the recycle bin">
      <ul class="space-y-1">${list.slice(0, shown).map((b) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-2 text-sm" data-id="${esc(b.id)}">
        <span class="min-w-0 flex-1"><b class="break-words">${esc(b.title)}</b>
          <span class="block break-words text-xs text-slate-400">${esc(b.author || "Unknown author")} · was Calibre #${esc(b.id)} · removed ${esc(when(b.removed))}
            · ${b.files.length ? b.files.map((f) => `${esc(f.format)} ${size(f.size)}`).join(", ") : "no book file"}</span></span>
        ${b.files.length ? `<button type="button" data-back class="btn-secondary shrink-0 py-1 text-sm">Put back</button>` : ""}</li>`).join("") || `<li class="text-sm text-slate-400">Nothing matches.</li>`}</ul>
      ${list.length > shown ? `<button type="button" data-more class="btn-ghost mt-2 py-1 text-sm">Show ${fmtNum(Math.min(100, list.length - shown))} more of ${fmtNum(list.length)}</button>` : ""}
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
