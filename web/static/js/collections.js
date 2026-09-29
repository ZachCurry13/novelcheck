// 📚 Collections: shelves across libraries, made by a parent or filled by the
// AI, the AI's weekly ideas (parents keep or drop them), and the seasonal
// shelves. A collection opens in the Library, filtered to its books.
import { get, post, del } from "./api.js";
import { $, esc, attempt, canManage } from "./ui.js";
import { coverImg } from "./covers.js";
import { openNewCollection, openAICollection } from "./collectionai.js";
import { shelfTabsHTML } from "./shelftabs.js";

const covers = (c) => `<span class="flex gap-1">${(c.covers || []).map((id) => coverImg(id, "h-16 w-11")).join("")}</span>`;
const kindChip = (c) => (c.kind === "manual" ? `<span class="chip-cat">✋ ${esc(c.created_by || "Parent")}</span>` : `<span class="chip-flag">✨ AI</span>`);

export async function renderCollections(view, state) {
  const manager = canManage(state.user);
  view.innerHTML = `${shelfTabsHTML("collections")}
    <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
      <h1 class="text-2xl font-bold">📚 Collections</h1>
      ${manager ? `<div class="flex flex-wrap gap-2"><button data-act="new" class="btn-secondary">＋ New collection</button>
        <button data-act="ai" class="btn-primary">✨ Describe one for the AI</button></div>` : ""}
    </div>
    <div id="ideas" class="mb-6"></div>
    <div id="cols" class="mb-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3"></div>
    <section id="seasons"></section>`;
  const data = await attempt(() => get("/api/collections"));
  if (!data) return;
  const ideas = data.collections.filter((c) => c.kind === "idea");
  const cols = data.collections.filter((c) => c.kind !== "idea");

  $("#ideas", view).innerHTML = manager && ideas.length ? `<h2 class="mb-2 text-lg font-semibold">💡 Ideas from the AI</h2>
    <div class="grid gap-3 sm:grid-cols-2">${ideas.map((c) => `<div class="card space-y-2 ring-amber-800/60" data-col="${c.id}">
      <div class="flex items-start gap-3"><span class="text-2xl" aria-hidden="true">${esc(c.icon)}</span>
        <div class="min-w-0 flex-1"><p class="font-semibold">${esc(c.name)}</p><p class="text-xs text-slate-400">${esc(c.theme)} · ${c.books} books</p></div></div>
      ${covers(c)}
      <div class="flex flex-wrap gap-2"><button data-act="keep" class="btn-primary py-1.5 text-sm">Keep</button>
        <a href="#/library?collection=${c.id}" class="btn-secondary py-1.5 text-sm">Look</a>
        <button data-act="drop" class="btn-ghost py-1.5 text-sm">Drop</button></div></div>`).join("")}</div>` : "";

  $("#cols", view).innerHTML = cols.map((c) => `<a href="#/library?collection=${c.id}" class="card flex flex-col gap-2 hover:ring-indigo-600">
      <div class="flex items-start gap-3"><span class="text-3xl" aria-hidden="true">${esc(c.icon)}</span>
        <div class="min-w-0 flex-1"><p class="font-semibold leading-snug">${esc(c.name)}</p>
          <p class="text-xs text-slate-400">${c.books} book${c.books === 1 ? "" : "s"}${c.description ? ` · ${esc(c.description)}` : ""}</p></div>
        ${manager ? kindChip(c) : ""}</div>
      ${covers(c)}</a>`).join("")
    || `<p class="card text-sm text-slate-400">No collections yet.${manager ? " Make one by hand, or describe one and let the AI fill it." : ""}</p>`;

  $("#seasons", view).innerHTML = `<h2 class="mb-2 text-lg font-semibold">🗓️ Seasonal shelves</h2>
    <p class="mb-2 text-sm text-slate-400">Found by words in titles, tags and descriptions${manager ? ", or build one with the AI for a better pick" : ""}.</p>
    <ul class="space-y-2">${data.seasons.map((s) => `<li class="card flex flex-wrap items-center gap-3 py-2">
      <a href="#/library?season=${s.key}" class="flex min-w-0 flex-1 items-center gap-3"><span class="text-2xl" aria-hidden="true">${s.icon}</span>
        <span class="min-w-0"><span class="block font-semibold">${esc(s.name)}</span>
          <span class="block text-xs text-slate-400">${s.now ? "In season now · " : ""}${s.collection ? "your AI-built collection" : "found by words"}</span></span></a>
      ${manager ? `<button data-build="${s.key}" class="btn-ghost py-1.5 text-sm">✨ ${s.collection ? "Build again" : "Build with AI"}</button>` : ""}</li>`).join("")}</ul>
    ${manager && !data.ideas_on ? `<p class="mt-3 text-xs text-slate-500">Weekly AI ideas are off (Admin → AI &amp; Scans → Automatic rating).</p>` : ""}`;

  const reload = () => renderCollections(view, state);
  view.onclick = async (e) => {
    const act = e.target.closest("[data-act]")?.dataset.act;
    const id = e.target.closest("[data-col]")?.dataset.col;
    const build = e.target.closest("[data-build]")?.dataset.build;
    if (act === "new") openNewCollection(reload);
    else if (act === "ai") openAICollection({}, reload);
    else if (act === "keep" && id) (await attempt(() => post(`/api/collections/${id}/keep`), "Kept")) && reload();
    else if (act === "drop" && id) (await attempt(() => del(`/api/collections/${id}`), "Dropped")) && reload();
    else if (build) {
      const s = data.seasons.find((x) => x.key === build);
      openAICollection({ theme: s.theme, name: s.name, icon: s.icon, season: s.key }, reload);
    }
  };
}

