// 🔗 Match books on my e-readers: the books on a Kindle that NovelCheck could not
// match to the library. Match one by hand (search the library and pick it) or
// mark it "not a book" (a thumbnail, KOReader's own guide). The choice sticks.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { coverImg } from "./covers.js";
import { readTime } from "./progress.js";

function dialog() {
  let d = document.getElementById("kindle-books-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "kindle-books-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

const ref = (i) => ({ kind: i.kind, device: i.device, filename: i.filename, title: i.title, authors: i.authors, md5: i.md5 });

function row(i, n) {
  const where = i.kind === "device" ? `on ${esc(i.device)}` : "from KOReader's statistics";
  const found = i.match_title ? `<span class="block text-xs text-sky-300">🔎 Open Library: ${esc(i.match_title)}${i.match_author ? ` by ${esc(i.match_author)}` : ""}</span>` : "";
  return `<li data-n="${n}" class="card space-y-2 p-3">
    <div class="flex gap-3">
      <div class="w-10 shrink-0">${i.book_id ? coverImg(i.book_id, "h-14 w-10") : `<span class="flex h-14 w-10 items-center justify-center rounded bg-slate-800" aria-hidden="true">📄</span>`}</div>
      <div class="min-w-0 flex-1">
        <p class="font-semibold leading-snug">${esc(i.title || i.filename)}</p>
        <p class="break-words text-xs text-slate-400">${esc(i.authors || i.filename || "")} · ${where}${i.percent ? ` · ${Math.round(i.percent * 100)}%` : ""}</p>
        ${found}</div></div>
    <div class="flex flex-wrap gap-2"><button type="button" data-act="match" class="btn-secondary px-3 py-1 text-sm">🔗 Match…</button>
      <button type="button" data-act="ignore" class="btn-ghost px-3 py-1 text-sm" title="A file that isn't a book: hide it">🚫 Not a book</button></div>
    <div data-search class="hidden space-y-2"></div></li>`;
}

export async function openKindleBooks(onChange) {
  const d = dialog();
  d.innerHTML = `<div class="p-5"><p class="text-slate-400">Loading…</p></div>`;
  d.showModal();
  const data = await get("/api/me/kindle-books").catch(() => null);
  let items = data?.items || [];
  const paint = () => {
    d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🔗 Match books on my e-readers</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">${items.length ? "These books on your e-readers aren't matched to your library. Match each to its book, or mark it not a book." : "Everything on your e-readers is matched. 🎉"}</p>
      <ul class="space-y-2">${items.map(row).join("")}</ul></div>`;
  };
  paint();
  const finish = (n, msg) => {
    items.splice(n, 1);
    toast(msg);
    paint();
    onChange?.();
  };
  d.onclick = async (e) => {
    if (e.target.closest("[data-close]")) return d.close(); // clicking outside does not close it: your place is kept
    const li = e.target.closest("[data-n]");
    const btn = e.target.closest("[data-act],[data-pick]");
    if (!li || !btn) return;
    const n = Number(li.dataset.n);
    const item = items[n];
    if (btn.dataset.act === "ignore") {
      if (await attempt(() => post("/api/me/kindle-match", { ...ref(item), ignored: true }))) finish(n, "Hidden: not a book");
    } else if (btn.dataset.act === "match") {
      const box = li.querySelector("[data-search]");
      box.classList.remove("hidden");
      box.innerHTML = `<input class="input" data-q placeholder="Search your library by title or author" value="${esc(item.title || "")}" aria-label="Search your library">
        <ul data-results class="space-y-1"></ul>`;
      const q = box.querySelector("[data-q]");
      const results = box.querySelector("[data-results]");
      let timer;
      const search = async () => {
        const text = q.value.trim();
        if (text.length < 2) return (results.innerHTML = "");
        const r = await get(`/api/books?q=${encodeURIComponent(text)}&limit=8`).catch(() => null);
        const list = r?.books || [];
        results.innerHTML = list.length ? list.map((b) => `<li><button type="button" data-pick="${b.id}" class="btn-ghost w-full justify-start text-left text-sm">${esc(b.title)} <span class="text-slate-400">· ${esc(b.author || "")}</span></button></li>`).join("")
          : `<li class="text-xs text-slate-500">Nothing in your library matches that.</li>`;
      };
      q.oninput = () => {
        clearTimeout(timer);
        timer = setTimeout(search, 250);
      };
      q.focus();
      search();
    } else if (btn.dataset.pick) {
      if (await attempt(() => post("/api/me/kindle-match", { ...ref(item), book_id: Number(btn.dataset.pick) }))) finish(n, "Matched");
    }
  };
}
