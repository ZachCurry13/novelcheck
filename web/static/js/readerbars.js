// The reader's menu bars (reader.html, js/reader.js), the way e-readers have them: at the top Exit, the title,
// search in the book and a bookmark for this page; at the bottom a slider for where you are with the time left,
// and Contents, Aa (js/readermenu.js), a Day · Sepia · Night switch, your bookmarks, and a jump to where your
// Kindle is.
import { readerSettings, applySettings, keepSettings, THEMES } from "./readermenu.js";

const $ = (s) => document.querySelector(s);
const esc = (t) => String(t).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
const panel = () => $("#reader-panel");
const ICON = { day: "☀", sepia: "◐", night: "☾" };

export function bindBars(view, { id, api, info, here, closeMenu }) {
  let place = { cfi: "", fraction: 0, chapter: "" };
  let marks = [];
  const markBtn = $("#reader-mark-btn");

  // The Day · Sepia · Night switch: one tap goes to the next.
  const paintTheme = () => {
    const t = readerSettings().theme;
    $("#reader-theme-ic").textContent = ICON[t] || "☀";
    $("#reader-theme-l").textContent = THEMES[t]?.[0] || "Day";
  };
  $("#reader-theme-btn").onclick = () => {
    const s = readerSettings(), order = Object.keys(THEMES);
    s.theme = order[(order.indexOf(s.theme) + 1) % order.length];
    keepSettings(s);
    applySettings(view, s);
    paintTheme();
  };
  paintTheme();
  document.addEventListener("nc:reader-settings", paintTheme); // changed in the Aa panel

  // The slider: drag to anywhere in the book.
  const slider = $("#reader-slider");
  slider.oninput = () => ($("#reader-pct").textContent = `${Math.round(slider.value / 10)}%`);
  slider.onchange = () => view.goToFraction(slider.value / 1000);

  // Bookmarks, kept with your account.
  const isMarked = () => marks.some((m) => m.cfi === place.cfi);
  const paintMark = () => markBtn.setAttribute("aria-pressed", String(isMarked()));
  const loadMarks = async () => {
    const r = await api(`/api/books/${id}/bookmarks`).then((x) => (x.ok ? x.json() : null)).catch(() => null);
    marks = r?.bookmarks || [];
    paintMark();
  };
  markBtn.onclick = async () => {
    if (!place.cfi) return;
    const r = await api(`/api/books/${id}/bookmarks`, { method: isMarked() ? "DELETE" : "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ cfi: place.cfi, fraction: place.fraction, label: place.chapter }) }).then((x) => (x.ok ? x.json() : null)).catch(() => null);
    if (r) marks = r.bookmarks;
    paintMark();
  };
  $("#reader-marks-btn").onclick = () => {
    const p = panel();
    p.innerHTML = `<h2 class="text-base font-medium">Bookmarks</h2>
      ${marks.length ? `<ul class="mt-2 space-y-1">${marks.map((m) => `<li class="flex items-center gap-2"><button type="button" data-cfi="${esc(m.cfi)}" class="reader-toc-item flex-1">
        ${esc(m.label || "Bookmark")} <span class="opacity-60">· ${Math.round(m.fraction * 100)}%</span></button>
        <button type="button" data-drop="${esc(m.cfi)}" class="icon-btn" aria-label="Remove this bookmark">✕</button></li>`).join("")}</ul>`
        : `<p class="mt-2 text-sm opacity-70">No bookmarks yet. Tap 🔖 at the top to mark the page you're on.</p>`}`;
    p.hidden = false;
    p.onclick = async (e) => {
      const go = e.target.closest("[data-cfi]")?.dataset.cfi;
      if (go) return void (view.goTo(go), closeMenu());
      const drop = e.target.closest("[data-drop]")?.dataset.drop;
      if (!drop) return;
      const r = await api(`/api/books/${id}/bookmarks`, { method: "DELETE", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ cfi: drop }) }).then((x) => (x.ok ? x.json() : null)).catch(() => null);
      if (r) marks = r.bookmarks;
      paintMark();
      $("#reader-marks-btn").onclick();
    };
  };

  // Search in the book: results by chapter, with the words around each.
  $("#reader-search-btn").onclick = () => {
    const p = panel();
    p.innerHTML = `<h2 class="text-base font-medium">Search in the book</h2>
      <form class="mt-2 flex gap-2" data-find><input name="q" type="search" class="input min-w-0 flex-1" placeholder="Words to find" autocomplete="off" enterkeyhint="search">
        <button class="btn-primary">Find</button></form><p data-state class="mt-2 text-sm opacity-70"></p><ul data-hits class="mt-1 space-y-1"></ul>`;
    p.hidden = false;
    p.querySelector("input").focus();
    p.onsubmit = async (e) => {
      e.preventDefault();
      const q = e.target.q.value.trim();
      if (!q) return;
      const state = p.querySelector("[data-state]"), hits = p.querySelector("[data-hits]");
      state.textContent = "Searching…";
      hits.innerHTML = "";
      let n = 0;
      for await (const r of view.search({ query: q })) {
        if (r === "done") break;
        for (const s of r.subitems || []) {
          n++;
          hits.insertAdjacentHTML("beforeend", `<li><button type="button" data-cfi="${esc(s.cfi)}" class="reader-toc-item"><span class="opacity-60">${esc(r.label || "")}</span><br>
            …${esc(s.excerpt?.pre || "")}<b>${esc(s.excerpt?.match || q)}</b>${esc(s.excerpt?.post || "")}…</button></li>`);
        }
        state.textContent = `${n} found…`;
      }
      state.textContent = n ? `${n} found` : "Not found in this book.";
    };
    p.onclick = (e) => {
      const go = e.target.closest("[data-cfi]")?.dataset.cfi;
      if (go) view.goTo(go), closeMenu();
    };
  };

  // Where your Kindle (KOReader) is, when NovelCheck knows.
  const ko = info?.koreader;
  if (ko?.fraction > 0) {
    const b = $("#reader-ko-btn");
    b.hidden = false;
    $("#reader-ko-l").textContent = `${(ko.device || "Kindle").split(" ")[0]} ${Math.round(ko.fraction * 100)}%`;
    b.onclick = () => (view.goToFraction(ko.fraction), closeMenu());
  }

  loadMarks();
  // moved is called on every page turn: the slider, the percentage, the time left and the bookmark button.
  return (detail) => {
    place = { cfi: detail.cfi || "", fraction: detail.fraction || 0, chapter: detail.tocItem?.label?.trim() || "" };
    if (document.activeElement !== slider) slider.value = String(Math.round(place.fraction * 1000));
    $("#reader-pct").textContent = `${Math.round(place.fraction * 100)}%`;
    const min = detail.time?.total;
    $("#reader-left").textContent = min == null ? "" : min >= 60 ? `${Math.floor(min / 60)} h ${Math.round(min % 60)} min left` : `${Math.max(1, Math.round(min))} min left`;
    paintMark();
  };
}
