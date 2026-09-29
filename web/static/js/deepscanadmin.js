// Admin → 🧬 Deep Scan, in four sections: Review (big raises waiting for a
// decision), Running (requests, queued and reading scans), Results, and
// Settings (scan the next Up Next books after a cost estimate; readers whose
// Up Next is scanned automatically). Decisions apply in place; only the
// Running section refreshes on its own.
import { get, post, put } from "./api.js";
import { $, $$, esc, attempt, toast, fmtNum, fmtMoney, fmtMinutes } from "./ui.js";
import { openBook } from "./bookdialog.js";
import { isOpen, reviewCard, reviewHeader, openRow, resultRow, setMeters } from "./deepscanlists.js";
import { adminNavHTML } from "./adminnav.js";

const TABS = [["review", "Review"], ["running", "Running"], ["results", "Results"], ["settings", "Settings"]];
let timer = null;

export async function renderDeepScanAdmin(view, state) {
  clearInterval(timer);
  view.innerHTML = `
    ${adminNavHTML("@deepscan", true)}
    <h1 class="mb-1 text-2xl font-bold">🧬 Deep Scan</h1>
    <p class="mb-3 text-sm text-slate-400">The AI reads a book's whole EPUB, part by part, instead of guessing from the description.</p>
    <div id="model-warning"></div>
    <p id="deep-waiting" class="card mb-3 hidden text-sm text-amber-300">⏸ The Deep Scan machine isn't answering. Scans wait and carry on by themselves when it's switched on again.</p>
    <div class="mb-4 grid grid-cols-4 gap-1 rounded-xl bg-slate-900 p-1 ring-1 ring-slate-800" role="tablist">
      ${TABS.map(([k, label]) => `<button type="button" role="tab" data-tab="${k}" class="rounded-lg px-1 py-2 text-sm text-slate-300">${label}<span data-count="${k}" class="block text-xs text-slate-500"></span></button>`).join("")}
    </div>
    <section data-panel="review" class="space-y-3"><div data-head></div><ul data-list class="space-y-3"></ul></section>
    <section data-panel="running" class="space-y-3"><ul data-list class="space-y-3"></ul></section>
    <section data-panel="results"><ul data-list></ul></section>
    <section data-panel="settings" class="space-y-4"></section>`;

  let data = (await attempt(() => get("/api/admin/deep-scans"))) || { scans: [], users: [], top_n: 10 };
  let held = data.scans.filter((d) => d.held);
  const decided = new Set(); // a refresh already on its way mustn't bring these back
  const panel = (k) => $(`[data-panel="${k}"]`, view);
  const counts = () => {
    const open = data.scans.filter(isOpen).length;
    $('[data-count="review"]', view).textContent = held.length ? `${held.length} waiting` : "none";
    $('[data-count="running"]', view).textContent = open ? String(open) : "none";
  };
  const paintReview = () => {
    $("[data-head]", panel("review")).innerHTML = reviewHeader(held.length);
    $("[data-list]", panel("review")).innerHTML = held.map(reviewCard).join("");
  };
  // The Running list in run order; waiting scans can be dragged. It's only
  // redrawn when something changed, and never mid-drag.
  let sortable = null;
  let dragging = false;
  let shown = "";
  const paintRunning = () => {
    const open = data.scans.filter(isOpen);
    const sig = open.map((d) => `${d.id}:${d.status}:${d.parts_done}`).join(",");
    if (dragging || sig === shown) return;
    shown = sig;
    const list = $("[data-list]", panel("running"));
    let place = 0;
    list.innerHTML = open.map((d) => openRow(d, d.status === "reading" ? 0 : ++place)).join("")
      || `<li class="card text-sm text-slate-400">Nothing waiting or running. Start scans under Settings.</li>`;
    setMeters(list);
    sortable?.destroy();
    sortable = window.Sortable?.create(list, {
      handle: ".drag-handle", draggable: "[data-sortable]", animation: 150, ghostClass: "sortable-ghost",
      onStart: () => (dragging = true),
      onEnd: async () => {
        dragging = false;
        const ids = [...list.querySelectorAll("[data-sortable]")].map((li) => Number(li.dataset.scan));
        if (await attempt(() => put("/api/admin/deep-scans/order", { ids }), "Order saved")) {
          const byID = new Map(data.scans.map((d) => [d.id, d]));
          data.scans = [...data.scans.filter((d) => !ids.includes(d.id)), ...ids.map((id) => byID.get(id))];
          shown = "";
          paintRunning();
        }
      },
    });
  };
  const paintResults = () => {
    const done = data.scans.filter((d) => !isOpen(d) && !d.held);
    $("[data-list]", panel("results")).innerHTML = done.map(resultRow).join("") || `<li class="text-sm text-slate-500">No Deep Scans yet.</li>`;
  };
  const show = (k) => {
    TABS.forEach(([t]) => {
      panel(t).classList.toggle("hidden", t !== k);
      const b = $(`[data-tab="${t}"]`, view);
      b.classList.toggle("bg-slate-800", t === k);
      b.classList.toggle("text-white", t === k);
      b.setAttribute("aria-selected", String(t === k));
    });
    history.replaceState(null, "", `#/deepscan?${k}`);
  };

  paintModelWarning($("#model-warning", view), data);
  $("#deep-waiting", view).classList.toggle("hidden", !data.waiting);
  renderSettings(panel("settings"), data);
  paintReview();
  paintRunning();
  paintResults();
  counts();
  const asked = TABS.map(([k]) => k).find((k) => location.hash.includes(`?${k}`));
  show(asked || (held.length ? "review" : data.scans.some(isOpen) ? "running" : "results"));

  view.onclick = async (e) => {
    const tab = e.target.closest("[data-tab]")?.dataset.tab;
    if (tab) return show(tab);
    const item = e.target.closest("[data-scan]");
    if (e.target.closest("[data-open]") && item) return openBook(Number(item.dataset.book), state);
    const btn = e.target.closest("[data-act]");
    if (!btn) return;
    const act = btn.dataset.act;
    if (act === "keep-all" || act === "accept-all") {
      const accept = act === "accept-all";
      if (!confirm(accept ? `Apply the Deep Scan level to all ${held.length} books?` : `Keep the current rating of all ${held.length} books? The scans' suggestions are dropped.`)) return;
      const r = await attempt(() => post(`/api/admin/deep-scans/${act}`));
      if (!r) return;
      toast(accept ? `Accepted ${r.accepted} rating${r.accepted === 1 ? "" : "s"}` : `Kept ${r.kept} rating${r.kept === 1 ? "" : "s"}`);
      return renderDeepScanAdmin(view, state);
    }
    if (!item) return;
    btn.disabled = true;
    const ok = await attempt(() => post(`/api/admin/deep-scans/${item.dataset.scan}/${act}`));
    if (!ok) {
      btn.disabled = false;
      return;
    }
    // Decide in place: the card goes, the counts drop, the page stays where it is.
    const id = Number(item.dataset.scan);
    if (act === "accept" || act === "keep") {
      decided.add(id);
      held = held.filter((d) => d.id !== id);
      data.scans = data.scans.map((d) => (d.id === id ? { ...d, held: false, status: act === "keep" ? "declined" : d.status, new_level: act === "accept" ? d.proposed_level : d.new_level } : d));
      item.remove();
      toast(act === "accept" ? "Accepted" : "Kept the old rating");
      if (!held.length) paintReview();
      else $("[data-head]", panel("review")).innerHTML = reviewHeader(held.length);
      paintResults();
    } else {
      await refresh();
    }
    counts();
  };

  // Every 5 seconds: progress of running scans. Finished ones move to
  // Results; new big raises are added to Review without redrawing the cards.
  const refresh = async () => {
    const d = await get("/api/admin/deep-scans").catch(() => null);
    if (!d || !document.body.contains(view)) return clearInterval(timer);
    const known = new Set(held.map((x) => x.id));
    const arrived = d.scans.filter((x) => x.held && !known.has(x.id) && !decided.has(x.id));
    data = d;
    $("#deep-waiting", view).classList.toggle("hidden", !d.waiting);
    if (arrived.length) {
      held = held.concat(arrived);
      if (held.length === arrived.length) paintReview();
      else {
        $("[data-list]", panel("review")).insertAdjacentHTML("beforeend", arrived.map(reviewCard).join(""));
        $("[data-head]", panel("review")).innerHTML = reviewHeader(held.length);
      }
    }
    paintRunning();
    paintResults();
    counts();
  };
  timer = setInterval(refresh, 5000);
  return () => {
    clearInterval(timer);
    sortable?.destroy();
  };
}

