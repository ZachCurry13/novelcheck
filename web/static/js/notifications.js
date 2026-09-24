// Bell icon for admins and editors: problems NovelCheck noticed on its own
// (from the server), plus messages shown on this device (from toasts).
import { copyText } from "./copy.js";
import { diagnoseLater } from "./diagnose.js";
import { get, post, api } from "./api.js";
import { $, esc, messageLog, canManage } from "./ui.js";

const LEVEL = {
  error: ["⛔", "Problem", "text-rose-300"],
  warning: ["⚠️", "Warning", "text-amber-300"],
  info: ["ℹ️", "Info", "text-sky-300"],
};

let data = { unread: 0, items: [] };
let timer = null;

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

function paintPanel() {
  const panel = $("#bell-panel");
  const items = data.items.map((n) => {
    const [icon, label, cls] = LEVEL[n.level] || LEVEL.warning;
    return `<li class="flex gap-3 border-t border-slate-800 py-3 ${n.read ? "opacity-60" : ""}">
      <span aria-hidden="true">${icon}</span>
      <div class="min-w-0 flex-1 text-sm">
        <p><span class="${cls} font-semibold">${label}:</span> ${esc(n.message)}${n.count > 1 ? ` <span class="text-slate-500">(×${n.count})</span>` : ""}</p>
        <p class="text-xs text-slate-500">${esc(when(n.updated_at))}
          ${n.link ? ` · <a href="${esc(n.link)}" data-go class="underline">Fix it</a>` : ""}
          ${n.read ? "" : ` · <button data-read="${n.id}" class="underline">Mark read</button>`}
          · <button data-copy-msg="${esc(n.message)}" class="underline">📋 Copy</button>
          <span class="admin-link"> · <button data-diagnose="${esc(n.message)}" class="underline">🩺 Diagnose</button></span></p>
      </div>
    </li>`;
  }).join("");
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
        <ul>${items || `<li class="py-2 text-sm text-slate-400">✓ No problems reported.</li>`}</ul>
      </section>
      <section>
        <p class="label">Messages on this device</p>
        <ul>${local || `<li class="py-2 text-sm text-slate-400">None yet.</li>`}</ul>
      </section>
    </div>`;
  messageLog.forEach((m) => (m.seen = true));
  paintBadge();
}

export function initBell(state) {
  const bell = $("#bell-btn");
  const show = canManage(state.user);
  bell.classList.toggle("hidden", !show);
  clearInterval(timer);
  if (!show) return;
  const panel = $("#bell-panel");
  bell.onclick = async () => {
    await refresh();
    paintPanel();
    panel.querySelectorAll(".admin-link").forEach((a) => a.classList.toggle("hidden", state.user.role !== "admin"));
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
    if (e.target === panel || e.target.closest("[data-close]") || e.target.closest("[data-go]")) return panel.close();
    const read = e.target.closest("[data-read]");
    if (read) data = await post("/api/notifications/read", { id: Number(read.dataset.read) });
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
