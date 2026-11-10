// Debug recording, the person's side. An admin can turn recording on for a few minutes (Settings → Developer).
// While it is on, everyone's NovelCheck says so in a bar at the top and sends what happens here to the server:
// pages opened, buttons tapped, what was typed (never passwords, codes or keys), failed requests and errors.
// The server notes every call it answers itself, so successful calls are not repeated from here.
import { setSink } from "./debugrec.js";

const CHECK_EVERY = 60 * 1000;
const SECRET = /pass|pin\b|token|secret|key|code|auth|card|cvv|ssn/i;
let queue = [];
let on = false;
let until = 0;
let bar = null;
let flushing = null;
const realFetch = window.fetch.bind(window); // the page's own requests are not recorded as taps

function add(kind, detail) {
  if (!on) return;
  queue.push({ t: Date.now(), kind, detail: String(detail).replace(/\s+/g, " ").slice(0, 500) });
  if (queue.length >= 40) flush();
}

async function flush() {
  if (!queue.length) return;
  const events = queue.splice(0, 300);
  try {
    const res = await realFetch("/api/activity/events", {
      method: "POST", credentials: "same-origin", keepalive: true,
      headers: { "Content-Type": "application/json", "X-NovelCheck": "1" },
      body: JSON.stringify({ events }),
    });
    if (res.ok) apply(await res.json());
  } catch {
    /* offline: those lines are lost; recording is for the minutes the person is online */
  }
}

const label = (el) => (el.getAttribute("aria-label") || el.name || el.id || el.placeholder || el.textContent || el.tagName).trim().replace(/\s+/g, " ").slice(0, 60);

// typed: a field changed. Passwords and anything that looks like a secret are never kept.
function typed(e) {
  const el = e.target;
  if (!el || !["INPUT", "TEXTAREA", "SELECT"].includes(el.tagName)) return;
  const type = (el.type || "").toLowerCase();
  if (["password", "hidden", "file"].includes(type)) return;
  const name = `${el.name || ""} ${el.id || ""} ${el.autocomplete || ""}`;
  const shown = SECRET.test(name) ? "(hidden)" : type === "checkbox" || type === "radio" ? (el.checked ? "on" : "off") : el.value.slice(0, 200);
  add("input", `${label(el)} = ${shown}`);
}

function showBar() {
  if (!bar) {
    bar = document.createElement("div");
    bar.id = "debug-bar";
    bar.setAttribute("role", "status");
    document.body.prepend(bar);
    document.body.classList.add("debugging");
  }
  const mins = Math.max(0, Math.ceil((until * 1000 - Date.now()) / 60000));
  bar.textContent = `🔴 Debug mode: an admin is recording what happens in NovelCheck (pages, taps, what you type except passwords) to fix a bug. Ends in about ${mins} min.`;
}

function apply(state) {
  const was = on;
  on = Boolean(state?.on);
  until = state?.until || 0;
  if (on) showBar();
  else if (bar) {
    bar.remove();
    bar = null;
    document.body.classList.remove("debugging");
  }
  if (on && !was) {
    add("page", `recording on at ${location.hash || "/"}`);
    flushing = flushing || setInterval(flush, 4000);
  }
  if (!on && flushing) {
    clearInterval(flushing);
    flushing = null;
  }
}

export async function check() {
  if (document.visibilityState !== "visible") return;
  try {
    const res = await realFetch("/api/activity/status", { credentials: "same-origin", headers: { "X-NovelCheck": "1" } });
    if (res.ok) apply(await res.json());
  } catch {
    /* offline */
  }
}

export function startActivity() {
  setSink((kind, msg) => {
    if (kind === "net" && !/-> [45]\d\d|could not connect/.test(msg)) return; // the server notes successful calls
    add(kind, msg);
  });
  document.addEventListener("change", typed, true);
  document.addEventListener("visibilitychange", check);
  window.addEventListener("pagehide", flush);
  setInterval(() => (on ? showBar() : null), 30000);
  setInterval(check, CHECK_EVERY);
  check();
}
