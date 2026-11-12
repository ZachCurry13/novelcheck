// NovelCheck's reader, laid out like KOReader: the book fills the screen, tap the left third to go back and
// the rest to go forward, tap the top band for the menu (contents and reading settings, js/readermenu.js), and
// a slim status line shows the chapter and how far you are. The book engine is foliate-js (vendor/foliate).
// Your place is saved to NovelCheck as you read; when KOReader's progress sync has you further on, the reader
// offers to jump there (by percentage, so to about a page).
import "/vendor/foliate/view.js";
import { readerSettings, applySettings, openContents, openSettings, closePanel } from "./readermenu.js";

const $ = (s) => document.querySelector(s);
const id = Number(new URLSearchParams(location.search).get("id"));
const api = (path, opts = {}) => fetch(path, { credentials: "same-origin", ...opts, headers: { "X-NovelCheck": "1", ...(opts.headers || {}) } });

let view;
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

// save keeps your place on the server a moment after you stop turning pages.
function save(cfi, fraction) {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    if (!cfi || cfi === lastSaved) return;
    lastSaved = cfi;
    api(`/api/books/${id}/position`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cfi, fraction }) }).catch(() => {});
  }, 1500);
}

function toggleMenu(show = $("#reader-top").hidden) {
  $("#reader-top").hidden = !show;
  if (!show) closePanel();
}

// tap decides what a tap at screen position x, y does (KOReader's zones).
function tap(x, y) {
  if (!$("#reader-top").hidden || !$("#reader-panel").hidden) return toggleMenu(false);
  if (y < window.innerHeight * 0.15) return toggleMenu(true);
  if (x < window.innerWidth / 3) view.prev();
  else view.next();
}

function onLoad({ detail: { doc } }) {
  // Taps land inside the book's frame: turn the frame's position into a position on the screen.
  doc.addEventListener("click", (e) => {
    if (e.target.closest?.("a[href]") || doc.getSelection()?.toString()) return;
    const frame = doc.defaultView.frameElement?.getBoundingClientRect() || { left: 0, top: 0 };
    tap(frame.left + e.clientX, frame.top + e.clientY);
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
  save(detail.cfi, status.fraction);
}

async function start() {
  if (!id) return note("No book was picked. Go back to NovelCheck and press Read on a book.");
  note("Opening the book…");
  const [info, file] = await Promise.all([
    api(`/api/books/${id}/position`).then((r) => (r.ok ? r.json() : null)),
    api(`/api/books/${id}/download`).then(async (r) => {
      if (r.status === 401) location.href = "/";
      if (!r.ok) return null;
      const name = /filename="([^"]+)"/.exec(r.headers.get("Content-Disposition") || "")?.[1] || "book.epub";
      return new File([await r.blob()], name);
    }),
  ]).catch(() => [null, null]);
  if (!file) return note("This book has no file NovelCheck can open here (it reads EPUB, MOBI, AZW3 and FB2).");
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
    return note(`This book couldn't be opened: ${err.message || err}`);
  }
  applySettings(view, readerSettings());
  note("");
  const here = info?.here;
  await view.init({ lastLocation: here?.cfi || undefined, showTextStart: !here?.cfi });
  // KOReader's progress sync has you further on (or you have never read it here): offer to go there.
  const ko = info?.koreader;
  if (ko && ko.fraction > 0 && (!here || (ko.at > here.updated_at && Math.abs(ko.fraction - (here.fraction || 0)) > 0.005))) {
    const pct = Math.round(ko.fraction * 100);
    note(`Your ${ko.device || "e-reader"} is at ${pct}%.`, { label: `Go to ${pct}%`, run: () => view.goToFraction(ko.fraction) });
  }
  document.addEventListener("keydown", onKey);
  $("#reader-toc-btn").onclick = () => openContents(view);
  $("#reader-settings-btn").onclick = () => openSettings(view);
}

start();
