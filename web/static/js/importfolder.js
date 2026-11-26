// The watch folder (admins, Integrations page): another app drops a book (a folder with the e-book and extra files, or a zip) into a
// folder; NovelCheck adds the e-book to Calibre every few minutes, then deletes what it took it from. Failures go to "failed".
import { get, put, post } from "./api.js";
import { esc, attempt } from "./ui.js";

export async function importFolderCard(view) {
  const host = document.createElement("section");
  host.className = "card mb-4 space-y-2";
  view.append(host);
  const d = await attempt(() => get("/api/admin/import"));
  if (!d) return;
  const draw = (d) => {
    host.innerHTML = `<h2 class="font-semibold">Add books from a folder</h2>
      <p class="text-sm text-slate-400">Another app can drop a book into this folder: the e-book file, a folder with the e-book and other files, or a zip of one. Every few minutes NovelCheck unpacks zips, adds the e-book to Calibre (checked with Hardcover), and deletes what it took it from. The extra files (a txt file, a cover) are ignored. A book it couldn't add is moved to a <b>failed</b> folder inside it, with a notice. Files are left alone until they haven't changed for a minute. The folder must be one NovelCheck can read and write (for example inside its data folder).</p>
      <label class="toggle"><input type="checkbox" data-on ${d.enabled ? "checked" : ""}> Watch this folder</label>
      <input data-dir class="input w-full" value="${esc(d.dir)}" maxlength="300">
      <label class="toggle"><input type="checkbox" data-keep ${d.keep ? "checked" : ""}> Keep what it added in a <b>done</b> folder instead of deleting it</label>
      <div class="flex flex-wrap gap-2"><button type="button" data-save class="btn-secondary py-1 text-sm">Save</button>
      <button type="button" data-now class="btn-ghost py-1 text-sm">Look now</button></div>
      <p class="text-xs text-slate-400">${d.last ? "Last look: " + esc(d.last) : "Not looked yet."}</p>`;
    host.querySelector("[data-save]").onclick = async () => {
      const r = await attempt(() => put("/api/admin/import", { enabled: host.querySelector("[data-on]").checked, dir: host.querySelector("[data-dir]").value, keep: host.querySelector("[data-keep]").checked }), "Saved");
      if (r) draw(r);
    };
    host.querySelector("[data-now]").onclick = async () => {
      if (await attempt(() => post("/api/admin/import/now"), "Looking now; refresh in a minute to see what happened")) setTimeout(async () => { const r = await attempt(() => get("/api/admin/import")); if (r) draw(r); }, 15000);
    };
  };
  draw(d);
}
