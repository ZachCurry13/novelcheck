// Library → Find duplicates → "Listed twice": entries that may be the same book (for example one
// added to a reading list before the e-book arrived, or an old and a new edition). Each pair shows
// both books' details and the evidence for and against, how sure NovelCheck is, and which one it
// would keep. You can join them keeping either one: the joined book shows that one's title and
// details, and everything (Nightstand, wishlist, ratings, copies) moves onto it. The doubtful
// pairs ask before joining.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { coverImg } from "./covers.js";
import { openBook } from "./bookdialog.js";

const BADGES = {
  sure: ["✅ Very likely the same book", "text-emerald-300"],
  likely: ["🟡 Probably the same book", "text-amber-300"],
  check: ["🔴 Check carefully: these may be different books", "text-rose-300"],
};

export const badge = (p) => `<p class="text-sm font-semibold ${BADGES[p.confidence][1]}">${BADGES[p.confidence][0]}</p>`;

export const reasons = (p) => `<ul class="space-y-0.5 text-xs">${(p.reasons || []).map((r) => `<li class="${r.good ? "text-emerald-300" : "text-amber-300"}">${r.good ? "✓" : "⚠"} ${esc(r.text)}</li>`).join("")}</ul>`;

export const confirmJoin = (p) => p.confidence !== "check" || confirm("These may be different books (for example a book and its spin-off). Join them anyway?");

// suggestion says which one to keep, when one stands out ("the left one: its file is newer").
export const suggestion = (p) => (p.suggest ? `<p class="text-xs text-sky-300">👉 Suggested to keep: <b>${p.suggest === "a" ? "the first" : "the second"}</b>, because ${esc(p.suggest_why)}.</p>` : "");

const size = (b) => (b >= 1 << 20 ? `${(b / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`);
const day = (t) => (t ? new Date(t.length > 10 ? t : t + "T00:00:00").toLocaleDateString() : "");
const series = (b) => (b.series ? `<p class="text-sky-300">📚 ${esc(b.series)}${b.series_index ? ` #${b.series_index}` : ""}</p>` : b.series_index ? `<p class="text-sky-300">📚 Book ${b.series_index}</p>` : "");

// side is one book of the pair: its details, and a button to keep this one.
const side = (b, which, p) => `<div class="flex min-w-0 flex-1 gap-3 rounded-lg ${p.suggest === which ? "bg-sky-950/40 ring-1 ring-sky-700" : "bg-slate-800/60"} p-3 text-sm">
    <div class="w-14 shrink-0">${coverImg(b.id, "h-20 w-14")}</div><div class="min-w-0 flex-1 space-y-1">
    <p class="break-words"><button type="button" data-open="${b.id}" class="text-left font-semibold underline decoration-dotted" title="Open this book to look at it">${esc(b.title)}</button></p><p class="text-slate-400">${esc(b.author || "No author")}</p>${series(b)}
    ${b.isbn ? `<p class="text-xs text-slate-500">ISBN ${esc(b.isbn)}</p>` : ""}
    ${b.snippet ? `<p class="text-xs italic text-slate-400">${esc(b.snippet)}</p>` : `<p class="text-xs text-slate-600">No description</p>`}
    <p class="flex flex-wrap gap-1 text-xs">${b.owned ? `<span class="chip-none">In a library</span>` : `<span class="chip-pending">Not in a library</span>`}
      ${b.formats ? b.formats.split(",").map((f) => `<span class="chip-fmt">${esc(f)}</span>`).join(" ") : ""}</p>
    <p class="text-xs text-slate-400">${b.added ? `${b.added_exact ? "Added to the library" : "First seen by NovelCheck"} ${esc(day(b.added))}` : "Not added to a library"}${b.modified ? ` · file changed in Calibre ${esc(day(b.modified))}` : ""}${b.size ? ` · file ${size(b.size)}` : ""}${b.readers ? ` · on ${b.readers} Nightstand${b.readers === 1 ? "" : "s"}` : ""}${b.wanted ? ` · ${b.wanted} wanting it` : ""}</p>
    <button type="button" data-same="merge" data-keep="${which}" class="btn-primary mt-1 py-1 text-sm">Join, keep this one</button></div></div>`;

const card = (p) => `<li class="card space-y-2" data-a="${p.a.id}" data-b="${p.b.id}" data-conf="${p.confidence}">
    ${badge(p)}<div class="flex flex-col gap-2 sm:flex-row">${side(p.a, "a", p)}${side(p.b, "b", p)}</div>${reasons(p)}${suggestion(p)}
    <div class="flex flex-wrap gap-2"><button type="button" data-same="not" class="btn-ghost py-1 text-sm">Not the same</button></div></li>`;

export async function renderSameBooks(host, onChange, state) {
  const r = await attempt(() => get("/api/admin/same-books"));
  if (!r) return;
  const pairs = r.pairs || [];
  if (!pairs.length) {
    host.innerHTML = `<p class="text-sm text-slate-400">No books are listed twice. When one is added to a list before it's in the library, NovelCheck joins them by itself once the library has it, but only when it is sure.</p>`;
    return;
  }
  const by = (c) => pairs.filter((p) => p.confidence === c);
  const sure = [...by("sure"), ...by("likely")];
  const check = by("check");
  host.innerHTML = `<p class="mb-2 text-sm text-slate-400">${pairs.length} possible pair${pairs.length === 1 ? "" : "s"}, the surest first. Read the evidence under each pair, then press <b>Join, keep this one</b> on the book you want to remain: the joined book shows that one's title and details (for example a newer edition), and everything moves onto it, ratings included. <b>Not the same</b> hides the pair for good. NovelCheck only joins pairs by itself when it is very sure.</p>
    <ul class="space-y-3">${sure.map(card).join("")}</ul>
    ${check.length ? `<details class="mt-3 rounded-lg border border-rose-900/50 p-2"><summary class="cursor-pointer text-sm text-rose-300">🔴 ${check.length} pair${check.length === 1 ? "" : "s"} to check carefully (often a book and its spin-off)</summary>
      <ul class="mt-2 space-y-3">${check.map(card).join("")}</ul></details>` : ""}`;
  host.onclick = async (e) => {
    const open = e.target.closest("[data-open]");
    if (open) return openBook(Number(open.dataset.open), state);
    const btn = e.target.closest("[data-same]");
    const li = e.target.closest("li[data-a]");
    if (!btn || !li) return;
    const act = btn.dataset.same;
    const a = Number(li.dataset.a);
    const b = Number(li.dataset.b);
    const p = pairs.find((x) => x.a.id === a && x.b.id === b);
    if (act === "merge" && p && !confirmJoin(p)) return;
    const keep = btn.dataset.keep ? (btn.dataset.keep === "a" ? a : b) : 0;
    if (await attempt(() => post(`/api/admin/same-books/${act}`, { a, b, keep }), act === "merge" ? "Joined" : "Okay, kept apart")) {
      li.remove();
      onChange?.();
      if (!host.querySelector("li[data-a]")) renderSameBooks(host, onChange, state);
    } else toast("That didn't work", true);
  };
}
