// The player's sleep timer, chapter list and bookmarks.
import { post, del } from "./api.js";
import { $, esc, attempt } from "./ui.js";
import { fmtClock } from "./audioui.js";

const SLEEP = [["Sleep timer: off", "0"], ["15 minutes", "15"], ["30 minutes", "30"], ["45 minutes", "45"], ["60 minutes", "60"]];

export function extrasHTML(d) {
  return `<select id="pl-sleep" class="input w-auto py-1" aria-label="Sleep timer">${SLEEP.map(([l, v]) => `<option value="${v}">${l}</option>`).join("")}
      ${d.chapters.length ? `<option value="ch">End of this chapter</option>` : ""}</select>
    ${d.chapters.length ? `<select id="pl-chapters" class="input min-w-0 flex-1 py-1" aria-label="Chapters">${d.chapters.map((c, i) => `<option value="${i}">${esc(c.title)} · ${fmtClock(c.start)}</option>`).join("")}</select>` : ""}
    <button type="button" id="pl-mark" class="btn-secondary py-1 text-sm">🔖 Bookmark</button>
    <ul id="pl-marks" class="w-full space-y-1 text-sm"></ul>`;
}

export function bindExtras(dlg, { bookId, d, now, goTo, audio, setSleep }) {
  const marks = [...d.bookmarks];
  const draw = () => {
    $("#pl-marks", dlg).innerHTML = marks.sort((a, b) => a.time - b.time).map((m) => `<li class="flex items-center justify-between gap-2">
      <button type="button" data-mark="${m.time}" class="min-w-0 truncate text-left underline">${fmtClock(m.time)} · ${esc(m.title)}</button>
      <button type="button" data-unmark="${m.time}" class="btn-ghost px-2 py-0 text-xs" aria-label="Delete bookmark">✕</button></li>`).join("");
  };
  draw();
  $("#pl-sleep", dlg).onchange = (e) => setSleep(e.target.value === "ch" ? 0 : Number(e.target.value), e.target.value === "ch");
  const chapters = $("#pl-chapters", dlg);
  if (chapters) chapters.onchange = () => goTo(d.chapters[Number(chapters.value)].start, !audio.paused);
  $("#pl-mark", dlg).onclick = async () => {
    const t = Math.floor(now());
    const title = $("#pl-chapter", dlg).textContent || "Bookmark";
    if (await attempt(() => post(`/api/books/${bookId}/listen/bookmark`, { time: t, title }), "Bookmark saved")) {
      marks.push({ time: t, title });
      draw();
    }
  };
  $("#pl-marks", dlg).onclick = async (e) => {
    const go = e.target.closest("[data-mark]")?.dataset.mark;
    const rm = e.target.closest("[data-unmark]")?.dataset.unmark;
    if (go) return goTo(Number(go), !audio.paused);
    if (rm && (await attempt(() => del(`/api/books/${bookId}/listen/bookmark?time=${rm}`)))) {
      marks.splice(marks.findIndex((m) => String(m.time) === rm), 1);
      draw();
    }
  };
}
