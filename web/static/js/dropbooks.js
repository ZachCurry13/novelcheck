// Library → Add books: drag e-book files onto the box (or choose them). Each goes straight into Calibre, and
// NovelCheck asks Hardcover what it really is. A sure match is fixed by itself; an unsure one shows Hardcover's best
// guess in a small form, to confirm or correct. For admins and people an admin allowed (Readers).
import { esc, toast } from "./ui.js";

const EXT = /\.(epub|pdf|mobi|azw3|azw|kepub|fb2|cbz)$/i;
export const mayUpload = (user) => !!user && (user.role === "admin" || user.can_upload);

export const dropBoxHTML = () => `<details id="add-books" class="card mb-3"><summary class="cursor-pointer text-sm font-semibold">Add books</summary>
  <div data-drop class="mt-2 cursor-pointer rounded-lg p-4 text-center text-sm text-slate-300" style="display:grid;place-items:center;min-height:5rem;border:2px dashed rgb(var(--md-outline-variant))">
    Drop e-books here, or tap to choose them (epub, pdf, mobi, azw3, kepub, fb2, cbz).<br>
    <span class="text-xs text-slate-500">They go into the library, and Hardcover checks the title and author.</span></div>
  <input type="file" data-pick multiple accept=".epub,.pdf,.mobi,.azw3,.azw,.kepub,.fb2,.cbz" class="hidden">
  <ul data-rows class="mt-2 space-y-2"></ul></details>`;

async function post(path, body) {
  const opts = { method: "POST", credentials: "same-origin", headers: { "X-NovelCheck": "1" }, body };
  if (!(body instanceof FormData)) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

// say draws one row from the server's answer.
function say(li, name, r) {
  const t = esc(r.title || name);
  if (r.status === "matched") {
    li.innerHTML = `✓ <b>${t}</b> added${r.fixed?.length ? ` <span class="text-xs text-slate-400">(checked with Hardcover; fixed the ${esc(r.fixed.join(", "))})</span>` : ` <span class="text-xs text-slate-400">(checked with Hardcover)</span>`}`;
  } else if (r.status === "unsure") {
    const g = r.guess || {};
    li.innerHTML = `<p>⚠ <b>${t}</b> added, but Hardcover couldn't be sure what it is. ${g.title ? "Its best guess is filled in:" : "Type its title and author:"}</p>
      <form class="mt-1" style="display:grid;gap:.5rem;grid-template-columns:repeat(auto-fit,minmax(10rem,1fr))"><input name="title" class="input" aria-label="Title" value="${esc(g.title || r.title || "")}">
        <input name="author" class="input" aria-label="Author" value="${esc(g.author || (r.authors || []).join(" & "))}"><button class="btn-primary py-1 text-sm">Save</button></form>`;
    li.querySelector("form").addEventListener("submit", async (e) => {
      e.preventDefault();
      const f = e.target;
      try {
        say(li, name, await post("/api/upload/fix", { calibre_id: r.calibre_id, title: f.title.value, author: f.author.value }));
      } catch (err) {
        toast(err.message, true);
      }
    });
  } else {
    li.innerHTML = `✓ <b>${t}</b> added <span class="text-xs text-slate-400">(Hardcover wasn't asked, so it keeps the file's own details)</span>`;
  }
}

export function bindDropBox(host) {
  const zone = host.querySelector("[data-drop]"), pick = host.querySelector("[data-pick]"), rows = host.querySelector("[data-rows]");
  const add = async (files) => {
    for (const f of files) {
      const li = document.createElement("li");
      li.className = "rounded-lg border border-slate-800 p-2 text-sm";
      rows.prepend(li);
      if (!EXT.test(f.name)) {
        li.textContent = `✕ ${f.name}: not an e-book file`;
        continue;
      }
      li.textContent = `Sending ${f.name}…`;
      const form = new FormData();
      form.append("file", f, f.name);
      try {
        say(li, f.name, await post("/api/upload/ebook", form));
      } catch (err) {
        li.textContent = `✕ ${f.name}: ${err.message}`;
      }
    }
  };
  zone.addEventListener("click", () => pick.click());
  pick.addEventListener("change", () => { add([...pick.files]); pick.value = ""; });
  zone.addEventListener("dragover", (e) => { e.preventDefault(); zone.style.borderColor = "rgb(var(--md-primary))"; });
  zone.addEventListener("dragleave", () => zone.style.borderColor = "rgb(var(--md-outline-variant))");
  zone.addEventListener("drop", (e) => {
    e.preventDefault();
    zone.style.borderColor = "rgb(var(--md-outline-variant))";
    add([...e.dataTransfer.files]);
  });
}
