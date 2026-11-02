// Settings → Connections → Audiobookshelf: link your own account so your place
// in each audiobook is yours. Without it, everyone shares the family account.
import { get, put, del } from "./api.js";
import { esc, attempt } from "./ui.js";

export async function renderABSCard(host) {
  const cur = await get("/api/me/audiobookshelf").catch(() => null);
  if (!cur || !cur.available) return void (host.hidden = true);
  host.hidden = false;
  const draw = (c) => {
    host.innerHTML = `<h2 class="text-lg font-semibold">🎧 Audiobookshelf</h2>
      ${c.linked ? `<p class="text-sm">Linked as <b>${esc(c.name)}</b>. Your place in each audiobook, bookmarks and Continue listening are yours.</p>
        <button type="button" data-abs="off" class="btn-ghost">Unlink</button>`
      : `<p class="text-sm text-slate-400">Audiobooks play in NovelCheck. Right now your place is kept on the family's shared Audiobookshelf account. Link your own to keep it separate.
          Your password is used once to sign in and isn't kept.</p>
        <form data-abs-form class="grid gap-2 sm:grid-cols-3"><input name="username" required class="input" placeholder="Audiobookshelf name" autocomplete="username" aria-label="Audiobookshelf name">
          <input name="password" type="password" required class="input" placeholder="Password" autocomplete="current-password" aria-label="Audiobookshelf password">
          <button class="btn-primary">Link my account</button></form>`}`;
  };
  draw(cur);
  host.onclick = async (e) => {
    if (!e.target.closest("[data-abs=off]")) return;
    const r = await attempt(() => del("/api/me/audiobookshelf"), "Unlinked");
    if (r) draw(r);
  };
  host.onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    const r = await attempt(() => put("/api/me/audiobookshelf", { username: f.username.value, password: f.password.value }), "Linked");
    if (r) draw(r);
  };
}
