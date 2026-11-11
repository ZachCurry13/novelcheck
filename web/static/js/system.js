// Admin → Integrations (the page that took over from System checks): everything NovelCheck is made of or
// talks to, in one place. Each row shows how it stands and what can be done; "Check now" tests every
// connection and pins the result to its row. Resource use lives on the Usage page.
import { adminNavHTML } from "./adminnav.js";
import { get, post } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";
import { renderDiagnose } from "./diagnose.js";
import { copyBar, bindCopy } from "./copy.js";

const BADGE = { ok: "integ-ok", update: "integ-update", warn: "integ-warn", off: "integ-off", info: "integ-off" };
const CHECK = { ok: ["✓", "text-emerald-300"], warn: ["⚠️", "text-amber-300"], error: ["⛔", "text-rose-300"], skip: ["–", "text-slate-400"] };

// What each row's button does: the request, and what to say when it went through.
const ACTIONS = {
  "db-check": { run: () => post("/api/admin/integrations/db-check"), say: (r) => r.message },
  "models-check": { run: () => post("/api/admin/aitools/check-updates"), say: () => "Checking your models; this takes a minute" },
  "calibre-sync": { run: () => post("/api/admin/calibre-sync"), say: () => "Reading the Calibre library" },
  "abs-sync": { run: () => post("/api/admin/audiobookshelf/sync"), say: () => "Reading Audiobookshelf" },
  "discover-refresh": { run: () => post("/api/admin/discover/refresh"), say: () => "Updating the Discover lists" },
  "covers-find": { run: () => post("/api/admin/covers/find"), say: () => "Looking for covers in the background" },
};

const ago = (iso) => {
  const m = Math.round((Date.now() - new Date(iso)) / 60000);
  return m < 2 ? "just now" : m < 120 ? `${m} min ago` : m < 2880 ? `${Math.round(m / 60)} h ago` : `${Math.round(m / 1440)} d ago`;
};

function rowHTML(r) {
  const checks = (r.checks || []).map((c) => {
    const [icon, cls] = CHECK[c.status] || CHECK.skip;
    return `<p class="text-xs ${cls}">${icon} ${esc(c.name)}: ${esc(c.message)}${c.fix ? ` <span class="text-slate-400">💡 ${esc(c.fix)}</span>` : ""}</p>`;
  }).join("");
  return `<li class="integ-row">
    <div class="min-w-0 flex-1 space-y-1">
      <p class="flex flex-wrap items-center gap-2"><b>${esc(r.name)}</b> <span class="integ-badge ${BADGE[r.badge] || "integ-off"}">${esc(r.label)}</span></p>
      ${r.detail ? `<p class="break-words text-sm text-slate-400">${esc(r.detail)}</p>` : ""}
      ${checks}
    </div>
    <div class="flex shrink-0 flex-col items-end gap-1">
      ${r.action ? `<button type="button" data-do="${esc(r.action)}" class="${r.badge === "update" ? "btn-primary" : "btn-secondary"} py-1 text-sm">${esc(r.do)}</button>` : ""}
      ${r.link ? `<a href="${esc(r.link)}" class="text-xs text-slate-400 underline">Set up</a>` : ""}
    </div></li>`;
}

export async function renderSystem(view) {
  view.innerHTML = `
    ${adminNavHTML("@system")}
    <h1 class="mb-1 text-2xl font-bold">Integrations</h1>
    <p class="mb-4 text-sm text-slate-400">Everything NovelCheck is made of and talks to, in one place. Keys and passwords stay on your server and are never shown again.</p>
    <div class="card mb-4 flex flex-wrap items-center gap-3">
      <button id="integ-check" class="btn-primary">Check now</button>
      <span id="integ-copy" class="hidden">${copyBar("integ-text", "novelcheck-integrations.txt")}</span>
      <span id="integ-when" class="basis-full text-sm text-slate-400"></span>
    </div>
    <div id="integ-body"><p class="text-slate-400">Loading…</p></div>
    <p class="my-4 text-sm text-slate-400">Looking for CPU, memory, network or AI token use? See <a href="#/usage" class="underline">Usage</a>.</p>
    <details class="card"><summary class="cursor-pointer font-semibold">🩺 Diagnose with AI</summary><div id="diagnose" class="mt-3"></div></details>`;
  renderDiagnose($("#diagnose", view));
  const body = $("#integ-body", view);
  const btn = $("#integ-check", view);
  let data = null;
  let timer = null;
  bindCopy($("#integ-copy", view), {
    "integ-text": () => (data?.groups || []).map((g) => `== ${g.title} ==\n` + g.rows.map((r) =>
      `${r.name}: ${r.label}${r.detail ? ` · ${r.detail}` : ""}` + (r.checks || []).map((c) => `\n   ${c.status}: ${c.name}: ${c.message}`).join("")).join("\n")).join("\n\n"),
  });

  const paint = () => {
    body.innerHTML = data.groups.map((g) => `<section class="card mb-4">
      <h2 class="integ-title">${esc(g.title)}</h2>
      <p class="mb-2 text-sm text-slate-400">${esc(g.intro)}</p>
      <ul class="integ-list">${g.rows.map(rowHTML).join("")}</ul></section>`).join("");
    btn.disabled = data.checking;
    btn.textContent = data.checking ? "Checking… (a local AI can take a minute)" : "Check now";
    $("#integ-when", view).textContent = data.checking ? "" : data.checked_at ? `Last checked ${ago(data.checked_at)}.` : "Not checked since NovelCheck started. Checking asks each AI model a tiny question (a few tokens).";
    $("#integ-copy", view).classList.remove("hidden");
  };
  const load = async () => {
    clearTimeout(timer);
    if (!view.isConnected) return;
    const r = await get("/api/admin/integrations").catch(() => null);
    if (!r) return;
    data = r;
    paint();
    if (r.checking) timer = setTimeout(load, 3000);
  };

  btn.addEventListener("click", async () => {
    const r = await attempt(() => post("/api/admin/integrations/check"));
    if (!r) return;
    data = r;
    paint();
    timer = setTimeout(load, 3000);
  });
  body.addEventListener("click", async (e) => {
    const b = e.target.closest("[data-do]");
    const a = b && ACTIONS[b.dataset.do];
    if (!a) return;
    b.disabled = true;
    const r = await attempt(a.run);
    b.disabled = false;
    if (!r) return;
    toast(a.say(r), r.ok === false);
    setTimeout(load, 2500); // the row shows its new state
  });
  await load();
  return () => clearTimeout(timer);
}