function paintModelWarning(box, data) {
  if (!data.model_warning) return;
  box.innerHTML = `<div class="mb-4 space-y-2 rounded-lg bg-amber-950/50 p-3 text-sm text-amber-200"><p>⚠️ ${esc(data.model_warning)}</p>
    ${data.suggest_model ? `<button id="use-model" class="btn-secondary py-1 text-sm">Use ${esc(data.suggest_model)} for Deep Scan</button>
      <span class="text-xs text-amber-200/80">(already on your Ollama; other ratings keep their model)</span>` : ""}</div>`;
  $("#use-model", box)?.addEventListener("click", async () => {
    if (await attempt(() => put("/api/admin/settings", { deep_read_model: data.suggest_model }), `Deep Scan now uses ${data.suggest_model}`)) box.innerHTML = "";
  });
}

async function renderSettings(host, data) {
  host.innerHTML = `
    <section class="card space-y-3">
      <h2 class="text-lg font-semibold">Scan the next books in Up Next</h2>
      <p class="text-sm text-slate-400">A scan costs about as much as rating 100–200 books from their descriptions, so you see the estimate first.</p>
      <div class="flex flex-wrap items-center gap-2">
        <select id="top-n" class="input w-auto">${[10, 20, 30].map((n) => `<option value="${n}">Next ${n} books</option>`).join("")}</select>
        <button id="estimate" class="btn-primary">Estimate cost…</button>
      </div>
      <div id="estimate-out" class="space-y-2"></div>
    </section>
    <section class="card space-y-3">
      <h2 class="text-lg font-semibold">Always scan these readers' Up Next</h2>
      <p class="text-sm text-slate-400">Pick up to 3 accounts (for example the kids). New books in their Up Next are Deep Scanned automatically.</p>
      <div id="deep-users" class="grid gap-1 sm:grid-cols-2"></div>
      <button id="save-users" class="btn-secondary">Save</button>
    </section>`;
  $("#top-n", host).value = String([10, 20, 30].includes(data.top_n) ? data.top_n : 10);
  const users = (await attempt(() => get("/api/admin/users"))) || [];
  const picked = data.users || [];
  $("#deep-users", host).innerHTML = users.map((u) => `<label class="toggle min-h-[2.5rem]"><input type="checkbox" value="${u.id}" ${picked.includes(u.id) ? "checked" : ""}> ${esc(u.username)}</label>`).join("")
    || `<p class="text-sm text-slate-400">No accounts yet.</p>`;
  $("#deep-users", host).addEventListener("change", (e) => {
    if ($$("#deep-users input:checked", host).length > 3) {
      e.target.checked = false;
      toast("Pick up to 3 accounts", true);
    }
  });
  $("#save-users", host).addEventListener("click", () => {
    const ids = $$("#deep-users input:checked", host).map((cb) => cb.value).join(",");
    attempt(() => put("/api/admin/settings", { deep_scan_users: ids }), "Saved. Their new Up Next books will be Deep Scanned.");
  });
  $("#estimate", host).addEventListener("click", async () => {
    const n = $("#top-n", host).value;
    const est = await attempt(() => get(`/api/admin/deep-scans/next?n=${n}`));
    const out = $("#estimate-out", host);
    if (!est) return;
    if (!est.books) {
      out.innerHTML = `<p class="text-sm text-slate-400">Nothing to scan: every book waiting in Up Next is already scanned or has no EPUB in Calibre.</p>`;
      return;
    }
    out.innerHTML = `<p class="rounded-lg bg-slate-800 p-3 text-sm">Deep Scanning <b>${est.books} book${est.books === 1 ? "" : "s"}</b>
        (~${fmtNum(est.estimate.tokens)} tokens / ~${fmtMoney(est.cost)}${est.minutes ? ` / ${fmtMinutes(est.minutes)}` : ""}):<br><span class="text-slate-400">${est.titles.map(esc).join(" · ")}</span></p>
      <button id="go" class="btn-primary">Start ${est.books} Deep Scan${est.books === 1 ? "" : "s"}</button>`;
    $("#go", host).onclick = async () => {
      const r = await attempt(() => post(`/api/admin/deep-scans/next?n=${n}`));
      if (r) {
        toast(`Started ${r.queued} Deep Scan${r.queued === 1 ? "" : "s"}`);
        location.hash = "#/deepscan?running"; // redraws the page on the Running section
      }
    };
  });
}
