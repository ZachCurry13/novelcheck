// 💡 Suggested Reads at the bottom of Up Next: books from the family's
// library picked for this reader (and, when the admin allows, a few the
// family doesn't own), with 👍/👎 to steer what comes next.
import { get, post, del, qs } from "./api.js";
import { esc, attempt, toast, classChip } from "./ui.js";
import { seriesText } from "./titlefix.js";
import { openBook } from "./bookdialog.js";
import { coverImg, noCover } from "./covers.js";
import { openTaste } from "./taste.js";
import { on } from "./modules.js";

const SUBTITLE = {
  free: "Matched from your library",
  ai: "✨ Picked by AI from your library, refreshed daily",
  ai_outside: "✨ Picked by AI, refreshed daily",
};

export async function renderSuggestions(host, state, onQueued, polls = 0) {
  // "From": every library, or just one (say, a Kindle someone imported). Remembered on this device.
  let from = "";
  try {
    from = localStorage.getItem("nc:suggest-from") || "";
  } catch {
    /* private mode */
  }
  const [data, cats] = await Promise.all([attempt(() => get("/api/suggestions" + qs({ catalog: from }))), get("/api/catalogs").catch(() => [])]);
  const libs = (cats || []).filter((c) => c.name !== "Looked up" && c.book_count > 0);
  if (!data || !host.isConnected) return;
  const taste = on(state.user, "taste");
  const liked = new Set(); // cards given a 👍 this visit: they show their add button
  const asking = new Set(); // cards just given a 👎: they ask "Why not?"
  const outside = data.outside.map((o, i) => ({ ...o, key: `o${i}` }));

  const draw = () => {
    const cards = data.items.map((it) => (asking.has(`b${it.book.id}`) ? whyCard(`b${it.book.id}`, it.book) : libraryCard(it, liked.has(`b${it.book.id}`))))
      .concat(outside.map((o) => (asking.has(o.key) ? whyCard(o.key, {}) : outsideCard(o, liked.has(o.key)))));
    host.innerHTML = `<section class="mt-10">
      <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
        <h2 class="text-lg font-bold">💡 Suggested Reads${taste ? ` <button data-taste class="btn-ghost ml-1 px-2 py-0.5 text-xs font-normal">🎯 Your taste</button>` : ""}</h2>
        <p class="flex flex-wrap items-center gap-2 text-xs text-slate-500">${libs.length > 1 ? `<label>From <select data-from class="input w-auto py-0.5 text-xs">
          <option value="">All libraries</option>${libs.map((c) => `<option value="${c.id}" ${String(c.id) === from ? "selected" : ""}>${esc(c.name)}</option>`).join("")}</select></label>` : ""}
          <span>${data.refreshing ? "✨ The AI is picking new suggestions…" : SUBTITLE[data.mode]}</span></p>
      </div>
      ${cards.length ? `<ul class="flex max-w-full snap-x gap-3 overflow-x-auto pb-2">${cards.join("")}</ul>`
        : `<p class="text-sm text-slate-500">Add a few books to Up Next (or finish some) and suggestions will show up here.${taste ? ` Or <button data-taste class="underline">🎯 mark a few books you know</button> to get started.` : ""}</p>`}
      ${data.up + data.down ? `<p class="mt-1 text-xs text-slate-500">Your feedback so far: 👍 ${data.up} · 👎 ${data.down} ·
        <button data-reset class="underline">Start over</button></p>` : ""}
    </section>`;
  };
  draw();

  const drop = (key) => {
    data.items = data.items.filter((it) => `b${it.book.id}` !== key);
    outside.splice(0, outside.length, ...outside.filter((o) => o.key !== key));
  };
  host.onchange = (e) => {
    if (!e.target.matches("[data-from]")) return;
    try {
      localStorage.setItem("nc:suggest-from", e.target.value);
    } catch {
      /* private mode */
    }
    renderSuggestions(host, state, onQueued);
  };
  host.onclick = async (e) => {
    const card = e.target.closest("[data-sg]");
    if (e.target.closest("[data-taste]")) return openTaste(() => renderSuggestions(host, state, onQueued));
    if (e.target.closest("[data-reset]")) {
      if (confirm("Forget all your 👍 and 👎? Books you hid can be suggested again.")) {
        if (await attempt(() => del("/api/suggestions/votes"), "Starting over")) renderSuggestions(host, state, onQueued);
      }
      return;
    }
    if (!card) return;
    const key = card.dataset.sg;
    const item = data.items.find((it) => `b${it.book.id}` === key);
    const out = outside.find((o) => o.key === key);
    const who = item ? { book_id: item.book.id } : { title: out.title, author: out.author };
    if (e.target.closest("[data-open]") && item) return openBook(item.book.id, state, onQueued);
    const vote = e.target.closest("[data-vote]")?.dataset.vote;
    if (vote === "1") {
      if (await attempt(() => post("/api/suggestions/vote", { ...who, vote: 1 }))) {
        liked.add(key);
        data.up++;
        draw();
      }
    } else if (vote === "-1") {
      // Hidden straight away; "Why not?" (optional) teaches the suggestions what to avoid.
      if (await attempt(() => post("/api/suggestions/vote", { ...who, vote: -1 }))) {
        asking.add(key);
        data.down++;
        draw();
      }
    } else if (e.target.closest("[data-why]")) {
      const reason = e.target.closest("[data-why]").dataset.why;
      if (reason && !(await attempt(() => post("/api/suggestions/vote", { ...who, vote: -1, reason }), "Got it. Your suggestions will learn from that."))) return;
      asking.delete(key);
      drop(key);
      draw();
    } else if (e.target.closest("[data-add]") && item) {
      if (await attempt(() => post("/api/queue", { book_id: item.book.id }), "Added to Up Next")) {
        drop(key);
        draw();
        onQueued?.();
      }
    } else if (e.target.closest("[data-wish]") && out) {
      const ok = await attempt(() => post("/api/suggestions/wish", { title: out.title, author: out.author, reason: out.reason }),
        "Added to the family wishlist. NovelCheck is rating it now.");
      if (ok) {
        drop(key);
        draw();
      }
    }
  };
  // The AI works in the background: look again shortly (a local AI can take a few minutes).
  if (data.refreshing && polls < 8) setTimeout(() => host.isConnected && renderSuggestions(host, state, onQueued, polls + 1), 20000);
}

