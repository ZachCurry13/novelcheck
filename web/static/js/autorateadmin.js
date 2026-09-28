// Admin → AI & Scans → Automatic rating: whether waiting books are rated by
// themselves while the AI is idle, and optionally only between set hours
// (in this browser's time zone). The fields carry data-key, so the settings
// form saves them with everything else.
import { get } from "./api.js";
import { $, esc } from "./ui.js";

const hourOptions = (sel) => Array.from({ length: 24 }, (_, h) =>
  `<option value="${h}" ${h === sel ? "selected" : ""}>${String(h).padStart(2, "0")}:00</option>`).join("");

export async function renderAutoRate(host, settings) {
  const [from, to] = /^\d{1,2}-\d{1,2}$/.test(settings.auto_rate_hours || "") ? settings.auto_rate_hours.split("-").map(Number) : [23, 7];
  const limited = Boolean(settings.auto_rate_hours);
  const choice = settings.auto_rate || "";
  host.innerHTML = `
    <p class="text-sm text-slate-400">When the AI has nothing to do, NovelCheck rates the books waiting for a rating by itself: books in someone's Up Next
      or wishlist first, then the newest. It keeps to the hourly token limit below.</p>
    <div><label class="label" for="s-auto_rate">Rate waiting books automatically</label>
      <select id="s-auto_rate" data-key="auto_rate" class="input">
        <option value="" ${choice === "" ? "selected" : ""}>Default: on with a local AI (Ollama), off with a paid one</option>
        <option value="on" ${choice === "on" ? "selected" : ""}>On</option>
        <option value="off" ${choice === "off" ? "selected" : ""}>Off (only "Analyze next batch")</option>
      </select></div>
    <label class="toggle"><input type="checkbox" id="auto-hours-on" ${limited ? "checked" : ""}> Only rate automatically between set hours (keeps the GPU free the rest of the day)</label>
    <div id="auto-hours" class="flex flex-wrap items-center gap-2 text-sm ${limited ? "" : "hidden"}">
      From <select id="auto-from" class="input w-auto">${hourOptions(from)}</select>
      until <select id="auto-to" class="input w-auto">${hourOptions(to)}</select>
      <span class="text-xs text-slate-400">(${esc(Intl.DateTimeFormat().resolvedOptions().timeZone || "your time")})</span>
    </div>
    <input type="hidden" data-key="auto_rate_hours" value="${esc(settings.auto_rate_hours || "")}">
    <input type="hidden" data-key="auto_rate_tz" value="${esc(settings.auto_rate_tz || "")}">
    <p id="auto-status" class="text-xs text-slate-400"></p>
    <div><label class="label" for="s-collection_ideas">AI collection ideas (about once a week)</label>
      <select id="s-collection_ideas" data-key="collection_ideas" class="input">
        <option value="" ${!settings.collection_ideas ? "selected" : ""}>Default: on with a local AI, off with a paid one</option>
        <option value="on" ${settings.collection_ideas === "on" ? "selected" : ""}>On</option>
        <option value="off" ${settings.collection_ideas === "off" ? "selected" : ""}>Off</option>
      </select>
      <p class="mt-1 text-xs text-slate-400">The AI proposes up to 3 themed collections from your library, within the hours above; keep or drop them on the Collections page.</p></div>`;
  const sync = () => {
    const on = $("#auto-hours-on", host).checked;
    $("#auto-hours", host).classList.toggle("hidden", !on);
    const from = $("#auto-from", host).value;
    const to = $("#auto-to", host).value;
    $('[data-key="auto_rate_hours"]', host).value = on && from !== to ? `${from}-${to}` : "";
    $('[data-key="auto_rate_tz"]', host).value = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
  };
  host.addEventListener("change", sync);
  const a = await get("/api/activity").catch(() => null);
  if (a) {
    $("#auto-status", host).textContent = `${a.waiting.toLocaleString()} book${a.waiting === 1 ? "" : "s"} waiting to be rated · automatic rating is ${a.auto ? (a.hours_open ? "on" : "on, outside its hours now") : "off"}.`;
  }
}
