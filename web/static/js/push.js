// "📱 Phone notifications" on the Profile page: turn Web Push on or off for
// this device (see internal/push). Parents get the 🔔 notices; everyone gets
// "your book is on its way".
import { get, post } from "./api.js";
import { esc, attempt, toast, canManage } from "./ui.js";

const isIOS = () => /iPhone|iPad|iPod/.test(navigator.userAgent);
const installed = () => window.matchMedia?.("(display-mode: standalone)").matches || navigator.standalone === true;

// Why this device can't get notifications, or "" if it can.
function blocker() {
  if (!window.isSecureContext) {
    return "Notifications need NovelCheck's secure <b>https://</b> address (an admin sets it up under <b>Admin → Delivery & Services → Remote access</b>). Open NovelCheck at that address, then come back here.";
  }
  if (isIOS() && !installed()) {
    return "On iPhone and iPad: tap <b>Share → Add to Home Screen</b>, open NovelCheck from the new icon, then turn notifications on here.";
  }
  if (!("serviceWorker" in navigator) || !("PushManager" in window) || !("Notification" in window)) {
    return "This browser can't show notifications from NovelCheck.";
  }
  if (Notification.permission === "denied") {
    return "Notifications are blocked for NovelCheck in this browser's settings. Allow them there, then reload this page.";
  }
  return "";
}

// The server's key as bytes, for PushManager.subscribe.
function keyBytes(b64) {
  const s = (b64 + "===".slice((b64.length + 3) % 4)).replace(/-/g, "+").replace(/_/g, "/");
  return Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
}

function deviceName() {
  const ua = navigator.userAgent;
  const os = /iPhone/.test(ua) ? "iPhone" : /iPad/.test(ua) ? "iPad" : /Android/.test(ua) ? "Android"
    : /Windows/.test(ua) ? "Windows" : /Mac OS X/.test(ua) ? "Mac" : "Computer";
  const browser = /Edg\//.test(ua) ? "Edge" : /Firefox\//.test(ua) ? "Firefox" : /Chrome\//.test(ua) ? "Chrome"
    : /Safari\//.test(ua) ? "Safari" : "browser";
  return `${os} · ${browser}`;
}

// SHA-256 of the endpoint in hex: how GET /api/push names each device.
async function endpointKey(endpoint) {
  const sum = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(endpoint));
  return [...new Uint8Array(sum)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

// serviceWorker.ready never settles if registration failed, so give up after a while.
const workerReady = () => Promise.race([navigator.serviceWorker.ready, new Promise((r) => setTimeout(() => r(null), 4000))]);

export async function renderPushCard(host, user) {
  const manager = canManage(user);
  const intro = manager
    ? "Get NovelCheck's 🔔 notices on this phone or computer, even when the app is closed: problems, new books from Calibre, finished ratings."
    : "Get a notice when a book you started is on its way to your reader.";
  const why = blocker();
  const reg = why ? null : await workerReady();
  const sub = reg ? await reg.pushManager.getSubscription() : null;
  const info = (await attempt(() => get("/api/push"))) || { devices: [] };
  const key = sub ? await endpointKey(sub.endpoint) : "";
  const here = info.devices.find((d) => d.key === key); // this device, as the server knows it
  const mine = Boolean(sub && here);
  const others = info.devices.filter((d) => d !== here);
  host.innerHTML = `
    <h2 class="text-lg font-semibold">📱 Phone notifications</h2>
    <p class="text-sm text-slate-300">${intro}</p>
    ${why || !reg ? `<p class="rounded-lg bg-slate-800/60 p-3 text-sm text-amber-200">${why || "This browser isn't ready for notifications yet. Reload the page and try again."}</p>` : `
      <p class="text-sm">${mine ? "✅ On for this device." : "Off for this device."}</p>
      ${manager ? `<label class="block"><span class="label">What to send</span>
        <select data-push="scope" class="input">
          <option value="all">Problems and everyday events</option>
          <option value="problems" ${here?.scope === "problems" ? "selected" : ""}>Only problems</option>
        </select></label>` : ""}
      <div class="flex flex-wrap gap-2">
        ${mine ? `<button data-push="test" class="btn-secondary">Send a test</button>
          <button data-push="off" class="btn-ghost">Turn off on this device</button>`
          : `<button data-push="on" class="btn-primary">Turn on for this device</button>`}
      </div>`}
    ${others.length ? `<p class="text-xs text-slate-500">Also on for ${others.length} other device${others.length === 1 ? "" : "s"}: ${esc(others.map((d) => d.device || "device").join(", "))}.</p>` : ""}`;
  if (why || !reg) return;

  const scope = () => host.querySelector("[data-push=scope]")?.value || "all";
  const subscribe = async () => {
    const opts = { userVisibleOnly: true, applicationServerKey: keyBytes(info.public_key) };
    try {
      return await reg.pushManager.subscribe(opts);
    } catch {
      // Subscribed earlier with a different server key: start over.
      await (await reg.pushManager.getSubscription())?.unsubscribe();
      return reg.pushManager.subscribe(opts);
    }
  };
  host.onchange = async (e) => {
    if (e.target.dataset.push === "scope" && mine) {
      await attempt(() => post("/api/push/subscribe", { ...sub.toJSON(), scope: scope(), device: deviceName() }), "Saved");
    }
  };
  host.onclick = async (e) => {
    const act = e.target.closest("[data-push]")?.dataset.push;
    if (act === "on") {
      if ((await Notification.requestPermission()) !== "granted") return toast("Notifications weren't allowed on this device", true);
      const s = await attempt(subscribe);
      if (!s) return;
      const ok = await attempt(() => post("/api/push/subscribe", { ...s.toJSON(), scope: scope(), device: deviceName() }), "Notifications are on for this device");
      if (ok) renderPushCard(host, user);
    } else if (act === "off" && sub) {
      await attempt(() => post("/api/push/unsubscribe", { endpoint: sub.endpoint }));
      await sub.unsubscribe().catch(() => {});
      toast("Notifications are off for this device");
      renderPushCard(host, user);
    } else if (act === "test") {
      const r = await attempt(() => post("/api/push/test"));
      if (r) toast(r.sent ? `Test sent to ${r.sent} device${r.sent === 1 ? "" : "s"}. It should appear in a moment.` : `The test didn't go through: ${r.error || "unknown error"}`, !r.sent);
    }
  };
}
