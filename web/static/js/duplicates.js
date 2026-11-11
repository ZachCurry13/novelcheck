// Library → Find duplicates: books that are in Calibre more than once. Each book shows its copies side by
// side (cover, formats with sizes, last change) and you pick the one to keep; the others go to Calibre's
// recycle bin through the Content server. Books whose copies look alike come first and can be done in one go;
// the ones with a doubt (see dupdoubt.go) are handled one at a time.
import { get, post } from "./api.js";
import { $, $$, esc, attempt, toast, can } from "./ui.js";
import { renderSameBooks } from "./samebooks.js";

const MB = 1 << 20;
const fmtSize = (b) => (!b ? "" : b >= MB ? `${(b / MB).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`);
const copies = (n) => `${n} cop${n === 1 ? "y" : "ies"}`;
const day = (s) => {
  const d = new Date(String(s || "").replace(" ", "T"));
  return s && !isNaN(d) && d.getFullYear() > 1990 ? d.toLocaleDateString([], { year: "numeric", month: "short", day: "numeric" }) : "";
};

function copyHTML(g, e, canRemove) {
  const keep = e.calibre_id === g.keep;
  const files = e.files.length ? e.files.map((f) => `<span class="chip-fmt">${esc(f.format.toUpperCase())}${f.size ? ` ${fmtSize(f.size)}` : ""}</span>`).join(" ")
    : `<span class="text-xs text-slate-400">an empty Calibre entry (no book file in it)</span>`;
  return `<label class="dup-copy${keep ? " keep" : ""}" data-copy="${esc(e.calibre_id)}">
    <img class="dup-cover" loading="lazy" alt="" src="/api/books/${g.book_id}/cover?copy=${encodeURIComponent(e.calibre_id)}">
    <span class="min-w-0 space-y-1">
      <span class="block text-sm"><b>Calibre #${esc(e.calibre_id)}</b>${day(e.modified) ? ` <span class="text-xs text-slate-400">· changed ${esc(day(e.modified))}</span>` : ""}</span>
      <span class="flex flex-wrap gap-1">${files}</span>
      ${e.doubt ? `<span class="block text-xs text-amber-300">⚠️ ${esc(e.doubt)}</span>` : ""}
      <span data-detail class="block space-y-0.5 text-xs text-slate-300"></span>
      ${canRemove ? `<span class="toggle text-sm"><input type="radio" name="keep-${g.book_id}" value="${esc(e.calibre_id)}" ${keep ? "checked" : ""}> <span data-keep-label>${keep ? "Keep this one" : "Keep this one instead"}</span></span>` : keep ? `<span class="chip-none">✓ Suggested keep</span>` : ""}
    </span></label>`;
}

function bookHTML(g, canRemove) {
  const others = g.entries.length - 1;
  return `<section class="card space-y-2" data-book="${g.book_id}">
    <h3 class="font-semibold">${esc(g.title)} <span class="font-normal text-slate-400">· ${esc(g.author || "Unknown author")}</span></h3>
    <p class="text-xs text-slate-400">Suggested to keep #${esc(g.keep)}: ${esc(g.why || "it is the best copy")}.</p>
    <div class="dup-copies">${g.entries.map((e) => copyHTML(g, e, canRemove)).join("")}</div>
    <button type="button" data-compare class="btn-ghost py-1 text-sm">🔍 Compare what Calibre has for each</button>
    ${canRemove ? `<button type="button" data-one class="${g.sure ? "btn-secondary" : "btn-ghost"} py-1 text-sm">Remove the other ${others === 1 ? "copy" : `${others} copies`}${g.sure ? "" : " anyway"}</button>` : ""}
  </section>`;
}

