// An event in the Library (#/library?event=5): the Library's own filters and
// sorts on the event's books, a banner saying when it ends (parents can edit,
// pin, archive or restore, and delete it), and on each card Claim on Amazon
// and "✓ I claimed it", which files the book in the library parents pick.
import { get, post, del } from "./api.js";
import { esc, attempt, toast, canManage } from "./ui.js";
import { openEditEvent } from "./eventnew.js";

const CLAIM_LIB = "nc:event-claim-lib";
const store = (v) => {
  try {
    if (v === undefined) return localStorage.getItem(CLAIM_LIB) || "";
    localStorage.setItem(CLAIM_LIB, v);
  } catch {
    /* private mode */
  }
  return "";
};

// when says when an event ends or went to the archive.
export function eventWhen(ev) {
  const at = (t) => new Date(t).toLocaleString([], { weekday: "short", month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
  if (ev.archived_at) return `archived ${new Date(ev.archived_at).toLocaleDateString([], { month: "short", day: "numeric" })}`;
  if (ev.ends_at) return `ends ${at(ev.ends_at)}`;
  return ev.pinned ? "pinned" : `archived in ${ev.days_left} day${ev.days_left === 1 ? "" : "s"}`;
}

// eventShelf fills host with the event's banner and returns what the
// Library's cards need: extra(book) and click(e) (true when it was ours).
export async function eventShelf(host, id, state, reload) {
  const manager = canManage(state.user);
  const kid = state.user.role === "restricted";
  const [data, cats] = await Promise.all([get(`/api/events/${id}`).catch(() => null), manager ? get("/api/catalogs").catch(() => []) : []]);
  if (!data) {
    host.innerHTML = `<div class="card mb-3 text-sm text-slate-400">That event is gone. <a href="#/events" class="underline">All events</a></div>`;
    return null;
  }
  const ev = data.event;
  const marks = new Map(data.books.map((b) => [b.id, b]));
  const libs = (cats || []).filter((c) => c.source !== "calibre" && !c.physical);
  const pick = () => (libs.some((c) => String(c.id) === store()) ? store() : libs[0] ? String(libs[0].id) : "new");
  const btn = (b, label, title = "") => `<button data-e="${b}" class="btn-ghost py-1.5 text-sm"${title ? ` title="${title}"` : ""}>${label}</button>`;
  host.innerHTML = `<div class="card mb-3 space-y-2">
    <div class="flex items-start gap-2">
      <span class="text-2xl" aria-hidden="true">${ev.archived_at ? "🗄" : ev.pinned ? "📌" : "🎉"}</span>
      <div class="min-w-0 flex-1"><p class="font-semibold">${esc(ev.name)}</p>
        <p class="text-xs text-slate-400">${ev.books} books · ${ev.rated} rated${ev.rated < ev.books && !ev.archived_at ? " (the rest are being rated, first in line)" : ""} · ${eventWhen(ev)}
          ${ev.source_url && !kid ? ` · <a href="${esc(ev.source_url)}" target="_blank" rel="noopener noreferrer" class="underline">event page ↗</a>` : ""}</p></div>
      <a href="#/events" class="btn-ghost shrink-0 py-1.5 text-sm" title="All events">✕<span class="hidden sm:inline"> All events</span></a>
    </div>
    ${manager ? `<div class="flex flex-wrap gap-1">${btn("edit", "✏️ Edit", "Rename it or change when it ends")}
      ${ev.archived_at ? btn("restore", "↩ Restore") : `${btn("pin", ev.pinned ? "Unpin" : "📌 Pin", "Pinned events without an end stay out of the archive")}${btn("archive", "🗄 Archive")}`}
      ${btn("delete", "🗑 Delete", "Delete this event (books you claimed stay)")}</div>` : ""}
    ${ev.archived_at ? `<p class="text-xs text-amber-300">This event is over. Its books and ratings are kept here; Amazon's prices may have changed.</p>` : ""}
    ${manager && !ev.archived_at ? `<label class="flex min-w-0 flex-wrap items-center gap-2 text-xs text-slate-400">Books you claim go to
      <select data-claim-lib class="input w-auto min-w-0 py-1 text-sm">${libs.map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join("")}
        <option value="new">＋ New library…</option></select></label>` : ""}
  </div>`;
  const sel = host.querySelector("[data-claim-lib]");
  if (sel) sel.value = pick();

  host.onchange = async (e) => {
    if (!e.target.matches("[data-claim-lib]")) return;
    if (e.target.value === "new") {
      const name = prompt("Name the library for claimed books:", "Kindle (Amazon)");
      const r = name && (await attempt(() => post("/api/catalogs", { name, source: "drive" })));
      if (!r) return (e.target.value = pick());
      e.target.insertAdjacentHTML("afterbegin", `<option value="${r.id}">${esc(r.name)}</option>`);
      libs.push(r);
      e.target.value = String(r.id);
    }
    store(e.target.value);
  };
  host.onclick = async (e) => {
    const b = e.target.closest("[data-e]")?.dataset.e;
    const again = () => eventShelf(host, id, state, reload).then(() => reload());
    if (b === "edit") openEditEvent(ev, again);
    else if (b === "pin" && (await attempt(() => post(`/api/events/${id}/pin`, { pinned: !ev.pinned }), ev.pinned ? "Unpinned" : "Pinned"))) again();
    else if (b === "archive" && (await attempt(() => post(`/api/events/${id}/archive`, { archived: true }), "Archived: find it under All events"))) location.hash = "#/events";
    else if (b === "restore" && (await attempt(() => post(`/api/events/${id}/archive`, { archived: false }), "Restored"))) again();
    else if (b === "delete" && confirm(`Delete "${ev.name}" for good? Books you claimed stay in your libraries. (Archive keeps it instead.)`)) {
      if (await attempt(() => del(`/api/events/${id}`), "Event deleted")) location.hash = "#/events";
    }
  };

  const extra = (book) => {
    const m = marks.get(book.id);
    if (!m) return "";
    if (m.owned) return `<div data-ev class="flex flex-wrap gap-1"><span class="chip-cat">📚 Yours</span></div>`;
    const q = encodeURIComponent(`${m.title} ${m.author || ""}`.trim());
    const amazon = m.link || (m.asin ? `https://www.amazon.com/dp/${encodeURIComponent(m.asin)}` : `https://www.amazon.com/s?k=${q}&i=digital-text`);
    return `<div data-ev class="flex flex-wrap gap-2">
      ${kid ? "" : `<a href="${esc(amazon)}" target="_blank" rel="noopener noreferrer" class="btn-primary py-1.5 text-sm">${ev.archived_at ? "Amazon ↗" : "Claim on Amazon ↗"}</a>`}
      ${manager ? `<button data-ev-act="claimed" class="btn-secondary py-1.5 text-sm">✓ I claimed it</button>` : ""}
      ${m.wished ? `<span class="chip-cat">⭐ Wishlist</span>` : `<button data-ev-act="wish" class="btn-ghost py-1.5 text-sm">⭐ Wishlist</button>`}
    </div>`;
  };
  const click = async (e) => {
    const box = e.target.closest("[data-ev]");
    if (!box) return false;
    const act = e.target.closest("[data-ev-act]")?.dataset.evAct;
    const bookID = Number(e.target.closest("[data-book]")?.dataset.book);
    const m = marks.get(bookID);
    if (!act || !m) return true; // the Amazon link, or a gap between the buttons
    e.stopPropagation();
    if (act === "wish" && (await attempt(() => post(`/api/books/${bookID}/wish`, {}), "Added to the wishlist"))) m.wished = true;
    else if (act === "claimed") {
      const lib = host.querySelector("[data-claim-lib]")?.value;
      if (!lib || lib === "new") return toast("Pick the library claimed books go to (at the top)", true), true;
      if (await attempt(() => post(`/api/events/${id}/books/${bookID}/claim`, { catalog_id: Number(lib) }), `${m.title} is in your library now`)) m.owned = true;
    }
    box.outerHTML = extra(m);
    return true;
  };
  return { extra, click };
}
