// Settings → Developer (admins): turn debug recording on for a few minutes, see the timeline of what everyone
// did while it was on, and copy or delete it. The same lines go into bug reports (server report).
import { get, put, del } from "./api.js";
import { attempt } from "./ui.js";
import { copyText } from "./copy.js";
import { check as checkRecording } from "./activityrec.js";

export const activityCardHTML = () => `<div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">Debug recording</h2>
  <p class="text-sm text-slate-400">While it is on, everyone's NovelCheck shows a bar saying so, and the server keeps a timeline of what happens: pages opened, buttons tapped, what is typed (never passwords, codes or keys), every call the server answers (including KOReader and OPDS), and errors. It stops by itself (an hour at most), lines are deleted after a week, and the latest ones are added to bug reports.</p>
  <p id="act-state" class="text-sm font-semibold"></p>
  <div class="flex flex-wrap gap-2"><button type="button" data-act-start="5" class="btn-primary py-1 text-sm">Record 5 min</button>
    <button type="button" data-act-start="15" class="btn-secondary py-1 text-sm">15 min</button>
    <button type="button" data-act-start="60" class="btn-secondary py-1 text-sm">60 min</button>
    <button type="button" data-act-start="0" class="btn-ghost py-1 text-sm">Stop</button></div>
  <pre id="act-log" class="max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-3 text-xs text-slate-300"></pre>
  <div class="flex flex-wrap gap-2"><button type="button" data-act="refresh" class="btn-secondary py-1 text-sm">Refresh</button>
    <button type="button" data-act="copy" class="btn-secondary py-1 text-sm">Copy timeline</button>
    <button type="button" data-act="clear" class="btn-ghost py-1 text-sm">Delete timeline</button></div></div>`;

const line = (e) => `${e.at.slice(11, 23)} ${(e.user || "-").padEnd(9)} ${e.source === "server" ? "srv" : "web"} ${e.kind.padEnd(5)} ${e.detail}`;

export async function bindActivityCard(host) {
  const state = host.querySelector("#act-state");
  const log = host.querySelector("#act-log");
  const draw = async () => {
    const r = await get("/api/admin/activity").catch(() => null);
    if (!r) return;
    const mins = Math.ceil((r.seconds_left || 0) / 60);
    state.textContent = r.on ? `Recording: about ${mins} min left` : "Off";
    state.className = `text-sm font-semibold ${r.on ? "text-red-300" : "text-slate-400"}`;
    log.textContent = r.events.length ? r.events.map(line).join("\n") : "Nothing recorded yet. Press Record, do the thing that goes wrong (on any device), then come back and press Refresh.";
  };
  host.addEventListener("click", async (e) => {
    const start = e.target.closest("[data-act-start]");
    if (start) {
      if (await attempt(() => put("/api/admin/activity", { minutes: Number(start.dataset.actStart) }), Number(start.dataset.actStart) ? "Recording started: everyone sees the bar" : "Recording stopped")) {
        await checkRecording(); // the bar on this screen follows at once
        draw();
      }
      return;
    }
    const k = e.target.closest("[data-act]")?.dataset.act;
    if (k === "refresh") draw();
    else if (k === "copy") {
      const txt = await attempt(() => fetch("/api/admin/activity?format=text").then((x) => x.text()));
      if (txt) copyText(txt);
    } else if (k === "clear" && confirm("Delete the whole timeline?") && (await attempt(() => del("/api/admin/activity"), "Deleted"))) draw();
  });
  draw();
}
