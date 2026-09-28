// The lists on Admin → 🧬 Deep Scan: scans waiting for review (Accept / Keep
// in place, without reloading the page), scans waiting or running (with
// progress), and recent results. Titles and covers open the book's window.
import { esc, fmtNum } from "./ui.js";
import { coverImg } from "./covers.js";
import { deepChangeLine, when } from "./deepscan.js";

const SOURCE = { admin: "started by an admin", request: "requested", batch: "Up Next batch", auto: "automatic" };

export const isOpen = (d) => ["requested", "queued", "reading"].includes(d.status);

// The parts that set the suggested level, from the scan's notes.
function evidence(d) {
  let notes = [];
  try {
    notes = JSON.parse(d.notes || "[]").filter((n) => n.level >= 3);
  } catch {
    /* no notes */
  }
  return notes;
}

const title = (d, cls = "font-semibold leading-snug") =>
  `<button type="button" data-open class="text-left ${cls}">${esc(d.title)}</button>`;

export function reviewCard(d) {
  const notes = evidence(d);
  return `<li class="card space-y-3" data-scan="${d.id}" data-book="${d.book_id}">
    <div class="flex gap-3">
      <button type="button" data-open class="shrink-0" title="Book details">${coverImg(d.book_id, "h-20 w-14")}</button>
      <div class="min-w-0 flex-1 space-y-1">
        ${title(d)}
        <p class="truncate text-sm text-slate-400">${esc(d.author || "")}</p>
        <p class="text-sm font-semibold text-amber-300">⚠️ Level ${d.prev_level} → Level ${d.proposed_level}</p>
      </div>
    </div>
    <details class="rounded-lg bg-slate-800/60 px-3 py-2 text-sm">
      <summary class="cursor-pointer py-1 text-slate-300">What the AI found (${notes.length} part${notes.length === 1 ? "" : "s"})</summary>
      <ul class="mt-1 list-disc space-y-1 pl-5 text-slate-300">${notes.map((n) => `<li><b>${esc(n.label)}</b> · Level ${n.level}: ${esc(n.note)}</li>`).join("")
        || "<li>No parts with sexual content were noted.</li>"}</ul>
      <p class="mt-2 text-xs text-slate-400">Every part listed passed a second check for sexual content on the page.</p>
    </details>
    <div class="grid grid-cols-2 gap-2">
      <button data-act="accept" class="btn-primary">Accept Level ${d.proposed_level}</button>
      <button data-act="keep" class="btn-secondary">Keep Level ${d.prev_level}</button>
    </div></li>`;
}

export function reviewHeader(n) {
  if (!n) return `<p class="card text-sm text-slate-300">✓ Nothing waits for your review.</p>`;
  return `<div class="space-y-2">
    <p class="text-sm text-slate-400">These scans would raise a book by 2 or more levels, so they don't apply until you decide.
      Scans made before NovelCheck 1.18.2 used an older Level 3, so check what the AI found.</p>
    <div class="flex flex-wrap gap-2">
      <button data-act="accept-all" class="btn-secondary py-1.5 text-sm">Accept all ${n}</button>
      <button data-act="keep-all" class="btn-ghost py-1.5 text-sm">Keep all old ratings</button>
    </div></div>`;
}

export function openRow(d) {
  const pct = d.parts_total ? Math.round((100 * d.parts_done) / d.parts_total) : 0;
  return `<li class="card space-y-2" data-scan="${d.id}" data-book="${d.book_id}">
    <div>${title(d)} <span class="text-sm text-slate-400">${esc(d.author || "")}</span></div>
    <p class="text-xs text-slate-400">${esc(SOURCE[d.source] || d.source)}${d.requested_by ? ` by ${esc(d.requested_by)}` : ""}
      · ${fmtNum(d.words)} words${d.reason ? ` · “${esc(d.reason)}”` : ""}</p>
    ${d.status === "reading" ? `<div class="space-y-1"><div class="meter-track bg-slate-800"><div class="meter-fill bg-indigo-500" data-pct="${pct}"></div></div>
      <p class="text-xs text-slate-400">📖 Part ${d.parts_done + 1} of ${d.parts_total} (${pct}%)</p></div>`
      : `<p class="text-xs text-slate-400">${d.status === "requested" ? "Waiting for your approval" : "Waiting to start"}</p>`}
    <div class="flex flex-wrap gap-2">
      ${d.status === "requested" ? `<button data-act="approve" class="btn-primary py-1.5 text-sm">Approve</button><button data-act="decline" class="btn-ghost py-1.5 text-sm">Decline</button>`
        : `<button data-act="cancel" class="btn-ghost py-1.5 text-sm">Cancel</button>`}
    </div></li>`;
}

export function resultRow(d) {
  const outcome = d.status === "error" ? `<p class="text-sm text-rose-300">Failed: ${esc(d.error)}</p>`
    : d.status === "done" ? deepChangeLine(d) || `<p class="text-sm text-slate-400">Level ${d.new_level}${d.prev_level === d.new_level ? " (unchanged)" : ""}</p>`
      : `<p class="text-sm text-slate-500">${d.status === "declined" ? (d.proposed_level != null ? `Kept Level ${d.prev_level} (the scan suggested ${d.proposed_level})` : "Declined") : "Cancelled"}</p>`;
  return `<li class="flex gap-3 border-t border-slate-800 py-3" data-scan="${d.id}" data-book="${d.book_id}">
    <div class="min-w-0 flex-1 space-y-0.5">${title(d, "text-sm font-semibold leading-snug")}
      ${outcome}
      <p class="text-xs text-slate-500">${esc(when(d.updated_at || d.created_at).toLocaleDateString())} · ${esc(SOURCE[d.source] || d.source)}</p></div></li>`;
}

// setMeters sizes progress bars (the CSP allows no inline styles in markup).
export function setMeters(host) {
  host.querySelectorAll("[data-pct]").forEach((el) => (el.style.width = `${el.dataset.pct}%`));
}
