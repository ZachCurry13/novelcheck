// 📖 What a kid is reading, on their card in Admin → Users (parents): the
// books they're reading now with how far along they are, and what they
// opened lately in KOReader.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { ago, pct, progressBar, setBars } from "./progress.js";

function readingHTML(data) {
  const reading = data.reading || [];
  const inReading = new Set(reading.map((i) => i.book_id));
  const opened = (data.books || []).filter((b) => !b.book_id || !inReading.has(b.book_id)).slice(0, 3);
  if (!reading.length && !opened.length) return `<p class="text-xs text-slate-500">📖 Not reading anything in Up Next right now.</p>`;
  return `<div class="space-y-2 rounded-lg bg-slate-800/60 p-2 text-sm">
    <p class="label mb-0">📖 Reading now</p>
    ${reading.map((i) => `<div class="space-y-1"><p class="leading-snug">${esc(i.title)}</p>
      ${i.progress ? `${progressBar(i.progress)}<p class="text-xs text-slate-400">${pct(i.progress)}% · ${ago(i.progress.at)}</p>`
        : `<p class="text-xs text-slate-500">No progress from an e-reader yet</p>`}</div>`).join("") || `<p class="text-xs text-slate-500">Nothing in Up Next right now.</p>`}
    ${opened.length ? `<p class="text-xs text-slate-400">Lately in KOReader: ${opened.map((b) => `${esc(b.title)} (${Math.round(b.percent * 100)}%, ${ago(b.last_open)})`).join(" · ")}</p>` : ""}
  </div>`;
}

// fillKidReading fills each kid card's [data-reading] box.
export function fillKidReading(root) {
  root.querySelectorAll("[data-reading]").forEach(async (box) => {
    const data = await get(`/api/admin/users/${box.dataset.reading}/reading`).catch(() => null);
    if (!data) return;
    box.innerHTML = readingHTML(data);
    setBars(box);
  });
}
