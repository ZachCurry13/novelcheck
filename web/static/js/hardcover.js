// Settings → 📚 Hardcover: paste your Hardcover token and NovelCheck pulls in
// what you're reading and have read (hourly, or Sync now), so Nightstand and the
// progress bars follow. The token stays on the server.
import { get, put, post } from "./api.js";
import { esc, attempt } from "./ui.js";
import { ago } from "./progress.js";

export async function renderHardcover(host) {
  const draw = (s) => {
    const at = s.synced_at ? Date.parse(s.synced_at) / 1000 : 0;
    host.innerHTML = `<h2 class="text-lg font-semibold">📚 Hardcover</h2>
      ${s.connected ? `<p class="text-sm">${s.username ? `Connected as <b>${esc(s.username)}</b>. ` : "Connected. "}${at ? `Last synced ${esc(ago(at))}` : "Not synced yet"}:
          ${s.books} book${s.books === 1 ? "" : "s"} (reading and read), ${s.matched} in your library.</p>
        <label class="toggle"><input type="checkbox" data-hc-push ${s.push ? "checked" : ""}> Also send my reading progress to Hardcover</label>
        <p class="text-xs text-slate-500">Off by default, since it changes your Hardcover account. When on, books you're reading or finished on your Nightstand are added to your Hardcover shelves with how far you are (matched by ISBN, so a book without one is skipped), and never lowering anything already there.</p>
        <div class="flex flex-wrap gap-2"><button type="button" data-hc="sync" class="btn-secondary">Sync now</button>
          <button type="button" data-hc="off" class="btn-ghost">Disconnect</button></div>`
      : `<p class="text-sm text-slate-400">Free. Books you're reading or have read on Hardcover show up here, and books in your Nightstand move to Reading or Finished by themselves. Each book is matched by ISBN, then by title and author.</p>
        <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
          <li>On hardcover.app, open your profile menu → <b>Settings</b> → <b>Hardcover API</b> and copy your token.</li>
          <li>Paste it here and press <b>Connect</b>. Tokens can expire; connect again if syncing stops.</li></ol>
        <form data-hc-form class="flex flex-wrap gap-2"><input name="token" type="password" required autocomplete="off" class="input min-w-0 flex-1" placeholder="Hardcover API token" aria-label="Hardcover API token">
          <button class="btn-primary">Connect</button></form>`}`;
  };
  const s = await get("/api/me/hardcover").catch(() => null);
  if (!s) return;
  draw(s);
  host.onclick = async (e) => {
    const act = e.target.closest("[data-hc]")?.dataset.hc;
    if (act === "sync") {
      const r = await attempt(() => post("/api/me/hardcover/sync"), "Hardcover synced");
      if (r) draw(r);
    } else if (act === "off" && confirm("Disconnect Hardcover? NovelCheck forgets your token and what it synced.")) {
      const r = await attempt(() => put("/api/me/hardcover", { token: "" }), "Disconnected");
      if (r) draw(r);
    }
  };
  host.onchange = async (e) => {
    if (!e.target.matches("[data-hc-push]")) return;
    const r = await attempt(() => put("/api/me/hardcover/push", { on: e.target.checked }), e.target.checked ? "Progress will be sent to Hardcover" : "Not sending progress to Hardcover");
    if (r) draw(r);
    else e.target.checked = !e.target.checked;
  };
  host.onsubmit = async (e) => {
    e.preventDefault();
    const r = await attempt(() => put("/api/me/hardcover", { token: e.target.token.value }), "Hardcover connected");
    if (r) draw(r);
  };
}
