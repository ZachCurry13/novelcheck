// Admin → 🧬 Deep Scan: scan the next Up Next books (with a cost estimate
// first), pick up to 3 accounts to scan automatically, approve requests,
// watch progress, and see which ratings the full text changed.
import { get, post, put } from "./api.js";
import { $, $$, esc, attempt, toast, fmtNum, fmtMoney } from "./ui.js";
import { deepChangeLine, when } from "./deepscan.js";

const SOURCE = { admin: "started by an admin", request: "requested", batch: "Up Next batch", auto: "automatic" };

export async function renderDeepScanAdmin(view) {
  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">🧬 Deep Scan</h1>
    <p class="mb-4 text-sm text-slate-400">The AI reads a book's whole EPUB, part by part, instead of guessing from the description.
      A scan costs about as much as rating 100–200 books from their descriptions, so NovelCheck always shows the estimate first.</p>
    <div id="model-warning"></div>
    <div class="grid gap-4 lg:grid-cols-2">
      <section class="card space-y-3">
        <h2 class="text-lg font-semibold">Scan the next books in Up Next</h2>
        <div class="flex flex-wrap items-center gap-2">
          <select id="top-n" class="input w-auto">${[10, 20, 30].map((n) => `<option value="${n}">Next ${n} books</option>`).join("")}</select>
          <button id="estimate" class="btn-primary">Estimate cost…</button>
        </div>
        <div id="estimate-out"></div>
      </section>
      <section class="card space-y-3">
        <h2 class="text-lg font-semibold">Always scan these readers' Up Next</h2>
        <p class="text-sm text-slate-400">Pick up to 3 accounts (for example the kids). New books in their Up Next are Deep Scanned automatically.</p>
        <div id="deep-users" class="grid gap-1 sm:grid-cols-2"></div>
        <button id="save-users" class="btn-secondary">Save</button>
      </section>
    </div>
    <section id="scans" class="mt-6 space-y-3"></section>`;

  const data = (await attempt(() => get("/api/admin/deep-scans"))) || { scans: [], users: [], top_n: 10 };
  if (data.model_warning) {
    const box = $("#model-warning", view);
    box.innerHTML = `<div class="mb-4 space-y-2 rounded-lg bg-amber-950/50 p-3 text-sm text-amber-200"><p>⚠️ ${esc(data.model_warning)}</p>
      ${data.suggest_model ? `<button id="use-model" class="btn-secondary py-1 text-sm">Use ${esc(data.suggest_model)} for Deep Scan</button>
        <span class="text-xs text-amber-200/80">(already on your Ollama; other ratings keep their model)</span>` : ""}</div>`;
    $("#use-model", box)?.addEventListener("click", async () => {
      if (await attempt(() => put("/api/admin/settings", { deep_read_model: data.suggest_model }), `Deep Scan now uses ${data.suggest_model}`)) box.innerHTML = "";
    });
  }
  $("#top-n", view).value = String([10, 20, 30].includes(data.top_n) ? data.top_n : 10);
  const users = (await attempt(() => get("/api/admin/users"))) || [];
  $("#deep-users", view).innerHTML = users.map((u) => `<label class="toggle"><input type="checkbox" value="${u.id}" ${data.users.includes(u.id) ? "checked" : ""}> ${esc(u.username)}</label>`).join("");
  renderScans($("#scans", view), data.scans, () => renderDeepScanAdmin(view));

  $("#deep-users", view).addEventListener("change", (e) => {
    if ($$("#deep-users input:checked", view).length > 3) {
      e.target.checked = false;
      toast("Pick up to 3 accounts", true);
    }
  });
  $("#save-users", view).addEventListener("click", () => {
    const ids = $$("#deep-users input:checked", view).map((cb) => cb.value).join(",");
    attempt(() => put("/api/admin/settings", { deep_scan_users: ids }), "Saved. Their new Up Next books will be Deep Scanned.");
  });
  $("#estimate", view).addEventListener("click", async () => {
    const n = $("#top-n", view).value;
    const est = await attempt(() => get(`/api/admin/deep-scans/next?n=${n}`));
    const out = $("#estimate-out", view);
    if (!est) return;
    if (!est.books) {
      out.innerHTML = `<p class="text-sm text-slate-400">Nothing to scan: every book waiting in Up Next is already scanned or has no EPUB in Calibre.</p>`;
      return;
    }
    out.innerHTML = `<p class="rounded-lg bg-slate-800 p-3 text-sm">Deep Scanning <b>${est.books} book${est.books === 1 ? "" : "s"}</b>
        (~${fmtNum(est.estimate.tokens)} tokens / ~${fmtMoney(est.cost)}):<br><span class="text-slate-400">${est.titles.map(esc).join(" · ")}</span></p>
      <button id="go" class="btn-primary">Start ${est.books} Deep Scan${est.books === 1 ? "" : "s"}</button>`;
    $("#go", view).onclick = async () => {
      const r = await attempt(() => post(`/api/admin/deep-scans/next?n=${n}`));
      if (r) {
        toast(`Started ${r.queued} Deep Scan${r.queued === 1 ? "" : "s"}`);
        renderDeepScanAdmin(view);
      }
    };
  });
  const timer = setInterval(async () => {
    const d = await get("/api/admin/deep-scans").catch(() => null);
    if (d && document.body.contains(view)) renderScans($("#scans", view), d.scans, () => renderDeepScanAdmin(view));
  }, 5000);
  return () => clearInterval(timer);
}

function renderScans(host, scans, reload) {
  const open = scans.filter((d) => ["requested", "queued", "reading"].includes(d.status));
  const held = scans.filter((d) => d.held);
  const done = scans.filter((d) => !open.includes(d) && !d.held);
  // A big raise waits here with what the AI found in the parts that set it.
  const evidence = (d) => {
    let notes = [];
    try {
      notes = JSON.parse(d.notes || "[]").filter((n) => n.level >= 3);
    } catch {
      /* no notes */
    }
    return notes.map((n) => `<li><b>${esc(n.label)}</b> · Level ${n.level}: ${esc(n.note)}</li>`).join("");
  };
  const heldRow = (d) => `<li class="card space-y-2" data-scan="${d.id}">
    <p><b>${esc(d.title)}</b> <span class="text-sm text-slate-400">${esc(d.author || "")}</span></p>
    <p class="text-sm font-semibold text-amber-300">⚠️ Suggests Level ${d.proposed_level} (now Level ${d.prev_level})</p>
    <ul class="list-disc space-y-1 pl-5 text-sm text-slate-300">${evidence(d) || "<li>No parts with sexual content were noted.</li>"}</ul>
    <p class="text-xs text-slate-400">Every part above passed a second check for sexual content on the page. Accept only if this matches the book.</p>
    <div class="flex flex-wrap gap-2"><button data-act="accept" class="btn-primary py-1 text-sm">Accept Level ${d.proposed_level}</button>
      <button data-act="keep" class="btn-ghost py-1 text-sm">Keep Level ${d.prev_level}</button></div></li>`;
  const row = (d) => `<li class="card space-y-1" data-scan="${d.id}">
    <p><b>${esc(d.title)}</b> <span class="text-sm text-slate-400">${esc(d.author || "")}</span></p>
    <p class="text-xs text-slate-400">${esc(SOURCE[d.source] || d.source)}${d.requested_by ? ` by ${esc(d.requested_by)}` : ""} · ${esc(when(d.created_at).toLocaleString())}
      · ${fmtNum(d.words)} words, ~${fmtNum(d.est_tokens)} tokens${d.reason ? ` · “${esc(d.reason)}”` : ""}</p>
    ${d.status === "reading" ? `<p class="text-sm">📖 Reading part ${d.parts_done + 1} of ${d.parts_total}…</p>` : ""}
    ${d.status === "error" ? `<p class="text-sm text-rose-300">Failed: ${esc(d.error)}</p>` : ""}
    ${d.status === "done" ? deepChangeLine(d) || `<p class="text-sm text-slate-400">Done: Level ${d.new_level}${d.prev_level === d.new_level ? " (unchanged)" : ""}</p>` : ""}
    ${["declined", "cancelled"].includes(d.status) ? `<p class="text-sm text-slate-500">${d.status === "declined" ? (d.proposed_level != null ? `Kept Level ${d.prev_level} (the scan suggested ${d.proposed_level})` : "Declined") : "Cancelled"}</p>` : ""}
    <div class="flex flex-wrap gap-2">
      ${d.status === "requested" ? `<button data-act="approve" class="btn-primary py-1 text-sm">Approve</button><button data-act="decline" class="btn-ghost py-1 text-sm">Decline</button>` : ""}
      ${["queued", "reading"].includes(d.status) ? `<button data-act="cancel" class="btn-ghost py-1 text-sm">Cancel</button>` : ""}
    </div></li>`;
  host.innerHTML = `${held.length ? `<h2 class="text-lg font-semibold">Waiting for your review</h2>
    <p class="text-sm text-slate-400">These scans would raise a book by 2 or more levels, so they don't apply until you accept them.</p>
    <ul class="space-y-2">${held.map(heldRow).join("")}</ul>` : ""}
    <h2 class="text-lg font-semibold">Waiting and running</h2>
    <ul class="space-y-2">${open.map(row).join("") || `<li class="text-sm text-slate-500">Nothing waiting.</li>`}</ul>
    <h2 class="pt-2 text-lg font-semibold">Recent results</h2>
    <ul class="space-y-2">${done.map(row).join("") || `<li class="text-sm text-slate-500">No Deep Scans yet.</li>`}</ul>`;
  host.onclick = async (e) => {
    const act = e.target.closest("[data-act]")?.dataset.act;
    const id = e.target.closest("[data-scan]")?.dataset.scan;
    if (!act || !id) return;
    if (await attempt(() => post(`/api/admin/deep-scans/${id}/${act}`))) reload();
  };
}
