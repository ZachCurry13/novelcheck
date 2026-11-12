// 📚 Libby: on a book, whether the person's libraries (one or several) lend it (e-book copies, holds, wait)
// and the audiobook, each with a link to it there, where Libby's Borrow or Place hold is one tap away. On
// Discover and in the Library, small chips say the same; and Settings holds the list of libraries.
import { get, put, post, del } from "./api.js";
import { esc, attempt } from "./ui.js";

const num = (n) => Number(n || 0).toLocaleString();
const wait = (a) => (a.wait_days ? `, about ${num(a.wait_days)} day${a.wait_days === 1 ? "" : "s"}` : "");

function status(a) {
  return a.available ? `<b class="text-emerald-300">Available now</b> (${num(a.free)} of ${num(a.copies)} copies)`
    : `<b class="text-amber-300">${num(a.holds)} hold${a.holds === 1 ? "" : "s"}</b> on ${num(a.copies)} cop${a.copies === 1 ? "y" : "ies"}${wait(a)}`;
}

const link = (a, verbs) => `<a href="${esc(a.url)}" target="_blank" rel="noopener noreferrer" class="btn-secondary mt-1 inline-flex py-1 text-sm">${a.available ? verbs[0] : a.holdable ? verbs[1] : verbs[2]}</a>`;

// One library's answer for a book: the e-book, and the audiobook when it lends one.
function libraryBlock(l, many) {
  if (l.error) return `<div><p class="font-semibold">${esc(l.name)}</p><p class="text-slate-400">Couldn't check it just now.</p></div>`;
  const a = l.availability;
  const ebook = a?.found ? `<p>E-book: ${status(a)}${a.kindle ? " · Kindle supported" : ""}</p>${link(a, ["Borrow in Libby ↗", "Place a hold in Libby ↗", "Open at the library ↗"])}` : `<p class="text-slate-400">Not an e-book here.</p>`;
  const audio = l.audio?.found ? `<p class="mt-2">Audiobook: ${status(l.audio)}</p>${link(l.audio, ["Borrow the audiobook ↗", "Hold the audiobook ↗", "Open the audiobook ↗"])}` : "";
  return `<div>${many ? `<p class="font-semibold">${esc(l.name)}</p>` : ""}${ebook}${audio}</div>`;
}

// renderBookLibby fills host in a book's window.
export async function renderBookLibby(host, bookId) {
  const r = await get(`/api/books/${bookId}/libby`).catch(() => null);
  if (!r) return;
  if (!r.configured) {
    host.innerHTML = `<p class="text-xs text-slate-500">Borrow from your library? <a href="#/settings?s=connections" data-close class="underline">Set your Libby library in Settings</a></p>`;
    return;
  }
  const libs = r.libraries || [];
  const found = libs.filter((l) => l.availability?.found || l.audio?.found);
  const show = libs.length > 1 && found.length ? found : libs; // several libraries: show the ones that have it
  const none = libs.length > 1 && !found.length ? `<p class="text-slate-400">Not at ${libs.map((l) => esc(l.name)).join(" or ")}.</p>` : "";
  host.innerHTML = `<div class="space-y-2 rounded-lg bg-slate-800/60 p-3 text-sm"><p class="label">${libs.length > 1 ? "Your libraries" : esc(r.library)} (Libby)</p>
    ${none || show.map((l) => libraryBlock(l, libs.length > 1)).join("")}</div>`;
}

// markLibby puts a small chip on each card in host that has a [data-libby="<book id>"] placeholder: available
// now, or the holds. Books no library lends get nothing. Answers are remembered while the page is open.
const seen = new Map(); // book id -> availability, or null when not at a library
const seenAudio = new Map(); // book id -> the audiobook's availability, when a library lends it
let configured = null;

function chip(a, icon = "📚") {
  if (!a) return "";
  const text = a.available ? `${icon} Available` : `${icon} ${Number(a.holds).toLocaleString()} hold${a.holds === 1 ? "" : "s"}`;
  const where = a.library ? ` at ${a.library}` : "";
  const tip = a.available ? `A copy is available now${where} (Libby)` : `${a.wait_days ? `About ${a.wait_days} days. ` : ""}On hold${where} (Libby)`;
  return `<a href="${esc(a.url)}" target="_blank" rel="noopener noreferrer" class="${a.available ? "chip-none" : "chip-wait"}" title="${esc(tip)}" data-libby-link>${text}</a>`;
}

// forget drops what was remembered (the libraries changed).
function forget() {
  seen.clear();
  seenAudio.clear();
  configured = null;
}

export async function markLibby(host) {
  if (configured === false) return;
  const paint = () => host.querySelectorAll("[data-libby]").forEach((el) => {
    if (seen.has(el.dataset.libby)) el.innerHTML = chip(seen.get(el.dataset.libby)) + chip(seenAudio.get(el.dataset.libby), "🎧");
  });
  paint();
  const ids = [...new Set([...host.querySelectorAll("[data-libby]")].map((el) => el.dataset.libby))].filter((id) => !seen.has(id)).slice(0, 60);
  if (!ids.length) return;
  const r = await post("/api/libby/check", { ids: ids.map(Number) }).catch(() => null);
  if (!r) return;
  configured = r.configured;
  if (!r.configured) return;
  for (const id of ids) {
    seen.set(id, r.books?.[id]?.found ? r.books[id] : null);
    seenAudio.set(id, r.audio?.[id] || null);
  }
  paint();
}

