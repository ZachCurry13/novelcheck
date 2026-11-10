// 🔎 Search your Libby libraries for any book, whether or not NovelCheck has it, and put it on the Nightstand.
// Used on Discover (a box under the heading) and in the Library when a search finds nothing.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";

const num = (n) => Number(n || 0).toLocaleString();

function line(icon, a) {
  if (!a) return "";
  const where = a.library ? ` · ${esc(a.library)}` : "";
  const state = a.available ? `<b class="text-emerald-300">Available now</b>` : `<b class="text-amber-300">${num(a.holds)} hold${a.holds === 1 ? "" : "s"}${a.wait_days ? `, about ${num(a.wait_days)} days` : ""}</b>`;
  return `<a href="${esc(a.url)}" target="_blank" rel="noopener noreferrer" class="block text-sm underline-offset-2 hover:underline">${icon} ${state}${where} ↗</a>`;
}

function row(b, i) {
  return `<li class="card flex gap-3 p-3">
    ${b.cover_img ? `<img src="${esc(b.cover_img || "")}" alt="" loading="lazy" referrerpolicy="no-referrer" class="h-24 w-16 shrink-0 rounded object-cover">` : `<div class="h-24 w-16 shrink-0 rounded bg-slate-800"></div>`}
    <div class="min-w-0 flex-1 space-y-1"><p class="font-semibold leading-tight">${esc(b.title)}</p><p class="text-sm text-slate-400">${esc(b.author)}</p>
      ${line("📚", b.ebook)}${line("🎧", b.audio)}
      <button type="button" data-lsadd="${i}" class="btn-secondary mt-1 py-1 text-sm">➕ Add to Nightstand</button></div></li>`;
}

// libbySearchBox fills host with a search box and its results. If q is given it fills the box (auto: search at once).
export function libbySearchBox(host, q = "", auto = false) {
  host.innerHTML = `<form data-lsform class="flex flex-wrap gap-2"><input name="q" type="search" value="${esc(q)}" class="input min-w-0 flex-1" placeholder="Search your libraries: title, author or series" aria-label="Search your libraries" autocomplete="off">
    <button class="btn-primary">Search</button></form><div data-lsout class="mt-3"></div>`;
  const out = host.querySelector("[data-lsout]");
  let books = [];
  const run = async (text) => {
    if (text.trim().length < 2) return;
    out.innerHTML = `<p class="text-sm text-slate-400">Searching your libraries…</p>`;
    const r = await get(`/api/libby/search?q=${encodeURIComponent(text.trim())}`).catch(() => null);
    if (!r) out.innerHTML = `<p class="text-sm text-slate-400">Couldn't search just now.</p>`;
    else if (!r.configured) out.innerHTML = `<p class="text-sm text-slate-400"><a href="#/settings?s=connections" class="underline">Add your Libby library in Settings</a> to search it.</p>`;
    else {
      books = r.books || [];
      out.innerHTML = books.length ? `<ul class="space-y-2">${books.map(row).join("")}</ul>` : `<p class="text-sm text-slate-400">${esc(r.error || "Your libraries don't have anything for that.")}</p>`;
    }
  };
  host.querySelector("[data-lsform]").addEventListener("submit", (e) => {
    e.preventDefault();
    run(e.target.q.value);
  });
  out.addEventListener("click", async (e) => {
    const btn = e.target.closest("[data-lsadd]");
    const b = btn && books[Number(btn.dataset.lsadd)];
    if (!b) return;
    btn.disabled = true;
    const made = await attempt(() => post("/api/libby/add", { title: b.title, author: b.author, cover: b.cover || "" }));
    const queued = made && (await attempt(() => post("/api/queue", { book_id: made.book_id }), "Added to your Nightstand"));
    if (queued) btn.textContent = "✓ On your Nightstand";
    else btn.disabled = false;
  });
  if (q && auto) run(q);
}
