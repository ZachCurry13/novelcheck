// Admin → Calibre Library → Formats: keep the chosen formats of each Calibre
// book and move the others to Calibre's recycle bin (Undo for a week), and
// have Calibre convert books with no EPUB. Everything goes through Calibre's
// Content server, one job at a time.
import { get, post } from "./api.js";
import { esc, attempt, toast, fmtNum } from "./ui.js";

const MB = 1 << 20;
const fmtSize = (b) => (!b ? "" : b >= 1024 * MB ? `${(b / 1024 / MB).toFixed(1)} GB` : b >= MB ? `${(b / MB).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`);
const when = (s) => new Date(s.replace(" ", "T") + (/(Z|[+-]\d\d:?\d\d)$/.test(s) ? "" : "Z"))
  .toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
const count = (n, one, many = one + "s") => `${fmtNum(n)} ${n === 1 ? one : many}`;
const JOB = { remove: "Removing formats", restore: "Putting formats back", convert: "Converting to EPUB" };
const row = "rounded-lg bg-slate-800/60 p-2 text-sm";

function dialog() {
  let d = document.getElementById("formats-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "formats-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

function jobHTML(j) {
  if (!j?.kind) return "";
  if (j.running) {
    const at = j.done + j.skipped + j.failed;
    return `<div class="space-y-2 rounded-lg bg-indigo-950/40 p-3 ring-1 ring-indigo-700/50">
      <div class="flex items-center justify-between gap-2"><b class="text-sm">${JOB[j.kind]}… ${fmtNum(at)} of ${fmtNum(j.total)}</b>
        <button type="button" data-f="stop" class="btn-ghost py-1 text-xs">Stop</button></div>
      <div class="h-2 overflow-hidden rounded bg-slate-800"><div class="h-full bg-indigo-500" data-pct="${j.total ? Math.round((at / j.total) * 100) : 0}"></div></div>
      ${j.current ? `<p class="truncate text-xs text-slate-400">${esc(j.current)}${j.kind === "convert" && j.percent ? ` · ${j.percent}%` : ""}</p>` : ""}</div>`;
  }
  return `<div class="space-y-1 rounded-lg p-3 text-sm ${j.failed || /^Stopped/.test(j.note) ? "bg-amber-950/40 text-amber-100" : "bg-emerald-950/40 text-emerald-100"}">
    <p>${esc(j.note)}${j.skipped ? ` ${count(j.skipped, "skipped")}: Calibre had changed ${j.skipped === 1 ? "it" : "them"} already.` : ""}</p>
    ${j.failed ? `<details><summary class="cursor-pointer">${count(j.failed, "didn't work", "didn't work")}</summary>
      <ul class="mt-1 list-disc space-y-1 pl-5 text-xs">${j.errors.map((e) => `<li>${esc(e)}</li>`).join("")}</ul></details>` : ""}</div>`;
}

function keepHTML(data, busy) {
  const fmts = Object.entries(data.formats).sort((a, b) => b[1] - a[1]);
  if (!fmts.length) return `<p class="text-sm text-slate-400">No Calibre books with files yet.</p>`;
  const books = data.groups.map((g) => `<li class="${row}"><b>${esc(g.title)}</b>
    <span class="block text-xs text-slate-400">${esc(g.author || "Unknown author")} · keeps ${g.keep.map(esc).join(", ")} · removes
      ${g.remove.map((f) => `${esc(f.format.toUpperCase())}${f.size ? ` (${fmtSize(f.size)})` : ""}`).join(", ")}</span></li>`).join("");
  return `<section class="space-y-3">
    <h3 class="font-semibold">Keep only these formats</h3>
    <p class="text-sm text-slate-400">Each book that has a format you keep loses its other formats, into Calibre's recycle bin. Books without one are left alone.</p>
    <div class="flex flex-wrap gap-2">${fmts.map(([f, n]) => `<label class="flex items-center gap-2 ${row} py-1">
      <input type="checkbox" name="keep" value="${esc(f)}" ${data.keep.includes(f) ? "checked" : ""}> ${esc(f)}
      <span class="text-xs text-slate-500">${fmtNum(n)}</span></label>`).join("")}</div>
    ${data.file_count ? `<p class="text-sm">This removes <b>${count(data.file_count, "file")}</b> from ${count(data.group_count, "book")}${data.bytes ? ` (${fmtSize(data.bytes)})` : ""}.</p>
      <details><summary class="cursor-pointer text-sm text-sky-300">See the books${data.group_count > data.groups.length ? ` (the first ${fmtNum(data.groups.length)})` : ""}</summary>
        <ul class="mt-2 space-y-1">${books}</ul></details>
      <button type="button" data-f="remove" class="btn-danger" ${busy}>Remove ${count(data.file_count, "file")}</button>`
    : `<p class="text-sm text-slate-400">Nothing to remove with these ticks.</p>`}</section>`;
}

function convertHTML(data, busy) {
  const n = data.convert_count;
  const books = data.convert.map((f) => `<li class="flex items-center justify-between gap-2 ${row}">
    <span class="min-w-0"><b class="block truncate">${esc(f.title)}</b><span class="text-xs text-slate-400">${esc(f.author || "Unknown author")} · from ${esc(f.format.toUpperCase())}</span></span>
    <button type="button" data-convert="${f.book_id}" class="btn-ghost shrink-0 py-1 text-xs" ${busy}>Convert</button></li>`).join("");
  return `<section class="space-y-3 border-t border-slate-800 pt-4">
    <h3 class="font-semibold">Convert to EPUB</h3>
    ${n ? `<p class="text-sm text-slate-400">${count(n, "book has", "books have")} no EPUB but a file Calibre can convert. Calibre makes the EPUB with its own conversion settings, one book at a time, on the computer it runs on. The original file stays.</p>
      <details><summary class="cursor-pointer text-sm text-sky-300">See the books${n > data.convert.length ? ` (the first ${fmtNum(data.convert.length)})` : ""}</summary>
        <ul class="mt-2 space-y-1">${books}</ul></details>
      <button type="button" data-f="convert-all" class="btn-secondary" ${busy}>Convert ${n === 1 ? "it" : `all ${fmtNum(n)}`}</button>`
    : `<p class="text-sm text-slate-400">Every book Calibre could convert has an EPUB. 🎉</p>`}</section>`;
}

function undoHTML(data, busy) {
  if (!data.batches.length) return "";
  return `<section class="space-y-3 border-t border-slate-800 pt-4">
    <h3 class="font-semibold">Removed formats</h3>
    <p class="text-sm text-slate-400">Undo puts them back from Calibre's recycle bin, for ${data.undo_days} days.</p>
    ${data.batches.map((b) => `<div class="space-y-2 ${row} p-3">
      <div class="flex flex-wrap items-center justify-between gap-2"><span>${esc(when(b.removed_at))} · ${count(b.files, "file")} (${esc(b.formats.replace(/,/g, ", "))})${b.size ? ` · ${fmtSize(b.size)}` : ""}</span>
        <button type="button" data-undo-batch="${esc(b.batch)}" class="btn-secondary py-1 text-xs" ${busy}>Undo all</button></div>
      <details><summary class="cursor-pointer text-xs text-sky-300">Files</summary><ul class="mt-2 space-y-1">
        ${b.items.map((it) => `<li class="flex items-center justify-between gap-2 text-xs"><span class="flex min-w-0 items-center gap-2"><span class="chip-fmt shrink-0">${esc(it.format)}</span><span class="truncate">${esc(it.title)}</span></span>
          <button type="button" data-undo="${it.id}" class="btn-ghost shrink-0 py-0.5 text-xs" ${busy}>Undo</button></li>`).join("")}
        ${b.files > b.items.length ? `<li class="text-xs text-slate-500">…and ${fmtNum(b.files - b.items.length)} more</li>` : ""}</ul></details></div>`).join("")}</section>`;
}

const noServer = `<p class="rounded-lg bg-amber-950/40 p-3 text-sm text-amber-200">Connect Calibre's Content server first (One-click removal, in this section),
  with a Calibre user allowed to make changes.</p>`;

// openFormats shows the format tools; a running job is followed until it ends.
export async function openFormats() {
  const d = dialog();
  let data = null, keep = null, watching = false, timer = null;
  d.innerHTML = `<div class="max-h-[85vh] space-y-4 overflow-y-auto p-5">
    <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🗂️ Formats in Calibre</h2>
      <button type="button" data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    <div id="fmt-job"></div><div id="fmt-body" class="space-y-4"></div></div>`;
  const bars = () => d.querySelectorAll("[data-pct]").forEach((e) => (e.style.width = e.dataset.pct + "%"));
  const showJob = () => {
    d.querySelector("#fmt-job").innerHTML = watching ? jobHTML(data.job) : "";
    bars();
  };
  const render = () => {
    const busy = data.job.running || !data.configured ? "disabled" : "";
    d.querySelector("#fmt-body").innerHTML = (data.configured ? "" : noServer) + keepHTML(data, busy) + convertHTML(data, busy) + undoHTML(data, busy);
    showJob();
  };
  const poll = async () => {
    clearTimeout(timer);
    if (!d.open) return;
    const j = await get("/api/admin/formats/job").catch(() => null);
    if (j) data.job = j;
    showJob();
    if (!j || j.running) timer = setTimeout(poll, 1500);
    else load(); // the lists changed
  };
  const load = async () => {
    const r = await attempt(() => get("/api/admin/formats" + (keep ? "?keep=" + encodeURIComponent(keep.join(",")) : "")));
    if (!r) return false;
    data = r;
    keep = r.keep;
    watching ||= r.job.running;
    render();
    if (r.job.running) timer = setTimeout(poll, 1500);
    return true;
  };
  const start = async (url, body) => {
    const j = await attempt(() => post(url, body));
    if (!j) return;
    data.job = j;
    watching = true;
    render();
    timer = setTimeout(poll, 800);
  };
  d.onchange = (e) => {
    if (!e.target.matches("[name=keep]")) return;
    const ticked = [...d.querySelectorAll("[name=keep]:checked")].map((c) => c.value);
    if (!ticked.length) {
      e.target.checked = true;
      return toast("Keep at least one format", true);
    }
    keep = ticked;
    load();
  };
  d.onclick = async (e) => {
    const t = e.target.closest("[data-f],[data-convert],[data-undo],[data-undo-batch]");
    const f = t?.dataset.f;
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    if (!t || t.disabled) return;
    if (f === "stop") {
      await attempt(() => post("/api/admin/formats/job/stop"), "Stopping after the current one");
    } else if (f === "remove") {
      if (confirm(`Remove ${count(data.file_count, "file")} from ${count(data.group_count, "book")}, keeping ${keep.join(", ")}? They go to Calibre's recycle bin, and Undo puts them back for ${data.undo_days} days.`)) {
        start("/api/admin/formats/remove", { keep, expected_files: data.file_count });
      }
    } else if (f === "convert-all") {
      if (confirm(`Convert ${count(data.convert_count, "book")} to EPUB? Calibre does one at a time, so a long list takes a while.`)) start("/api/admin/formats/convert", { book_ids: [] });
    } else if (t.dataset.convert) {
      start("/api/admin/formats/convert", { book_ids: [Number(t.dataset.convert)] });
    } else if (t.dataset.undoBatch) {
      if (confirm("Put all of these back in Calibre?")) start("/api/admin/formats/undo", { batch: t.dataset.undoBatch });
    } else if (t.dataset.undo) {
      start("/api/admin/formats/undo", { id: Number(t.dataset.undo) });
    }
  };
  d.onclose = () => clearTimeout(timer);
  if (await load()) d.showModal();
}
