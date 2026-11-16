// Fix a book's title and author (parents and admins), for books that came
// from Check a book, a list, an event or Discover. A Calibre book's title is
// changed in Calibre instead (titlefix.js).
import { put } from "./api.js";
import { esc, attempt } from "./ui.js";

function dialog() {
  let d = document.getElementById("rename-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "rename-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

// openRename asks for the title and author; onSaved(id) gets the book's id
// (another one when it joined a book NovelCheck already had by that name).
export function openRename(b, onSaved) {
  const d = dialog();
  d.innerHTML = `<form method="dialog" class="space-y-3 p-5">
    <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-semibold">Title and author</h2>
      <button type="button" data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    <label class="block text-sm"><span class="label">Title</span>
      <input name="title" class="input w-full" maxlength="300" required value="${esc(b.title)}"></label>
    <label class="block text-sm"><span class="label">Author</span>
      <input name="author" class="input w-full" maxlength="200" value="${esc(b.author || "")}"></label>
    <p class="text-xs text-slate-400">The old name is remembered, so checking the book again by it finds this one. If NovelCheck already has a book by the new name, the two become one.</p>
    <div class="flex justify-end gap-2"><button type="button" data-close class="btn-ghost">Cancel</button><button class="btn-primary">Save</button></div>
  </form>`;
  d.querySelectorAll("[data-close]").forEach((x) => (x.onclick = () => d.close()));
  d.querySelector("form").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    const r = await attempt(() => put(`/api/books/${b.id}/name`, { title: f.title.value, author: f.author.value }),
      "Title and author saved");
    if (!r) return;
    d.close();
    onSaved?.(r.id);
  };
  d.showModal();
  d.querySelector("input[name=title]").focus();
}
