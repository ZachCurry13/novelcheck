// Admin dashboard figures and banners: rating progress, tokens, spending,
// waiting requests and the re-rate offer.
import { $, esc, fmtNum, fmtMoney, canHere } from "./ui.js";

// Rough cost of rating one book, from the running average (free with Ollama).
export const perBookCost = (s) => (s.cost_spent && s.usage.total_calls ? s.cost_spent / s.usage.total_calls : 0);

function banner(el, show, html) {
  if (!el) return; // a notice that lives on another tab (or isn't this person's)
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
    tile("Spent to date", fmtMoney(s.cost_spent), `${fmtMoney(s.cost_spent_30d || 0)} in the last 30 days${s.cost_power ? ` · ${fmtMoney(s.cost_power)} of it electricity (estimate)` : ""} · ${fmtNum(s.usage.total_calls)} calls${s.main_free ? " · your own AI counts as free (enter its watts under Machines &amp; costs to count electricity)" : ""}`),
    tile("Est. to finish library", fmtMoney(s.cost_projected), `≈ ${fmtNum(s.tokens_projected)} tokens remaining`),
  ].join("");
  banner($("#deep-banner", view), (s.pending_deep || s.deep_review) && canHere("deep"), [
    s.deep_review && `<b>${fmtNum(s.deep_review)}</b> Deep Scan result${s.deep_review === 1 ? "" : "s"} would raise a rating a lot and ${s.deep_review === 1 ? "waits" : "wait"} for you to check.`,
    s.pending_deep && `<b>${fmtNum(s.pending_deep)}</b> Deep Scan request${s.pending_deep === 1 ? " is" : "s are"} waiting for your approval.`,
  ].filter(Boolean).join(" ") + ` <a href="#/deepscan?review" class="ml-2 underline">Review</a>`);
  banner($("#problems-banner", view), s.problem_reports && canHere("system"),
    `<b>${fmtNum(s.problem_reports)}</b> problem${s.problem_reports === 1 ? " or idea" : "s or ideas"} from your family. <button data-act="problem-reports" class="ml-2 underline">Review</button>`);
  banner($("#box-banner", view), s.box_sets,
    `<b>${fmtNum(s.box_sets)}</b> book${s.box_sets === 1 ? " looks" : "s look"} like a box set (several books in one). <button data-act="box-sets" class="ml-2 underline">Check and split</button>`);
  // What waits on the other tabs shows as a count on their tab.
  tabCount(view, "cleanup", canHere("calibre") ? (s.pending_deletes || 0) + (s.cover_reports || 0) + (s.title_fixes || 0) + (s.box_sets || 0) : 0);
  tabCount(view, "system", canHere("system") ? s.problem_reports || 0 : 0);

  let rerate = "";
  if (s.non_english) {
    rerate = `<b>${fmtNum(s.non_english)}</b> book summar${s.non_english === 1 ? "y isn't" : "ies aren't"} in English.
      <button data-act="rerate-language" class="btn-secondary ml-2 py-1">Re-rate them in English</button>
      <span class="block text-xs text-slate-400">The AI rewrites them in the language chosen under Extras. They stay in the library meanwhile.</span>`;
  }
  if (s.rerate_candidates) {
    const est = perBookCost(s) * s.rerate_candidates;
    rerate += `${s.non_english ? `<hr class="my-2 border-slate-700">` : ""}🌶️ <b>${fmtNum(s.rerate_candidates)}</b> book${s.rerate_candidates === 1 ? " was" : "s were"} ${s.changed_books ? `changed in Calibre since they were rated (${fmtNum(s.changed_books)}), or were ` : ""}rated before your latest rule changes (pepper wording, custom filters or the new content details).
      <button data-act="rerate" class="btn-secondary ml-2 py-1">Re-rate with the current rules</button>
      <span class="block text-xs text-slate-400">They stay in the library with their old rating until the new one arrives. Books you rated yourself and Deep Scanned books keep their rating; the AI only adds their content details.${est ? ` Estimated cost ≈ ${fmtMoney(est)}.` : ""}</span>`;
  }
  banner($("#rerate", view), rerate !== "", rerate);
  const g = s.genres || {};
  banner($("#genres-banner", view), canHere("ai") && (g.missing || g.running), g.running
    ? `Filling in genres with AI… <b>${fmtNum(g.done)}</b> of ${fmtNum(g.total)} books. <button data-act="genres-stop" class="ml-2 underline">Stop</button>`
    : `<b>${fmtNum(g.missing)}</b> library book${g.missing === 1 ? " has" : "s have"} no genre (no tags in Calibre).
      <button data-act="genres-fill" class="btn-secondary ml-2 py-1">Fill in with AI${g.est_cost ? ` (≈ ${fmtMoney(g.est_cost)})` : ""}</button>
      <span class="block text-xs text-slate-400">New books are filled in by themselves (same switch and hours as automatic rating), a batch of books per AI call, within your hourly token cap. Tags you add in Calibre always win.${g.error ? ` <span class="text-rose-400">Last run stopped: ${esc(g.error)}</span>` : ""}</span>`);

  const w = s.worker;
  const sync = s.calibre_last_sync;
  const doing = { idle: "The AI is idle", analyzing: "Rating now", enriching: "Looking up", delaying: "Pausing between books", rate_limited: "Waiting for the hourly limit" }[w.state] || esc(w.state);
  $("#worker", view).innerHTML = `<b>${doing}</b>${w.current_title ? `: ${esc(w.current_title)}` : ""}
    · ${fmtNum(w.queue_length)} in the queue${w.last_error ? ` · <span class="text-rose-400">last error: ${esc(w.last_error)}</span>` : ""}
    <span class="block text-xs text-slate-400">Calibre: ${s.calibre_available ? "library found" : "<span class='text-amber-400'>no library selected (Delivery &amp; Services → Calibre Library)</span>"}${sync
      ? ` · last read ${esc(new Date(sync.at).toLocaleString())} (${fmtNum(sync.result?.books)} books${sync.error ? `, <span class="text-rose-400">${esc(sync.error)}</span>` : ""})` : ""}</span>`;
}

// tabCount shows how many things wait on another Admin tab, on that tab's button.
function tabCount(view, tab, n) {
  const btn = view.querySelector(`#admin-tabs [data-tab="${tab}"]`);
  if (!btn) return;
  let dot = btn.querySelector(".tab-count");
  if (!n) return dot?.remove();
  if (!dot) {
    dot = document.createElement("span");
    dot.className = "tab-count";
    btn.append(dot);
  }
  dot.textContent = n > 99 ? "99+" : String(n);
}
