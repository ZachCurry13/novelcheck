// Admin → Library cleanup: what is left to tidy, and one button that runs the steps in order: tidy
// titles in Calibre, convert books to EPUB, remove the extra Calibre copies of a book (keeping its
// best one), remove the other formats where an EPUB exists, then re-read the library. Before every
// step a review window lists exactly what would change, with a tick box for each item, and nothing
// is changed until you confirm. Each step uses the same checked tools as before (Content server,
// Undo for formats).
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { review } from "./cleanreview.js";

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const gb = (b) => (b >= 1e9 ? `${(b / 1e9).toFixed(1)} GB` : `${Math.max(1, Math.round(b / 1e6))} MB`);
const plural = (n, one, many) => `${n} ${n === 1 ? one : many || one + "s"}`;

function dialog() {
  let d = document.getElementById("cleanup-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "cleanup-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

// The steps, in the order they run. Converting comes before removing copies, so a book is
// never judged on a file that is about to get a better one.
const rows = (c) => [
  { id: "pairs", n: c.pairs, title: "Join duplicates in NovelCheck", note: `${plural(c.pairs, "possible pair")}. Do this first, by hand: you pick what is the same book.`, btn: "Review…" },
  { id: "titles", n: c.titles, title: "Tidy titles in Calibre", note: `${plural(c.titles, "title")} with track or series numbers.`, btn: "Review…" },
  { id: "convert", n: c.convert, title: "Convert to EPUB", note: `${plural(c.convert, "book")} with no EPUB. Calibre converts them one at a time; the originals stay until the last step.${c.convert_failed ? ` ${plural(c.convert_failed, "book")} did not convert before: see why, or leave them as they are, under Each tool → Formats….` : ""}`, btn: "Review…" },
  { id: "dups", n: c.dup_count, title: "Remove extra Calibre copies of a book", note: `${plural(c.dup_count, "extra entry", "extra entries")}: the same book in Calibre more than once. Each book keeps its best copy (EPUB first); the rest go to Calibre's recycle bin.`, btn: "Review…" },
  { id: "formats", n: c.remove_files, title: "Remove other formats where an EPUB exists", note: `${plural(c.remove_files, "file")} (${gb(c.remove_bytes)}). Undo for 7 days under Formats….`, btn: "Review…" },
];

const fmts = (e) => (e?.files || []).map((f) => `${f.format.toUpperCase()}${f.size ? ` ${gb(f.size)}` : ""}`).join(", ") || "no files";

// What each step shows in its review window, and what it does with what stays ticked.
const reviews = {
  titles: async () => {
    const r = await attempt(() => get("/api/admin/title-fixes"));
    if (!r) return null;
    return { title: "Tidy titles in Calibre", intro: "Calibre's title is changed to the tidy one, and Calibre renames the book's folder to match. Untick a title to leave it.", confirm: "Tidy the ticked titles",
      items: r.fixes.map((f) => ({ id: f.id, label: `${f.calibre_title} → ${f.title}`, detail: f.author || "" })) };
  },
  convert: async () => {
    const r = await attempt(() => get("/api/admin/formats"));
    if (!r) return null;
    return { title: "Convert to EPUB", intro: "Calibre converts each book on this list, one at a time; this can take a while. The original file stays until the last step. A book that fails keeps its file. Untick a book to leave it.", confirm: "Convert the ticked books",
      items: (r.convert || []).map((f) => ({ id: f.book_id, label: f.title, detail: `${f.author || ""} · ${String(f.format).toUpperCase()} → EPUB` })),
      more: r.convert_count > (r.convert || []).length ? `…and ${r.convert_count - r.convert.length} more, left for another run.` : "" };
  },
  dups: async () => {
    const r = await attempt(() => get("/api/admin/calibre/duplicates"));
    if (!r) return null;
    const items = [];
    for (const g of r.groups || []) {
      const keep = g.entries.find((e) => e.calibre_id === g.keep);
      for (const e of g.entries) {
        if (e.calibre_id !== g.keep) items.push({ id: e.calibre_id, label: g.title, detail: `${g.author || ""} · removes Calibre #${e.calibre_id} (${fmts(e)}) · keeps #${g.keep} (${fmts(keep)})`, warn: e.doubt || "" });
      }
    }
    return { title: "Remove extra Calibre copies", intro: "These have the same title and author as another Calibre entry. NovelCheck can't open the books to compare them, so check the list: an entry whose files look different from the copy that stays (much bigger or smaller, or with a PDF or audiobook of its own) is marked and left unticked. For each ticked entry the best copy stays (EPUB first, then more files, then the larger) and the entry listed here goes to Calibre's recycle bin (in Calibre: Remove books → Restore recently deleted).", confirm: "Remove the ticked copies", items };
  },
  formats: async () => {
    const r = await attempt(() => get("/api/admin/formats"));
    if (!r) return null;
    return { title: "Remove other formats", fixed: true, intro: "Each book here has an EPUB, and only its other e-book files (MOBI, AZW3 and the like) are removed, into Calibre's recycle bin; you can undo for 7 days under Formats…. PDFs, comics and audiobooks always stay. A book with no EPUB is never touched, and a book whose EPUB is missing or empty is skipped. All or nothing: to keep a file, tick more formats under Formats….", confirm: `Remove ${plural(r.file_count, "file")}`,
      items: (r.groups || []).map((g) => ({ id: g.book_id, label: g.title, detail: `keeps ${g.keep.concat(g.stays || []).join(", ")} · removes ${g.remove.map((f) => `${f.format.toUpperCase()}${f.size ? ` ${gb(f.size)}` : ""}`).join(", ")}` })),
      more: r.group_count > (r.groups || []).length ? `…and ${r.group_count - r.groups.length} more books, not listed here.` : "" };
  },
};

// openCleanup shows the cleanup in a dialog (from the Calibre Library card).
export async function openCleanup() {
  const d = dialog();
  d.showModal();
  await mountCleanup(d, () => d.close());
}

// mountCleanup fills host with the cleanup steps; close is given when host is a dialog.
export async function mountCleanup(host, close) {
  host.innerHTML = `<div class="p-5"><p class="text-slate-400">Looking at your library…</p></div>`;
  let c = await attempt(() => get("/api/admin/cleanup"));
  if (!c) return close?.();
  let busy = false;
  let status = {};
  let log = [];
  const say = (m) => {
    log.push(m);
    paint();
  };
  const paint = () => {
    const list = rows(c);
    const todo = list.filter((r) => r.id !== "pairs" && r.n > 0).length;
    host.innerHTML = `<div class="${close ? "max-h-[88vh] overflow-y-auto p-5" : ""} space-y-3">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🧹 ${close ? "Library cleanup" : "The guided cleanup"}</h2>
        ${close ? `<button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button>` : ""}</div>
      <p class="text-sm text-slate-400">Every step shows you exactly what it will change first, and you can leave items out. Nothing changes until you confirm.</p>
      ${c.configured ? "" : `<p class="rounded-lg bg-amber-950/40 p-3 text-sm text-amber-200">Connect the Calibre Content server first (Admin → Delivery &amp; Services → Calibre Library). Changes to Calibre go through it.</p>`}
      <ul class="space-y-2">${list.map((r, i) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-3">
        <div class="min-w-0 flex-1"><p class="font-semibold">${status[r.id] || (r.n ? "⬜" : "✅")} ${i}. ${esc(r.title)}</p><p class="text-xs text-slate-400">${r.n ? esc(r.note) : "Nothing to do."}</p></div>
        ${r.n && c.configured ? `<button type="button" data-step="${r.id}" class="btn-secondary py-1 text-sm" ${busy ? "disabled" : ""}>${r.btn}</button>` : ""}</li>`).join("")}</ul>
      <button type="button" data-run class="btn-primary w-full" ${busy || !todo || !c.configured ? "disabled" : ""}>▶ Go through steps 1 to 4, reviewing each</button>
      <p class="text-xs text-slate-400">Then on the Kindle: <b>Update plugin</b> → <b>Report books on this device</b> → <b>Tidy book names</b> → <b>Download all Nightstand</b>.</p>
      ${log.length ? `<pre class="max-h-40 overflow-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-2 text-xs text-slate-300">${esc(log.join("\n"))}</pre>` : ""}</div>`;
  };
  const reload = async () => {
    c = (await attempt(() => get("/api/admin/cleanup"))) || c;
    paint();
  };
  const job = async () => {
    for (;;) {
      const j = await get("/api/admin/formats/job");
      say(`  ${j.kind}: ${j.done}/${j.total} done${j.skipped ? `, ${j.skipped} skipped` : ""}${j.failed ? `, ${j.failed} failed` : ""}${j.current ? ` · ${j.current}` : ""}`);
      if (!j.running) return j;
      await sleep(2500);
    }
  };
  const refresh = async () => {
    say("Reading the Calibre library…");
    const r = await attempt(() => post("/api/admin/cleanup/refresh"));
    if (r?.running) say("Still reading (a big library takes a few minutes). It carries on by itself; the numbers here update when you come back.");
  };
  const failures = (j) => {
    if (j.failed) say(`${plural(j.failed, "book")} failed. Open Settings → Developer → "Last format job" in the bug report for the reasons (or the Formats… window).`);
  };
  // What each step does with the ticked ids it was given.
  const steps = {
    pairs: async () => {
      close?.();
      location.hash = "#/duplicates";
      return true;
    },
    titles: async (ids) => {
      const r = await attempt(() => post("/api/admin/title-fixes", { ids }));
      if (!r) return false;
      say(`Tidied ${plural(r.fixed, "title")} in Calibre.`);
      (r.skipped || []).forEach((m) => say(`  not changed: ${m}`));
      return true;
    },
    convert: async (ids) => {
      if (!(await attempt(() => post("/api/admin/formats/convert", { book_ids: ids.map(Number) })))) return false;
      say(`Converting ${plural(ids.length, "book")} in the background. You can leave this page; NovelCheck sends a notice when it is done. Come back to review the next steps.`);
      return "background";
    },
    dups: async (ids) => {
      const r = await attempt(() => post("/api/admin/calibre/duplicates/remove", { remove: ids }));
      if (r) say(`Removed ${plural(r.removed ?? ids.length, "extra copy", "extra copies")} (Calibre's recycle bin keeps them).${r.skipped ? ` ${plural(r.skipped, "entry", "entries")} left alone: already gone, or Calibre's title differs (see the 🔔 notices).` : ""}`);
      return !!r;
    },
    formats: async () => {
      if (!(await attempt(() => post("/api/admin/formats/remove", { keep: ["EPUB"], expected_files: c.remove_files })))) return false;
      failures(await job());
      return true;
    },
  };
  // run reviews a step, then does it. "cancelled" means you left it out at the review.
  const run = async (id) => {
    let ids = [];
    if (reviews[id]) {
      const spec = await reviews[id]();
      if (!spec) return false;
      if (!spec.items.length) {
        say(`Nothing to review for: ${rows(c).find((r) => r.id === id).title}.`);
        return true;
      }
      ids = await review(spec);
      if (!ids || !ids.length) {
        status[id] = "⏭";
        paint();
        return "cancelled";
      }
    }
    busy = true;
    status[id] = "⏳";
    paint();
    const ok = await steps[id](ids);
    status[id] = ok === "background" ? "⚙️" : ok ? "✅" : "⚠️";
    if (id !== "pairs" && ok !== "background") await refresh();
    busy = false;
    await reload();
    return ok;
  };
  host.onclick = async (e) => {
    if (e.target.closest("[data-close]")) return close?.();
    const step = e.target.closest("[data-step]")?.dataset.step;
    if (step) return run(step);
    if (e.target.closest("[data-run]")) {
      if (!confirm("You'll go through each step in turn. Before anything is changed you see the list for that step, and you can leave items out or stop. Start?")) return;
      log = [];
      for (const k of ["titles", "convert", "dups", "formats"]) {
        if (!rows(c).find((r) => r.id === k).n) continue; // an earlier step already took care of it
        say(`Step: ${rows(c).find((r) => r.id === k).title}`);
        const ok = await run(k);
        if (ok === "background") {
          busy = false;
          say("Stopped here on purpose: the next steps (extra copies, other formats) depend on the conversions, so review them once the notice says it is done.");
          return;
        }
        if (ok === "cancelled") {
          say("Stopped: you left this step out. Nothing after it was run.");
          return;
        }
        if (!ok) {
          say("Stopped: a step did not finish. Nothing after it was run.");
          return;
        }
      }
      say("Done. Now update the Kindle.");
      toast("Library cleanup finished");
    }
  };
  paint();
  // A job started earlier (maybe from another page, or before you left this one) is still running on the
  // server: show it, and follow it to the end.
  if (c.job?.running) {
    busy = true;
    say("A job is still running on the server (it does not stop when you leave this page):");
    failures(await job().catch(() => ({})));
    busy = false;
    await reload();
  }
}
