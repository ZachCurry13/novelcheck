// Bell icon for admins and editors: problems NovelCheck noticed on its own
// (from the server), plus messages shown on this device (from toasts).
import { copyText } from "./copy.js";
import { diagnoseLater } from "./diagnose.js";
import { get, post, api } from "./api.js";
import { $, esc, messageLog, canManage, can } from "./ui.js";

const LEVEL = {
  error: ["⛔", "Problem", "text-rose-300"],
  warning: ["⚠️", "Warning", "text-amber-300"],
  info: ["ℹ️", "Info", "text-sky-300"],
};

let data = { unread: 0, items: [] };
let timer = null;
let admin = false; // admins also get Diagnose and "Check everything"

function when(ts) {
  const d = new Date(ts.includes("T") ? ts : ts.replace(" ", "T") + "Z");
  return isNaN(d) ? "" : d.toLocaleString();
}

function paintBadge() {
  const badge = $("#bell-badge");
  const errorsHere = messageLog.filter((m) => m.isError && !m.seen).length;
  const n = data.unread + errorsHere;
  badge.textContent = n > 99 ? "99+" : String(n);
  badge.classList.toggle("hidden", n === 0);
}

async function refresh() {
  try {
    data = await get("/api/notifications");
  } catch {
    /* keep showing the last list */
  }
  paintBadge();
  if ($("#bell-panel").open) paintPanel();
}

// Deep Scans that wait for a decision: one card for all of them.
const isReview = (n) => n.source === "deep-scan-review" || (n.source === "deep-scan" && n.message.includes("suggests raising"));

// What the notice's button says, by where it leads.
function actionLabel(n) {
  if (isReview(n)) return "Review";
  if (n.link.startsWith("#/deepscan")) return "Open Deep Scan";
  if (n.link.startsWith("#/system")) return "Open System";
  if (n.link.startsWith("#/admin")) return "Open settings";
  return "Open";
}

function noticeCard(n) {
  const [icon, label, cls] = LEVEL[n.level] || LEVEL.warning;
  return `<li class="rounded-xl bg-slate-800/60 p-3 ${n.read ? "opacity-60" : ""}">
    <div class="flex gap-3">
      <span aria-hidden="true">${icon}</span>
      <div class="min-w-0 flex-1 space-y-1 text-sm">
        <p><span class="${cls} font-semibold">${label}:</span> ${esc(n.message)}${n.count > 1 ? ` <span class="text-slate-500">(×${n.count})</span>` : ""}</p>
        <p class="text-xs text-slate-500">${esc(when(n.updated_at))}</p>
      </div>
      <button data-dismiss="${n.id}" class="btn-ghost -mr-1 -mt-1 h-9 w-9 shrink-0 p-0" aria-label="Dismiss" title="Dismiss">✕</button>
    </div>
    <div class="mt-2 flex flex-wrap gap-2 pl-8">
      ${n.link ? `<a href="${esc(n.link)}" data-go data-read-on-go="${n.id}" class="btn-secondary text-sm">${actionLabel(n)}</a>` : ""}
      ${n.level === "error" ? `<button data-copy-msg="${esc(n.message)}" class="btn-ghost text-sm">📋 Copy</button>
        <button data-diagnose="${esc(n.message)}" class="btn-ghost admin-link text-sm">🩺 Diagnose</button>` : ""}
    </div>
  </li>`;
}

function reviewGroup(list) {
  const titles = list.map((n) => (n.message.match(/“([^”]+)”/) || [])[1]).filter(Boolean);
  return `<li class="rounded-xl bg-amber-950/40 p-3 ring-1 ring-amber-900/60">
    <div class="flex gap-3">
      <span aria-hidden="true">🧬</span>
      <div class="min-w-0 flex-1 space-y-1 text-sm">
        <p class="font-semibold text-amber-200">${list.length} Deep Scan${list.length === 1 ? " waits" : "s wait"} for your review</p>
        <p class="text-xs text-slate-400">Each would raise a book's rating by 2 or more levels.</p>
      </div>
      <button data-dismiss="${list.map((n) => n.id).join(",")}" class="btn-ghost -mr-1 -mt-1 h-9 w-9 shrink-0 p-0" aria-label="Dismiss" title="Dismiss these notices (the scans still wait on the Deep Scan page)">✕</button>
    </div>
    <div class="mt-2 space-y-2 pl-8">
      <a href="#/deepscan?review" data-go class="btn-primary text-sm">Review</a>
      <details class="text-xs text-slate-400"><summary class="cursor-pointer py-1">Show the books</summary>
        <ul class="list-disc pl-5">${titles.map((t) => `<li>${esc(t)}</li>`).join("")}</ul></details>
    </div>
  </li>`;
}