const votes = (liked, addButton) => `<div class="mt-auto flex items-center gap-1 pt-1">
  ${liked ? addButton : ""}
  <span class="flex-1"></span>
  ${liked ? `<span class="text-xs text-emerald-300">👍 Liked</span>` : `<button data-vote="1" class="btn-ghost px-2" title="More like this" aria-label="More like this">👍</button>`}
  <button data-vote="-1" class="btn-ghost px-2" title="Not for me: hide it and show fewer like it" aria-label="Not for me">👎</button></div>`;

function libraryCard({ book: b, reason, by_ai: byAI }, liked) {
  const series = seriesText(b);
  return `<li data-sg="b${b.id}" class="card flex w-64 shrink-0 snap-start flex-col gap-1.5">
    <div class="flex gap-2"><button data-open class="shrink-0" aria-label="Open ${esc(b.title)}">${coverImg(b.id, "h-20 w-14")}</button><div class="min-w-0">
    <button data-open class="text-left font-semibold leading-tight line-clamp-2 hover:underline">${esc(b.title)}</button>
    <p class="line-clamp-2 text-sm text-slate-400">${esc(b.author || "Unknown author")}${series ? ` · <span class="text-sky-300">${esc(series)}</span>` : ""}</p></div></div>
    <div class="flex flex-wrap gap-1">${classChip(b)}</div>
    <p class="text-xs text-slate-300">${byAI ? "✨ " : ""}${esc(reason)}</p>
    ${votes(liked, `<button data-add class="btn-primary py-1 text-sm">＋ Up Next</button>`)}
  </li>`;
}

function outsideCard(o, liked) {
  return `<li data-sg="${o.key}" class="card flex w-64 shrink-0 snap-start flex-col gap-1.5 border border-dashed border-slate-700">
    <div class="flex gap-2">${noCover()}<div class="min-w-0">
    <p class="font-semibold leading-tight line-clamp-2">${esc(o.title)}</p>
    <p class="line-clamp-2 text-sm text-slate-400">${esc(o.author || "Unknown author")}</p></div></div>
    <p><span class="chip-none" title="Not in your library, so NovelCheck hasn't rated it yet">Not in your library</span></p>
    <p class="text-xs text-slate-300">✨ ${esc(o.reason)}</p>
    ${votes(liked, `<button data-wish class="btn-secondary py-1 text-sm">⭐ Wishlist</button>`)}
  </li>`;
}

// "Why not?" after a 👎; each answer teaches the suggestions something different.
const WHY = [["story", "Not my kind of story"], ["author", "Not this author"], ["series", "Not this series"],
  ["spicy", "Too spicy"], ["read", "Already read it"]];

function whyCard(key, b) {
  const opts = WHY.filter(([k]) => (k !== "series" || b.series) && (k !== "spicy" || (b.spice_level ?? -1) > 0));
  return `<li data-sg="${key}" class="card flex w-64 shrink-0 snap-start flex-col gap-2">
    <p class="text-sm font-semibold">👎 Hidden. Why not?</p>
    <p class="text-xs text-slate-400">Optional, but it helps your next suggestions.</p>
    <div class="flex flex-wrap gap-1.5">${opts.map(([k, l]) => `<button data-why="${k}" class="btn-ghost border border-slate-700 px-2 py-1 text-xs">${l}</button>`).join("")}</div>
    <button data-why="" class="mt-auto self-start text-xs text-slate-400 underline">Skip</button>
  </li>`;
}
