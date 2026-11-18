// Profile: how NovelCheck looks, the page it opens on, and (parents) where
// books are added: Import books (Kindle, drive, lists) and Paper books.
import { get, put } from "./api.js";
import { $, esc, attempt, canManage } from "./ui.js";
import { on } from "./modules.js";
import { refreshUser } from "./app.js";
import { applyAppearance } from "./appearance.js";
import { NOT_LIBRARIES } from "./eventshelf.js";

const LOOKS = [
  ["theme", "Theme", [["", "Paperback and Ember · match my device"], ["dark", "Ember (dark)"], ["light", "Paperback (light)"],
    ["blue", "Classic blue · match my device"], ["blue-dark", "Classic blue · dark"], ["blue-light", "Classic blue · light"]]],
  ["font", "Font", [["", "Standard"], ["dyslexic", "OpenDyslexic (easier to read for some people with dyslexia)"]]],
  ["motion", "Motion", [["", "Match my device"], ["reduce", "Reduce motion"]]],
];

const PAGES = [
  ["check", "Check a book", (u) => canManage(u)],
  ["library", "Library", () => true],
  ["discover", "Discover", (u) => on(u, "discover")],
  ["queue", "▶️ Nightstand", (u) => on(u, "queue")],
  ["collections", "Collections", (u) => u.role !== "friend"],
  ["wishlist", "Wishlist", (u) => u.role !== "friend"],
  ["stats", "Reading stats", (u) => u.role === "friend"],
];

export function profileExtrasHTML(u) {
  const def = canManage(u) ? "Check a book" : u.role === "friend" ? "Discover" : "Library";
  return `
    ${canManage(u) ? `<div class="card space-y-2">
      <h2 class="text-lg font-semibold">Add books</h2>
      <div class="grid gap-2 sm:grid-cols-2">
        <a href="#/import" class="btn-secondary${on(u, "import") ? "" : " module-off"}" data-module="import">Import books<span class="sr-only">: a Kindle, a drive or a list</span></a>
        <a href="#/shelf" class="btn-secondary${on(u, "import") ? "" : " module-off"}" data-module="import">Paper books</a>
      </div>
      <p class="text-xs text-slate-400">Import a Kindle, a drive or a list from Goodreads or Amazon; or scan the barcodes of printed books.</p></div>` : ""}
    <div class="card space-y-2">
      <h2 class="text-lg font-semibold">Appearance</h2>
      <p class="text-xs text-slate-400">Just for you, on every device you sign in on.</p>
      <div class="grid gap-3 sm:grid-cols-3">${LOOKS.map(([k, label, opts]) => `<label class="block"><span class="label">${label}</span>
        <select data-look="${k}" class="input">${opts.map(([v, l]) => `<option value="${v}" ${(u[k] || "") === v ? "selected" : ""}>${l}</option>`).join("")}</select></label>`).join("")}</div></div>
    <div class="card space-y-2">
      <h2 class="text-lg font-semibold">Start page</h2>
      <label class="label" for="start-page">When you open NovelCheck or sign in, it starts on</label>
      <select id="start-page" class="input">
        <option value="">Default (${def})</option>
        ${PAGES.filter(([, , ok]) => ok(u)).map(([k, l]) => `<option value="${k}" ${u.start_page === k ? "selected" : ""}>${l}</option>`).join("")}
      </select></div>
    ${canManage(u) ? `<div class="card space-y-2">
      <h2 class="text-lg font-semibold">Free-book events</h2>
      <label class="label" for="claim-lib">Books I claim go to</label>
      <select id="claim-lib" class="input"><option value="0">Ask on each event</option></select>
      <p class="text-xs text-slate-400">Where books you claim at events go. You can pick another on each event.</p></div>` : ""}`;
}

export function bindProfileExtras(view, u = {}) {
  view.querySelectorAll("[data-look]").forEach((sel) => sel.addEventListener("change", async () => {
    const looks = Object.fromEntries([...view.querySelectorAll("[data-look]")].map((x) => [x.dataset.look, x.value]));
    applyAppearance(looks); // right away; saved with the account below
    if (await attempt(() => put("/api/me/appearance", looks), "Appearance saved")) await refreshUser();
  }));
  const claim = $("#claim-lib", view);
  if (claim) {
    get("/api/catalogs").then((cats) => { // the libraries a claim can go to, as on an event page
      const libs = (cats || []).filter((c) => c.source !== "calibre" && !c.physical && !NOT_LIBRARIES.includes(c.name));
      claim.insertAdjacentHTML("beforeend", libs.map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join(""));
      claim.value = String(libs.some((c) => c.id === u.claim_catalog) ? u.claim_catalog : 0);
    }).catch(() => {});
    claim.addEventListener("change", async () => {
      if (await attempt(() => put("/api/me/claim-library", { catalog_id: Number(claim.value) }),
        claim.value === "0" ? "You'll pick a library on each event" : "Claimed books go there now, on every device")) await refreshUser();
    });
  }
  $("#start-page", view)?.addEventListener("change", async (e) => {
    if (await attempt(() => put("/api/me/start-page", { page: e.target.value }), "Start page saved: NovelCheck opens there next time")) await refreshUser();
  });
}
