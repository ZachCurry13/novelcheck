// "Rating errors": books that failed to rate, grouped by reason, with plain
// explanations and Retry buttons. Shown on Usage (admins) and Admin.
import { get, post } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";
import { copyText } from "./copy.js";
import { diagnoseLater } from "./diagnose.js";

// What common errors mean, in plain words.
const HINTS = [
  [/first path segment|missing protocol scheme/i, "The AI address was missing http:// (fixed in NovelCheck 1.8.1). Just retry."],
  [/didn't answer within|deadline exceeded|timeout/i, "The AI took too long. Newer versions wait longer for Ollama; retry. If it keeps happening, the model may be too big for your GPU (see the ⚠️ tags in Admin → AI & Scans → LLM Analysis Engine → Find Ollama)."],
  [/connection refused|no such host|dial tcp|can't reach|unreachable/i, "The AI server was switched off or unreachable at the time. Check it's running, then retry."],
  [/401|403|api key|unauthori[sz]ed|invalid.*key/i, "The AI service rejected the API key. Check it in Admin → AI & Scans → LLM Analysis Engine, then retry."],
  [/429|rate limit|quota|insufficient|billing|credit/i, "The AI service's limit or credit ran out. Wait or add credit, then retry."],
  [/json|verdict|spice_level|classification|no choices/i, "The model's answer couldn't be read. A bigger model, or a fallback model, usually fixes this; then retry."],
  [/not found|404|model .* (does not exist|not found)/i, "The model name wasn't found on the AI server. Check the model name, then retry."],
];
const hintFor = (msg) => (HINTS.find(([re]) => re.test(msg)) || [null, ""])[1];

export async function renderErrors(host, isAdmin, onChange) {
  const data = await get("/api/admin/errors").catch(() => null);
  if (!data || !data.groups.length) {
    host.innerHTML = "";
    return;
  }
  const total = data.groups.reduce((a, g) => a + g.count, 0);
  host.innerHTML = `
    <section class="card mb-6 space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-lg font-semibold">⛔ Rating errors · ${total.toLocaleString()} book${total === 1 ? "" : "s"}</h2>
        <span class="flex flex-wrap gap-2">
          <a href="#/library" data-failed class="btn-ghost py-1 text-xs">Show in Library</a>
          <button data-retry-all class="btn-primary py-1">Retry all ${total.toLocaleString()}</button>
        </span>
      </div>
      <p class="text-sm text-slate-400">Grouped by what went wrong. Failed books aren't tried again on their own; retry them once the cause is fixed.</p>
      <ul class="space-y-3">${data.groups.map((g, i) => `
        <li class="rounded-lg bg-slate-800/60 p-3 text-sm" data-i="${i}">
          <div class="flex flex-wrap items-start justify-between gap-2">
            <p class="min-w-0 flex-1"><b>${g.count.toLocaleString()} book${g.count === 1 ? "" : "s"}</b> ·
              <span class="break-words font-mono text-xs text-rose-300">${esc(g.message)}</span></p>
            <span class="flex flex-wrap gap-1">
              <button data-retry="${i}" class="btn-secondary px-2 py-0.5 text-xs">Retry ${g.count.toLocaleString()}</button>
              <button data-copy="${i}" class="btn-ghost px-2 py-0.5 text-xs">📋 Copy</button>
              ${isAdmin ? `<button data-dx="${i}" class="btn-ghost px-2 py-0.5 text-xs">🩺 Diagnose</button>` : ""}
            </span>
          </div>
          ${hintFor(g.message) ? `<p class="mt-1 text-slate-300">💡 ${esc(hintFor(g.message))}</p>` : ""}
          <p class="mt-1 text-xs text-slate-500">For example: ${g.titles.map(esc).join(" · ")}${g.count > g.titles.length ? " …" : ""}</p>
        </li>`).join("")}</ul>
    </section>`;

  const retry = async (message, btn) => {
    btn.disabled = true;
    const r = await attempt(() => post("/api/admin/retry-errors", message === undefined ? {} : { message }));
    if (!r) return (btn.disabled = false);
    toast(`Retrying ${r.queued.toLocaleString()} book${r.queued === 1 ? "" : "s"}. They'll be rated in the background.`);
    await renderErrors(host, isAdmin, onChange);
    onChange?.();
  };
  host.onclick = (e) => {
    const b = e.target.closest("button, a");
    if (!b) return;
    const g = data.groups[Number(b.dataset.retry ?? b.dataset.copy ?? b.dataset.dx)];
    if (b.dataset.failed !== undefined) {
      try {
        sessionStorage.setItem("nc:libfilter", "failed");
      } catch {
        /* ignore */
      }
    } else if (b.dataset.retryAll !== undefined) retry(undefined, b);
    else if (b.dataset.retry !== undefined) retry(g.message, b);
    else if (b.dataset.copy !== undefined) copyText(`${g.count} books failed: ${g.message}\nExamples: ${g.titles.join("; ")}`);
    else if (b.dataset.dx !== undefined) diagnoseLater(`${g.count} books failed to rate with: ${g.message}`);
  };
}
