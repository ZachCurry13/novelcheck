// 🎉 Events (Stuff Your Kindle and other free-book days): the events, and one
// event's books with their ratings, what the family has, and Claim on
// Amazon. "✓ I claimed it" files a book in the library parents pick.
import { get, post, del } from "./api.js";
import { $, esc, attempt, toast, classChip, canManage } from "./ui.js";
import { coverImg } from "./covers.js";
import { openBook } from "./bookdialog.js";
import { loadContent, contentIcons } from "./content.js";
import { discoverTabsHTML } from "./discovertabs.js";
import { openNewEvent } from "./eventnew.js";
import { on } from "./modules.js";

const PREFS = "nc:event-view";
const prefs = () => {
  try {
    return JSON.parse(localStorage.getItem(PREFS) || "{}");
  } catch {
    return {};
  }
};
const savePrefs = (p) => {
  try {
    localStorage.setItem(PREFS, JSON.stringify(p));
  } catch {
    /* private mode */
  }
};

export async function renderEvents(view, state) {
  const id = new URLSearchParams(location.hash.split("?")[1] || "").get("id");
  if (id) return renderEvent(view, state, id);
  const manager = canManage(state.user);
  view.innerHTML = `${discoverTabsHTML("events", state.user)}
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><h1 class="text-2xl font-bold">🎉 Events</h1>
      ${manager ? `<button data-new class="btn-primary">＋ New event</button>` : ""}</div>
    <p class="mb-4 text-sm text-slate-400">Free-book days such as Stuff Your Kindle: see which books suit your family, which you already have${state.user.role === "restricted" ? "" : ", and claim them on Amazon"}.
      Events go away after 30 days unless a parent pins them.</p>
    <ul id="events" class="space-y-2"><li class="text-slate-400">Loading…</li></ul>`;
  const evs = (await attempt(() => get("/api/events"))) || [];
  $("#events", view).innerHTML = evs.map((e) => `<li><a href="#/events?id=${e.id}" class="card flex items-center gap-3 hover:ring-indigo-600">
      <span class="text-2xl" aria-hidden="true">${e.pinned ? "📌" : "🎉"}</span>
      <span class="min-w-0 flex-1"><span class="block font-semibold">${esc(e.name)}</span>
        <span class="block text-xs text-slate-400">${e.books} book${e.books === 1 ? "" : "s"} · ${e.rated} rated · ${e.pinned ? "pinned" : `goes in ${e.days_left} day${e.days_left === 1 ? "" : "s"}`}</span></span>
      <span aria-hidden="true">›</span></a></li>`).join("")
    || `<li class="card text-sm text-slate-400">No events yet.${manager ? " When a free-book day comes up, press ＋ New event and paste its list." : ""}</li>`;
  view.querySelector("[data-new]")?.addEventListener("click", openNewEvent);
}