// checkAll asks about every book on the page, 60 at a time, so a filter sees them all.
// onProgress(done, total) is called as it goes.
export async function checkAll(host, onProgress) {
  const all = [...new Set([...host.querySelectorAll("[data-libby]")].map((el) => el.dataset.libby))];
  for (;;) {
    const left = all.filter((id) => !seen.has(id));
    onProgress?.(all.length - left.length, all.length);
    if (!left.length || configured === false) return;
    await markLibby(host);
    if (all.filter((id) => !seen.has(id)).length === left.length) return; // nothing was learned: stop, don't loop
  }
}

// filterCards shows only the books that match mode: "here" (a library lends it), "audio" (it lends the
// audiobook), "now" (a copy is available now), or everything. Rows with nothing left are hidden.
export function filterCards(host, mode) {
  const ok = (id) => {
    const a = seen.get(id);
    const au = seenAudio.get(id);
    if (mode === "here") return Boolean(a);
    if (mode === "audio") return Boolean(au);
    if (mode === "now") return Boolean(a?.available || au?.available);
    return true;
  };
  host.querySelectorAll("[data-book]").forEach((c) => c.classList.toggle("hidden", Boolean(mode) && !ok(c.dataset.book)));
  host.querySelectorAll("section").forEach((s) => s.classList.toggle("hidden", Boolean(mode) && ![...s.querySelectorAll("[data-book]")].some((c) => !c.classList.contains("hidden"))));
}

export async function hasLibraries() {
  const r = await get("/api/me/libby").catch(() => null);
  return Boolean(r?.libraries?.length);
}

// markKeptAudio puts "at your library" on Library cards from what the background check kept (no waiting).
export async function markKeptAudio(host) {
  const els = [...host.querySelectorAll("[data-libaudio]")].filter((el) => !el.dataset.done);
  if (!els.length) return;
  const r = await post("/api/libby/audio", { ids: els.map((el) => Number(el.dataset.libaudio)).slice(0, 200) }).catch(() => null);
  for (const el of els.slice(0, 200)) {
    el.dataset.done = "1";
    const a = r?.audio?.[el.dataset.libaudio];
    if (a) el.innerHTML = chip(a, "🎧");
  }
}

// renderLibbyCard fills Settings' libraries card.
export async function renderLibbyCard(host) {
  const draw = (c) => {
    const libs = c.libraries || [];
    host.innerHTML = `<h2 class="text-lg font-semibold">Libby libraries</h2>
      ${libs.length ? `<ul class="space-y-1 text-sm">${libs.map((l) => `<li class="flex items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-2"><span><b>${esc(l.name)}</b> <span class="text-xs text-slate-500">(${esc(l.key)})</span></span>
          <button type="button" data-lb-rm="${esc(l.key)}" class="btn-ghost py-1 text-sm">Remove</button></li>`).join("")}</ul>
        <p class="text-sm text-slate-400">Books show whether any of these lend them, and the book that is available soonest wins. In the background NovelCheck also checks the audiobook of every book you don't have as audio and keeps the answer, so the Library can show 🎧 and filter by it.</p>
        <button type="button" data-lb-scan class="btn-secondary">Check my books now</button>`
      : `<p class="text-sm text-slate-400">Free. See whether your library lends a book, how long the hold is, and open it in Libby. Libby does the borrowing; NovelCheck can't sign in for you. You can add more than one library.</p>`}
      <p class="text-xs text-slate-500">${libs.length ? "Add another library: " : ""}type its key, the first part of its OverDrive address (lapl for lapl.overdrive.com). You can paste the whole address.</p>
      <form data-lb-form class="flex flex-wrap gap-2"><input name="library" required class="input min-w-0 flex-1" placeholder="e.g. lapl" autocomplete="off" aria-label="Library key">
        <button class="btn-primary">${libs.length ? "Add library" : "Save"}</button></form>`;
  };
  const cur = await get("/api/me/libby").catch(() => null);
  if (!cur) return;
  draw(cur);
  host.onclick = async (e) => {
    const rm = e.target.closest("[data-lb-rm]")?.dataset.lbRm;
    if (rm) {
      const r = await attempt(() => del(`/api/me/libby/${encodeURIComponent(rm)}`), "Library removed");
      if (r) {
        forget();
        draw(r);
      }
    } else if (e.target.closest("[data-lb-scan]")) {
      await attempt(() => post("/api/me/libby/scan", {}), "Checking your books in the background. The Library shows 🎧 as answers arrive.");
    }
  };
  host.onsubmit = async (e) => {
    e.preventDefault();
    const r = await attempt(() => put("/api/me/libby", { library: e.target.library.value }), "Library added");
    if (r) {
      forget();
      draw(r);
    }
  };
}
