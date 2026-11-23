// NovelCheck's reader, laid out like an e-reader: the book fills the screen, tap the left edge to go back and the
// right edge to go forward, and tap the middle for the menu (Exit, search and a bookmark at the top; where you are
// and the tools at the bottom: js/readerbars.js, js/readermenu.js). A slim status line shows the chapter and how
// far you are. It opens with the menu showing for a moment, so the way out is never hidden. The book engine is
// foliate-js (vendor/foliate).
// Your place is saved to NovelCheck as you read (offline, it waits on the device: js/offline.js); when KOReader's progress sync has you further on, the reader
// offers to jump there (by percentage, so to about a page).
import "/vendor/foliate/view.js";
import { readerSettings, applySettings, openContents, openSettings, closePanel } from "./readermenu.js";
import { bindBars } from "./readerbars.js";
import { startRecording, note as record } from "./debugrec.js";
import { savePosition, pendingPosition, flushPositions } from "./offline.js";

startRecording(); // the reader's errors reach the bug report like the app's

const $ = (s) => document.querySelector(s);
const id = Number(new URLSearchParams(location.search).get("id"));
const api = (path, opts = {}) => fetch(path, { credentials: "same-origin", ...opts, headers: { "X-NovelCheck": "1", ...(opts.headers || {}) } });

let view;
let moved = () => {}; // the bottom bar's slider and labels, once the book is open
let saveTimer;
let lastSaved = "";
const status = { chapter: "", fraction: 0, left: null };

function note(text, action) {
  const n = $("#reader-note");
  n.hidden = !text;
  n.innerHTML = "";
  if (!text) return;
  n.append(text);
  if (action) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "btn-text";
    b.textContent = action.label;
    b.onclick = () => { n.hidden = true; action.run(); };
    const x = document.createElement("button");
    x.type = "button";
    x.className = "btn-text";
    x.textContent = "Stay here";
    x.onclick = () => (n.hidden = true);
    n.append(b, x);
  }
}

function paintStatus() {
  const pct = Math.round(status.fraction * 100);
  const left = status.left != null ? ` · ${status.left} min left in chapter` : "";
  $("#reader-status").textContent = `${status.chapter ? `${status.chapter} · ` : ""}${pct}%${left}`;
}

// readerXPointer is the same place the way KOReader names it (a crengine XPointer): the book part (DocFragment,
// counted from 1 in the book's reading order) and the path of elements down to the one the page starts in, with
// [n] only where an element has same-named brothers. The server keeps it as KOReader progress, so KOReader opens
// at this paragraph. "" when the place can't be named.
export function readerXPointer(detail) {
  const index = detail.section?.current;
  let el = detail.range?.startContainer;
  if (index == null || !el) return "";
  if (el.nodeType !== 1) el = el.parentElement;
  const body = el?.ownerDocument?.body;
  if (!el || !body || (el !== body && !body.contains(el))) return "";
  const parts = [];
  for (let n = el; n && n !== body; n = n.parentElement) {
    const name = n.localName.toLowerCase();
    const same = [...n.parentElement.children].filter((c) => c.localName.toLowerCase() === name);
    parts.unshift(same.length > 1 ? `${name}[${same.indexOf(n) + 1}]` : name);
  }
  return `/body/DocFragment[${index + 1}]/body${parts.length ? `/${parts.join("/")}` : ""}`;
}

// save keeps your place on the server a moment after you stop turning pages (and, for KOReader, as an XPointer).
function save(cfi, fraction, xpointer) {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    if (!cfi || cfi === lastSaved) return;
    lastSaved = cfi;
    savePosition(id, { cfi, fraction, xpointer });
  }, 1500);
}

function toggleMenu(show = $("#reader-top").hidden) {
  $("#reader-top").hidden = !show;
  $("#reader-bottom").hidden = !show;
  $("#reader-status").hidden = show;
  if (!show) closePanel();
}

// A tap on the book itself (between the edge strips) opens the menu, or closes it (and any panel) when open.
function tap() {
  const open = !$("#reader-top").hidden || !$("#reader-panel").hidden;
  toggleMenu(!open);
}

// The edge strips turn pages; with the menu open, a tap there closes it first.
function edge(forward) {
  if (!$("#reader-top").hidden || !$("#reader-panel").hidden) return toggleMenu(false);
  if (forward) view.next();
  else view.prev();
}

