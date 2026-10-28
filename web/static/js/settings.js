// ⚙ Settings: everything you can set, in sections so it isn't one long page:
// General (look, delivery, notifications), Account (password, family device and
// PIN), Kindle & KOReader (sync, the plugin, your e-readers) and Connections
// (Hardcover, Libby).
// Profile keeps who you are; Admin keeps the technical settings.
import { put } from "./api.js";
import { $, esc, attempt } from "./ui.js";
import { refreshUser } from "./app.js";
import { on, deliveryOptions } from "./modules.js";
import { renderPushCard } from "./push.js";
import { amazonStepsHTML, openKOReaderSetup } from "./delivery.js";
import { profileExtrasHTML, bindProfileExtras } from "./profileextras.js";
import { familyCardHTML, bindFamilyCard } from "./familysettings.js";
import { renderSyncStatus } from "./syncstatus.js";
import { renderPluginCard } from "./pluginmanage.js";
import { renderHardcover } from "./hardcover.js";
import { renderLibbyCard } from "./libby.js";
import { tabStrip } from "./tabs.js";
import { renderSharingCard } from "./sharing.js";

const SECTIONS = { general: "General", account: "Account", kindle: "Kindle & KOReader", connections: "Connections" };

// sectionFromHash reads #/settings?s=kindle.
const sectionFromHash = () => {
  const s = new URLSearchParams(location.hash.split("?")[1] || "").get("s");
  return SECTIONS[s] ? s : "general";
};

function generalHTML(u) {
  return `${profileExtrasHTML(u)}
    <form id="delivery" class="card space-y-3${on(u, "queue") ? "" : " module-off"}">
      <h2 class="text-lg font-semibold">"Start Reading" delivery</h2>
      <select name="delivery_method" class="input">${deliveryOptions(u, u.delivery_method)}</select>
      <div><label class="label" for="kindle-email">Send-to-Kindle address</label>
        <input id="kindle-email" name="kindle_email" class="input" placeholder="name@kindle.com"></div>
      <details class="text-xs text-slate-400${on(u, "send_to_kindle") ? "" : " module-off"}"><summary class="cursor-pointer">Books are emailed from
        <b>${esc(u.delivery_from || "the admin's sender address")}</b>. How to approve it on Amazon</summary>
        <div class="mt-2">${amazonStepsHTML(u.delivery_from)}</div></details>
      <button class="btn-primary">Save delivery settings</button>
    </form>
    <div id="push-card" class="card space-y-3 lg:col-span-2"></div>`;
}

const accountHTML = (u) => `<form id="password" class="card space-y-3">
    <h2 class="text-lg font-semibold">Change password</h2>
    <input name="current_password" type="password" required placeholder="Current password" class="input" autocomplete="current-password">
    <input name="new_password" type="password" required minlength="8" placeholder="New password (8+ chars)" class="input" autocomplete="new-password">
    <button class="btn-primary">Update password</button>
  </form>
  <div id="sharing-card" class="card space-y-3 lg:col-span-2"></div>
  ${familyCardHTML(u)}`;

function kindleHTML(u) {
  const off = on(u, "koreader") ? "" : " module-off";
  return `<div class="card space-y-2 lg:col-span-2${off}">
      <h2 class="text-lg font-semibold">📖 KOReader setup</h2>
      <p class="text-sm text-slate-400">The catalog address, QR code and reading-sync steps for a Kindle or phone. With the plugin below you rarely need them: <b>NovelCheck → Set up everything</b> does it on the Kindle.</p>
      <button type="button" id="ko-setup" class="btn-secondary">Open KOReader setup</button></div>
    <div id="plugin-card" class="card space-y-2${off}"></div>
    <div id="sync-card" class="card space-y-2${off}"></div>`;
}

const connectionsHTML = () => `<div id="hardcover-card" class="card space-y-2"></div><div id="libby-card" class="card space-y-2"></div>`;

export async function renderSettings(view, state) {
  const u = state.user;
  let section = sectionFromHash();
  const draw = () => {
    view.innerHTML = `<h1 class="mb-3 text-2xl font-bold">⚙ Settings</h1>
      ${tabStrip({ id: "settings-tabs", attr: "data-section", label: "Settings", active: section, tabs: Object.entries(SECTIONS) })}
      <div class="grid gap-4 lg:grid-cols-2">${{ general: () => generalHTML(u), account: () => accountHTML(u), kindle: () => kindleHTML(u), connections: connectionsHTML }[section]()}</div>`;
    bind();
  };
  const bind = () => {
    if (section === "general") {
      const dform = $("#delivery", view);
      dform.kindle_email.value = u.kindle_email;
      dform.addEventListener("submit", async (e) => {
        e.preventDefault();
        const ok = await attempt(() => put("/api/me/delivery", {
          delivery_method: dform.delivery_method.value,
          kindle_email: dform.kindle_email.value.trim(),
        }), "Delivery settings saved");
        if (ok) await refreshUser();
      });
      bindProfileExtras(view);
      renderPushCard($("#push-card", view), u);
    } else if (section === "account") {
      renderSharingCard($("#sharing-card", view), u);
      bindFamilyCard(view, draw);
      $("#password", view).addEventListener("submit", async (e) => {
        e.preventDefault();
        const ok = await attempt(() => put("/api/me/password", Object.fromEntries(new FormData(e.target))), "Password updated");
        if (ok) e.target.reset();
      });
    } else if (section === "kindle") {
      $("#ko-setup", view).addEventListener("click", () => openKOReaderSetup());
      if (on(u, "koreader")) {
        renderSyncStatus($("#sync-card", view));
        renderPluginCard($("#plugin-card", view));
      }
    } else {
      renderHardcover($("#hardcover-card", view));
      renderLibbyCard($("#libby-card", view));
    }
  };
  view.onclick = (e) => {
    const k = e.target.closest("[data-section]")?.dataset.section;
    if (!k || k === section) return;
    section = k;
    history.replaceState(null, "", `#/settings?s=${k}`); // shareable, without reloading the page
    draw();
  };
  draw();
}
