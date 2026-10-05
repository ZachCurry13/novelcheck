// Admin control panel in four tabs: AI & Scans, Users & Rules, Delivery &
// Services, System & Toggles. Editors see the first two, without technical
// settings, secrets, backups or the destructive queue wipe. Admins other than
// the main admin see what their areas cover (the main admin gives areas).
import { get, post, qs } from "./api.js";
import { $, $$, esc, attempt, toast, fmtNum, fmtMoney, can } from "./ui.js";
import { renderUsers } from "./users.js";
import { renderCalibrePicker } from "./calibrepicker.js";
import { renderRemoteAccess } from "./remoteaccess.js";
import { renderCalibreServer } from "./calibreserver.js";
import { renderErrors } from "./errors.js";
import { on } from "./modules.js";
import { renderFlagsAdmin } from "./customflags.js";
import { renderSettingsTab } from "./adminsettings.js";
import { renderStats, perBookCost } from "./adminstats.js";
import { adminNavHTML, markAdminSection, tabsFor } from "./adminnav.js";
import { AREA_NAMES } from "./adminaccess.js";

const TABS = [["ai", "🤖 AI & Scans"], ["users", "👪 Users & Rules"], ["delivery", "📬 Delivery & Services"], ["system", "⚙️ System & Toggles"]];

// The open tab lives in the address (#/admin?tab=users) so a reload keeps it.
const tabFromHash = () => new URLSearchParams(location.hash.split("?")[1] || "").get("tab");

export async function renderAdmin(view, state) {
  const u = state.user;
  const isAdmin = u.role === "admin";
  const inArea = (area, html) => (can(u, area) ? html : "");
  const tabs = TABS.filter(([k]) => tabsFor().some(([t]) => t === k));
  const hasTab = (k) => tabs.some(([t]) => t === k);
  const access = isAdmin && !u.owner ? `<p class="mb-3 text-sm text-slate-400">Your access: ${(u.areas || []).map((a) => esc(AREA_NAMES[a] || a)).join(" · ") || "kids' accounts only"}.
    The main admin can change it.</p>` : "";
  let current = tabs.some(([k]) => k === tabFromHash()) ? tabFromHash() : "ai";
  view.innerHTML = `
    <h1 class="mb-3 text-2xl font-bold">${isAdmin ? "Admin Control Panel" : "Manage NovelCheck"}</h1>
    ${access}
    <div id="del-banner" class="mb-2 hidden rounded-lg bg-rose-950/50 p-3 text-sm"></div>
    <div id="deep-banner" class="mb-2 hidden rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    <div id="titles-banner" class="mb-2 hidden rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    <div id="covers-banner" class="mb-2 hidden rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    <div id="box-banner" class="mb-2 hidden rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    <div id="problems-banner" class="mb-2 hidden rounded-lg bg-slate-800/60 p-3 text-sm"></div>
    ${adminNavHTML(current)}
    <section data-panel="ai" class="space-y-6">
      <div id="stats" class="grid grid-cols-2 gap-3 lg:grid-cols-4"></div>
      <div id="errors"></div>
      <div class="card flex flex-wrap items-end gap-3">
        <div><label class="label" for="batch-size">Batch size</label>
          <input id="batch-size" type="number" min="0" max="500" value="20" class="input w-28" title="0 = all waiting books (up to 500)"></div>
        <button data-act="batch" class="btn-primary">Analyze batch</button>
        <button data-act="sync" class="btn-secondary">Sync Calibre now</button>
        ${inArea("deep", `<a href="#/deepscan" class="btn-secondary">🧬 Deep Scan…</a>`)}
        <button data-act="rerate-all" class="btn-ghost" title="Rate every AI-rated book again, e.g. after changing the AI or its rules">Re-rate whole library…</button>
        ${inArea("ai", `<button data-act="wipe" class="btn-ghost">Wipe pending queue</button>`)}
        <p id="worker" class="basis-full text-sm text-slate-400"></p>
        <div id="rerate" class="hidden basis-full rounded-lg bg-slate-800/60 p-3 text-sm"></div>
        <div id="genres-banner" class="hidden basis-full rounded-lg bg-slate-800/60 p-3 text-sm"></div>
      </div>
      ${inArea("ai", `<div id="custom-flags"></div><div id="settings-ai"></div>`)}
    </section>
    <section data-panel="users"><div id="users"></div></section>
    ${hasTab("delivery") ? `<section data-panel="delivery" class="space-y-4">
      <div id="settings-delivery"></div>
      <div class="card space-y-1 text-sm${on(state.user, "koreader") ? "" : " module-off"}" data-module="koreader">
        <p class="font-semibold">📖 KOReader</p>
        <p class="text-slate-400">Nothing to set up here: each reader finds their private catalog address and QR code under
          <b>Profile → KOReader setup</b>. For kids' e-readers, use the <b>📖 KOReader</b> button on their card in <b>Users &amp; Rules</b>.</p></div>
      ${inArea("system", `<div id="remote-access"></div>`)}
    </section>` : ""}
    ${inArea("system", `<section data-panel="system" class="space-y-4">
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
    markAdminSection(view, tab);
    history.replaceState(null, "", `#/admin?tab=${tab}`);
  };
  show(current);
  $("#admin-tabs", view).addEventListener("click", (e) => {
    const t = e.target.closest("[data-tab]")?.dataset.tab;
    if (t) {
      e.preventDefault(); // a section of this page: switch in place
      show(t);
    }
  });

  view.addEventListener("click", async (e) => {
    const act = e.target.closest("[data-act]")?.dataset.act;
    if (!act) return;
    if (act === "problem-reports") {
      import("./problems.js").then((m) => m.openProblemReports());
    } else if (act === "genres-fill" || act === "genres-stop") {
      if (act === "genres-fill" && !confirm("Ask the AI for the genres of the books Calibre has no tags for? It runs in the background, within your hourly token cap, and you can stop it any time.")) return;
      await attempt(() => post(`/api/admin/genres/${act === "genres-fill" ? "fill" : "stop"}`), act === "genres-fill" ? "Filling in genres…" : "Stopping after the current batch");
    } else if (act === "box-sets") {
      import("./boxsets.js").then((m) => m.openBoxSets());
    } else if (act === "cover-reports") {
      import("./covers.js").then((m) => m.openCoverReports());
    } else if (act === "tidy-titles") {
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
  renderErrors($("#errors", view), can(u, "system"), refresh);
  let stopRemote = null;
  if (isAdmin) {
    // Only the settings in this admin's areas come back; their cards are the ones shown.
    const settings = (await attempt(() => get("/api/admin/settings"))) || {};
    $("#batch-size", view).value = settings.batch_size ?? 20;
    for (const tab of ["ai", "delivery", "system"]) {
      const host = $(`#settings-${tab}`, view);
      if (host) renderSettingsTab(host, tab, settings, u);
    }
    const here = (id) => $(id, view);
    if (here("#custom-flags")) renderFlagsAdmin(here("#custom-flags"), refresh);
    if (here("#calibre-picker")) renderCalibrePicker(here("#calibre-picker"), refresh);
    if (here("#calibre-server")) renderCalibreServer(here("#calibre-server"));
    if (here("#remote-access")) stopRemote = await renderRemoteAccess(here("#remote-access"));
  }
  const timer = setInterval(refresh, 5000);
  await renderUsers($("#users", view), state.user);
  return () => {
    clearInterval(timer);
    stopRemote?.();
  };
}
