// Shared DOM helpers. All dynamic text goes through esc() before innerHTML.

export function esc(v) {
  return String(v ?? "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  })[c]);
}

export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

let toastTimer;
export function toast(msg, isError = false) {
  const t = $("#toast");
  t.textContent = msg;
  t.classList.toggle("text-rose-300", isError);
  t.classList.remove("hidden");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.add("hidden"), 3500);
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

export function classChip(book) {
  const c = book.classification;
  if (!c) {
    const label = { queued: "Queued", processing: "Analyzing…", error: "Analysis Error" }[book.status || book.book_status];
    return `<span class="chip-pending">${esc(label || "Pending Analysis")}</span>`;
  }
  const cls = { "No Spice": "chip-none", "Closed Door": "chip-closed", "Open Door": "chip-open" }[c] || "chip-pending";
  return `<span class="${cls}">${esc(c)}</span>`;
}

export const FLAG_LABELS = {
  nudity: "Nudity",
  solo_acts: "Solo Acts",
  heavy_innuendo: "Heavy Innuendo",
  lgbtq: "LGBTQ+ Content",
  dark_occult: "Dark Occult / Demonic",
};

export function flagChips(b) {
  const on = [];
  if (b.nudity) on.push("Nudity");
  if (b.solo_acts) on.push("Solo Acts");
  if (b.heavy_innuendo) on.push("Heavy Innuendo");
  if (b.lgbtq_content) on.push("LGBTQ+");
  if (b.dark_occult || b.demonic_presence) on.push("Dark Occult");
  if (b.playful_fantasy && !b.dark_occult) on.push("Fantasy Magic");
  return on.map((f) => `<span class="chip-flag">${esc(f)}</span>`).join(" ");
}

// Admins and editors share the management views; only admins see technical settings.
export const canManage = (user) => user?.role === "admin" || user?.role === "editor";

export function fmtNum(n) {
  return Number(n || 0).toLocaleString();
}

export function fmtMoney(n) {
  return "$" + Number(n || 0).toFixed(n < 1 ? 4 : 2);
}
