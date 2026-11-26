// The wish feed card (admins, on the Wishlist page): a secret link to an RSS list of books still wanted. It shows only the
// title, author and format, never who asked, and a book leaves it once it is in the library.
import { get, post, del } from "./api.js";
import { esc, attempt } from "./ui.js";

export async function wishFeedCard(view) {
  const host = document.createElement("section");
  host.className = "card mt-6 space-y-2";
  view.append(host);
  const draw = (f) => {
    host.innerHTML = `<h2 class="font-semibold">Share the wanted list</h2>
      <p class="text-sm text-slate-400">A link to an RSS feed of the books and audiobooks still wanted: title, author and format only, never who asked. A book leaves it once it is in the library. Anyone who has the link can read it without signing in, so share it only with people you trust; making a new link stops the old one.</p>
      ${f.enabled ? `<input readonly class="input w-full text-sm" value="${esc(f.url)}" data-url>
        <div class="flex flex-wrap gap-2"><button type="button" data-copy class="btn-secondary py-1 text-sm">Copy link</button>
        <button type="button" data-new class="btn-ghost py-1 text-sm">Make a new link</button>
        <button type="button" data-off class="btn-ghost py-1 text-sm">Turn off</button></div>`
        : `<button type="button" data-new class="btn-secondary py-1 text-sm">Turn on the feed</button>`}`;
    host.querySelector("[data-url]")?.addEventListener("focus", (e) => e.target.select());
    host.querySelector("[data-copy]")?.addEventListener("click", async () => {
      try { await navigator.clipboard.writeText(f.url); } catch { host.querySelector("[data-url]").select(); }
    });
    host.querySelector("[data-new]")?.addEventListener("click", async () => {
      if (f.enabled && !confirm("Make a new link? The old one stops working.")) return;
      const n = await attempt(() => post("/api/admin/wish-feed"));
      if (n) draw(n);
    });
    host.querySelector("[data-off]")?.addEventListener("click", async () => {
      const n = await attempt(() => del("/api/admin/wish-feed"), "Feed turned off");
      if (n) draw(n);
    });
  };
  const f = await attempt(() => get("/api/admin/wish-feed"));
  if (f) draw(f);
}
