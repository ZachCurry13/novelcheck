// 🧬 Deep Scan: the AI reads a whole book (EPUB) in parts. In the book
// window: status, notes per part (for parents, each scene and 📖 to read it
// in the book), and start / request buttons.
import { get, post } from "./api.js";
import { esc, attempt, toast, fmtNum, fmtMoney, fmtMinutes, canManage, can } from "./ui.js";
import { PEPPERS, levelBadge, levelTally, kindOf } from "./peppers.js";

const STATUS = {
  requested: "Deep Scan requested; waiting for an admin to approve it.",
  queued: "Waiting to be Deep Scanned.",
  declined: "An admin declined the Deep Scan request.",
  cancelled: "The Deep Scan was cancelled.",
};

// deepChip marks a book whose rating comes from reading the whole text.
export const deepChip = (b) => (b.analysis_model?.startsWith("deep: ")
  ? `<span class="chip-cat" title="The AI read the whole book">Deep Scan</span>` : "");

// when reads the database's timestamps ("2026-09-24 22:40:34" or RFC 3339).
export const when = (ts) => new Date(ts.includes("T") ? ts : ts.replace(" ", "T") + "Z");

// deepChangeLine renders "Level 2 → Level 4" for a finished scan that changed the rating.
export function deepChangeLine(d) {
  if (d?.held) {
    return `<p class="text-sm font-semibold text-amber-300">Deep Scan suggests Level ${d.proposed_level} (now Level ${d.prev_level}). An admin will review it before it applies.</p>`;
  }
  if (!d || d.status !== "done" || d.prev_level === null || d.new_level === null || d.new_level === d.prev_level) return "";
  const up = d.new_level > d.prev_level;
  return `<p class="text-sm font-semibold ${up ? "text-amber-300" : "text-emerald-300"}">${up ? "Rating changed via Deep Scan" : "✓ Deep Scan lowered the rating"}: Level ${d.prev_level} → Level ${d.new_level}</p>`;
}

// aboutText is what the AI saw in a part (its note, then the scene), as
// sentences. It names characters and says what happens, so it is shown only in
// the reader, when a parent opens that part of the book.
export const aboutText = (n) => [n.note, n.scene].filter(Boolean).map((s) => s.trim())
  .map((s) => (/[.!?…]$/.test(s) ? s : `${s}.`)).join(" ");

