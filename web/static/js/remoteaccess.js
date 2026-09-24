// Admin → Remote access: built-in Cloudflare Tunnel settings and status.
import { get, put } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";
import { renderMarkdown } from "./markdown.js";

const STATE_LABELS = {
  off: ["Off", "chip-pending"],
  starting: ["Starting…", "chip-closed"],
  retrying: ["Retrying", "chip-open"],
  connected: ["Connected", "chip-none"],
  not_installed: ["Not available in this version", "chip-open"],
};

export async function renderRemoteAccess(host) {
  const box = document.createElement("div");
  host.replaceChildren(box);
  const data = await attempt(() => get("/api/admin/tunnel"));
  if (!data) return () => {};
  box.innerHTML = `
    <section class="card mb-8 space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-lg font-semibold">Remote access (Cloudflare Tunnel)</h2>
        <span id="ra-state"></span>
      </div>
      <p class="text-sm text-slate-400">Use NovelCheck away from home through a free Cloudflare Tunnel, with no router changes.</p>
      <div class="flex flex-wrap gap-2">
        <button type="button" data-ra="guide" class="btn-secondary py-1 text-xs">Show setup steps</button>
      </div>
      <div id="ra-guide" class="hidden rounded-lg bg-slate-800/60 p-4 text-sm leading-relaxed text-slate-300"></div>
      <form id="ra-form" class="grid gap-3 md:grid-cols-2">
        <label class="block"><span class="label">Tunnel token</span>
          <input name="token" type="password" autocomplete="off" class="input"
            placeholder="${data.has_token ? "Saved. Paste a new one to replace it" : "Paste the token (eyJ…) or the whole command from Cloudflare"}"></label>
        <label class="block"><span class="label">Public address</span>
          <input name="hostname" class="input" placeholder="books.yourname.com" value="${esc(data.hostname)}"></label>
        <label class="toggle md:col-span-2"><input type="checkbox" name="enabled" ${data.enabled ? "checked" : ""}> Turn on remote access</label>
        <div class="md:col-span-2 flex flex-wrap items-center gap-3">
          <button class="btn-primary">Save &amp; connect</button>
          <a id="ra-link" class="hidden text-sm underline" target="_blank" rel="noopener noreferrer"></a>
        </div>
      </form>
      <details class="text-xs text-slate-500"><summary class="cursor-pointer">Connector log</summary>
        <pre id="ra-log" class="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-slate-950 p-2"></pre></details>
    </section>`;

  const show = (st, hostname) => {
    const [label, cls] = STATE_LABELS[st.state] || [st.state, "chip-pending"];
    $("#ra-state", box).innerHTML = `<span class="${cls}">${esc(label)}</span>`;
    $("#ra-log", box).textContent = (st.last_error ? `Last problem: ${st.last_error}\n\n` : "") + (st.log || []).join("\n");
    const link = $("#ra-link", box);
    link.classList.toggle("hidden", !(hostname && st.state === "connected"));
    if (hostname) {
      link.href = `https://${hostname}`;
      link.textContent = `Open https://${hostname}`;
    }
  };
  show(data.status, data.hostname);

  box.addEventListener("click", (e) => {
    if (e.target.closest("[data-ra=guide]")) {
      const g = $("#ra-guide", box);
      g.innerHTML = renderMarkdown(data.guide);
      g.classList.toggle("hidden");
    }
  });

  $("#ra-form", box).addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = e.target;
    const body = { token: f.token.value, hostname: f.hostname.value, enabled: f.enabled.checked };
    const st = await attempt(() => put("/api/admin/tunnel", body));
    if (!st) return;
    f.token.value = "";
    data.hostname = body.hostname.replace(/^https?:\/\//, "").replace(/\/$/, "");
    toast(body.enabled ? "Connecting to Cloudflare…" : "Remote access turned off");
    show(st, data.hostname);
  });

  const timer = setInterval(async () => {
    const d = await get("/api/admin/tunnel").catch(() => null);
    if (d) show(d.status, d.hostname);
  }, 4000);
  return () => clearInterval(timer);
}
