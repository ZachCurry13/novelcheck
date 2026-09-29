// What the AI is doing, for parents: a pill in the header ("⚡ 38" while
// books are being rated, "⏸ 38" while paused for the hourly limit, "🧬 4/12"
// during a Deep Scan, "🌙 38" when books wait for the automatic-rating hours)
// and an "nc:activity" event that pages with live status listen to.
import { get } from "./api.js";
import { canManage } from "./ui.js";

const EVERY = 10000;
let timer = null;
let listening = false;
let allowed = false;

async function tick() {
  if (!allowed || document.visibilityState !== "visible" || document.getElementById("app-view")?.classList.contains("hidden")) return; // signed out
  const a = await get("/api/activity").catch(() => null);
  if (!a) return;
  paint(document.getElementById("activity-pill"), a);
  window.dispatchEvent(new CustomEvent("nc:activity", { detail: a }));
}

export function initActivity(state) {
  clearInterval(timer);
  allowed = canManage(state.user);
  const pill = document.getElementById("activity-pill");
  pill?.classList.add("hidden");
  if (!allowed || !pill) return;
  if (!listening) {
    listening = true;
    window.addEventListener("hashchange", tick);
    document.addEventListener("visibilitychange", tick);
  }
  tick();
  timer = setInterval(tick, EVERY);
}

// refreshActivity asks right away (e.g. after starting a batch).
export const refreshActivity = tick;

function paint(pill, a) {
  if (!pill) return;
  const busy = a.state && a.state !== "idle";
  const n = a.waiting ? String(a.waiting) : "";
  let show = "";
  let title = "";
  let href = "#/library?spice=Pending";
  if (a.deep_waiting) {
    show = "🧬⏸";
    title = "Deep Scans wait for the Deep Scan machine to be switched on";
    href = "#/deepscan?running";
  } else if (a.deep) {
    show = `🧬 ${a.deep.part}/${a.deep.parts}`;
    title = `Deep Scan of “${a.deep.title}”: part ${a.deep.part} of ${a.deep.parts}`;
    href = "#/deepscan?running";
  } else if (busy) {
    const paused = a.state === "rate_limited";
    show = `${paused ? "⏸" : "⚡"} ${n}`;
    title = paused ? `Paused for the hourly AI limit; ${a.waiting} waiting` : `Rating “${a.title}”${a.waiting ? `; ${a.waiting} more waiting` : ""}`;
  } else if (a.waiting && a.auto && !a.hours_open) {
    show = `🌙 ${n}`;
    title = `${a.waiting} books wait for the automatic-rating hours`;
  }
  pill.classList.toggle("hidden", !show);
  pill.textContent = show.trim();
  pill.title = title;
  pill.setAttribute("aria-label", title);
  pill.href = href;
}
