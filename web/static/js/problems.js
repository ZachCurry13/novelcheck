// 🐞 "Report a problem or idea" for everyone (no GitHub account needed):
// it goes to the admins, who can pass a NovelCheck bug on with Diagnose.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";
import { copyText } from "./copy.js";
import { diagnoseLater } from "./diagnose.js";
import { when } from "./deepscan.js";

function dialog(id) {
  let d = document.getElementById(id);
  if (!d) {
    d = document.createElement("dialog");
    d.id = id;
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

export function openReportProblem() {
  const d = dialog("report-problem-dialog");
  const page = location.hash || "#/";
  d.innerHTML = `<form class="space-y-3 p-5">
    <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🐞 Report a problem or idea</h2>
      <button type="button" data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    <label class="block"><span class="label">What happened, or what would you like?</span>
      <textarea name="text" required maxlength="4000" rows="5" class="input" placeholder="What were you doing, and what went wrong? Ideas are welcome too."></textarea></label>
    <p class="text-xs text-slate-400">Your admin gets this, with the page you were on (${esc(page)}) and your device type.</p>
    <button class="btn-primary">Send to my admin</button>
  </form>`;
  const form = d.querySelector("form");
  d.onclick = (e) => {
    if (e.target === d || e.target.closest("[data-close]")) d.close();
  };
  form.onsubmit = async (e) => {
    e.preventDefault();
    if (await attempt(() => post("/api/problems", { text: form.text.value, page }), "Sent. Thanks! Your admin will take a look.")) d.close();
  };
  d.showModal();
  form.text.focus();
}

// openProblemReports is the admin's list.
export async function openProblemReports() {
  const d = dialog("problem-reports-dialog");
  let reports = [];
  const load = async () => {
    const data = await attempt(() => get("/api/admin/problems"));
    if (!data) return false;
    reports = data.reports;
    d.innerHTML = `<div class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">🐞 Problems and ideas from your family</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
      <p class="text-sm text-slate-400">If it looks like a NovelCheck bug, <b>🩺 Diagnose</b> checks your setup and can send it to the developer.</p>
      <ul class="space-y-2">${reports.map((r, i) => `<li class="space-y-2 rounded-lg bg-slate-800/60 p-3">
        <p class="text-xs text-slate-400"><b class="text-slate-200">${esc(r.username)}</b> · ${esc(when(r.created_at).toLocaleString())} · on ${esc(r.page || "?")}</p>
        <p class="whitespace-pre-line text-sm">${esc(r.text)}</p>
        <div class="flex flex-wrap gap-2">
          <button data-dx="${i}" class="btn-secondary py-1 text-sm">🩺 Diagnose</button>
          <button data-copy="${i}" class="btn-ghost py-1 text-sm">📋 Copy</button>
          <button data-done="${r.id}" class="btn-ghost py-1 text-sm">✓ Done</button></div></li>`).join("")
        || `<li class="text-sm text-slate-500">Nothing waiting. 🎉</li>`}</ul></div>`;
    return true;
  };
  if (!(await load())) return;
  const about = (r) => `${r.username} reported (on ${r.page}, NovelCheck ${r.version}, ${r.device}):\n${r.text}`;
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    const dx = e.target.closest("[data-dx]")?.dataset.dx;
    const cp = e.target.closest("[data-copy]")?.dataset.copy;
    const done = e.target.closest("[data-done]")?.dataset.done;
    if (dx !== undefined) {
      d.close();
      diagnoseLater(about(reports[Number(dx)]));
    } else if (cp !== undefined) {
      copyText(about(reports[Number(cp)]));
    } else if (done && (await attempt(() => post(`/api/admin/problems/${done}/done`), "Marked done"))) {
      load();
    }
  };
  d.showModal();
}
