// Library → Add books: drag e-book files onto the box (or choose them). Each goes straight into Calibre, and
// NovelCheck asks Hardcover what it really is. A sure match is fixed by itself; an unsure one shows Hardcover's best
// guess in a small form, to confirm or correct. For admins and people an admin allowed (Readers).
import { esc, toast } from "./ui.js";

const EXT = /\.(epub|pdf|mobi|azw3|azw|kepub|fb2|cbz)$/i;
export const mayUpload = (user) => !!user && (user.role === "admin" || user.can_upload);

export const dropBoxHTML = () => `<details id="add-books" class="card mb-3"><summary class="cursor-pointer text-sm font-semibold">Add books</summary>
  <div data-drop class="mt-2 cursor-pointer rounded-lg p-4 text-center text-sm text-slate-300">
    Drop e-books here, or tap to choose them (epub, pdf, mobi, azw3, kepub, fb2, cbz).<br>
    <span class="text-xs text-slate-500">They go into the library, and Hardcover checks the title and author.</span></div>
  <input type="file" data-pick multiple accept=".epub,.pdf,.mobi,.azw3,.azw,.kepub,.fb2,.cbz" class="hidden">
  <ul data-rows class="mt-2 space-y-2"></ul></details>`;

async function post(path, body) {
  const res = await fetch(path, { method: "POST", credentials: "same-origin", headers: { "X-NovelCheck": "1", "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

// upload sends one file and reports how much has gone (0 to 1); fetch can't, XMLHttpRequest can.
function upload(file, onProgress) {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append("file", file, file.name);
    const x = new XMLHttpRequest();
    x.open("POST", "/api/upload/ebook");
    x.setRequestHeader("X-NovelCheck", "1");
    x.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total);
    x.upload.onload = () => onProgress(1);
    x.onerror = () => reject(new Error("couldn't reach NovelCheck"));
    x.onload = () => {
      let data = {};
      try { data = JSON.parse(x.responseText); } catch {}
      if (x.status >= 200 && x.status < 300) resolve(data);
      else reject(new Error(data.error || x.statusText || "failed"));
    };
    x.send(form);
  });
}

const size = (n) => (n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`);
// The page's security policy refuses style="" in markup, so these are set from script (element.style).
const ZONE = "display:grid;place-items:center;min-height:5rem;border:2px dashed rgb(var(--md-outline-variant))";
const TRACK = "height:6px;border-radius:3px;background:rgb(var(--md-outline-variant) / .4);overflow:hidden";
const FORM = "display:grid;gap:.5rem;grid-template-columns:repeat(auto-fit,minmax(10rem,1fr))";
const fill = (pct, color) => `display:block;height:100%;width:${pct}%;background:${color};transition:width .15s`;
const OK = "rgb(var(--md-primary))", WARN = "#f59e0b", BAD = "#ef4444";

// say draws one row from the server's answer.
function say(row, name, r) {
  const bar = row.querySelector("[data-fill]");
  if (bar) bar.style.cssText = fill(100, r.status === "unsure" ? WARN : OK);
  const li = row.querySelector("[data-status]");
  const t = esc(r.title || name);
  if (r.status === "matched") {
    li.innerHTML = `✓ <b>${t}</b> added${r.fixed?.length ? ` <span class="text-xs text-slate-400">(checked with Hardcover; fixed the ${esc(r.fixed.join(", "))})</span>` : ` <span class="text-xs text-slate-400">(checked with Hardcover)</span>`}`;
  } else if (r.status === "unsure") {
    const g = r.guess || {};
    li.innerHTML = `<p>⚠ <b>${t}</b> added, but Hardcover couldn't be sure what it is. ${g.title ? "Its best guess is filled in:" : "Type its title and author:"}</p>
      <form class="mt-1"><input name="title" class="input" aria-label="Title" value="${esc(g.title || r.title || "")}">
        <input name="author" class="input" aria-label="Author" value="${esc(g.author || (r.authors || []).join(" & "))}"><button class="btn-primary py-1 text-sm">Save</button></form>`;
    li.querySelector("form").style.cssText = FORM;
    li.querySelector("form").addEventListener("submit", async (e) => {
      e.preventDefault();
      const f = e.target;
      try {
        say(row, name, await post("/api/upload/fix", { calibre_id: r.calibre_id, title: f.title.value, author: f.author.value }));
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
      li.innerHTML = `<div class="flex items-baseline justify-between gap-2"><b class="min-w-0 truncate">${esc(f.name)}</b><span class="shrink-0 text-xs text-slate-400">${size(f.size)}</span></div>
        <div data-track class="my-1"><span data-fill></span></div><div data-status class="text-xs text-slate-300"></div>`;
      rows.prepend(li);
      li.querySelector("[data-track]").style.cssText = TRACK;
      li.querySelector("[data-fill]").style.cssText = fill(0, OK);
      const status = li.querySelector("[data-status]"), bar = li.querySelector("[data-fill]");
      if (!EXT.test(f.name)) {
        bar.style.cssText = fill(100, BAD);
        status.textContent = "✕ Not an e-book file";
        continue;
      }
      status.textContent = "Uploading… 0%";
      try {
        const r = await upload(f, (p) => {
          bar.style.cssText = fill(Math.round(p * 100), OK);
          status.textContent = p >= 1 ? "Checking with Hardcover…" : `Uploading… ${Math.round(p * 100)}%`;
        });
        say(li, f.name, r);
      } catch (err) {
        bar.style.cssText = fill(100, BAD);
        status.textContent = `✕ ${err.message}`;
      }
    }
  };
  zone.style.cssText = ZONE;
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
