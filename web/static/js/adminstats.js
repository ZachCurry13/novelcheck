// Admin dashboard figures and banners: rating progress, tokens, spending,
// waiting requests and the re-rate offer.
import { $, esc, fmtNum, fmtMoney } from "./ui.js";

// Rough cost of rating one book, from the running average (free with Ollama).
export const perBookCost = (s) => (s.cost_spent && s.usage.total_calls ? s.cost_spent / s.usage.total_calls : 0);

function banner(el, show, html) {
  el.classList.toggle("hidden", !show);
  if (show) el.innerHTML = html;
}

export function renderStats(view, s) {
  const c = s.counts;
  const cap = s.tokens_per_hour || 0;
  const pct = cap ? Math.min(100, Math.round((s.usage.last_hour_tokens / cap) * 100)) : 0;
  const tile = (label, value, sub = "") => `<div class="card"><p class="label">${esc(label)}</p>
    <p class="stat">${value}</p>${sub ? `<p class="text-xs text-slate-500">${sub}</p>` : ""}</div>`;
  $("#stats", view).innerHTML = [
    tile("Analyzed", fmtNum(c.analyzed), `${fmtNum(c.pending)} pending · ${fmtNum(c.queued + c.processing)} queued · ${fmtNum(c.error)} errors`),
    tile("Tokens this hour", fmtNum(s.usage.last_hour_tokens), cap ? `${pct}% of ${fmtNum(cap)} cap` : "No hourly cap"),
    tile("Spent to date", fmtMoney(s.cost_spent), `${fmtNum(s.usage.total_prompt_tokens + s.usage.total_completion_tokens)} tokens · ${fmtNum(s.usage.total_calls)} calls`),
    tile("Est. to finish library", fmtMoney(s.cost_projected), `≈ ${fmtNum(s.tokens_projected)} tokens remaining`),
  ].join("");
  const admin = document.body.dataset.role === "admin";
  banner($("#deep-banner", view), s.pending_deep && admin,
    `🧬 <b>${fmtNum(s.pending_deep)}</b> Deep Scan request${s.pending_deep === 1 ? " is" : "s are"} waiting for your approval. <a href="#/deepscan" class="ml-2 underline">Review</a>`);
  banner($("#del-banner", view), s.pending_deletes && admin,
    `🗑 <b>${fmtNum(s.pending_deletes)}</b> book${s.pending_deletes === 1 ? " is" : "s are"} waiting for your delete review. <a href="#/deletions" class="ml-2 underline">Review</a>`);

  let rerate = "";
  if (s.non_english) {
    rerate = `🌐 <b>${fmtNum(s.non_english)}</b> book summar${s.non_english === 1 ? "y isn't" : "ies aren't"} in English.
      <button data-act="rerate-language" class="btn-secondary ml-2 py-1">Re-rate them in English</button>
      <span class="block text-xs text-slate-400">The AI rewrites them in the language chosen under LLM Analysis Engine. They stay in the library meanwhile.</span>`;
  }
  if (s.rerate_candidates) {
    const est = perBookCost(s) * s.rerate_candidates;
    rerate += `${s.non_english ? `<hr class="my-2 border-slate-700">` : ""}🌶️ <b>${fmtNum(s.rerate_candidates)}</b> book${s.rerate_candidates === 1 ? " was" : "s were"} ${s.changed_books ? `changed in Calibre since they were rated (${fmtNum(s.changed_books)}), or were ` : ""}rated before your latest rule changes (pepper wording or custom filters).
      <button data-act="rerate" class="btn-secondary ml-2 py-1">Re-rate with the current rules</button>
      <span class="block text-xs text-slate-400">They stay in the library with their old rating until the new one arrives. Hand-rated books are left alone.${est ? ` Estimated cost ≈ ${fmtMoney(est)}.` : ""}</span>`;
  }
  banner($("#rerate", view), rerate !== "", rerate);

  const w = s.worker;
  const sync = s.calibre_last_sync;
  $("#worker", view).innerHTML = `Worker: <b>${esc(w.state)}</b>${w.current_title ? ` — ${esc(w.current_title)}` : ""}
    · ${fmtNum(w.queue_length)} waiting${w.last_error ? ` · <span class="text-rose-400">last error: ${esc(w.last_error)}</span>` : ""}
    <br>Calibre: ${s.calibre_available ? "library found" : "<span class='text-amber-400'>no library selected (Delivery &amp; Services → Calibre Library)</span>"}${sync
      ? ` · last sync ${esc(new Date(sync.at).toLocaleString())} (${fmtNum(sync.result?.books)} books${sync.error ? `, <span class="text-rose-400">${esc(sync.error)}</span>` : ""})` : ""}`;
}
