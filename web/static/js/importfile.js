// Import a book list file (Goodreads, StoryGraph, Hardcover, LibraryThing,
// Amazon data download, or any spreadsheet saved as CSV) or text copied from
// Amazon's Content Library page. Everything is read in the browser; only
// titles, authors and ISBNs are sent to NovelCheck.

// parseCSV reads CSV or TSV text (quoted fields, "" escapes, newlines in quotes).
export function parseCSV(text) {
  text = text.replace(/^﻿/, "");
  const first = text.split(/\r?\n/, 1)[0];
  const delim = (first.match(/\t/g) || []).length > (first.match(/,/g) || []).length ? "\t" : ",";
  const rows = [];
  let row = [], field = "", quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (c === '"' && text[i + 1] === '"') { field += '"'; i++; }
      else if (c === '"') quoted = false;
      else field += c;
    } else if (c === '"' && field === "") quoted = true;
    else if (c === delim) { row.push(field); field = ""; }
    else if (c === "\n" || c === "\r") {
      if (c === "\r" && text[i + 1] === "\n") i++;
      row.push(field); field = "";
      if (row.some((f) => f.trim() !== "")) rows.push(row);
      row = [];
    } else field += c;
  }
  row.push(field);
  if (row.some((f) => f.trim() !== "")) rows.push(row);
  return rows;
}

const norm = (h) => h.toLowerCase().replace(/[^a-z0-9/]+/g, " ").trim();
const pick = (headers, names) => {
  for (const n of names) {
    const i = headers.findIndex((h) => norm(h) === n);
    if (i >= 0) return i;
  }
  return -1;
};

// detect finds the useful columns and guesses where the file came from.
export function detect(headers) {
  const h = headers.map(norm);
  const cols = {
    title: pick(headers, ["title", "book title", "productname", "product name", "name", "book"]),
    author: pick(headers, ["author", "authors", "author s", "primary author", "author l f", "author last first", "contributors", "creator"]),
    isbn: pick(headers, ["isbn13", "isbn 13", "isbn", "isbn/uid", "isbn10", "isbn 10", "isbns"]),
    asin: pick(headers, ["asin"]),
    shelf: pick(headers, ["exclusive shelf", "read status", "reading status", "status", "state", "shelf", "collections"]),
    // For books on the "read" shelf: the day finished, the pages and your own stars, which Reading stats counts.
    finished: pick(headers, ["date read", "last date read", "dates read", "date finished", "finish date", "finished date", "end date", "date completed", "completed", "finished"]),
    pages: pick(headers, ["number of pages", "pages", "page count"]),
    rating: pick(headers, ["my rating", "star rating", "rating"]),
    binding: pick(headers, ["binding", "format"]),
    reads: pick(headers, ["read count"]),
    // Libby's timeline lists each loan, hold and return: the same title many times, so rows are merged.
  };
  const source = h.includes("exclusive shelf") && h.includes("book id") ? "Goodreads"
    : h.includes("isbn/uid") || (h.includes("read status") && h.includes("star rating")) ? "StoryGraph"
    : h.includes("productname") || (h.includes("asin") && !h.includes("title")) ? "Amazon"
    : h.includes("hardcover book id") || h.some((x) => x.includes("hardcover")) ? "Hardcover"
    : h.some((x) => x === "activity" || x === "action") && h.some((x) => x === "library" || x === "format" || x === "activity date") ? "Libby"
    : h.includes("author last first") || h.includes("book id") && h.includes("collections") ? "LibraryThing"
    : "a spreadsheet";
  return { cols, source };
}

// cleanISBN handles Goodreads' ="0439023483" style and stray characters.
const cleanISBN = (v = "") => v.replace(/[^0-9Xx]/g, "").toUpperCase().slice(0, 13);

// "Yarros, Rebecca" -> "Rebecca Yarros"; lists keep the first author.
function cleanAuthor(a = "") {
  a = a.split(/;|\s&\s|\s+and\s+/)[0].trim();
  const m = a.match(/^([^,]+),\s*([^,]+)$/);
  return m ? `${m[2]} ${m[1]}` : a;
}