export async function renderDuplicates(view, state) {
  const isAdmin = can(state.user, "calibre"); // may remove the extra copies
  view.innerHTML = `
    <a href="#/library" class="text-sm text-slate-400 underline">← Library</a>
    <h1 class="mb-1 mt-2 text-2xl font-bold">Duplicates in Calibre</h1>
    <p class="mb-4 max-w-3xl text-sm text-slate-400">Books that are in Calibre more than once (same title and author). For each, pick the copy to keep;
      the others go to Calibre's recycle bin (in Calibre: Remove books → Restore recently deleted).</p>
    <div class="mb-3 flex flex-wrap items-center gap-3 text-sm">
      <button id="dup-recheck" class="btn-secondary py-1">🔄 Check again</button>
      <span id="dup-synced" class="text-slate-400"></span>
    </div>
    <details id="dup-twice" class="card mb-4"><summary class="cursor-pointer font-semibold">Books listed twice in NovelCheck (a different list: entries to join, nothing is removed from Calibre)</summary><div id="same-books" class="mt-2"></div></details>
    <div id="dup-result"></div>
    <div id="dup-body"><p class="text-slate-400">Looking for duplicates…</p></div>`;
  // Open by itself when there are pairs to look at (the guided cleanup sends people here for them).
  Promise.resolve(renderSameBooks($("#same-books", view), undefined, state)).then(() => ($("#dup-twice", view).open = !!$("#same-books li[data-a]", view)));
  const body = $("#dup-body", view);
  const result = $("#dup-result", view);
  let data = null;
  let gone = 0;

  // recheck re-reads the Calibre library, then reloads the list.
  $("#dup-recheck", view).addEventListener("click", async (e) => {
    e.target.disabled = true;
    e.target.textContent = "Reading Calibre…";
    const r = await attempt(() => post("/api/admin/calibre-sync?wait=1"));
    e.target.disabled = false;
    e.target.textContent = "🔄 Check again";
    if (r) load();
  });

  const group = (id) => data.groups.find((g) => String(g.book_id) === String(id));
  // others lists the copies of a book that are not the one picked to keep.
  const others = (card) => {
    const keep = $("input[type=radio]:checked", card)?.value;
    return group(card.dataset.book).entries.map((e) => e.calibre_id).filter((id) => id !== keep);
  };
  const say = (text, good = true) => (result.innerHTML = `<div class="card mb-4 ${good ? "text-emerald-200" : "text-amber-200"}">${esc(text)}</div>`);
  const bar = () => {
    const sure = $$("[data-sure] [data-book]", body);
    const b = $("#dup-bar", body);
    if (b) b.classList.toggle("hidden", !sure.length);
    if (b && sure.length) $("span", b).textContent = `${sure.length} book${sure.length === 1 ? "" : "s"} whose copies look the same.`;
    $$("[data-group]", body).forEach((sec) => {
      const n = $$("[data-book]", sec).length;
      sec.classList.toggle("hidden", !n); // a group that is done goes away
      $("[data-count]", sec).textContent = n;
    });
  };

  async function load() {
    data = await attempt(() => get("/api/admin/calibre/duplicates"));
    if (!data) return;
    $("#dup-synced", view).textContent = `Calibre last read ${data.synced_at ? new Date(data.synced_at).toLocaleString() : "never"}`;
    if (!data.groups.length) {
      body.innerHTML = `<div class="card text-slate-300">✓ No duplicates found. Every book is in Calibre only once.</div>`;
      return;
    }
    const sure = data.groups.filter((g) => g.sure);
    const check = data.groups.filter((g) => !g.sure);
    const cant = data.can_remove ? "" : `<p class="mb-3 text-sm text-slate-400">${isAdmin
      ? `To remove copies from here, turn on <a href="#/admin?tab=delivery" class="underline">Admin → Delivery &amp; Services → Calibre Library → One-click removal</a>.`
      : "Only an admin can remove books from Calibre."} <button type="button" id="dup-copy" class="underline">Copy a Calibre search for the suggested removals</button></p>`;
    body.innerHTML = `${cant}
      ${sure.length ? `<section data-group data-sure class="mb-6 space-y-3"><h2 class="text-xl font-semibold">✅ Looks like the same book (<span data-count></span>)</h2>
        <p class="text-sm text-slate-400">Each copy's files match the one that stays in kind and size. NovelCheck can't open the books to compare the text, so glance at the covers.</p>
        ${sure.map((g) => bookHTML(g, data.can_remove)).join("")}</section>` : ""}
      ${check.length ? `<section data-group class="space-y-3"><h2 class="text-xl font-semibold">⚠️ Check these (<span data-count></span>)</h2>
        <p class="text-sm text-slate-400">Something differs: maybe another edition, an omnibus, or a copy with a PDF or audiobook of its own. These are never removed in one go.</p>
        ${check.map((g) => bookHTML(g, data.can_remove)).join("")}</section>` : ""}
      ${data.can_remove ? `<div id="dup-bar" class="save-bar hidden"><span></span><button type="button" data-all class="btn-primary">Remove their extra copies</button></div>` : ""}`;
    bar();
  }

  body.addEventListener("change", (e) => {
    const card = e.target.closest("[data-book]");
    if (!card || e.target.type !== "radio") return;
    $$(".dup-copy", card).forEach((c) => {
      const on = c.dataset.copy === e.target.value;
      c.classList.toggle("keep", on);
      $("[data-keep-label]", c).textContent = on ? "Keep this one" : "Keep this one instead";
    });
  });

  body.addEventListener("click", async (e) => {
    if (e.target.closest("#dup-copy")) {
      const search = data.groups.flatMap((g) => g.entries.filter((x) => x.calibre_id !== g.keep && !x.doubt).map((x) => `id:${x.calibre_id}`)).join(" or ");
      try {
        await navigator.clipboard.writeText(search);
        toast("Copied. Paste it into Calibre's search bar, select all, and press Delete.");
      } catch {
        prompt("Copy this into Calibre's search bar:", search);
      }
      return;
    }
    const cmp = e.target.closest("[data-compare]");
    if (cmp) return void compare(cmp.closest("[data-book]"), cmp);
    const one = e.target.closest("[data-one]");
    if (one) {
      const card = one.closest("[data-book]");
      const g = group(card.dataset.book);
      const ids = others(card);
      const doubts = g.entries.filter((x) => ids.includes(x.calibre_id) && x.doubt).map((x) => `#${x.calibre_id}: ${x.doubt}`);
      if (doubts.length && !confirm(`Remove ${copies(ids.length)} of “${g.title}” although:\n\n${doubts.join("\n")}\n\nThey go to Calibre's recycle bin.`)) return;
      one.disabled = true;
      one.textContent = "Removing…";
      // "later": the answer comes at once and Calibre is read again in the background.
      const r = await attempt(() => post("/api/admin/calibre/duplicates/remove", { remove: ids, later: true }));
      if (!r) { // refused: the list here is behind Calibre, so read it again
        one.disabled = false;
        one.textContent = "Try again";
        setTimeout(load, 2500);
        return;
      }
      gone += r.removed;
      card.remove();
      bar();
      say(`✓ Removed ${copies(gone)} so far (in Calibre's recycle bin).`);
      if (!$("[data-book]", body)) { // the last one: read Calibre again, then show what is left (if anything)
        body.innerHTML = `<p class="text-slate-400">Reading Calibre again…</p>`;
        await attempt(() => post("/api/admin/calibre-sync?wait=1"));
        load();
      }
      return;
    }
    const all = e.target.closest("[data-all]");
    if (!all) return;
    const cards = $$("[data-sure] [data-book]", body);
    const ids = cards.flatMap(others);
    if (!ids.length || !confirm(`Move ${copies(ids.length)} of ${cards.length} book${cards.length === 1 ? "" : "s"} to Calibre's recycle bin? The copy marked "Keep this one" of each book stays.`)) return;
    all.disabled = true;
    all.textContent = "Removing…";
    const r = await attempt(() => post("/api/admin/calibre/duplicates/remove", { remove: ids }));
    all.disabled = false;
    all.textContent = "Remove their extra copies";
    if (!r) return;
    await load(); // the server read Calibre again before answering
    say(`✓ Removed ${copies(r.removed)} (in Calibre's recycle bin)${r.skipped ? `; ${r.skipped} left alone: already gone, or Calibre's title differs (see the 🔔 notices)` : ""}.`, !r.skipped);
  });
  await load();

  // compare shows, under each copy, what Calibre itself has for it; what differs from the copy to keep is marked.
  async function compare(card, btn) {
    const g = group(card.dataset.book);
    btn.disabled = true;
    btn.textContent = "Asking Calibre…";
    const r = await attempt(() => get("/api/admin/calibre/duplicates/details?ids=" + g.entries.map((x) => x.calibre_id).join(",")));
    btn.disabled = false;
    btn.textContent = "🔍 Compare what Calibre has for each";
    if (!r) return;
    const keep = $("input[type=radio]:checked", card)?.value || g.keep;
    const base = r.details[keep] || {};
    const FIELDS = [["title", "Title in Calibre"], ["authors", "By"], ["series", "Series"], ["ids", "ISBN and ids"], ["tags", "Tags"], ["about", "About"]];
    $$(".dup-copy", card).forEach((c) => {
      const d = r.details[c.dataset.copy];
      $("[data-detail]", c).innerHTML = !d ? `<span class="text-slate-500">Calibre has nothing for this entry.</span>` : FIELDS.map(([k, label]) => {
        const differs = c.dataset.copy !== keep && (d[k] || "") !== (base[k] || "");
        return d[k] || differs ? `<span class="block break-words"><span class="text-slate-500">${label}:</span> ${esc(d[k] || "(none)")}${differs ? ` <span class="text-amber-300">· differs</span>` : ""}</span>` : "";
      }).join("") + (d.open ? `<a href="${esc(d.open)}" target="_blank" rel="noopener noreferrer" class="underline">Open in Calibre-Web ↗</a>` : "");
    });
  }
}
