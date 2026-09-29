// Shared DOM helpers. All dynamic text goes through esc() before innerHTML.
import { pepperChip } from "./peppers.js";
import { copyText } from "./copy.js";
import { diagnoseLater } from "./diagnose.js";

export function esc(v) {
  return String(v ?? "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  })[c]);
}

export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

// Messages shown on this device, newest first (listed under the bell).
export const messageLog = [];

let toastTimer;
// toast shows a short message. Errors stay until closed (so they can't vanish
// before you read them) and every message is kept in messageLog. link, if
// given ({label, href}), adds a link such as "View Up Next".
export function toast(msg, isError = false, link = null) {
  const t = $("#toast");
  t.replaceChildren();
  const text = document.createElement("span");
  text.textContent = msg;
  t.append(text);
  if (link) {
    const a = document.createElement("a");
    a.href = link.href;
    a.textContent = link.label;
    a.className = "ml-3 whitespace-nowrap font-semibold text-indigo-300 underline";
    a.onclick = () => t.classList.add("hidden");
    t.append(a);
  }
  if (isError) {
    const btn = (label, title, fn) => {
      const b = document.createElement("button");
      b.textContent = label;
      b.title = title;
      b.className = "ml-2 px-1 text-slate-300 hover:text-white";
      b.onclick = fn;
      t.append(b);
    };
    btn("📋", "Copy this message", async (e) => {
      // Flash ✓ on the button; a "Copied" toast would replace this error.
      const b = e.currentTarget;
      if (await copyText(String(msg), true)) {
        b.textContent = "✓";
        setTimeout(() => (b.textContent = "📋"), 1500);
      }
    });
    // Admins can ask the AI what went wrong.
    if (document.body.dataset.role === "admin") btn("🩺", "Diagnose with AI", () => { t.classList.add("hidden"); diagnoseLater(String(msg)); });
    const close = document.createElement("button");
    close.textContent = "✕";
    close.title = "Close";
    close.className = "ml-3 px-1 text-slate-400 hover:text-white";
    close.onclick = () => t.classList.add("hidden");
    t.append(close);
  }
  t.classList.toggle("toast-error", isError);
  t.classList.remove("hidden");
  clearTimeout(toastTimer);
  if (!isError) toastTimer = setTimeout(() => t.classList.add("hidden"), link ? 6000 : 3500);
  messageLog.unshift({ at: new Date(), text: String(msg), isError });
  messageLog.length = Math.min(messageLog.length, 30);
  window.dispatchEvent(new CustomEvent("nc:message"));
}

// Runs an async action, surfacing failures as a toast.
export async function attempt(fn, okMsg) {
  try {
    const r = await fn();
    if (okMsg) toast(okMsg);
    return r;
  } catch (e) {
    toast(e.message || String(e), true);
    return undefined;
  }
}

// Books without a rating yet: where they are on the way to one.
const STATUS_CHIPS = {
  pending: ["chip-wait", "⏳ Waiting", "Waiting to be rated"],
  queued: ["chip-wait", "⏳ In line", "In line to be rated"],
  processing: ["chip-busy", "⚡ Rating now…", "The AI is rating this book"],
  error: ["chip-fail", "⚠ Rating failed", "Rating this book failed; a parent can try again"],
};

export function classChip(book) {
  if (book.spice_level !== null && book.spice_level !== undefined) return pepperChip(book.spice_level);
  const c = book.classification;
  if (!c) {
    const [cls, label, tip] = STATUS_CHIPS[book.status || book.book_status] || STATUS_CHIPS.pending;
    return `<span class="${cls}" data-status title="${tip}">${label}</span>`;
  }
  const cls = { "No Spice": "chip-none", "Closed Door": "chip-closed", "Open Door": "chip-open" }[c] || "chip-pending";
  // Rated before the pepper scale: show the older label until re-rated.
  return `<span class="${cls}" title="Older rating; re-rate for peppers">${esc(c)}</span>`;
}

// Age groups (same order and levels as the server's store.AgeGroups).
export const AGE_GROUPS = [
  [1, "Young kids", "up to 8"],
  [2, "Middle grade", "9–12"],
  [3, "Teens", "13–15"],
  [4, "Young adult", "16–17"],
  [5, "Adults", "18+"],
];
export const ageLabel = (lvl) => {
  const g = AGE_GROUPS.find(([l]) => l === lvl);
  return g ? `${g[1]} (${g[2]})` : "";
};
export const ageChip = (b) => (b.age_level ? `<span class="chip-cat" title="Age group set by ${esc(b.age_set_by)}">👪 ${esc(ageLabel(b.age_level))}</span>` : "");

// GitHub issue form for suggesting new filters (the footer links to all forms).
export const FILTER_IDEA_URL = "https://github.com/ZachCurry13/novelcheck/issues/new?template=filter_suggestion.yml";

// Hide checkboxes in the Library, in display order.
export const HIDE_LABELS = {
  open_door: "Open Door",
  nudity: "Nudity",
  solo_acts: "Solo Acts",
  heavy_innuendo: "Heavy Innuendo",
  dark_occult: "Dark Occult / Demonic",
};

export const FLAG_LABELS = {
  nudity: "Nudity",
  solo_acts: "Solo Acts",
  heavy_innuendo: "Heavy Innuendo",
  dark_occult: "Dark Occult / Demonic",
};

export function flagChips(b) {
  const on = [];
  if (b.nudity) on.push("Nudity");
  if (b.solo_acts) on.push("Solo Acts");
  if (b.heavy_innuendo) on.push("Heavy Innuendo");
  if (b.dark_occult || b.demonic_presence) on.push("Dark Occult");
  if (b.playful_fantasy && !b.dark_occult) on.push("Fantasy Magic");
  const chips = on.map((f) => `<span class="chip-flag">${esc(f)}</span>`);
  if (b.approved) chips.unshift(`<span class="chip-none" title="Marked OK by ${esc(b.approved_by)}">✓ OK'd by parent</span>`);
  return chips.join(" ");
}

// Admins and editors share the management views; only admins see technical settings.
export const canManage = (user) => user?.role === "admin" || user?.role === "editor";

export function fmtNum(n) {
  return Number(n || 0).toLocaleString();
}

export function fmtMoney(n) {
  return "$" + Number(n || 0).toFixed(n < 1 ? 4 : 2);
}

// fmtMinutes says roughly how long something takes ("about 40 min").
export function fmtMinutes(m) {
  return m >= 90 ? `about ${(m / 60).toFixed(1)} hours` : `about ${Math.max(1, Math.round(m))} min`;
}
