// Admin control panel in four tabs: AI & Scans, Users & Rules, Delivery &
// Services, System & Toggles. Editors see the first two, without technical
// settings, secrets, backups or the destructive queue wipe.
import { get, post, qs } from "./api.js";
import { $, $$, esc, attempt, toast, fmtNum, fmtMoney } from "./ui.js";
import { renderUsers } from "./users.js";
import { renderCalibrePicker } from "./calibrepicker.js";
import { renderRemoteAccess } from "./remoteaccess.js";
import { renderCalibreServer } from "./calibreserver.js";
import { renderErrors } from "./errors.js";
import { on } from "./modules.js";
import { renderFlagsAdmin } from "./customflags.js";
import { renderSettingsTab } from "./adminsettings.js";
import { renderStats, perBookCost } from "./adminstats.js";

const TABS = [["ai", "🤖 AI & Scans"], ["users", "👪 Users & Rules"], ["delivery", "📬 Delivery & Services"], ["system", "⚙️ System & Toggles"]];

// The open tab lives in the address (#/admin?tab=users) so a reload keeps it.
const tabFromHash = () => new URLSearchParams(location.hash.split("?")[1] || "").get("tab");

export async function renderAdmin(view, state) {
  const isAdmin = state.user.role === "admin";
  const adminOnly = (html) => (isAdmin ? html : "");
  const tabs = isAdmin ? TABS : TABS.slice(0, 2);
  let current = tabs.some(([k]) => k === tabFromHash()) ? tabFromHash() : "ai";
  view.innerHTML = `
    <h1 class="mb-3 text-2xl font-bold">${isAdmin ? "Admin Control Panel" : "Manage NovelCheck"}</h1>
    <div id="del-banner" class="mb-2 hidden rounded-lg bg-rose-950/50 p-3 text-sm"></div>
    <div id="deep-banner" class="mb-2 hidden rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    <div id="titles-banner" class="mb-2 hidden rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    <nav id="admin-tabs" class="mb-4 flex gap-1 overflow-x-auto border-b border-slate-800">${tabs.map(([k, l]) =>
      `<button data-tab="${k}" class="nav-link rounded-b-none">${l}</button>`).join("")}</nav>
    <section data-panel="ai" class="space-y-6">
      <div id="stats" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4"></div>
      <div id="errors"></div>
      <div class="card flex flex-wrap items-end gap-3">
        <div><label class="label" for="batch-size">Batch size</label>
          <input id="batch-size" type="number" min="0" max="500" value="20" class="input w-28" title="0 = all waiting books (up to 500)"></div>
        <button data-act="batch" class="btn-primary">Analyze batch</button>
        <button data-act="sync" class="btn-secondary">Sync Calibre now</button>
        ${adminOnly(`<a href="#/deepscan" class="btn-secondary">🧬 Deep Scan…</a>`)}
        <button data-act="rerate-all" class="btn-ghost" title="Rate every AI-rated book again, e.g. after changing the AI or its rules">Re-rate whole library…</button>
        ${adminOnly(`<button data-act="wipe" class="btn-ghost">Wipe pending queue</button>`)}
        <p id="worker" class="basis-full text-sm text-slate-400"></p>
        <div id="rerate" class="hidden basis-full rounded-lg bg-slate-800/60 p-3 text-sm"></div>
      </div>
      ${adminOnly(`<div id="custom-flags"></div><div id="settings-ai"></div>`)}
    </section>
    <section data-panel="users"><div id="users"></div></section>
    ${adminOnly(`<section data-panel="delivery" class="space-y-4">
      <div id="settings-delivery"></div>
      <div class="card space-y-1 text-sm${on(state.user, "koreader") ? "" : " module-off"}" data-module="koreader">
        <p class="font-semibold">📖 KOReader</p>
        <p class="text-slate-400">Nothing to set up here: each reader finds their private catalog address and QR code under
          <b>Profile → KOReader setup</b>. For kids' e-readers, use the <b>📖 KOReader</b> button on their card in <b>Users &amp; Rules</b>.</p></div>
      <div id="remote-access"></div>
    </section>
    <section data-panel="system" class="space-y-4">
      <div id="settings-system"></div>
      <div class="card flex flex-wrap gap-2">
        <a href="/api/admin/backup" class="btn-secondary" download>Download novelcheck.db (backup)</a>
        <a href="#/system" class="btn-ghost">🩺 System checks</a><a href="#/usage" class="btn-ghost">📈 Usage</a>
      </div>
    </section>`)}`;

  const show = (tab) => {
    current = tab;
    $$("[data-panel]", view).forEach((p) => p.classList.toggle("hidden", p.dataset.panel !== tab));
    $$("[data-tab]", view).forEach((b) => b.classList.toggle("active", b.dataset.tab === tab));
    history.replaceState(null, "", `#/admin?tab=${tab}`);
  };
  show(current);
  $("#admin-tabs", view).addEventListener("click", (e) => {
    const t = e.target.closest("[data-tab]")?.dataset.tab;
    if (t) show(t);
  });

  view.addEventListener("click", async (e) => {
    const act = e.target.closest("[data-act]")?.dataset.act;
    if (!act) return;
    if (act === "tidy-titles") {
      import("./titlefix.js").then((m) => m.openTitleFixes());
    } else if (act === "batch") {
      const r = await attempt(() => post("/api/admin/analyze-batch" + qs({ size: $("#batch-size", view).value })));
      if (r) toast(`Queued ${r.queued} books`);
    } else if (act === "wipe") {
      if (!confirm("Return all queued books to Pending Analysis?")) return;
      const r = await attempt(() => post("/api/admin/wipe-queue"));
      if (r) toast(`Reset ${r.reset} books to pending`);
    } else if (act === "rerate-language") {
      const r = await attempt(() => post("/api/admin/rerate", { which: "language" }));
      if (r) toast(`Re-rating ${r.queued} books so their summaries are in English`);
    } else if (act === "rerate") {
      const r = await attempt(() => post("/api/admin/rerate"));
      if (r) toast(`Re-rating ${r.queued} books with the current pepper rules`);
    } else if (act === "rerate-all") {
      const s = await attempt(() => get("/api/admin/status"));
      if (!s) return;
      const est = perBookCost(s) * s.ai_rated;
      if (!confirm(`Re-rate all ${fmtNum(s.ai_rated)} AI-rated books?${est ? ` Estimated cost ≈ ${fmtMoney(est)}.` : ""}\n\nThey stay in the library with their current rating until the new one arrives. Hand-rated books are left alone.`)) return;
      const r = await attempt(() => post("/api/admin/rerate", { which: "all" }));
      if (r) toast(`Re-rating ${r.queued} books`);
    } else if (act === "sync") {
      await attempt(() => post("/api/admin/calibre-sync"), "Calibre sync started");
    } else if (act === "smtp-test") {
      const to = prompt("Send test email to:");
      const val = (k) => $(`[data-key="${k}"]`, view)?.value ?? ""; // test what's on screen, even before Save
      if (to) {
        await attempt(() => post("/api/admin/smtp-test", {
          to, smtp_host: val("smtp_host"), smtp_port: val("smtp_port"), smtp_username: val("smtp_username"),
          smtp_password: val("smtp_password"), smtp_from: val("smtp_from"),
        }), "Test email sent. If it looks right, click Save settings.");
      }
    }
    refresh();
  });

  async function refresh() {
    const s = await attempt(() => get("/api/admin/status"));
    if (s) renderStats(view, s);
  }
  await refresh();
  renderErrors($("#errors", view), isAdmin, refresh);
  let stopRemote = null;
  if (isAdmin) {
    const settings = (await attempt(() => get("/api/admin/settings"))) || {};
    $("#batch-size", view).value = settings.batch_size ?? 20;
    for (const tab of ["ai", "delivery", "system"]) renderSettingsTab($(`#settings-${tab}`, view), tab, settings, state.user);
    renderFlagsAdmin($("#custom-flags", view), refresh);
    renderCalibrePicker($("#calibre-picker", view), refresh);
    renderCalibreServer($("#calibre-server", view));
    stopRemote = await renderRemoteAccess($("#remote-access", view));
  }
  const timer = setInterval(refresh, 5000);
  await renderUsers($("#users", view), state.user);
  return () => {
    clearInterval(timer);
    stopRemote?.();
  };
}
