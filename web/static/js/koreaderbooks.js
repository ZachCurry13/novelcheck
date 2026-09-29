// 📱 On your KOReader (Up Next): every book opened on the reader's KOReader
// devices, from KOReader's reading statistics (synced to their NovelCheck
// folder): how far, for how long, and when. Books in the library open their
// window.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { coverImg } from "./covers.js";
import { openBook } from "./bookdialog.js";
import { openKOReaderSetup } from "./delivery.js";
import { ago, readTime, progressBar, setBars } from "./progress.js";

const SHOW = 8;

function row(b) {
  const inner = `${b.book_id ? coverImg(b.book_id, "h-14 w-10")
    : `<span class="flex h-14 w-10 shrink-0 items-center justify-center rounded bg-slate-800 text-lg" aria-hidden="true">📄</span>`}
    <span class="min-w-0 flex-1 space-y-1">
      <span class="block font-semibold leading-snug line-clamp-2">${esc(b.title)}</span>
      <span class="block truncate text-xs text-slate-400">${esc(b.authors || "Unknown author")}${b.book_id ? "" : " · not in your library"}</span>
      ${progressBar({ percent: b.percent })}
      <span class="block text-xs text-slate-400">${Math.round(b.percent * 100)}%${b.read_seconds ? ` · ${readTime(b.read_seconds)} read` : ""}${b.last_open ? ` · ${ago(b.last_open)}` : ""}</span>
    </span>`;
  return `<li>${b.book_id
    ? `<button type="button" data-open="${b.book_id}" class="card flex w-full items-center gap-3 p-2 text-left hover:ring-indigo-600">${inner}</button>`
    : `<div class="card flex items-center gap-3 p-2">${inner}</div>`}</li>`;
}

export async function renderKOReaderBooks(host, state, onChange) {
  const data = await get("/api/me/koreader-books").catch(() => null);
  if (!data) return;
  const books = data.books || [];
  let all = false;
  const paint = () => {
    if (!books.length) {
      // A hint for KOReader readers who haven't turned the statistics sync on.
      host.innerHTML = state.user.delivery_method !== "koreader" ? ""
        : `<p class="card mt-8 text-sm text-slate-400">📱 See every book you open in KOReader here, with how far you are:
          turn on its reading statistics sync. <button type="button" data-setup class="underline">KOReader setup</button></p>`;
      return;
    }
    host.innerHTML = `<div class="mt-8"><h2 class="label">📱 On your KOReader</h2>
      <p class="mb-2 text-xs text-slate-400">Every book opened on your KOReader devices${data.synced_at ? ` · synced ${ago(data.synced_at)}` : ""}</p>
      <ul class="space-y-2">${(all ? books : books.slice(0, SHOW)).map(row).join("")}</ul>
      ${books.length > SHOW && !all ? `<button type="button" data-more class="btn-ghost mt-2 text-sm">Show all ${books.length}</button>` : ""}</div>`;
    setBars(host);
  };
  paint();
  host.onclick = (e) => {
    if (e.target.closest("[data-setup]")) return openKOReaderSetup();
    if (e.target.closest("[data-more]")) {
      all = true;
      return paint();
    }
    const id = e.target.closest("[data-open]")?.dataset.open;
    if (id) openBook(Number(id), state, onChange);
  };
}
