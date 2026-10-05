// The lists on Admin → 🧬 Deep Scan: scans waiting for review (Accept / Keep
// in place, without reloading the page), scans waiting or running (with
// progress), and recent results. Titles and covers open the book's window.
import { esc, fmtNum } from "./ui.js";
import { coverImg } from "./covers.js";
import { deepChangeLine, when, aboutText } from "./deepscan.js";

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
  const explicit = notes.filter((n) => n.level >= 4).length;
  const levels = [0, 1, 2, 3, 4, 5].map((l) => `<option value="${l}" ${l === d.proposed_level ? "selected" : ""}>Level ${l}</option>`).join("");
  return `<li class="card space-y-3" data-scan="${d.id}" data-book="${d.book_id}">
    <div class="flex gap-3">
      <button type="button" data-open class="shrink-0" title="Book details">${coverImg(d.book_id, "h-20 w-14")}</button>
      <div class="min-w-0 flex-1 space-y-1">
        ${title(d)}
        <p class="truncate text-sm text-slate-400">${esc(d.author || "")}</p>
        <p class="text-sm font-semibold text-amber-300">⚠️ Level ${d.prev_level ?? "–"} → Level ${d.proposed_level}</p>
      </div>
    </div>
    ${d.proposed_level >= 4 && explicit === 1 ? `<p class="rounded-lg bg-amber-950/40 p-2 text-sm text-amber-200">This rests on one passage. Read it and decide; the book gets a note for parents either way.</p>` : ""}
    <details class="rounded-lg bg-slate-800/60 px-3 py-2 text-sm" ${explicit === 1 ? "open" : ""}>
      <summary class="cursor-pointer py-1 text-slate-300">What the AI found (${notes.length} part${notes.length === 1 ? "" : "s"})</summary>
      <ul class="mt-1 space-y-2 text-slate-300">${notes.map((n) => `<li class="rounded-md bg-slate-900/60 p-2">
        <p><b>${esc(n.label)}</b> · Level ${n.level}</p>
        ${n.note || n.scene ? `<details class="mt-1 text-xs text-slate-400"><summary class="cursor-pointer">What happens (spoilers)</summary>
          <p class="mt-1">${esc(aboutText(n))}</p></details>` : ""}
        <button type="button" data-read data-from="${n.from || 0}" data-to="${n.to || 0}" data-label="${esc(n.label)}"
          data-about="${esc(aboutText(n))}"
          class="mt-1 text-xs font-semibold text-sky-300 underline">📖 Read this part in the book</button></li>`).join("")
        || "<li>No parts with sexual content were noted.</li>"}</ul>
      <p class="mt-2 text-xs text-slate-400">A part counts as Level 4 or more only when the AI names sentences that really are in the book and a second question agrees.</p>
    </details>
    <div class="grid grid-cols-2 gap-2">
      <button data-act="accept" class="btn-primary">Accept Level ${d.proposed_level}</button>
      <button data-act="keep" class="btn-secondary">${d.prev_level === null || d.prev_level === undefined ? "Keep the old rating" : `Keep Level ${d.prev_level}`}</button>
    </div>
    <div class="flex flex-wrap items-center gap-2 text-sm text-slate-400">Or set
      <select data-level class="input w-auto py-1 text-sm">${levels}</select>
      <button data-act="set" class="btn-ghost py-1 text-sm">Set this level</button></div></li>`;
}

export function reviewHeader(n) {
  if (!n) return `<p class="card text-sm text-slate-300">✓ Nothing waits for your review.</p>`;
  return `<div class="space-y-2">
    <p class="text-sm text-slate-400">These scans would raise a book by 2 or more levels, or to Level 4 or more on one passage, so they don't apply until you decide.
      Tap 📖 to read a part in the book; the right level can also be one in between (Or set…). Each decision leaves a note for parents on the book.</p>
    <div class="flex flex-wrap gap-2">
      <button data-act="accept-all" class="btn-secondary py-1.5 text-sm">Accept all ${n}</button>
      <button data-act="keep-all" class="btn-ghost py-1.5 text-sm">Keep all old ratings</button>
    </div></div>`;
}

// openRow is a scan waiting or running; place is its place in line (1 =
// next), and waiting ones can be dragged to change the order.
export function openRow(d, place) {
  const pct = d.parts_total ? Math.round((100 * d.parts_done) / d.parts_total) : 0;
  const waiting = d.status !== "reading";
  return `<li class="card space-y-2" data-scan="${d.id}" data-book="${d.book_id}" ${waiting ? "data-sortable" : ""}>
    <div class="flex items-start gap-2">${waiting ? `<span class="drag-handle -ml-2" title="Drag to change the order" aria-label="Drag to change the order">⠿</span>` : ""}
      <div class="min-w-0 flex-1"><span class="text-xs font-semibold ${waiting ? "text-slate-400" : "text-indigo-300"}">${waiting ? (place === 1 ? "Next" : `#${place} in line`) : "Reading now"}</span><br>
      ${title(d)} <span class="text-sm text-slate-400">${esc(d.author || "")}</span></div></div>
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
