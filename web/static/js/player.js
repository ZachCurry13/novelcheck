// 🎧 The audiobook player: plays the book through NovelCheck (the audio comes
// from Audiobookshelf), keeps the person's place, and has speed, a sleep timer,
// chapters and bookmarks.
import { get, put } from "./api.js";
import { $, esc, attempt } from "./ui.js";
import { fmtClock } from "./audioui.js";
import { bindExtras, extrasHTML } from "./playerextras.js";

const SPEEDS = [0.75, 1, 1.15, 1.25, 1.5, 1.75, 2, 2.5];
const keep = (k, v) => {
  try {
    if (v === undefined) return localStorage.getItem(k);
    localStorage.setItem(k, v);
  } catch {
    /* private mode */
  }
  return null;
};

export async function openPlayer(bookId) {
  const info = await attempt(() => get(`/api/books/${bookId}/listen`));
  if (!info) return;
  const d = info.detail;
  let dlg = document.getElementById("player-dialog");
  if (!dlg) {
    dlg = document.createElement("dialog");
    dlg.id = "player-dialog";
    dlg.className = "dialog";
    document.body.append(dlg);
  }
  const total = d.duration || d.tracks.reduce((a, t) => a + t.duration, 0);
  const speed = Number(keep("nc:speed")) || 1;
  dlg.innerHTML = `<div class="max-h-[90vh] space-y-4 overflow-y-auto p-5">
    <div class="flex items-start justify-between gap-3"><div class="flex min-w-0 gap-3">
        <img src="/api/books/${bookId}/cover?size=l" alt="" class="h-24 w-16 shrink-0 rounded object-cover">
        <div class="min-w-0"><h2 class="break-words text-lg font-semibold">${esc(info.title)}</h2><p class="text-sm text-slate-400">${esc(info.author || "")}</p>
          ${info.narrator ? `<p class="text-xs text-slate-500">${esc(info.narrator)}</p>` : ""}<p id="pl-chapter" class="mt-1 text-sm text-indigo-300"></p></div></div>
      <button data-close class="btn-ghost px-2 text-xl" aria-label="Close player">✕</button></div>
    <audio id="pl-audio" preload="metadata"></audio>
    <div class="space-y-1"><input id="pl-seek" type="range" min="0" max="${Math.floor(total)}" step="1" value="0" class="w-full accent-indigo-500" aria-label="Position in the book">
      <div class="flex justify-between text-xs tabular-nums text-slate-400"><span id="pl-now">0:00</span><span id="pl-left"></span></div></div>
    <div class="flex items-center justify-center gap-3">
      <button type="button" data-skip="-30" class="btn-secondary px-3" aria-label="Back 30 seconds">⏪ 30</button>
      <button type="button" id="pl-play" class="btn-primary h-14 w-14 rounded-full p-0 text-2xl" aria-label="Play">▶</button>
      <button type="button" data-skip="30" class="btn-secondary px-3" aria-label="Forward 30 seconds">30 ⏩</button></div>
    <div class="flex flex-wrap items-center gap-2">
      <label class="text-sm">Speed <select id="pl-speed" class="input w-auto py-1">${SPEEDS.map((s) => `<option value="${s}" ${s === speed ? "selected" : ""}>${s}×</option>`).join("")}</select></label>
      ${extrasHTML(d)}</div>
    <p id="pl-msg" class="hidden text-sm text-rose-300"></p></div>`;
  const audio = $("#pl-audio", dlg), seek = $("#pl-seek", dlg), play = $("#pl-play", dlg);
  const url = (t) => `/api/books/${bookId}/audio/${encodeURIComponent(t.ino)}`;
  let cur = 0, lastSave = 0, sleepAt = 0, stopAtChapter = false, dragging = false, played = false;
  const now = () => d.tracks[cur].start + (audio.currentTime || 0);
  const chapterAt = (t) => d.chapters.findIndex((c) => t >= c.start && t < c.end);
  const save = (finished = false) => {
    if (!played) return Promise.resolve(); // opened but never played: leave their place alone
    lastSave = Date.now();
    return put(`/api/books/${bookId}/listen/progress`, { at: now(), duration: total, finished }).catch(() => {});
  };
  const paint = () => {
    const t = now();
    if (!dragging) seek.value = Math.floor(t);
    $("#pl-now", dlg).textContent = fmtClock(t);
    $("#pl-left", dlg).textContent = `-${fmtClock(Math.max(0, total - t) / audio.playbackRate)} left`;
    const c = chapterAt(t);
    $("#pl-chapter", dlg).textContent = c >= 0 ? d.chapters[c].title : "";
    const sel = $("#pl-chapters", dlg);
    if (sel && c >= 0) sel.value = String(c);
  };
  // goTo moves to a place in the whole book, changing the file when needed.
  const goTo = (t, andPlay = false) => {
    t = Math.min(Math.max(0, t), total);
    let i = d.tracks.findIndex((x) => t >= x.start && t < x.start + x.duration);
    if (i < 0) i = d.tracks.length - 1;
    const set = () => {
      audio.currentTime = t - d.tracks[i].start;
      paint();
      if (andPlay) audio.play().catch(() => {});
    };
    if (i !== cur || !audio.getAttribute("src")) {
      cur = i;
      audio.src = url(d.tracks[i]);
      audio.addEventListener("loadedmetadata", set, { once: true });
    } else set();
  };
  const startAt = d.progress && !d.progress.finished && d.progress.current_time > 0 ? d.progress.current_time : 0;
  audio.playbackRate = speed;
  goTo(d.tracks.length ? startAt : 0);
  audio.onplay = () => {
    played = true;
    play.textContent = "⏸";
    play.setAttribute("aria-label", "Pause");
  };
  audio.onpause = () => {
    play.textContent = "▶";
    play.setAttribute("aria-label", "Play");
    if (dlg.open) save();
  };
  audio.ontimeupdate = () => {
    paint();
    if (Date.now() - lastSave > 15000 && !audio.paused) save();
    if (sleepAt && Date.now() >= sleepAt) {
      sleepAt = 0;
      audio.pause();
    }
    const c = chapterAt(now());
    if (stopAtChapter && c >= 0 && now() >= d.chapters[c].end - 0.4) {
      stopAtChapter = false;
      audio.pause();
    }
  };
  audio.onended = () => {
    if (cur < d.tracks.length - 1) goTo(d.tracks[cur + 1].start, true);
    else save(true);
  };
  audio.onerror = () => {
    const m = $("#pl-msg", dlg);
    m.textContent = "That part couldn't be played. Check that Audiobookshelf is reachable, then try again.";
    m.classList.remove("hidden");
  };
  play.onclick = () => (audio.paused ? audio.play().catch(() => {}) : audio.pause());
  seek.oninput = () => {
    dragging = true;
    $("#pl-now", dlg).textContent = fmtClock(seek.value);
  };
  seek.onchange = () => {
    dragging = false;
    goTo(Number(seek.value), !audio.paused);
  };
  $("#pl-speed", dlg).onchange = (e) => {
    audio.playbackRate = Number(e.target.value);
    keep("nc:speed", e.target.value);
  };
  dlg.onclick = (e) => {
    const skip = e.target.closest("[data-skip]")?.dataset.skip;
    if (skip) goTo(now() + Number(skip), !audio.paused);
    if (e.target === dlg || e.target.closest("[data-close]")) dlg.close();
  };
  dlg.onclose = () => {
    save(); // before the sound is cleared, so the place is kept
    audio.pause();
    audio.removeAttribute("src");
    audio.load();
    if ("mediaSession" in navigator) navigator.mediaSession.metadata = null;
  };
  bindExtras(dlg, { bookId, d, now, goTo, audio, setSleep: (min, atChapter) => {
    sleepAt = min ? Date.now() + min * 60000 : 0;
    stopAtChapter = atChapter;
  } });
  if ("mediaSession" in navigator) {
    navigator.mediaSession.metadata = new MediaMetadata({ title: info.title, artist: info.author || "", artwork: [{ src: `/api/books/${bookId}/cover?size=l`, sizes: "480x480", type: "image/jpeg" }] });
    navigator.mediaSession.setActionHandler("play", () => audio.play());
    navigator.mediaSession.setActionHandler("pause", () => audio.pause());
    navigator.mediaSession.setActionHandler("seekbackward", () => goTo(now() - 30, !audio.paused));
    navigator.mediaSession.setActionHandler("seekforward", () => goTo(now() + 30, !audio.paused));
  }
  dlg.showModal();
  paint();
}
