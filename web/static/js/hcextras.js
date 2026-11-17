// What the book page shows from Hardcover's readers (api/hcmore.go): the content warnings and moods they
// tagged the book with (folded, since a warning can give the story away), the audiobook edition's length and
// narrators, and "More like this", the books they find similar, marked when the family has them.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { fmtLen } from "./audioui.js";

const list = (s) => String(s || "").split(",").map((x) => x.trim()).filter(Boolean);

// hcTagsHTML is the warnings and moods line, or "".
export function hcTagsHTML(b) {
  const warn = list(b.hc_warnings), moods = list(b.hc_moods);
  if (!warn.length && !moods.length) return "";
  return `<div class="hc-tags">
    ${moods.length ? `<p class="text-xs text-slate-400"><b class="text-slate-300">Mood:</b> ${moods.map(esc).join(" · ")}</p>` : ""}
    ${warn.length ? `<details class="hc-warn"><summary class="text-xs text-slate-300">Content warnings from Hardcover readers (${warn.length})</summary>
      <div class="mt-1 flex flex-wrap gap-1">${warn.map((w) => `<span class="chip-flag">${esc(w)}</span>`).join("")}</div>
      <p class="mt-1 text-xs text-slate-500">Tagged by readers on Hardcover; NovelCheck's AI reads them when it rates the book.</p></details>` : ""}</div>`;
}

// renderHCMore fills host with the audiobook line and "More like this"; open(id) shows a family book.
export async function renderHCMore(host, b, { hasAudio, open }) {
  if (!host) return;
  const m = await get(`/api/books/${b.id}/hardcover`).catch(() => null);
  if (!m || !host.isConnected) return;
  const audio = !hasAudio && m.audio_seconds > 0
    ? `<p class="text-xs text-slate-400"><b class="text-slate-300">Audiobook exists:</b> ${fmtLen(m.audio_seconds)}${m.narrators?.length ? `, read by ${m.narrators.map(esc).join(", ")}` : ""}</p>` : "";
  const sim = (m.similar || []).slice(0, 10);
  host.innerHTML = audio + (sim.length ? `<div><span class="label">More like this</span>
    <div class="hc-sim" role="list">${sim.map((x) => {
      const go = x.book_id ? `data-open="${x.book_id}"` : `data-check="${esc(`${x.title} ${x.author || ""}`.trim())}"`;
      return `<button type="button" role="listitem" class="hc-sim-item" ${go}>
        ${x.cover ? `<img src="${esc(x.cover)}" alt="" loading="lazy" referrerpolicy="no-referrer">` : `<span class="hc-sim-blank" aria-hidden="true"></span>`}
        <span class="hc-sim-t">${esc(x.title)}</span><span class="hc-sim-a">${esc(x.author || "")}</span>
        <span class="hc-sim-m">${x.book_id ? "✓ We have it" : x.hc_rating > 0 ? `★ ${x.hc_rating.toFixed(1)}` : ""}</span></button>`;
    }).join("")}</div></div>` : "");
  host.onclick = (e) => {
    const t = e.target.closest("[data-open],[data-check]");
    if (!t) return;
    if (t.dataset.open) return open(Number(t.dataset.open));
    location.hash = `#/check?q=${encodeURIComponent(t.dataset.check)}`;
    t.closest("dialog")?.close();
  };
}