// whyHTML answers "why this level?" at the top, without spoilers: what the level means, how many parts
// reached it, and those parts by name with the kind of content. A book's level is the highest level found
// anywhere in it, so these parts are the reason. Parents open a part in the book to see what happens.
function whyHTML(notes, parents, rated = null) {
  const top = Math.max(...notes.map((n) => n.level));
  const those = notes.map((n, i) => ({ n, i })).filter((x) => x.n.level === top);
  const p = PEPPERS[top];
  return `<div class="card-outlined space-y-2 p-3">
    <p class="flex items-center gap-2 font-semibold">${levelBadge(top)}<span>${rated == null || rated === top ? "Why" : "The highest parts are"} Level ${top}${p ? `: ${esc(p.name)}` : ""}</span></p>
    ${rated != null && rated !== top ? `<p class="text-sm text-amber-200">The book itself is rated Level ${rated}: the scan's own checks, or a parent, settled on that after these parts were noted.</p>` : ""}
    ${p ? `<p class="text-sm text-slate-300">${esc(p.desc)}</p>` : ""}
    <p class="text-sm text-slate-300"><b>${those.length} of ${notes.length}</b> noted part${notes.length === 1 ? "" : "s"} reached Level ${top}. A book gets the highest level found anywhere in it, so ${those.length === 1 ? "this part is" : "these parts are"} the reason:</p>
    <ul class="space-y-1 text-sm">${those.map(({ n, i }) => `<li><span class="font-semibold">${esc(n.label)}</span>
      <span class="text-slate-400">· ${esc(kindOf(n.level))}</span>
      ${parents ? `<button type="button" data-read="${i}" class="ml-1 text-xs font-semibold text-sky-300 underline">Read it</button>` : ""}</li>`).join("")}</ul>
</div>`;
}

// foundHTML lists the parts a scan noted: first why the book got its level (whyHTML), then every part, where
// and how high, without what happens (no spoilers). Parents get 📖 to read the part in the book, which
// also shows what the AI saw there.
function foundHTML(notes, parents, rated = null) {
  const list = `${notes.length > 3 ? levelTally(notes) : ""}<ul class="mt-2 space-y-2 text-sm">${notes.map((n, i) => `<li class="flex items-start gap-3${parents ? " rounded-md bg-slate-900/60 p-2" : ""}">
    ${levelBadge(n.level)}
    <div class="min-w-0 flex-1"><p class="font-semibold leading-snug">${esc(n.label)}</p>
      <p class="text-xs text-slate-400">Level ${n.level}${PEPPERS[n.level] ? ` · ${esc(PEPPERS[n.level].name)}` : ""} · ${esc(kindOf(n.level))}</p>
      ${parents ? `<button type="button" data-read="${i}" class="mt-1 block text-left text-xs font-semibold text-sky-300 underline">Read this part in the book</button>` : ""}</div></li>`).join("")}</ul>`;
  if (!parents) return `${whyHTML(notes, false, rated)}${list}`;
  return `${whyHTML(notes, true, rated)}<details class="rounded-lg bg-slate-900/40 px-3 py-2"><summary class="cursor-pointer py-1 text-sm text-slate-300">
    Every part the AI noted (${notes.length})</summary><div class="mt-1">${list}</div></details>`;
}

// renderDeepSection fills host (in the book window) and wires its buttons.
export async function renderDeepSection(host, bookId, user, title = "") {
  const info = await get(`/api/books/${bookId}/deep-scan`).catch(() => null);
  if (!info) return;
  const d = info.latest;
  const open = d && ["requested", "queued", "reading"].includes(d.status);
  const admin = can(user, "deep"); // starts and stops scans; anyone else asks
  let body = "";
  if (d?.status === "reading") {
    body = `<p class="text-sm">Reading part ${d.parts_done + 1} of ${d.parts_total}…</p>`;
  } else if (d?.status === "done") {
    const notes = JSON.parse(d.notes || "[]");
    body = `<p class="text-sm text-emerald-300">Deep Scanned: the whole book was read${d.model ? ` by ${esc(d.model)}` : ""} (${esc(when(d.updated_at).toLocaleDateString())}).</p>
      ${deepChangeLine(d)}
      ${notes.length ? foundHTML(notes, canManage(user), d.held ? d.proposed_level : d.new_level)
        : `<p class="text-sm text-slate-400">No romance or flagged content was noted anywhere in the text.</p>`}`;
  } else if (d?.status === "error") {
    body = `<p class="text-sm text-rose-300">The last Deep Scan failed: ${esc(d.error)}</p>`;
  } else if (d && STATUS[d.status]) {
    body = `<p class="text-sm text-slate-300">${STATUS[d.status]}</p>`;
  }
  let action = "";
  if (!open && info.available) {
    const e = info.estimate;
    const cost = `${fmtNum(e.tokens)} tokens${info.cost ? `, about ${fmtMoney(info.cost)}` : ""}${info.minutes ? `, ${fmtMinutes(info.minutes)}` : ""}`;
    action = admin
      ? `<button data-deep="start" class="btn-secondary" title="Reads the whole book: ${esc(cost)}">${d?.status === "done" ? "Deep Scan again" : "Deep Scan this book"}</button>
         <span class="text-xs text-slate-500">${fmtNum(e.words)} words in ${e.parts} parts · ${esc(cost)}</span>`
      : `<button data-deep="request" class="btn-secondary">Request a Deep Scan</button>`;
  } else if (!info.available && !d && admin) {
    action = `<p class="text-xs text-slate-500">${esc(info.why || "")}</p>`;
  }
  if (!body && !action) return;
  host.innerHTML = `<div class="space-y-2 rounded-lg bg-slate-800/60 p-3"><p class="label">Deep Scan</p>${body}
    ${admin && d?.status === "reading" ? `<button data-deep="cancel" data-id="${d.id}" class="btn-ghost text-xs">Stop this Deep Scan</button>` : ""}
    <div class="flex flex-wrap items-center gap-2">${action}</div></div>`;
  host.onclick = async (e) => {
    const read = e.target.closest("[data-read]");
    if (read && d) {
      const n = JSON.parse(d.notes || "[]")[Number(read.dataset.read)];
      if (!n) return;
      return import("./deepreader.js").then((m) => m.openPassage(d.id, title,
        { from: n.from || 0, to: n.to || 0, label: n.label, about: aboutText(n) }));
    }
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
