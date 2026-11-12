// Nightstand → Continue listening: the audiobooks the person is partway through.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { coverImg } from "./covers.js";
import { fmtLen } from "./audioui.js";
import { setBars } from "./progress.js";
import { pepperChip } from "./peppers.js";

export async function renderListening(host) {
  const r = await get("/api/listening").catch(() => null);
  if (!r?.books?.length) {
    host.innerHTML = "";
    return;
  }
  host.innerHTML = `<section class="mb-8"><h2 class="label">Continue listening</h2>
    <div class="grid gap-3 sm:grid-cols-2">${r.books.map((b) => {
      const pct = Math.min(100, Math.round((b.progress || b.at / (b.duration_sec || 1)) * 100));
      const left = Math.max(0, b.duration_sec - b.at);
      return `<button type="button" data-listen="${b.id}" class="card flex gap-3 text-left transition hover:ring-indigo-600">
        ${coverImg(b.id, "h-20 w-14")}
        <span class="min-w-0 flex-1 space-y-1"><span class="block font-semibold leading-tight line-clamp-2">${esc(b.title)}</span>
          <span class="block truncate text-sm text-slate-400">${esc(b.author || "")}${b.narrator ? ` · 🎙 ${esc(b.narrator)}` : ""}</span>
          <span class="block h-1.5 overflow-hidden rounded-full bg-slate-800"><span class="block h-full rounded-full bg-indigo-500" data-pct="${pct}"></span></span>
          <span class="flex flex-wrap items-center gap-2 text-xs text-slate-400">${pct}%${b.duration_sec ? ` · ${fmtLen(left)} left` : ""}
            ${b.spice_level === null || b.spice_level === undefined ? "" : pepperChip(b.spice_level)}</span></span></button>`;
    }).join("")}</div></section>`;
  setBars(host);
  host.onclick = async (e) => {
    const id = e.target.closest("[data-listen]")?.dataset.listen;
    if (id) (await import("./player.js")).openPlayer(Number(id));
  };
}
