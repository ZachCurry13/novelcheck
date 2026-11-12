// Review covers (Admin → Library cleanup → Books with no cover): one book at a time, the title and author it has,
// a cleaner title to save to Calibre when its title carries series numbers or brackets, and cover candidates from
// Open Library and Google Books to tap. A search box tries other words; Skip moves on.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

export async function openCoverReview(onDone) {
  let d = document.getElementById("cover-review");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "cover-review";
    d.className = "dialog";
    d.setAttribute("aria-label", "Review covers");
    document.body.append(d);
  }
  d.innerHTML = `<div class="p-5 text-sm text-slate-400">Finding the books with no cover…</div>`;
  if (!d.open) d.showModal();
  const r = await get("/api/admin/covers/missing?limit=2000").catch(() => null);
  const books = r?.books || [];
  let i = 0, fixed = 0, data = null, q = "";
  const preview = (u) => `/api/admin/covers/preview?u=${encodeURIComponent(u)}`;
  const paint = () => {
    if (i >= books.length) {
      d.innerHTML = `<div class="space-y-3 p-5"><h2 class="text-lg font-medium">${books.length ? "That's all of them" : "Every book has a cover"}</h2>
        <p class="text-sm text-slate-400">${fixed ? `${fixed} cover${fixed === 1 ? "" : "s"} chosen.` : ""}</p>
        <button type="button" data-close class="btn-primary">Done</button></div>`;
      return;
    }
    const b = books[i];
    const cands = data?.candidates || [];
    d.innerHTML = `<div class="max-h-[85vh] space-y-4 overflow-y-auto p-5">
      <div class="flex items-center justify-between gap-3"><p class="text-sm text-slate-400">Book ${i + 1} of ${books.length} with no cover</p>
        <button type="button" data-close class="icon-btn" aria-label="Close"><span class="ms" aria-hidden="true">close</span></button></div>
      <div><p class="book-title text-xl font-semibold leading-snug">${esc(b.title)}</p><p class="text-slate-400">${esc(b.author || "Unknown author")}</p></div>
      ${data?.clean_title ? `<div class="card-outlined space-y-2 p-3"><p class="text-sm">A cleaner title: <b>${esc(data.clean_title)}</b></p>
        <button type="button" data-title class="btn-tonal">Use this title in Calibre</button></div>` : ""}
      <div class="space-y-2"><p class="text-sm font-medium">Tap the right cover</p>
        ${!data ? `<p class="text-sm text-slate-400">Looking…</p>`
          : cands.length ? `<div class="grid grid-cols-3 gap-3 sm:grid-cols-4">${cands.map((c, k) => `<button type="button" data-pick="${k}" class="space-y-1 text-left">
              <img src="${esc(preview(c.url))}" alt="" class="aspect-[2/3] w-full rounded-md object-cover shadow">
              <span class="block truncate text-xs text-slate-400">${esc(c.title)}</span></button>`).join("")}</div>`
          : `<p class="text-sm text-slate-400">No covers found under this title. Try other words below.</p>`}</div>
      <form data-search class="flex gap-2"><input name="q" class="input" placeholder="Search other words (a title, a series)" value="${esc(q)}">
        <button class="btn-secondary shrink-0">Search</button></form>
      <div class="flex justify-end"><button type="button" data-skip class="btn-text">Skip</button></div></div>`;
  };
  const load = async () => {
    data = null;
    paint();
    if (i >= books.length) return;
    data = await get(`/api/admin/covers/suggest?id=${books[i].id}${q ? `&q=${encodeURIComponent(q)}` : ""}`).catch(() => ({ candidates: [] }));
    paint();
  };
  const next = () => {
    i++;
    q = "";
    load();
  };
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) {
      d.close();
      return onDone?.();
    }
    if (e.target.closest("[data-skip]")) return next();
    const pick = e.target.closest("[data-pick]");
    if (pick) {
      const c = data.candidates[Number(pick.dataset.pick)];
      pick.disabled = true;
      if (await attempt(() => post("/api/admin/covers/choose", { id: books[i].id, url: c.url }))) {
        fixed++;
        toast("Cover saved");
        next();
      } else pick.disabled = false;
      return;
    }
    if (e.target.closest("[data-title]")) {
      const t = data.clean_title;
      if (await attempt(() => post(`/api/books/${books[i].id}/calibre-title`, { title: t }), "Title saved in Calibre")) {
        books[i].title = t;
        data.clean_title = "";
        q = "";
        load();
      }
    }
  };
  d.onsubmit = (e) => {
    e.preventDefault();
    q = e.target.q.value.trim();
    load();
  };
  load();
}
