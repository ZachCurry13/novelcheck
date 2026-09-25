// "Import Local Drive / Kindle": walk a picked folder in the browser, extract
// e-book metadata, and send it to a chosen or new catalog.
import { get, post } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";
import { extractMeta, BOOK_EXT } from "./bookmeta.js";
import { MTP_HELP, parseTitleList } from "./importlist.js";
import { parseCSV, detect, rowsToBooks, looksLikeAmazonPaste, parseAmazonPaste } from "./importfile.js";
import { IMPORT_GUIDES } from "./importguides.js";

const supportsPicker = typeof window.showDirectoryPicker === "function";

export async function renderImport(view) {
  const catalogs = ((await attempt(() => get("/api/catalogs"))) || []).filter((c) => c.source !== "calibre");
  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">Import books</h1>
    <p class="mb-6 text-sm text-slate-400">Plug in a Kindle or e-reader and pick its <code>documents</code> folder, or,
      with no cable, bring in a list from Amazon, Goodreads, StoryGraph, Hardcover or a spreadsheet (second box).
      Everything is read in your browser; only titles, authors and identifiers are sent to NovelCheck.</p>
    <div class="card mb-4 space-y-4">
      <div class="flex flex-wrap gap-3">
        ${supportsPicker ? `<button id="pick-btn" class="btn-primary">Choose folder…</button>` : ""}
        <label class="btn-${supportsPicker ? "secondary" : "primary"} cursor-pointer">
          ${supportsPicker ? "Or select files (fallback)" : "Choose folder…"}
          <input id="dir-input" type="file" webkitdirectory multiple class="hidden">
        </label>
        <button id="list-btn" class="btn-ghost">Paste a list</button>
      </div>
      <div id="list-box" class="hidden space-y-2">
        <label class="label" for="list-text">Books, one per line: "Title by Author", "Title - Author", or just the title</label>
        <textarea id="list-text" rows="8" class="input" placeholder="Fourth Wing by Rebecca Yarros&#10;The Hobbit - J.R.R. Tolkien&#10;Wonder"></textarea>
        <button id="list-use" class="btn-secondary">Use this list</button>
      </div>
      <p id="scan-status" class="text-sm text-slate-400">No folder selected.</p>
      ${MTP_HELP}
    </div>
    <div class="card mb-4 space-y-3">
      <h2 class="text-lg font-semibold">📄 From Amazon, Goodreads, StoryGraph, Hardcover or a spreadsheet</h2>
      <p class="text-sm text-slate-400">No Kindle cable needed. Get a list of your books from the app or website you use (steps below),
        then pick the file here, or copy the page and use <b>Paste a list</b> above.</p>
      <label class="btn-primary inline-flex cursor-pointer">Choose a list file (.csv)
        <input id="list-file" type="file" accept=".csv,.tsv,.txt,text/csv,text/plain" class="hidden"></label>
      <div id="file-info" class="hidden space-y-2 rounded-lg bg-slate-800/60 p-3 text-sm"></div>
      ${IMPORT_GUIDES}
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
          <input id="new-cat" class="input" placeholder="e.g. Kids' Kindle">
        </div>
      </div>
      <div class="max-h-96 overflow-y-auto rounded-lg ring-1 ring-slate-800">
        <table class="w-full text-left text-sm">
          <thead class="sticky top-0 bg-slate-900 text-slate-400"><tr>
            <th class="p-2">Title</th><th class="p-2">Author</th><th class="p-2">Format</th><th class="p-2"><span class="sr-only">Remove</span></th></tr></thead>
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
    const list = [];
    let i = 0;
    for (const { file, path } of entries) {
      status.textContent = `Reading ${++i} of ${entries.length}: ${file.name}`;
      list.push(await extractMeta(file, path));
    }
    show(list);
    if (!list.length) status.textContent += ` If this is a Kindle, see "Kindle shows up as a device" below.`;
  }

  function show(list) {
    found = list;
    status.textContent = `Found ${found.length} e-book${found.length === 1 ? "" : "s"}.`;
    $("#found", view).innerHTML = found.map((b, i) => `<tr class="border-t border-slate-800">
      <td class="p-2">${esc(b.title)}</td><td class="p-2 text-slate-400">${esc(b.author)}</td>
      <td class="p-2 uppercase text-slate-500">${esc(b.format === "list" ? "list" : b.format)}</td>
      <td class="p-2 text-right"><button type="button" data-drop="${i}" class="btn-ghost px-2 py-0 text-xs" title="Leave this one out">✕</button></td></tr>`).join("");
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

  $("#list-btn", view).addEventListener("click", () => {
    $("#list-box", view).classList.toggle("hidden");
    $("#list-text", view).focus();
  });
  $("#list-use", view).addEventListener("click", () => {
    const text = $("#list-text", view).value;
    // Text copied from Amazon's Content Library is recognised and cleaned up.
    const amazon = looksLikeAmazonPaste(text);
    const list = amazon ? parseAmazonPaste(text) : parseTitleList(text);
    if (amazon) suggestName("Amazon Kindle library");
    if (!list.length) return toast("Type or paste at least one title", true);
    show(list);
  });

  // ✕ leaves a row out of the import (e.g. an app in Amazon's list).
  $("#found", view).addEventListener("click", (e) => {
    const b = e.target.closest("[data-drop]");
    if (!b) return;
    found.splice(Number(b.dataset.drop), 1);
    show(found);
  });

  // A list file from another app: detect its columns, offer shelf choices.
  const suggestName = (name) => {
    if (catSelect.value === "new" && !$("#new-cat", view).value) $("#new-cat", view).value = name;
  };
  $("#list-file", view).addEventListener("change", async (e) => {
    const file = e.target.files[0];
    e.target.value = "";
    if (!file) return;
    const rows = parseCSV(await file.text());
    const info = $("#file-info", view);
    info.classList.remove("hidden");
    if (rows.length < 2) return (info.innerHTML = `<p class="text-amber-300">That file looks empty.</p>`);
    const { cols, source } = detect(rows[0]);
    if (cols.title < 0) {
      info.innerHTML = `<p class="text-amber-300">Couldn't find a <b>Title</b> column. The columns are: ${rows[0].map(esc).join(", ")}.
        Rename the title column to <b>Title</b> (and the author column to <b>Author</b>) and try again.</p>`;
      return;
    }
    const data = rows.slice(1);
    const shelfNames = cols.shelf >= 0 ? [...new Set(data.map((r) => (r[cols.shelf] || "").trim() || "(none)"))].sort() : [];
    const apply = () => {
      const on = new Set([...info.querySelectorAll("[data-shelf]:checked")].map((c) => c.dataset.shelf));
      show(rowsToBooks(data, cols, shelfNames.length ? on : null));
    };
    info.innerHTML = `<p>✓ Read <b>${esc(file.name)}</b> from ${esc(source)}: ${data.length.toLocaleString()} rows.
        Title column <b>${esc(rows[0][cols.title])}</b>${cols.author >= 0 ? `, author column <b>${esc(rows[0][cols.author])}</b>` : " (no author column found)"}.</p>
      ${shelfNames.length ? `<p class="label mb-0">Which ${esc(rows[0][cols.shelf])} to import?</p>
        <div class="flex flex-wrap gap-x-4 gap-y-1">${shelfNames.map((n) => `<label class="toggle"><input type="checkbox" data-shelf="${esc(n)}" checked> ${esc(n)}</label>`).join("")}</div>` : ""}`;
    info.onchange = apply;
    suggestName(source === "a spreadsheet" ? file.name.replace(/\.[^.]+$/, "") : source === "Amazon" ? "Amazon Kindle library" : `${source} library`);
    apply();
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