function paintPanel() {
  const panel = $("#bell-panel");
  const reviews = data.items.filter((n) => isReview(n) && !n.read);
  const rest = data.items.filter((n) => !isReview(n));
  const items = (reviews.length ? reviewGroup(reviews) : "") + rest.map(noticeCard).join("");
  const local = messageLog.map((m) => `<li class="border-t border-slate-800 py-2 text-sm ${m.isError ? "text-rose-300" : "text-slate-300"}">
      ${m.isError ? "⛔ " : ""}${esc(m.text)} <span class="text-xs text-slate-500">· ${esc(m.at.toLocaleTimeString())}</span></li>`).join("");
  panel.innerHTML = `
    <div class="max-h-[80vh] space-y-4 overflow-y-auto p-5">
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-lg font-bold">Notifications</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button>
      </div>
      <div class="flex flex-wrap gap-2">
        <button data-all class="btn-secondary py-1 text-xs" ${data.unread ? "" : "disabled"}>Mark all read</button>
        <button data-clear class="btn-ghost py-1 text-xs" ${data.items.length ? "" : "disabled"}>Clear all</button>
        <a href="#/system" data-go class="btn-ghost py-1 text-xs admin-link">Check everything →</a>
        <button data-copy-all class="btn-ghost py-1 text-xs">📋 Copy all</button>
      </div>
      <section>
        <p class="label">From NovelCheck</p>
        <ul class="space-y-2">${items || `<li class="py-2 text-sm text-slate-400">✓ No problems reported.</li>`}</ul>
      </section>
      <section>
        <p class="label">Messages on this device</p>
        <ul>${local || `<li class="py-2 text-sm text-slate-400">None yet.</li>`}</ul>
      </section>
    </div>`;
  panel.querySelectorAll(".admin-link").forEach((a) => a.classList.toggle("hidden", !admin));
  messageLog.forEach((m) => (m.seen = true));
  paintBadge();
}

export function initBell(state) {
  const bell = $("#bell-btn");
  const show = canManage(state.user);
  admin = can(state.user, "system"); // Diagnose and "Check everything" need System
  bell.classList.toggle("hidden", !show);
  clearInterval(timer);
  if (!show) return;
  const panel = $("#bell-panel");
  bell.onclick = async () => {
    await refresh();
    paintPanel();
    panel.showModal();
  };
  panel.onclick = async (e) => {
    const cm = e.target.closest("[data-copy-msg]");
    if (cm) return copyText(cm.dataset.copyMsg);
    if (e.target.closest("[data-copy-all]")) {
      return copyText([...data.items.map((n) => `[${n.updated_at}] ${n.level} ${n.source}: ${n.message}${n.count > 1 ? ` (x${n.count})` : ""}`),
        ...messageLog.map((m) => `[${m.at.toISOString()}] this device: ${m.text}`)].join("\n") || "No messages.");
    }
    const dx = e.target.closest("[data-diagnose]");
    if (dx) {
      panel.close();
      return diagnoseLater(dx.dataset.diagnose);
    }
    const go = e.target.closest("[data-go]");
    if (go?.dataset.readOnGo) post("/api/notifications/read", { id: Number(go.dataset.readOnGo) }).then((d) => { data = d; paintBadge(); }).catch(() => {});
    if (e.target === panel || e.target.closest("[data-close]") || go) return panel.close();
    const dismiss = e.target.closest("[data-dismiss]");
    if (dismiss) data = await post("/api/notifications/dismiss", { ids: dismiss.dataset.dismiss.split(",").map(Number) });
    else if (e.target.closest("[data-all]")) data = await post("/api/notifications/read", {});
    else if (e.target.closest("[data-clear]")) {
      if (!confirm("Delete all notifications?")) return;
      data = await api("/api/notifications", { method: "DELETE" });
      messageLog.length = 0;
    } else return;
    paintPanel();
  };
  window.addEventListener("nc:message", paintBadge);
  refresh();
  timer = setInterval(refresh, 60000);
  // Also check when moving between pages, so new items show up promptly.
  window.addEventListener("hashchange", refresh);
}
