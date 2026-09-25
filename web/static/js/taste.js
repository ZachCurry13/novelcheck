// 🎯 Your reading taste: an optional list of library books to mark (want to
// read, read & liked, read & didn't like, not for me). The answers steer
// 💡 Suggested Reads. Nothing here uses AI.
import { get, post, qs } from "./api.js";
import { esc, attempt, classChip } from "./ui.js";
import { seriesText } from "./titlefix.js";

const MARKS = [["want", "📖 Want to read"], ["liked", "❤️ Read & liked"], ["disliked", "👎 Read, didn't like"], ["notwant", "🙅 Not for me"]];
const LABEL = { ...Object.fromEntries(MARKS), up: "👍 Liked a suggestion", down: "👎 Hid a suggestion" };
const WHY = { story: "not my kind of story", author: "not this author", series: "not this series", spicy: "too spicy", read: "already read it" };

function dialog() {
  let d = document.getElementById("taste-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "taste-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

export async function openTaste(onDone) {
  const d = dialog();
  let books = [];
  let marks = [];
  let tab = "rate";
  let changed = false;
  const chosen = new Map(); // book id -> answer given this visit
  const load = async (exclude = []) => {
    const data = await attempt(() => get("/api/taste" + qs({ exclude: exclude.join(",") })));
    if (!data) return null;
    marks = data.marks;
    return data.books;
  };
  const draw = () => {
    d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🎯 Your reading taste</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">Optional: mark any books you know and skip the rest. Your answers shape your 💡 Suggested Reads, and you can change them any time.</p>
      <nav class="flex gap-1 border-b border-slate-800">
        <button data-tab="rate" class="nav-link rounded-b-none ${tab === "rate" ? "active" : ""}">Rate books</button>
        <button data-tab="mine" class="nav-link rounded-b-none ${tab === "mine" ? "active" : ""}">Your answers (${marks.length})</button></nav>
      ${tab === "rate" ? rateHTML(books, chosen) : mineHTML(marks)}
    </div>`;
  };
  const first = await load();
  if (!first) return;
  books = first;
  draw();

  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) {
      d.close();
      if (changed) onDone?.();
      return;
    }
    const t = e.target.closest("[data-tab]")?.dataset.tab;
    if (t) {
      tab = t;
      if (t === "mine") await load(books.map((b) => b.id)); // fresh answers
      return draw();
    }
    if (e.target.closest("[data-more]")) {
      const next = await load(books.map((b) => b.id));
      if (next) {
        books = next;
        d.querySelector("div").scrollTop = 0;
        draw();
      }
      return;
    }
    const btn = e.target.closest("[data-mark]");
    if (btn) {
      const id = Number(btn.closest("[data-book]").dataset.book);
      const mark = chosen.get(id) === btn.dataset.mark ? "" : btn.dataset.mark; // tap again to clear
      if (await attempt(() => post("/api/taste", { book_id: id, mark }))) {
        chosen.set(id, mark);
        changed = true;
        draw();
      }
    }
  };
  d.onchange = async (e) => {
    const sel = e.target.closest("[data-answer]");
    if (!sel) return;
    const m = marks[Number(sel.dataset.answer)];
    const who = m.book_id ? { book_id: m.book_id } : { title: m.title, author: m.author };
    if (await attempt(() => post("/api/taste", { ...who, mark: sel.value === "remove" ? "" : sel.value }))) {
      changed = true;
      await load(books.map((b) => b.id));
      draw();
    }
  };
  d.showModal();
}

function rateHTML(books, chosen) {
  if (!books.length) return `<p class="text-sm text-slate-500">You've been through every book there is to rate. 🎉</p>`;
  return `<ul class="space-y-2">${books.map((b) => {
    const cur = chosen.get(b.id) || "";
    const series = seriesText(b);
    return `<li data-book="${b.id}" class="rounded-lg bg-slate-800/60 p-3">
      <div class="flex flex-wrap items-baseline justify-between gap-2">
        <span class="min-w-0"><b>${esc(b.title)}</b> <span class="text-xs text-slate-400">${esc(b.author || "Unknown author")}${series ? ` · ${esc(series)}` : ""}</span></span>
        ${classChip(b)}</div>
      <div class="mt-2 grid grid-cols-2 gap-1.5 sm:grid-cols-4">${MARKS.map(([k, l]) => `<button data-mark="${k}" aria-pressed="${cur === k}"
        class="${cur === k ? "btn-primary" : "btn-ghost border border-slate-700"} px-2 py-1 text-xs">${l}</button>`).join("")}</div>
    </li>`;
  }).join("")}</ul>
  <div class="flex flex-wrap gap-2"><button data-more class="btn-secondary">Show 20 more</button><button data-close class="btn-primary">Done</button></div>`;
}

function mineHTML(marks) {
  if (!marks.length) return `<p class="text-sm text-slate-500">No answers yet. Mark a few books under Rate books, or use 👍 / 👎 on your suggestions.</p>`;
  return `<ul class="space-y-2">${marks.map((m, i) => {
    const opts = MARKS.map(([k, l]) => [k, l]);
    if (m.mark === "up" || m.mark === "down") opts.unshift([m.mark, LABEL[m.mark] + (WHY[m.reason] ? ` (${WHY[m.reason]})` : "")]);
    return `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-2 text-sm">
      <span class="min-w-0"><b>${esc(m.title)}</b> <span class="block text-xs text-slate-400">${esc(m.author || "Unknown author")}${m.book_id ? "" : " · not in your library"}</span></span>
      <select data-answer="${i}" class="input w-auto py-1 text-xs" aria-label="Your answer for ${esc(m.title)}">
        ${opts.map(([k, l]) => `<option value="${k}" ${k === m.mark ? "selected" : ""}>${esc(l)}</option>`).join("")}
        <option value="remove">✕ Remove</option></select></li>`;
  }).join("")}</ul>`;
}
