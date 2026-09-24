// "Import Local Drive / Kindle": walk a picked folder in the browser, extract
// e-book metadata, and send it to a chosen or new catalog.
import { get, post } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";
import { extractMeta, BOOK_EXT } from "./bookmeta.js";

const supportsPicker = typeof window.showDirectoryPicker === "function";

export async function renderImport(view) {
  const catalogs = ((await attempt(() => get("/api/catalogs"))) || []).filter((c) => c.source !== "calibre");
  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">Import Local Drive / Kindle</h1>
    <p class="mb-6 text-sm text-slate-400">Connect your Kindle or e-reader by USB, then pick its
      <code>documents/</code> folder. Files are read locally in your browser — only titles, authors and
      identifiers are sent to NovelCheck.</p>
    <div class="card mb-4 space-y-4">
      <div class="flex flex-wrap gap-3">
        ${supportsPicker ? `<button id="pick-btn" class="btn-primary">Choose folder…</button>` : ""}
        <label class="btn-${supportsPicker ? "secondary" : "primary"} cursor-pointer">
          ${supportsPicker ? "Or select files (fallback)" : "Choose folder…"}
          <input id="dir-input" type="file" webkitdirectory multiple class="hidden">
        </label>
      </div>
      <p id="scan-status" class="text-sm text-slate-400">No folder selected.</p>
    </div>
    <div id="review" class="card hidden space-y-4">
      <div class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="label" for="cat-select">Destination catalog</label>
          <select id="cat-select" class="input">
            <option value="new">+ Create new catalog…</option>
            ${catalogs.map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join("")}
          </select>
        </div>
        <div id="new-cat-wrap">
          <label class="label" for="new-cat">New catalog name</label>
          <input id="new-cat" class="input" placeholder="e.g. Jenna's Kindle">
        </div>
      </div>
      <div class="max-h-96 overflow-y-auto rounded-lg ring-1 ring-slate-800">
        <table class="w-full text-left text-sm">
          <thead class="sticky top-0 bg-slate-900 text-slate-400"><tr>
            <th class="p-2">Title</th><th class="p-2">Author</th><th class="p-2">Format</th></tr></thead>
          <tbody id="found"></tbody>
        </table>
      </div>
      <button id="import-btn" class="btn-primary">Import books</button>
    </div>`;

  let found = [];
  const status = $("#scan-status", view);
  const catSelect = $("#cat-select", view);
  catSelect.value = catalogs.length ? String(catalogs[0].id) : "new";
  const syncNewField = () => $("#new-cat-wrap", view).classList.toggle("hidden", catSelect.value !== "new");
  catSelect.addEventListener("change", syncNewField);
  syncNewField();

  async function process(entries) {
    found = [];
    let i = 0;
    for (const { file, path } of entries) {
      status.textContent = `Reading ${++i} of ${entries.length}: ${file.name}`;
      found.push(await extractMeta(file, path));
    }
    status.textContent = `Found ${found.length} e-book${found.length === 1 ? "" : "s"}.`;
    $("#found", view).innerHTML = found.map((b) => `<tr class="border-t border-slate-800">
      <td class="p-2">${esc(b.title)}</td><td class="p-2 text-slate-400">${esc(b.author)}</td>
      <td class="p-2 uppercase text-slate-500">${esc(b.format)}</td></tr>`).join("");
    $("#review", view).classList.toggle("hidden", !found.length);
  }

  $("#pick-btn", view)?.addEventListener("click", async () => {
    try {
      const dir = await window.showDirectoryPicker({ mode: "read" });
      status.textContent = "Scanning folder…";
      await process(await walk(dir, dir.name));
    } catch (e) {
      if (e.name !== "AbortError") toast(e.message, true);
    }
  });

  // iOS Safari / Firefox fallback: <input webkitdirectory> yields a flat FileList.
  $("#dir-input", view).addEventListener("change", async (e) => {
    const entries = [...e.target.files]
      .filter((f) => BOOK_EXT.test(f.name))
      .map((f) => ({ file: f, path: f.webkitRelativePath || f.name }));
    await process(entries);
  });

  $("#import-btn", view).addEventListener("click", async (e) => {
    const body = { books: found };
    if (catSelect.value === "new") {
      body.catalog_name = $("#new-cat", view).value.trim();
      if (!body.catalog_name) return toast("Enter a name for the new catalog", true);
    } else {
      body.catalog_id = Number(catSelect.value);
    }
    e.target.disabled = true;
    const r = await attempt(() => post("/api/import/drive", body));
    e.target.disabled = false;
    if (r) {
      toast(`Imported ${r.imported} books${r.skipped ? `, skipped ${r.skipped}` : ""}`);
      location.hash = `#/library`;
    }
  });
}

// Recursively collect e-book files from a FileSystemDirectoryHandle.
async function walk(dirHandle, prefix, out = [], depth = 0) {
  if (depth > 8) return out;
  for await (const [name, handle] of dirHandle.entries()) {
    const path = `${prefix}/${name}`;
    if (handle.kind === "directory") {
      if (!name.startsWith(".") && !name.endsWith(".sdr")) await walk(handle, path, out, depth + 1);
    } else if (BOOK_EXT.test(name)) {
      out.push({ file: await handle.getFile(), path });
    }
  }
  return out;
}