// bindingKind turns a list's binding or format ("Kindle Edition", "Mass Market Paperback", "Audible Audio",
// "digital") into paper, ebook or audio; "" when it says nothing useful.
export function bindingKind(cell = "") {
  const v = String(cell).toLowerCase();
  if (/audio|audible|\bcd\b|mp3/.test(v)) return "audio";
  if (/kindle|e-?book|digital|nook|epub/.test(v)) return "ebook";
  if (/paper|hard|board|bound|binding|print/.test(v)) return "paper";
  return "";
}

// lastDay finds the last day written in a cell as YYYY-MM-DD: "2026/03/14" (Goodreads), "2026-03-14", or
// several read-throughs "2025/01/02-2025/01/09, 2026/03/01-2026/03/14" (StoryGraph). "" when there is none.
export function lastDay(cell = "") {
  const all = [...String(cell).matchAll(/(\d{4})[/-](\d{1,2})[/-](\d{1,2})/g)].map((m) => `${m[1]}-${m[2].padStart(2, "0")}-${m[3].padStart(2, "0")}`);
  return all.sort().pop() || "";
}

export function rowsToBooks(rows, cols, shelves) {
  const seen = new Set();
  const out = [];
  for (const r of rows) {
    // Goodreads adds the series: "Fourth Wing (The Empyrean, #1)" -> "Fourth Wing".
    const title = (r[cols.title] || "").trim().replace(/\s+/g, " ").replace(/\s*\([^()]*#\s*[\d.]+\)\s*$/, "");
    if (!title) continue;
    if (shelves && cols.shelf >= 0 && !shelves.has((r[cols.shelf] || "").trim() || "(none)")) continue;
    const author = cols.author >= 0 ? cleanAuthor(r[cols.author]) : "";
    const key = `${title}|${author}`.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({
      title, author, format: "list", path: `list:${title} | ${author}`,
      isbn: cols.isbn >= 0 ? cleanISBN(r[cols.isbn]) : "",
      asin: cols.asin >= 0 ? (r[cols.asin] || "").trim() : "",
      // On the "read" shelf (Goodreads) or read (StoryGraph): counts as finished.
      // Other apps (Bookly) say finished or completed, or give only the day finished.
      read: cols.shelf >= 0 ? /^(read|finished|completed|done)$/i.test((r[cols.shelf] || "").trim())
        : cols.finished >= 0 && lastDay(r[cols.finished]) !== "",
      finished: cols.finished >= 0 ? lastDay(r[cols.finished]) : "",
      pages: cols.pages >= 0 ? Math.max(0, parseInt(r[cols.pages], 10) || 0) : 0,
      rating: cols.rating >= 0 ? Math.min(5, Math.max(0, parseFloat(r[cols.rating]) || 0)) : 0,
      binding: cols.binding >= 0 ? bindingKind(r[cols.binding]) : "",
      reads: cols.reads >= 0 ? Math.max(0, parseInt(r[cols.reads], 10) || 0) : 0,
    });
  }
  return out;
}

// Text copied from Amazon's Content Library shows each book as title, author,
// then a line like "Acquired on September 12, 2024". Everything else on the
// page (buttons, menus) is skipped.
const AMAZON_DATE = /^(acquired|purchased|borrowed|returned|shared|received)\b.*\d{4}\s*$/i;
const AMAZON_JUNK = /^(deliver|download|mark as|read now|return|more actions|add to|remove|clear|select|show|sort|filter|view|kindle|ebook|book$|books$|sample|ku|prime reading|comixology|loan|manage|your|content|devices|preferences|privacy|digital|help|next|previous|page \d|\d+$|☐|✓|✔)/i;

export function looksLikeAmazonPaste(text) {
  return text.split(/\r?\n/).filter((l) => AMAZON_DATE.test(l.trim())).length >= 2;
}

export function parseAmazonPaste(text) {
  const out = [];
  const seen = new Set();
  let recent = [];
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line) continue;
    if (AMAZON_DATE.test(line)) {
      const [title, author] = recent.length >= 2 ? recent.slice(-2) : [recent[0], ""];
      recent = [];
      if (!title) continue;
      const clean = author.replace(/^by\s+/i, "");
      const key = `${title}|${clean}`.toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      out.push({ title, author: clean, isbn: "", asin: "", format: "list", path: `list:${title} | ${clean}` });
    } else if (!AMAZON_JUNK.test(line) && line.length < 300) {
      recent.push(line);
    }
  }
  return out;
}