async function renderEvent(view, state, id) {
  const manager = canManage(state.user);
  const kid = state.user.role === "restricted";
  const [data, cats] = await Promise.all([attempt(() => get(`/api/events/${id}`)),
    manager ? get("/api/catalogs").catch(() => []) : [], loadContent()]);
  if (!data) return (location.hash = "#/events");
  const ev = data.event;
  const p = prefs();
  // Where claimed books go: the family's own e-book libraries.
  const libs = (cats || []).filter((c) => c.source !== "calibre" && !c.physical);
  const pickLib = () => {
    const want = String(p.claimLib || "");
    return libs.some((c) => String(c.id) === want) ? want : libs[0] ? String(libs[0].id) : "";
  };
  view.innerHTML = `${discoverTabsHTML("events", state.user)}
    <a href="#/events" class="mb-2 inline-block text-sm text-slate-400 hover:text-white">← All events</a>
    <div class="mb-2 flex flex-wrap items-start justify-between gap-2">
      <div class="min-w-0"><h1 class="text-2xl font-bold">${ev.pinned ? "📌 " : ""}${esc(ev.name)}</h1>
        <p class="text-xs text-slate-400">${ev.books} books · ${ev.rated} rated${ev.rated < ev.books ? " (the rest are being rated, first in line)" : ""}
          ${ev.source_url && !kid ? ` · <a href="${esc(ev.source_url)}" target="_blank" rel="noopener noreferrer" class="underline">event page ↗</a>` : ""}</p></div>
      ${manager ? `<div class="flex gap-2"><button data-pin class="btn-ghost py-1.5 text-sm">${ev.pinned ? "Unpin" : "📌 Pin"}</button>
        <button data-delete class="btn-ghost py-1.5 text-sm" title="Discard this event">🗑</button></div>` : ""}
    </div>
    <div class="card mb-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
      <label class="toggle"><input type="checkbox" data-pref="noSpicy" ${p.noSpicy ? "checked" : ""}> Hide Level 3+</label>
      <label class="toggle"><input type="checkbox" data-pref="noOwned" ${p.noOwned ? "checked" : ""}> Hide books we have</label>
      <label class="toggle"><input type="checkbox" data-pref="ratedOnly" ${p.ratedOnly ? "checked" : ""}> Rated only</label>
      <select data-pref="sort" class="input w-auto py-1 text-sm"><option value="">List order</option><option value="mild" ${p.sort === "mild" ? "selected" : ""}>Mildest first</option></select>
      ${manager ? `<label class="flex min-w-0 items-center gap-2 text-xs text-slate-400">Claimed books go to
        <select data-claim-lib class="input w-auto min-w-0 py-1 text-sm">${libs.map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join("")}
          <option value="new">＋ New library…</option></select></label>` : ""}
    </div>
    <p id="shown" class="mb-2 text-sm text-slate-400"></p>
    <ul id="books" class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3"></ul>`;
  const claimSel = $("[data-claim-lib]", view);
  if (claimSel) claimSel.value = pickLib() || "new";
  let books = data.books;

  const card = (b) => {
    const q = encodeURIComponent(`${b.title} ${b.author || ""}`.trim());
    const amazon = b.link || (b.asin ? `https://www.amazon.com/dp/${encodeURIComponent(b.asin)}` : `https://www.amazon.com/s?k=${q}&i=digital-text`);
    const marks = [b.owned ? `<span class="chip-cat">📚 Yours</span>` : "", b.queued ? `<span class="chip-cat">▶ Up Next</span>` : "",
      b.wished ? `<span class="chip-cat">⭐ Wishlist</span>` : ""].join(" ");
    let actions = "";
    if (b.owned) {
      actions = b.queued || !on(state.user, "queue") ? "" : `<button data-act="queue" class="btn-secondary py-1.5 text-sm">＋ Up Next</button>`;
    } else {
      actions = `${kid ? "" : `<a href="${esc(amazon)}" target="_blank" rel="noopener noreferrer" class="btn-primary py-1.5 text-sm">Claim on Amazon ↗</a>`}
        ${manager ? `<button data-act="claimed" class="btn-secondary py-1.5 text-sm">✓ I claimed it</button>` : ""}
        ${b.wished ? "" : `<button data-act="wish" class="btn-ghost py-1.5 text-sm">⭐ Wishlist</button>`}`;
    }
    return `<li class="card flex gap-3 p-3" data-book="${b.id}">
      <button type="button" data-open class="shrink-0" title="Book details">${coverImg(b.id, "h-20 w-14")}</button>
      <div class="min-w-0 flex-1 space-y-1">
        <button type="button" data-open class="text-left font-semibold leading-snug line-clamp-2">${esc(b.title)}</button>
        <p class="truncate text-xs text-slate-400">${esc(b.author || "Unknown author")}</p>
        <div class="flex flex-wrap gap-1">${classChip(b)} ${contentIcons(b)} ${marks}</div>
        <div class="flex flex-wrap gap-2 pt-1">${actions}</div>
      </div></li>`;
  };
  const paint = () => {
    const q = prefs();
    let list = books.filter((b) => !(q.noSpicy && (b.spice_level ?? 0) >= 3) && !(q.noOwned && b.owned) && !(q.ratedOnly && b.status !== "analyzed"));
    if (q.sort === "mild") list = [...list].sort((a, b) => (a.spice_level ?? 9) - (b.spice_level ?? 9));
    $("#shown", view).textContent = list.length === books.length ? `${books.length} books` : `${list.length} of ${books.length} books shown`;
    $("#books", view).innerHTML = list.map(card).join("") || `<li class="text-sm text-slate-400">No books match these choices.</li>`;
  };
  paint();

  view.onchange = async (e) => {
    const key = e.target.dataset.pref;
    if (key) {
      const q = prefs();
      q[key] = e.target.type === "checkbox" ? e.target.checked : e.target.value;
      savePrefs(q);
      return paint();
    }
    if (e.target.matches("[data-claim-lib]")) {
      if (e.target.value === "new") {
        const name = prompt("Name the library for claimed books:", "Kindle (Amazon)");
        const r = name && (await attempt(() => post("/api/catalogs", { name, source: "drive" })));
        if (!r) return (e.target.value = pickLib());
        e.target.insertAdjacentHTML("afterbegin", `<option value="${r.id}">${esc(r.name)}</option>`);
        e.target.value = String(r.id);
      }
      savePrefs({ ...prefs(), claimLib: e.target.value });
    }
  };
  view.onclick = async (e) => {
    if (e.target.closest("[data-pin]")) {
      if (await attempt(() => post(`/api/events/${id}/pin`, { pinned: !ev.pinned }), ev.pinned ? "Unpinned" : "Pinned: it stays until you discard it")) renderEvent(view, state, id);
      return;
    }
    if (e.target.closest("[data-delete]")) {
      if (confirm(`Discard "${ev.name}"? Books you claimed stay in your libraries.`) && (await attempt(() => del(`/api/events/${id}`), "Event discarded"))) location.hash = "#/events";
      return;
    }
    const li = e.target.closest("[data-book]");
    if (!li) return;
    const bookID = Number(li.dataset.book);
    const b = books.find((x) => x.id === bookID);
    if (e.target.closest("[data-open]")) return openBook(bookID, state, () => renderEvent(view, state, id));
    const act = e.target.closest("[data-act]")?.dataset.act;
    if (act === "queue" && (await attempt(() => post("/api/queue", { book_id: bookID }), "Added to Up Next"))) b.queued = true;
    else if (act === "wish" && (await attempt(() => post(`/api/books/${bookID}/wish`, {}), "Added to the wishlist"))) b.wished = true;
    else if (act === "claimed") {
      const lib = claimSel?.value;
      if (!lib || lib === "new") return toast("Pick the library claimed books go to (above the list)", true);
      if (await attempt(() => post(`/api/events/${id}/books/${bookID}/claim`, { catalog_id: Number(lib) }), `${b.title} is in your library now`)) b.owned = true;
    } else return;
    paint();
  };
}
