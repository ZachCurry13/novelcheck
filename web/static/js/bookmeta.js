// Client-side e-book metadata extraction. Only metadata leaves the browser.
// EPUB: dc:title / dc:creator / ISBN from the OPF package (via JSZip).
// MOBI/AZW3: PalmDB name + EXTH records 100 (author), 104 (ISBN), 113/504 (ASIN), 503 (title).
// Anything else, or on parse failure: filename heuristics.

export const BOOK_EXT = /\.(epub|mobi|azw3?|kfx|pdf)$/i;

export async function extractMeta(file, relPath) {
  const format = (file.name.match(BOOK_EXT)?.[1] || "").toLowerCase();
  const base = { ...fromFilename(file.name), path: relPath, format };
  try {
    if (format === "epub" && window.JSZip) return { ...base, ...clean(await epubMeta(file)) };
    if (["mobi", "azw", "azw3"].includes(format)) return { ...base, ...clean(await mobiMeta(file)) };
  } catch {
    /* fall back to filename metadata */
  }
  return base;
}

function clean(o) {
  return Object.fromEntries(Object.entries(o).filter(([, v]) => v && String(v).trim()));
}

// Kindle names look like "Title - Author_B00ABC1234_EBOK.azw" or "Title_B00ABC1234.azw3".
export function fromFilename(name) {
  let stem = name.replace(BOOK_EXT, "");
  let asin = "";
  const m = stem.match(/_(B0[0-9A-Z]{8})(?:_[A-Z]+)?$/);
  if (m) {
    asin = m[1];
    stem = stem.slice(0, m.index);
  }
  stem = stem.replace(/_/g, " ").trim();
  const parts = stem.split(/\s+-\s+/);
  if (parts.length >= 2) return { title: parts[0].trim(), author: parts.slice(1).join(" - ").trim(), asin };
  return { title: stem, author: "", asin };
}

async function epubMeta(file) {
  const zip = await window.JSZip.loadAsync(file);
  const container = await zip.file("META-INF/container.xml")?.async("string");
  if (!container) return {};
  const cdoc = new DOMParser().parseFromString(container, "application/xml");
  const opfPath = cdoc.querySelector("rootfile")?.getAttribute("full-path");
  const opf = opfPath && (await zip.file(opfPath)?.async("string"));
  if (!opf) return {};
  const doc = new DOMParser().parseFromString(opf, "application/xml");
  const pick = (tag) => [...doc.getElementsByTagNameNS("*", tag)].map((n) => n.textContent.trim()).filter(Boolean);
  const isbn = pick("identifier").map((s) => s.replace(/^urn:isbn:/i, "").replace(/-/g, ""))
    .find((s) => /^(97[89])?\d{9}[\dX]$/i.test(s)) || "";
  return { title: pick("title")[0], author: pick("creator").join(" & "), isbn };
}

async function mobiMeta(file) {
  // Header + EXTH live near the start; 256 KB covers virtually every file.
  const buf = new DataView(await file.slice(0, 256 * 1024).arrayBuffer());
  const u8 = new Uint8Array(buf.buffer);
  const dec = new TextDecoder("utf-8");
  const palmName = dec.decode(u8.slice(0, 32)).replace(/\0.*$/s, "").replace(/_/g, " ");
  const numRecords = buf.getUint16(76);
  if (numRecords < 1) return { title: palmName };
  const rec0 = buf.getUint32(78);
  if (dec.decode(u8.slice(rec0 + 16, rec0 + 20)) !== "MOBI") return { title: palmName };
  const mobiLen = buf.getUint32(rec0 + 20);
  const hasExth = (buf.getUint32(rec0 + 128) & 0x40) !== 0;
  const out = { title: palmName };
  const fullNameOff = buf.getUint32(rec0 + 84);
  const fullNameLen = buf.getUint32(rec0 + 88);
  if (fullNameLen > 0 && rec0 + fullNameOff + fullNameLen <= u8.length) {
    out.title = dec.decode(u8.slice(rec0 + fullNameOff, rec0 + fullNameOff + fullNameLen));
  }
  if (!hasExth) return out;
  let p = rec0 + 16 + mobiLen;
  if (dec.decode(u8.slice(p, p + 4)) !== "EXTH") return out;
  const count = buf.getUint32(p + 8);
  p += 12;
  const authors = [];
  for (let i = 0; i < count && p + 8 <= u8.length; i++) {
    const type = buf.getUint32(p);
    const len = buf.getUint32(p + 4);
    if (len < 8 || p + len > u8.length) break;
    const val = dec.decode(u8.slice(p + 8, p + len)).trim();
    if (type === 100) authors.push(val);
    else if (type === 104) out.isbn = val.replace(/-/g, "");
    else if (type === 113 || type === 504) out.asin = out.asin || val;
    else if (type === 503) out.title = val;
    p += len;
  }
  if (authors.length) out.author = authors.join(" & ");
  return out;
}
