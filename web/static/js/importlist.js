// Helpers for the Import page when a Kindle can't be opened as a folder:
// how to copy its books off a "device"-style Kindle, and a pasted title list.

// Newer Kindles connect over MTP ("device", not "drive"), which browsers can't open.
export const MTP_HELP = `
  <details class="rounded-lg bg-slate-800/60 p-4 text-sm text-slate-300">
    <summary class="cursor-pointer font-semibold text-slate-100">Kindle shows up as a "device", not a drive?</summary>
    <p class="mt-2">Newer Kindles connect like a phone, and web browsers can't open those. Two easy ways around it:</p>
    <p class="mt-3 font-semibold">A. Copy the books folder first (Windows)</p>
    <ol class="ml-5 list-decimal space-y-1">
      <li>Open <b>File Explorer</b> and click the Kindle under <b>This PC</b>, then <b>Internal Storage</b>.</li>
      <li>Copy the <b>documents</b> folder to your <b>Desktop</b> (right-click → Copy, then Paste on the Desktop).</li>
      <li>Back here, click <b>Choose folder…</b> and pick that copied <b>documents</b> folder.</li>
      <li>When the import is done you can delete the copy from the Desktop.</li>
    </ol>
    <p class="mt-2 text-xs text-slate-400">On a Mac, install the free <b>Android File Transfer</b> app to see the Kindle, then copy its <b>documents</b> folder the same way.</p>
    <p class="mt-3 font-semibold">B. Paste a list of titles</p>
    <p>No cable needed: click <b>Paste a list</b> above and type or paste the books, one per line.</p>
  </details>`;

// parseTitleList turns lines like "Title by Author", "Title - Author",
// "Title<TAB>Author" or just "Title" into import rows.
export function parseTitleList(text) {
  const seen = new Set();
  const out = [];
  for (let line of text.split(/\r?\n/)) {
    line = line.replace(/^\s*(?:[-*•]|\d+[.)])\s+/, "").trim();
    if (!line) continue;
    let title = line, author = "";
    const splitAt = (sep, last) => {
      const i = last ? line.lastIndexOf(sep) : line.indexOf(sep);
      if (i > 0 && i + sep.length < line.length) {
        title = line.slice(0, i).trim();
        author = line.slice(i + sep.length).trim();
        return true;
      }
      return false;
    };
    splitAt("\t") || splitAt(" by ", true) || splitAt(" — ") || splitAt(" – ") || splitAt(" - ", true);
    title = title.replace(/^["“]|["”]$/g, "").trim();
    const key = `${title}|${author}`.toLowerCase();
    if (!title || seen.has(key)) continue;
    seen.add(key);
    out.push({ title, author, isbn: "", asin: "", format: "list", path: `list:${title} | ${author}` });
  }
  return out;
}
