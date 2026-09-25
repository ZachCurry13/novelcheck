// 🧬 Deep Scan: the AI reads a whole book (EPUB) in parts. In the book
// window: status, notes per part, and start / request buttons.
import { get, post } from "./api.js";
import { esc, attempt, toast, fmtNum, fmtMoney } from "./ui.js";

const STATUS = {
  requested: "🕓 Deep Scan requested; waiting for an admin to approve it.",
  queued: "🕓 Waiting to be Deep Scanned.",
  declined: "An admin declined the Deep Scan request.",
  cancelled: "The Deep Scan was cancelled.",
};

// deepChip marks a book whose rating comes from reading the whole text.
export const deepChip = (b) => (b.analysis_model?.startsWith("deep: ")
  ? `<span class="chip-cat" title="The AI read the whole book">🧬 Deep Scan</span>` : "");

// when reads the database's timestamps ("2026-09-24 22:40:34" or RFC 3339).
export const when = (ts) => new Date(ts.includes("T") ? ts : ts.replace(" ", "T") + "Z");

// deepChangeLine renders "Level 2 → Level 4" for a finished scan that changed the rating.
export function deepChangeLine(d) {
  if (d?.held) {
    return `<p class="text-sm font-semibold text-amber-300">🧬 Deep Scan suggests Level ${d.proposed_level} (now Level ${d.prev_level}). An admin will review it before it applies.</p>`;
  }
  if (!d || d.status !== "done" || d.prev_level === null || d.new_level === null || d.new_level === d.prev_level) return "";
  const up = d.new_level > d.prev_level;
  return `<p class="text-sm font-semibold ${up ? "text-amber-300" : "text-emerald-300"}">${up ? "⚠️ Rating changed via Deep Scan" : "✓ Deep Scan lowered the rating"}: Level ${d.prev_level} → Level ${d.new_level}</p>`;
}

// renderDeepSection fills host (in the book window) and wires its buttons.
export async function renderDeepSection(host, bookId, user) {
  const info = await get(`/api/books/${bookId}/deep-scan`).catch(() => null);
  if (!info) return;
  const d = info.latest;
  const open = d && ["requested", "queued", "reading"].includes(d.status);
  const admin = user.role === "admin";
  let body = "";
  if (d?.status === "reading") {
    body = `<p class="text-sm">📖 Reading part ${d.parts_done + 1} of ${d.parts_total}…</p>`;
  } else if (d?.status === "done") {
    const notes = JSON.parse(d.notes || "[]");
    body = `<p class="text-sm text-emerald-300">🧬 Deep Scanned: the whole book was read${d.model ? ` by ${esc(d.model)}` : ""} (${esc(when(d.updated_at).toLocaleDateString())}).</p>
      ${deepChangeLine(d)}
      ${notes.length ? `<ul class="space-y-1 text-sm">${notes.map((n) => `<li><b>${esc(n.label)}</b> · Level ${n.level}${n.note ? `: ${esc(n.note)}` : ""}</li>`).join("")}</ul>`
        : `<p class="text-sm text-slate-400">No romance or flagged content was noted anywhere in the text.</p>`}`;
  } else if (d?.status === "error") {
    body = `<p class="text-sm text-rose-300">The last Deep Scan failed: ${esc(d.error)}</p>`;
  } else if (d && STATUS[d.status]) {
    body = `<p class="text-sm text-slate-300">${STATUS[d.status]}</p>`;
  }
  let action = "";
  if (!open && info.available) {
    const e = info.estimate;
    const cost = `${fmtNum(e.tokens)} tokens${info.cost ? `, about ${fmtMoney(info.cost)}` : ""}`;
    action = admin
      ? `<button data-deep="start" class="btn-secondary" title="Reads the whole book: ${esc(cost)}">🧬 ${d?.status === "done" ? "Deep Scan again" : "Deep Scan this book"}</button>
         <span class="text-xs text-slate-500">${fmtNum(e.words)} words in ${e.parts} parts · ${esc(cost)}</span>`
      : `<button data-deep="request" class="btn-secondary">🧬 Request a Deep Scan</button>`;
  } else if (!info.available && !d && admin) {
    action = `<p class="text-xs text-slate-500">${esc(info.why || "")}</p>`;
  }
  if (!body && !action) return;
  host.innerHTML = `<div class="space-y-2 rounded-lg bg-slate-800/60 p-3"><p class="label">🧬 Deep Scan</p>${body}
    ${admin && d?.status === "reading" ? `<button data-deep="cancel" data-id="${d.id}" class="btn-ghost text-xs">Stop this Deep Scan</button>` : ""}
    <div class="flex flex-wrap items-center gap-2">${action}</div></div>`;
  host.onclick = async (e) => {
    const act = e.target.closest("[data-deep]")?.dataset.deep;
    if (act === "start") {
      if (!confirm(`Deep Scan this book? The AI reads all ${fmtNum(info.estimate.words)} words (${fmtNum(info.estimate.tokens)} tokens${info.cost ? `, about ${fmtMoney(info.cost)}` : ""}).`)) return;
      if (await attempt(() => post(`/api/books/${bookId}/deep-scan`), "Deep Scan started. The rating updates when it's done.")) renderDeepSection(host, bookId, user);
    } else if (act === "request") {
      const reason = prompt("Why should the whole book be read? (optional)\nAn admin will review the request.", "");
      if (reason === null) return;
      if (await attempt(() => post(`/api/books/${bookId}/deep-scan`, { reason }), "Deep Scan requested. An admin will review it.")) renderDeepSection(host, bookId, user);
    } else if (act === "cancel") {
      if (await attempt(() => post(`/api/admin/deep-scans/${e.target.dataset.id}/cancel`))) {
        toast("Deep Scan stopped");
        renderDeepSection(host, bookId, user);
      }
    }
  };
}
