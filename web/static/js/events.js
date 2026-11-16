// 🎉 Events (Stuff Your Kindle and other free-book days): the events on now,
// and the archive. Opening one shows its books in the Library, with the
// Library's filters and sorts (eventshelf.js).
import { get } from "./api.js";
import { $, esc, attempt, canManage } from "./ui.js";
import { discoverTabsHTML } from "./discovertabs.js";
import { openNewEvent } from "./eventnew.js";
import { eventWhen, groupEvent, dayOf } from "./eventshelf.js";

export async function renderEvents(view, state) {
  const id = new URLSearchParams(location.hash.split("?")[1] || "").get("id");
  if (id) return location.replace(`#/library?event=${encodeURIComponent(id)}`); // older links
  const manager = canManage(state.user);
  view.innerHTML = `${discoverTabsHTML("events", state.user)}
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><h1 class="page-title">Events</h1>
      ${manager ? `<button data-new class="btn-primary">＋ New event</button>` : ""}</div>
    <p class="mb-4 text-sm text-slate-400">Free-book days such as Stuff Your Kindle: see which books suit your family, which you already have${state.user.role === "restricted" ? "" : ", and claim them on Amazon"}.
      An event moves to the archive when it ends (or after 30 days, unless a parent pins it).</p>
    <ul id="events" class="space-y-2"><li class="text-slate-400">Loading…</li></ul>
    <details id="archive" class="mt-5 hidden"><summary class="cursor-pointer text-sm font-semibold text-slate-300">Archive</summary>
      <ul class="mt-2 space-y-2"></ul></details>`;
  view.querySelector("[data-new]")?.addEventListener("click", openNewEvent);
  const evs = (await attempt(() => get("/api/events"))) || [];
  const on = evs.filter((e) => !e.archived_at);
  const old = evs.filter((e) => e.archived_at);
  const row = (e) => `<li><a href="#/library?event=${e.id}" class="card flex items-center gap-3 hover:ring-indigo-600${e.archived_at ? " opacity-80" : ""}">
      <span class="text-2xl" aria-hidden="true">${e.archived_at ? "🗄" : e.pinned ? "📌" : "🎉"}</span>
      <span class="min-w-0 flex-1"><span class="block font-semibold">${esc(e.name)}</span>
        <span class="block text-xs text-slate-400">${e.books} book${e.books === 1 ? "" : "s"} · ${e.rated} rated · ${eventWhen(e)}</span></span>
      <span aria-hidden="true">›</span></a></li>`;
  // Lists from the same day ("Thrillers · Nov 13", "Mystery · Nov 13") also get one entry for all of them.
  const days = new Map();
  for (const e of on) {
    const d = dayOf(e.name);
    if (d) days.set(d, [...(days.get(d) || []), e]);
  }
  const groupRow = (evs) => {
    const g = groupEvent(evs);
    return `<li><a href="#/library?event=${evs.map((e) => e.id).join(",")}" class="card-outlined flex items-center gap-3 hover:ring-indigo-600">
      <span class="text-2xl" aria-hidden="true">📚</span>
      <span class="min-w-0 flex-1"><span class="block font-semibold">${esc(g.name)}</span>
        <span class="block text-xs text-slate-400">${evs.length} lists · ${g.books} books · search them all at once</span></span>
      <span aria-hidden="true">›</span></a></li>`;
  };
  const shown = new Set();
  const rows = on.map((e) => {
    const d = dayOf(e.name), evs = days.get(d) || [];
    const head = evs.length > 1 && !shown.has(d) ? (shown.add(d), groupRow(evs)) : "";
    return head + row(e);
  });
  $("#events", view).innerHTML = rows.join("")
    || `<li class="card text-sm text-slate-400">No events on now.${manager ? " When a free-book day comes up, press ＋ New event and paste its list or its page's address." : ""}</li>`;
  const archive = $("#archive", view);
  archive.classList.toggle("hidden", !old.length);
  archive.querySelector("summary").textContent = `Archive (${old.length})`;
  archive.querySelector("ul").innerHTML = old.map(row).join("");
}
