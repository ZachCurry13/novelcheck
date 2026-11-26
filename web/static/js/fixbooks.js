// Admin → Library → Fix titles and covers: one list of the books that need something, from every finder: a tidier
// title for Calibre (the name finder), Hardcover's corrections (series, titles, authors, ISBN, descriptions, genres)
// and a missing cover. Each fix has its own Apply; the finders keep running by themselves behind it.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";
import { coverImg } from "./covers.js";
import { openCoverPicker } from "./coverpicker.js";

const FIELD = { title: "Title", authors: "Author", series: "Series", series_index: "Number in series", identifiers: "ISBN", comments: "Description", tags: "Genres" };
const PAGE = 30;

export async function openFixBooks(onDone) {
  const dlg = document.createElement("dialog");
  dlg.className = "dialog max-w-3xl";
  document.body.appendChild(dlg);
  dlg.showModal();
  dlg.addEventListener("close", () => { dlg.remove(); onDone?.(); });
  let shown = PAGE;
  const load = async () => {
    dlg.innerHTML = `<div class="p-4 text-slate-400">Gathering what needs fixing…</div>`;
    const [tf, hc, cv] = await Promise.all([get("/api/admin/title-fixes").catch(() => null), get("/api/admin/hc-clean").catch(() => null),
      get("/api/admin/covers/missing?limit=2000").catch(() => null)]);
    const books = new Map();
    const book = (id, title, author) => books.get(id) || books.set(id, { id, title, author, fixes: [] }).get(id);
    for (const f of tf?.fixes || []) book(f.id, f.title, f.author).fixes.push({ kind: "title", id: f.id, what: "Title in Calibre", from: f.calibre_title, to: f.title });
    for (const s of hc?.suggestions || []) book(s.book_id, s.title, "").fixes.push({ kind: "hc", id: s.id, what: FIELD[s.field] || s.field, from: s.current, to: s.shown, sure: s.confidence });
    for (const b of cv?.books || []) book(b.id, b.title, b.author).fixes.push({ kind: "cover", what: "No cover" });
    const list = [...books.values()];
    const job = cv?.job || {}, hjob = hc?.job || {};
    // what the cover finder is doing or did last, in words: it used to be a button that changed and nothing else
    const jobLine = job.running ? `Finding covers: ${job.checked} of ${job.total}${job.current ? ` · now looking for “${esc(job.current)}”` : ""}${job.found ? ` · found ${job.found} so far` : ""}`
      : job.total ? `Last time it looked at ${job.total} book${job.total === 1 ? "" : "s"} and found ${job.found}${job.skipped ? `; ${job.skipped} were skipped because they were searched this week` : ""}${job.retry ? `; ${job.retry} will be tried again (Google's daily limit)` : ""}. The books listed below have no cover on any site it searches: choose one by hand.` : "";
    if (job.running || hjob.running) setTimeout(() => dlg.open && load(), 3000); // watch it work
    const hs = hc?.suggestions || [];
    const SURE = hs.filter((x) => x.field !== "tags" && x.field !== "series_index" && !/typo/.test(x.confidence || "")); // not genres, not the series number, not a guessed title
    const sureCount = SURE.length + (tf?.fixes || []).length;
    const NAMES = { title: "titles", authors: "authors", series: "series", series_index: "series numbers", identifiers: "ISBNs", comments: "descriptions", tags: "genres" };
    const byField = {};
    for (const x of hs) byField[x.field] = (byField[x.field] || 0) + 1;
    const hline = hjob.running ? `Hardcover: ${hjob.kind === "apply" ? "applying" : "comparing"} ${hjob.done} of ${hjob.total}${hjob.note ? ` · ${esc(hjob.note)}` : ""}` : "";
    const bulk = list.length ? `<div class="space-y-2 rounded-lg border border-slate-700 p-3"><p class="font-semibold">Apply many at once</p>
      <p class="text-sm text-slate-400">Apply writes the change into your Calibre. “Sure” means matched by ISBN or by the exact title and author; genres and series numbers are left for you to choose, because Hardcover is wrong about them more often.${hjob.running ? " <b>The Hardcover job is still running: these buttons work when it finishes.</b>" : ""}</p>
      <p class="flex flex-wrap gap-2"><button type="button" data-fx="apply-sure" class="btn-primary py-1 text-sm" ${hjob.running || !sureCount ? "disabled" : ""}>Apply all the sure ones (${sureCount})</button>
      ${Object.keys(byField).map((f) => `<button type="button" data-fx="apply-field" data-field="${f}" class="btn-secondary py-1 text-xs" ${hjob.running ? "disabled" : ""}>All ${NAMES[f] || f} (${byField[f]})</button>`).join("")}</p></div>` : "";
    dlg.innerHTML = `<div class="space-y-3 p-4"><div class="flex flex-wrap items-center justify-between gap-2"><h3 class="text-lg font-semibold">Fix titles and covers (${list.length})</h3>
        <button type="button" data-close class="btn-ghost">Close</button></div>
      <p class="text-sm text-slate-400">Every book that needs something, with what the finders found. They keep looking by themselves; these buttons start them now.</p>
      ${jobLine || hline ? `<p class="rounded-lg bg-slate-800/60 p-2 text-sm">${[jobLine, hline].filter(Boolean).join("<br>")}</p>` : ""}
      ${bulk}
      ${cv?.left_out ? `<p class="text-sm text-slate-400">${cv.left_out} book${cv.left_out === 1 ? " is" : "s are"} left out of this list: no site has a cover for them. <button type="button" data-fx="unleave" class="underline">Show them again</button></p>` : ""}
      <p class="flex flex-wrap gap-2">${hc?.hardcover ? `<button type="button" data-fx="hc-scan" class="btn-secondary py-1 text-sm" ${hjob.running ? "disabled" : ""}>${hjob.running ? "Comparing with Hardcover…" : "Compare with Hardcover"}</button>` : `<a href="#/settings?s=connections" data-close class="btn-ghost py-1 text-sm">Connect Hardcover to compare</a>`}
        <button type="button" data-fx="covers-find" class="btn-secondary py-1 text-sm" ${job.running ? "disabled" : ""}>${job.running ? `Finding covers (${job.checked} of ${job.total})…` : "Find covers now"}</button>
        ${(tf?.fixes || []).length ? `<button type="button" data-fx="titles-all" class="btn-ghost py-1 text-sm">Apply every title (${tf.fixes.length})</button>` : ""}
        <button type="button" data-fx="review" class="btn-ghost py-1 text-sm">Review covers one by one</button></p>
      <ul class="divide-y divide-slate-800">${list.slice(0, shown).map((b) => `<li class="flex gap-3 py-2">${coverImg(b.id, "h-16 w-11")}
        <div class="min-w-0 flex-1"><p class="font-medium">${esc(b.title)}${b.author ? ` <span class="text-sm text-slate-400">· ${esc(b.author)}</span>` : ""}</p>
        <ul class="mt-1 space-y-1 text-sm">${b.fixes.map((f, i) => `<li class="flex flex-wrap items-center gap-2"><span class="text-slate-300">${esc(f.what)}${f.kind === "cover" ? "" : `: <s class="text-slate-500">${esc(f.from || "–")}</s> → ${esc(f.to)}`}${f.sure ? ` <span class="text-xs text-slate-500">(${esc(f.sure)})</span>` : ""}</span>
          ${f.kind === "cover" ? `<button type="button" data-fx="cover" data-book="${b.id}" class="btn-ghost py-0.5 text-xs">Choose cover</button><button type="button" data-fx="leave" data-book="${b.id}" class="btn-ghost py-0.5 text-xs" title="Not a real book, or one that needs no cover: stop listing it">Not a book: leave it out</button>`
            : `<button type="button" data-fx="apply" data-book="${b.id}" data-i="${i}" class="btn-secondary py-0.5 text-xs">Apply</button>${f.kind === "hc" ? `<button type="button" data-fx="skip" data-book="${b.id}" data-i="${i}" class="btn-ghost py-0.5 text-xs">Skip</button>` : ""}`}</li>`).join("")}</ul></div></li>`).join("")
      || `<li class="py-3 text-slate-400">Nothing waiting. ✓</li>`}</ul>
      ${list.length > shown ? `<button type="button" data-fx="more" class="btn-ghost">Show ${Math.min(PAGE, list.length - shown)} more</button>` : ""}</div>`;
    dlg.onclick = async (e) => {
      if (e.target === dlg || e.target.closest("[data-close]")) return dlg.close();
      const el = e.target.closest("[data-fx]");
      if (!el) return;
      const k = el.dataset.fx, b = books.get(Number(el.dataset.book)), f = b?.fixes[Number(el.dataset.i)];
      let ok = false;
      if (k === "more") { shown += PAGE; return load(); }
      if (k === "review") { dlg.close(); return import("./coverreview.js").then((m) => m.openCoverReview()); }
      if (k === "cover") return openCoverPicker(b, load, { everyone: true });
      if (k === "unleave") ok = await attempt(() => post("/api/admin/covers/leave-out/reset"), "Showing them again");
      if (k === "leave") ok = await attempt(() => post("/api/admin/covers/leave-out", { id: b.id }), "Left out of the cover lists");
      if (k === "apply-sure" || k === "apply-field") {
        const ids = k === "apply-sure" ? SURE.map((x) => x.id) : hs.filter((x) => x.field === el.dataset.field).map((x) => x.id);
        const titles = k === "apply-sure" ? (tf?.fixes || []).map((x) => x.id) : [];
        if (!ids.length && !titles.length) return;
        if (!confirm(`Write ${ids.length + titles.length} change${ids.length + titles.length === 1 ? "" : "s"} into Calibre now?\n\nCalibre keeps no undo for these.`)) return;
        if (titles.length) await attempt(() => post("/api/admin/title-fixes", { ids: titles }), "Titles tidied in Calibre");
        ok = ids.length ? await attempt(() => post("/api/admin/hc-clean/apply", { ids }), "Applying in the background") : true;
      }
      if (k === "hc-scan") ok = await attempt(() => post("/api/admin/hc-clean/scan"), "Comparing with Hardcover in the background");
      if (k === "covers-find") ok = await attempt(() => post("/api/admin/covers/find"), "Looking for covers in the background");
      if (k === "titles-all") ok = await attempt(() => post("/api/admin/title-fixes", { ids: tf.fixes.map((x) => x.id) }), "Titles tidied in Calibre");
      if (k === "apply" && f.kind === "title") ok = await attempt(() => post("/api/admin/title-fixes", { ids: [f.id] }), "Title tidied in Calibre");
      if (k === "apply" && f.kind === "hc") ok = await attempt(() => post("/api/admin/hc-clean/apply", { ids: [f.id] }), "Applied in Calibre");
      if (k === "skip") ok = await attempt(() => post("/api/admin/hc-clean/skip", { ids: [f.id] }), "Skipped");
      if (ok) load();
    };
  };
  load();
}