function onLoad({ detail: { doc } }) {
  // A tap on the book (not a link, not choosing words) is for the menu; the edges have their own strips.
  doc.addEventListener("click", (e) => {
    if (e.target.closest?.("a[href]") || doc.getSelection()?.toString()) return;
    tap();
  });
  doc.addEventListener("keydown", onKey);
}

function onKey(e) {
  if (e.key === "ArrowRight" || e.key === "PageDown" || e.key === " ") view.next();
  else if (e.key === "ArrowLeft" || e.key === "PageUp") view.prev();
  else if (e.key === "Escape") toggleMenu();
  else return;
  e.preventDefault();
}

function onRelocate({ detail }) {
  status.chapter = detail.tocItem?.label?.trim() || "";
  status.fraction = detail.fraction || 0;
  status.left = detail.time?.section != null ? Math.max(1, Math.round(detail.time.section)) : null;
  paintStatus();
  moved(detail);
  save(detail.cfi, status.fraction, readerXPointer(detail));
}

async function start() {
  if (!id) return note("No book was picked. Go back to NovelCheck and press Read on a book.");
  note("Opening the book…");
  await flushPositions();
  addEventListener("online", flushPositions);
  const [info, file] = await Promise.all([
    api(`/api/books/${id}/position`).then((r) => (r.ok ? r.json() : null)),
    api(`/api/books/${id}/download?keep=1`).then(async (r) => {
      if (r.status === 401) location.href = "/";
      if (!r.ok) return null;
      const name = /filename="([^"]+)"/.exec(r.headers.get("Content-Disposition") || "")?.[1] || "book.epub";
      return new File([await r.blob()], name);
    }),
  ]).catch(() => [null, null]);
  if (!file) {
    record("shown-error", `reader: no file for book ${id}`);
    return note("This book has no file NovelCheck can open here (it reads EPUB, MOBI, AZW3 and FB2).");
  }
  $("#reader-title").textContent = info?.title || "";
  document.title = `${info?.title || "Reading"} · NovelCheck`;
  $("#reader-close").href = `/#/library?book=${id}`;

  view = document.createElement("foliate-view");
  $("#reader-book").append(view);
  view.addEventListener("load", onLoad);
  view.addEventListener("relocate", onRelocate);
  try {
    await view.open(file);
  } catch (err) {
    record("shown-error", `reader: book ${id} couldn't be opened: ${err.message || err}`);
    return note(`This book couldn't be opened: ${err.message || err}`);
  }
  applySettings(view, readerSettings());
  note("");
  const pend = pendingPosition(id); // read offline and not sent yet: that place is the newest
  const here = pend ? { ...info?.here, cfi: pend.cfi, fraction: pend.fraction, updated_at: pend.at } : info?.here;
  // Where your e-reader is (KOReader's progress sync, or its reading statistics): when you read there more
  // recently than here, or never read it here, the book opens there (by percentage, so to about a page).
  const ko = info?.koreader;
  const useKo = ko && ko.fraction > 0.001 && (!here?.cfi || (ko.at > here.updated_at && Math.abs(ko.fraction - (here.fraction || 0)) > 0.005));
  await view.init({ lastLocation: useKo ? undefined : here?.cfi || undefined, showTextStart: !useKo && !here?.cfi });
  if (useKo) {
    await view.goToFraction(ko.fraction);
    const pct = Math.round(ko.fraction * 100);
    const where = `Opened at ${pct}%, where your ${ko.device || "e-reader"} is.`;
    if (here?.cfi) note(where, { label: `Back to ${Math.round((here.fraction || 0) * 100)}% (your place here)`, run: () => view.goTo(here.cfi) });
    else {
      note(where);
      setTimeout(() => note(""), 8000);
    }
  }
  document.addEventListener("keydown", onKey);
  $("#reader-prev").onclick = () => edge(false);
  $("#reader-next").onclick = () => edge(true);
  $("#reader-toc-btn").onclick = () => openContents(view);
  $("#reader-settings-btn").onclick = () => openSettings(view);
  moved = bindBars(view, { id, api, info, here, closeMenu: () => toggleMenu(false) });
  // The menu shows for a moment when the book opens, so nobody has to guess where the way out is.
  toggleMenu(true);
  setTimeout(() => {
    if ($("#reader-panel").hidden && !$("#reader-top").hidden) toggleMenu(false);
  }, 3500);
}

start();
