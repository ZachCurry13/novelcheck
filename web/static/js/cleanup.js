// Admin → Library cleanup: what is left to tidy, and one button that runs the guarded steps in
// order: tidy titles in Calibre, remove duplicate copies without an EPUB, convert the rest to
// EPUB, remove the other formats where an EPUB exists, then re-read the library. Every step uses
// the same checked tools as before (Content server, Undo for formats), and nothing is deleted
// without a summary first.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const gb = (b) => (b >= 1e9 ? `${(b / 1e9).toFixed(1)} GB` : `${Math.max(1, Math.round(b / 1e6))} MB`);

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

const rows = (c) => [
  { id: "pairs", n: c.pairs, title: "Join duplicates in NovelCheck", note: `${c.pairs} possible pair${c.pairs === 1 ? "" : "s"}. Do this first, by hand: you pick what is the same book.`, btn: "Review…" },
  { id: "titles", n: c.titles, title: "Tidy titles in Calibre", note: `${c.titles} title${c.titles === 1 ? "" : "s"} with track or series numbers.`, btn: "Tidy all" },
  { id: "dups", n: c.dup_count, title: "Remove extra Calibre copies that have no EPUB", note: `${c.dup_count} extra ${c.dup_count === 1 ? "entry" : "entries"} where another entry has the EPUB. They go to Calibre's recycle bin.`, btn: "Remove…" },
  { id: "convert", n: c.convert, title: "Convert to EPUB", note: `${c.convert} book${c.convert === 1 ? "" : "s"} with no EPUB. Calibre converts them one at a time; the originals stay for now.`, btn: "Convert all" },
  { id: "formats", n: c.remove_files, title: "Remove other formats where an EPUB exists", note: `${c.remove_files} file${c.remove_files === 1 ? "" : "s"} (${gb(c.remove_bytes)}). Undo for 7 days under Formats….`, btn: "Remove…" },
];

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
      ${c.configured ? "" : `<p class="rounded-lg bg-amber-950/40 p-3 text-sm text-amber-200">Connect the Calibre Content server first (Admin → Delivery &amp; Services → Calibre Library). Changes to Calibre go through it.</p>`}
      <ul class="space-y-2">${list.map((r) => `<li class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-800/60 p-3">
        <div class="min-w-0 flex-1"><p class="font-semibold">${status[r.id] || (r.n ? "⬜" : "✅")} ${esc(r.title)}</p><p class="text-xs text-slate-400">${r.n ? esc(r.note) : "Nothing to do."}</p></div>
        ${r.n && c.configured ? `<button type="button" data-step="${r.id}" class="btn-secondary py-1 text-sm" ${busy ? "disabled" : ""}>${r.btn}</button>` : ""}</li>`).join("")}</ul>
      <button type="button" data-run class="btn-primary w-full" ${busy || !todo || !c.configured ? "disabled" : ""}>▶ Run all the steps after the first, in order</button>
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
      say(`  ${j.kind}: ${j.done}/${j.total} done${j.failed ? `, ${j.failed} failed` : ""}${j.current ? ` · ${j.current}` : ""}`);
      if (!j.running) return j;
      await sleep(2500);
    }
  };
  const refresh = async () => {
    say("Reading the Calibre library…");
    await attempt(() => post("/api/admin/cleanup/refresh"));
  };
  const steps = {
    pairs: async () => {
      close?.();
      location.hash = "#/duplicates";
      return true;
    },
    titles: async () => !!(await attempt(() => post("/api/admin/title-fixes", { ids: c.title_ids }))) && (say("Titles tidied in Calibre."), true),
    dups: async () => {
      const r = await attempt(() => post("/api/admin/calibre/duplicates/remove", { remove: c.dup_ids }));
      if (r) say(`Removed ${r.removed ?? c.dup_count} extra copies (Calibre's recycle bin keeps them).`);
      return !!r;
    },
    convert: async () => {
      if (!(await attempt(() => post("/api/admin/formats/convert", {})))) return false;
      const j = await job();
      return !j.failed || (say(`${j.failed} could not be converted; those keep their original file.`), true);
    },
    formats: async () => {
      if (!(await attempt(() => post("/api/admin/formats/remove", { keep: ["EPUB"], expected_files: c.remove_files })))) return false;
      const j = await job();
      if (j.failed) say(`${j.failed} files could not be removed.`);
      return true;
    },
  };
  const run = async (id) => {
    busy = true;
    status[id] = "⏳";
    paint();
    const ok = await steps[id]();
    status[id] = ok ? "✅" : "⚠️";
    if (id !== "pairs") await refresh();
    busy = false;
    await reload();
    return ok;
  };
  host.onclick = async (e) => {
    if (e.target.closest("[data-close]")) return close?.();
    const step = e.target.closest("[data-step]")?.dataset.step;
    if (step) {
      const sure = { dups: `Remove ${c.dup_count} extra Calibre entries? They move to Calibre's recycle bin.`, formats: `Remove ${c.remove_files} files (${gb(c.remove_bytes)}) of other formats? You can undo for 7 days under Formats….` }[step];
      if (sure && !confirm(sure)) return;
      return run(step);
    }
    if (e.target.closest("[data-run]")) {
      const todo = ["titles", "dups", "convert", "formats"].filter((k) => rows(c).find((r) => r.id === k).n > 0);
      const text = `This will, in order:\n${todo.map((k, i) => `${i + 1}. ${rows(c).find((r) => r.id === k).title}: ${rows(c).find((r) => r.id === k).note}`).join("\n")}\n\nBooks that can't be converted keep their original file. Continue?`;
      if (!confirm(text)) return;
      log = [];
      for (const k of todo) {
        if (!rows(c).find((r) => r.id === k).n) continue; // an earlier step already took care of it
        say(`Step: ${rows(c).find((r) => r.id === k).title}`);
        if (!(await run(k))) {
          say("Stopped: a step did not finish. Nothing after it was run.");
          return;
        }
        if (k === "convert" || k === "dups") await reload(); // the next step looks at what Calibre has now
      }
      say("Done. Now update the Kindle.");
      toast("Library cleanup finished");
    }
  };
  paint();
}
