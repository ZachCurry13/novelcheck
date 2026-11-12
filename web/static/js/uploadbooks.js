// Admin → Library cleanup → Upload: e-books go into Calibre (through its Content server), audiobooks into
// Audiobookshelf. Experimental: "Check my servers" (Settings → Developer) shows whether each server has the route.
// Nothing is sent until the button is pressed and the list has been confirmed.
import { get } from "./api.js";
import { esc, toast } from "./ui.js";

const size = (n) => (n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`);

async function send(path, form) {
  const res = await fetch(path, { method: "POST", body: form, credentials: "same-origin", headers: { "X-NovelCheck": "1" } });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

export async function renderUpload(host) {
  const t = await get("/api/admin/upload/targets").catch(() => ({}));
  const libs = t.abs || [];
  host.innerHTML = `<p class="text-sm text-slate-400">Experimental. Run <b>Settings → Developer → Check my servers</b> first to see whether your Calibre and Audiobookshelf have the features this uses. Files are sent through NovelCheck: the remote address (Cloudflare) refuses more than 100 MB at a time, so send big audiobooks while on the home network.</p>
    <div class="mt-3 space-y-2"><h3 class="font-semibold">E-books into Calibre</h3>
      ${t.calibre ? "" : `<p class="text-sm text-amber-200">Set up the Calibre Content server first (Admin → Calibre Library).</p>`}
      <input id="up-eb" type="file" multiple accept=".epub,.pdf,.mobi,.azw3,.azw,.kepub,.fb2,.cbz" class="input">
      <button type="button" data-up="ebook" class="btn-primary py-1 text-sm" ${t.calibre ? "" : "disabled"}>Add to Calibre</button><p id="up-eb-out" class="text-sm text-slate-300"></p></div>
    <div class="mt-4 space-y-2"><h3 class="font-semibold">Audiobook into Audiobookshelf</h3>
      ${libs.length ? "" : `<p class="text-sm text-amber-200">${esc(t.abs_error || "Connect Audiobookshelf first (Admin → Audiobookshelf).")}</p>`}
      <div class="grid gap-2 sm:grid-cols-2"><input id="up-title" class="input" placeholder="Title" aria-label="Title"><input id="up-author" class="input" placeholder="Author" aria-label="Author">
        <input id="up-series" class="input" placeholder="Series (optional)" aria-label="Series">
        <select id="up-lib" class="input" aria-label="Library">${libs.map((l) => `<option value="${esc(l.id)}">${esc(l.name)}</option>`).join("")}</select>
        <select id="up-folder" class="input sm:col-span-2" aria-label="Folder"></select></div>
      <input id="up-ab" type="file" multiple accept=".m4b,.mp3,.m4a,.flac,.ogg,.opus,.aac,.wav,.jpg,.jpeg,.png,.cue,.nfo,.pdf,.epub" class="input">
      <button type="button" data-up="audio" class="btn-primary py-1 text-sm" ${libs.length ? "" : "disabled"}>Send to Audiobookshelf</button><p id="up-ab-out" class="text-sm text-slate-300"></p></div>`;
  const fillFolders = () => {
    const l = libs.find((x) => x.id === host.querySelector("#up-lib").value);
    host.querySelector("#up-folder").innerHTML = (l?.folders || []).map((f) => `<option value="${esc(f.id)}">${esc(f.path)}</option>`).join("");
  };
  host.querySelector("#up-lib").addEventListener("change", fillFolders);
  fillFolders();
  host.addEventListener("click", async (e) => {
    const k = e.target.closest("[data-up]")?.dataset.up;
    if (k === "ebook") {
      const files = [...host.querySelector("#up-eb").files];
      const out = host.querySelector("#up-eb-out");
      if (!files.length) return toast("Choose at least one file", true);
      if (!confirm(`Add ${files.length} file${files.length === 1 ? "" : "s"} to Calibre?\n${files.map((f) => `${f.name} (${size(f.size)})`).join("\n")}`)) return;
      e.target.disabled = true;
      const lines = [];
      for (const f of files) {
        out.textContent = `${lines.join(" · ")} Sending ${f.name}…`;
        const form = new FormData();
        form.append("file", f, f.name);
        try {
          const r = await send("/api/admin/upload/ebook", form);
          lines.push(`✓ ${r.title}`);
        } catch (err) {
          lines.push(`✕ ${f.name}: ${err.message}`);
        }
      }
      out.textContent = lines.join(" · ");
      e.target.disabled = false;
    } else if (k === "audio") {
      const files = [...host.querySelector("#up-ab").files];
      const out = host.querySelector("#up-ab-out");
      const title = host.querySelector("#up-title").value.trim();
      if (!files.length || !title) return toast("Give it a title and choose its files", true);
      if (!confirm(`Send “${title}” (${files.length} file${files.length === 1 ? "" : "s"}, ${size(files.reduce((n, f) => n + f.size, 0))}) to Audiobookshelf?`)) return;
      const form = new FormData();
      for (const [name, sel] of [["title", "#up-title"], ["author", "#up-author"], ["series", "#up-series"], ["library", "#up-lib"], ["folder", "#up-folder"]]) form.append(name, host.querySelector(sel).value);
      for (const f of files) form.append("files", f, f.name); // the fields come first, the files after
      e.target.disabled = true;
      out.textContent = "Sending… (a big audiobook takes a while: keep this page open)";
      try {
        const r = await send("/api/admin/upload/audiobook", form);
        out.textContent = `✓ Sent “${r.sent}”. Audiobookshelf will scan it shortly.`;
      } catch (err) {
        out.textContent = `✕ ${err.message}`;
      }
      e.target.disabled = false;
    }
  });
}
