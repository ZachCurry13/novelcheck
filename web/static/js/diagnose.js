// System → 🩺 Diagnose with AI: the connected AI reads the diagnostics report
// (no passwords or keys), explains the problem, and drafts a GitHub bug report.
import { post } from "./api.js";
import { $, esc, attempt } from "./ui.js";
import { copyBar, bindCopy, copyText } from "./copy.js";

const ISSUE_URL = "https://github.com/ZachCurry13/novelcheck/issues/new?template=bug_report.yml";
const URL_LIMIT = 7000; // GitHub rejects very long links; the full report is copied too
const PENDING_KEY = "nc:diagnose";

// diagnoseLater opens the System page with a problem already filled in.
export function diagnoseLater(problem) {
  try {
    sessionStorage.setItem(PENDING_KEY, problem);
  } catch {
    /* private mode: the box just starts empty */
  }
  if (location.hash === "#/system") window.dispatchEvent(new HashChangeEvent("hashchange"));
  else location.hash = "#/system";
}

export function renderDiagnose(host) {
  let pending = "";
  try {
    pending = sessionStorage.getItem(PENDING_KEY) || "";
    sessionStorage.removeItem(PENDING_KEY);
  } catch {
    /* ignore */
  }
  host.innerHTML = `
    <section class="card mb-6 space-y-3">
      <h2 class="text-lg font-semibold">🩺 Diagnose with AI</h2>
      <p class="text-sm text-slate-400">Your AI reads NovelCheck's diagnostics (settings, recent problems and logs, never passwords or keys),
        explains what's wrong, and writes a bug report you can send to the developer on GitHub.</p>
      <textarea id="dx-problem" rows="3" class="input" placeholder="What went wrong? (optional) e.g. 'Send test email fails' or paste an error">${esc(pending)}</textarea>
      <div class="flex flex-wrap items-center gap-2">
        <button id="dx-run" class="btn-primary">🩺 Diagnose</button>
        <span class="text-xs text-slate-500">Just the raw report:</span>
        <button id="dx-raw-copy" class="btn-ghost px-2 py-0.5 text-xs">📋 Copy diagnostics</button>
        <a href="/api/admin/diagnostics" download="novelcheck-diagnostics.txt" class="btn-ghost px-2 py-0.5 text-xs">⬇ Download diagnostics</a>
      </div>
      <div id="dx-out"></div>
    </section>`;
  const out = $("#dx-out", host);
  let report = "";
  let title = "";
  bindCopy(host, { "bug-report": () => report });

  $("#dx-raw-copy", host).addEventListener("click", async () => {
    const res = await fetch("/api/admin/diagnostics", { credentials: "same-origin" });
    if (res.ok) copyText(await res.text());
  });

  $("#dx-run", host).addEventListener("click", async (e) => {
    e.target.disabled = true;
    e.target.textContent = "Diagnosing… (a local AI can take a minute)";
    const d = await attempt(() => post("/api/admin/diagnose", { problem: $("#dx-problem", host).value }));
    e.target.disabled = false;
    e.target.textContent = "🩺 Diagnose";
    if (!d) return;
    report = d.report;
    title = d.title;
    const verdict = d.ai_error ? "" : d.is_bug
      ? `<span class="chip-open">Looks like a bug: please send the report</span>`
      : `<span class="chip-none">Looks like something you can fix</span>`;
    out.innerHTML = `
      <div class="space-y-3 rounded-lg bg-slate-800/60 p-4">
        ${d.ai_error ? `<p class="text-sm text-amber-300">⚠️ The AI couldn't help this time (${esc(d.ai_error)}). The bug report below still has the diagnostics.</p>` : `
        <div class="flex flex-wrap items-center gap-2"><b>${esc(d.title)}</b> ${verdict}</div>
        <p class="whitespace-pre-line text-sm text-slate-200">${esc(d.diagnosis)}</p>
        ${d.fix_steps?.length ? `<ol class="ml-5 list-decimal space-y-1 text-sm">${d.fix_steps.map((s) => `<li>${esc(s)}</li>`).join("")}</ol>` : ""}
        <p class="text-xs text-slate-500">Diagnosed by ${esc(d.model)}. AI can be wrong; check before changing settings.</p>`}
        <div class="flex flex-wrap items-center justify-between gap-2 border-t border-slate-700 pt-3">
          <span class="label mb-0">Bug report for GitHub</span>
          <span class="flex flex-wrap gap-2">${copyBar("bug-report", "novelcheck-bug-report.md")}
            <button id="dx-issue" class="btn-secondary py-1 text-xs">Open GitHub issue ↗</button></span>
        </div>
        <pre id="bug-report" class="max-h-72 select-text overflow-auto whitespace-pre-wrap break-all rounded bg-slate-950 p-2 font-mono text-xs"></pre>
        <p class="text-xs text-slate-500">Your web address is replaced with a placeholder. Read it over before posting; GitHub issues are public.</p>
      </div>`;
    $("#bug-report", out).textContent = report;
    $("#dx-issue", out).addEventListener("click", async () => {
      await copyText(report);
      let body = report;
      if (body.length > URL_LIMIT) body = body.slice(0, URL_LIMIT) + "\n\n…(cut off: select everything in this box and paste; the full report is on your clipboard)";
      window.open(`${ISSUE_URL}&title=${encodeURIComponent(title)}&report=${encodeURIComponent(body)}`, "_blank", "noopener");
    });
  });
}