// shelfBanner heads the Library when it shows a collection or a seasonal
// shelf: its name, and for parents what they can do with it.
export async function shelfBanner(host, { collection, season, seasons }, state, onChange) {
  const manager = canManage(state.user);
  const clear = `<a href="#/library" class="btn-ghost py-1.5 text-sm" title="Show all books">✕ All books</a>`;
  if (collection) {
    const c = await get(`/api/collections/${collection}`).catch(() => null);
    if (!c) return (host.innerHTML = "");
    host.innerHTML = `<div class="card mb-3 flex flex-wrap items-center gap-2">
      <span class="text-2xl" aria-hidden="true">${esc(c.icon)}</span>
      <div class="min-w-0 flex-1"><p class="font-semibold">${esc(c.name)}${c.kind === "idea" ? ` <span class="chip-closed">💡 idea</span>` : ""}</p>
        <p class="text-xs text-slate-400">${esc(c.description || c.theme || "")}</p></div>
      ${manager ? `${c.theme ? `<button data-b="more" class="btn-secondary py-1.5 text-sm">✨ Find more</button>` : ""}
        <button data-b="edit" class="btn-ghost py-1.5 text-sm">✏️ Edit</button>
        <button data-b="delete" class="btn-ghost py-1.5 text-sm" title="Delete this collection (the books stay)">🗑</button>` : ""}
      ${clear}</div>`;
    host.onclick = async (e) => {
      const b = e.target.closest("[data-b]")?.dataset.b;
      if (b === "more") openAICollection({ collection: c }, onChange);
      else if (b === "edit") openNewCollection(onChange, c);
      else if (b === "delete" && confirm(`Delete the collection "${c.name}"? Its books stay in your libraries.`)) {
        if (await attempt(() => del(`/api/collections/${c.id}`), "Collection deleted")) location.hash = "#/collections";
      }
    };
    return;
  }
  const s = (seasons || []).find((x) => x.key === season);
  if (!s) return (host.innerHTML = "");
  host.innerHTML = `<div class="card mb-3 flex flex-wrap items-center gap-2">
    <span class="text-2xl" aria-hidden="true">${s.icon}</span>
    <div class="min-w-0 flex-1"><p class="font-semibold">${esc(s.name)}</p>
      <p class="text-xs text-slate-400">${s.collection ? "Your AI-built collection" : "Found by words in titles, tags and descriptions"}</p></div>
    ${manager ? `<button data-b="build" class="btn-secondary py-1.5 text-sm">✨ ${s.collection ? "Build again" : "Build with AI"}</button>` : ""}
    ${clear}</div>`;
  host.onclick = (e) => {
    if (e.target.closest('[data-b="build"]')) openAICollection({ theme: s.theme, name: s.name, icon: s.icon, season: s.key }, onChange);
  };
}
