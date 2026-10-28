// 👥 Readers: the people in the family who share their reading profile, with a
// few numbers each; tap one for their reading stats (limited to what you may
// see). Private until a person turns sharing on in Settings → Account.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { ago } from "./progress.js";
import { fmtDur, startOfDay } from "./statscalc.js";
import { monthStart } from "./statsperiods.js";
import { renderStats } from "./stats.js";

const ROLES = { admin: "Parent", editor: "Parent", restricted: "Kid" };

const card = (r) => `<a href="#/readers?u=${r.id}" class="card block space-y-3 hover:ring-indigo-600">
  <div class="flex items-center gap-3"><span class="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-indigo-700 text-xl font-bold text-white" aria-hidden="true">${esc(r.username.slice(0, 1).toUpperCase())}</span>
    <div class="min-w-0"><p class="text-lg font-semibold">${esc(r.username)}</p>
      <p class="text-xs text-slate-400">${ROLES[r.role] || ""}${r.shared ? "" : " · not shared: you can see it as a parent"}</p></div></div>
  <div class="grid grid-cols-3 gap-2 text-center">
    <div class="rounded-lg bg-slate-800/60 p-2"><p class="font-bold">${esc(fmtDur(r.week_sec))}</p><p class="text-xs text-slate-400">last 7 days</p></div>
    <div class="rounded-lg bg-slate-800/60 p-2"><p class="font-bold">${esc(fmtDur(r.month_sec))}</p><p class="text-xs text-slate-400">this month</p></div>
    <div class="rounded-lg bg-slate-800/60 p-2"><p class="font-bold">${r.finished_year}</p><p class="text-xs text-slate-400">finished this year</p></div></div>
  ${r.now ? `<p class="text-sm text-slate-300">📖 Reading <b>${esc(r.now.title)}</b> · ${Math.round(r.now.percent * 100)}% · ${esc(ago(r.now.at))}</p>` : ""}</a>`;

export async function renderReaders(view, state) {
  const id = new URLSearchParams(location.hash.split("?")[1] || "").get("u");
  if (id) return renderStats(view, state, { url: `/api/readers/${Number(id)}/stats`, back: "#/readers" });
  const now = new Date();
  const q = new URLSearchParams({
    week: Math.floor((startOfDay(Date.now()) - 6 * 864e5) / 1000),
    month: Math.floor(monthStart(Date.now()) / 1000),
    year: Math.floor(new Date(now.getFullYear(), 0, 1).getTime() / 1000),
  });
  view.innerHTML = `<h1 class="mb-1 text-2xl font-bold">👥 Readers</h1><p class="mb-4 text-sm text-slate-400">Family members who share their reading. <a href="#/stats" class="underline">Your own stats</a></p><div id="readers" class="grid gap-3 sm:grid-cols-2"><p class="text-slate-400">Loading…</p></div>`;
  const data = await get(`/api/readers?${q}`).catch(() => null);
  const host = view.querySelector("#readers");
  const list = data?.readers || [];
  host.innerHTML = list.length ? list.map(card).join("")
    : `<div class="card space-y-2 text-sm text-slate-300 sm:col-span-2"><p>Nobody has shared their reading yet.</p>
      <p class="text-slate-400">Everyone's reading is private until they turn it on. In <b>Settings → Account → Sharing</b> you can share yours, and parents choose for their kids. People only see books they're allowed to see, never highlights or notes.</p>
      <a href="#/settings?s=account" class="btn-secondary inline-flex">Open Sharing settings</a></div>`;
}
