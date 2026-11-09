// Library → Find duplicates → "Listed twice": entries that may be the same book (for example one
// added to a reading list before the e-book arrived). Each pair shows both books' details and
// the evidence for and against, with how sure NovelCheck is. Joining moves everything (Nightstand,
// wishlist, ratings, copies) onto one entry; the doubtful pairs ask before joining.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

const BADGES = {
  sure: ["✅ Very likely the same book", "text-emerald-300"],
  likely: ["🟡 Probably the same book", "text-amber-300"],
  check: ["🔴 Check carefully: these may be different books", "text-rose-300"],
};

export const badge = (p) => `<p class="text-sm font-semibold ${BADGES[p.confidence][1]}">${BADGES[p.confidence][0]}</p>`;

export const reasons = (p) => `<ul class="space-y-0.5 text-xs">${(p.reasons || []).map((r) => `<li class="${r.good ? "text-emerald-300" : "text-amber-300"}">${r.good ? "✓" : "⚠"} ${esc(r.text)}</li>`).join("")}</ul>`;

export const confirmJoin = (p) => p.confidence !== "check" || confirm("These may be different books (for example a book and its spin-off). Join them anyway?");

const series = (b) => (b.series ? `<p class="text-sky-300">📚 ${esc(b.series)}${b.series_index ? ` #${b.series_index}` : ""}</p>` : b.series_index ? `<p class="text-sky-300">📚 Book ${b.series_index}</p>` : "");

const side = (b) => `<div class="min-w-0 flex-1 space-y-1 rounded-lg bg-slate-800/60 p-3 text-sm">
    <p class="break-words font-semibold">${esc(b.title)}</p><p class="text-slate-400">${esc(b.author || "No author")}</p>${series(b)}
    ${b.isbn ? `<p class="text-xs text-slate-500">ISBN ${esc(b.isbn)}</p>` : ""}
    ${b.snippet ? `<p class="text-xs italic text-slate-400">${esc(b.snippet)}</p>` : `<p class="text-xs text-slate-600">No description</p>`}
    <p class="flex flex-wrap gap-1 text-xs">${b.owned ? `<span class="chip-none">In a library</span>` : `<span class="chip-pending">Not in a library</span>`}
      ${b.formats ? b.formats.split(",").map((f) => `<span class="chip-fmt">${esc(f)}</span>`).join(" ") : ""}
      ${b.wanted ? `<span class="text-slate-400">on ${b.wanted} list${b.wanted === 1 ? "" : "s"}</span>` : ""}</p></div>`;

const card = (p) => `<li class="card space-y-2" data-a="${p.a.id}" data-b="${p.b.id}" data-conf="${p.confidence}">
    ${badge(p)}<div class="flex flex-col gap-2 sm:flex-row">${side(p.a)}${side(p.b)}</div>${reasons(p)}
    <div class="flex flex-wrap gap-2"><button type="button" data-same="merge" class="btn-primary py-1 text-sm">Join them</button>
      <button type="button" data-same="not" class="btn-ghost py-1 text-sm">Not the same</button></div></li>`;

export async function renderSameBooks(host, onChange) {
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
  host.innerHTML = `<p class="mb-2 text-sm text-slate-400">${pairs.length} possible pair${pairs.length === 1 ? "" : "s"}, the surest first. Read the evidence under each pair. <b>Join them</b> if it's one book: everything moves onto one entry and the other title is remembered. <b>Not the same</b> hides the pair for good. NovelCheck only joins pairs by itself when it is very sure.</p>
    <ul class="space-y-3">${sure.map(card).join("")}</ul>
    ${check.length ? `<details class="mt-3 rounded-lg border border-rose-900/50 p-2"><summary class="cursor-pointer text-sm text-rose-300">🔴 ${check.length} pair${check.length === 1 ? "" : "s"} to check carefully (often a book and its spin-off)</summary>
      <ul class="mt-2 space-y-3">${check.map(card).join("")}</ul></details>` : ""}`;
  host.onclick = async (e) => {
    const act = e.target.closest("[data-same]")?.dataset.same;
    const li = e.target.closest("li[data-a]");
    if (!act || !li) return;
    const p = pairs.find((x) => x.a.id === Number(li.dataset.a) && x.b.id === Number(li.dataset.b));
    if (act === "merge" && p && !confirmJoin(p)) return;
    const body = { a: Number(li.dataset.a), b: Number(li.dataset.b) };
    if (await attempt(() => post(`/api/admin/same-books/${act}`, body), act === "merge" ? "Joined" : "Okay, kept apart")) {
      li.remove();
      onChange?.();
      if (!host.querySelector("li[data-a]")) renderSameBooks(host, onChange);
    } else toast("That didn't work", true);
  };
}
