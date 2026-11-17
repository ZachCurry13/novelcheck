// Pull down to refresh, on purpose. The browser's own pull-to-refresh is switched off (it fired by accident while
// scrolling and reloaded the whole app), so this is a deliberate one: from the very top of a page, pull straight
// down about 150 px and hold it there for a moment (the label changes to "Release to refresh" and the phone gives
// a small buzz), then let go. A flick, a scroll that reaches the top, a sideways swipe or a touch that starts in
// a menu, a dialog, a text box or a row that scrolls sideways never refreshes. Refreshing draws the page again
// with fresh data (or reloads the app when a newer NovelCheck is ready).
import { pullRefresh } from "./appupdate.js";

const SHOW = 28; // finger travel before anything shows
const ARM = 150; // finger travel to arm it
const HOLD = 350; // ms it must be held there
const REDUCED = matchMedia("(prefers-reduced-motion: reduce)").matches;

let ind = null;
const indicator = () => {
  if (ind) return ind;
  ind = document.createElement("div");
  ind.className = "ptr";
  ind.setAttribute("role", "status");
  ind.setAttribute("aria-live", "polite");
  ind.innerHTML = `<span class="ptr-ring" aria-hidden="true"></span><span class="ptr-text"></span>`;
  document.body.append(ind);
  return ind;
};

const paint = (state, progress = 0, text = "") => {
  const el = indicator();
  el.dataset.state = state; // pull, armed or busy
  el.style.setProperty("--p", String(progress));
  el.querySelector(".ptr-text").textContent = text;
  el.classList.add("show");
};
const hide = () => ind?.classList.remove("show");

// startsSomewhereElse: a touch that begins inside something that scrolls, types or opens over the page.
const startsSomewhereElse = (el) => {
  if (!(el instanceof Element)) return true;
  if (document.querySelector("dialog[open]") || document.getElementById("app-view")?.classList.contains("hidden")) return true;
  if (el.closest("input, textarea, select, iframe, [contenteditable], #update-bar")) return true;
  for (let n = el; n && n !== document.body; n = n.parentElement) {
    const s = getComputedStyle(n);
    if (n.scrollTop > 0 || (/(auto|scroll)/.test(s.overflowX) && n.scrollWidth > n.clientWidth)) return true;
  }
  return false;
};

let y0 = 0, x0 = 0, tracking = false, armed = false, holdTimer = 0, busy = false;

const reset = () => {
  tracking = armed = false;
  clearTimeout(holdTimer);
};

document.addEventListener("touchstart", (e) => {
  reset();
  if (busy || e.touches.length !== 1 || window.scrollY > 0 || startsSomewhereElse(e.target)) return;
  x0 = e.touches[0].clientX;
  y0 = e.touches[0].clientY;
  tracking = true;
}, { passive: true });

document.addEventListener("touchmove", (e) => {
  if (!tracking) return;
  const t = e.touches[0], dy = t.clientY - y0, dx = t.clientX - x0;
  if (window.scrollY > 0 || dy < 0 || (dy > 12 && Math.abs(dx) > Math.abs(dy) * 0.6)) { // scrolling, or sideways: not a pull
    reset();
    hide();
    return;
  }
  if (dy < SHOW) return;
  const p = Math.min((dy - SHOW) / (ARM - SHOW), 1);
  if (p < 1) {
    armed = false;
    clearTimeout(holdTimer);
    holdTimer = 0;
    paint("pull", p, "Pull to refresh");
  } else if (!armed) {
    paint("pull", 1, "Keep holding…");
    if (!holdTimer) {
      holdTimer = setTimeout(() => {
        armed = true;
        paint("armed", 1, "Release to refresh");
        navigator.vibrate?.(15);
      }, HOLD);
    }
  }
}, { passive: true });

const finish = async (go) => {
  const wasArmed = armed;
  reset();
  if (!go || !wasArmed) return hide();
  busy = true;
  paint("busy", 1, "Refreshing…");
  await Promise.all([pullRefresh().catch(() => {}), new Promise((r) => setTimeout(r, REDUCED ? 300 : 700))]);
  hide();
  busy = false;
};
document.addEventListener("touchend", () => finish(true), { passive: true });
document.addEventListener("touchcancel", () => finish(false), { passive: true });
