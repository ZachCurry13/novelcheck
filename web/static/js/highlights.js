// A book's page: the highlights and notes you made in it on your e-reader (the KOReader plugin sends them;
// api/plugin_highlights.go). Folded away, since a quote can give the story away; nothing shows when there are none.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { copyText } from "./copy.js";

export async function renderHighlights(host, b) {
  if (!host) return;
  const r = await get(`/api/books/${b.id}/highlights`).catch(() => null);
  const list = r?.highlights || [];
  if (!list.length || !host.isConnected) return;
  host.innerHTML = `<details class="rounded-lg bg-slate-800/60 p-3 text-sm"><summary class="cursor-pointer font-semibold text-slate-200">My highlights <span class="font-normal text-slate-400">(${list.length})</span></summary>
    <ul class="mt-3 space-y-3">${list.map((h) => `<li class="border-l-2 border-slate-600 pl-3">
      ${h.text ? `<p class="text-slate-200">“${esc(h.text)}”</p>` : ""}
      ${h.note ? `<p class="mt-1 text-slate-300"><span class="text-slate-400">Note:</span> ${esc(h.note)}</p>` : ""}
      <p class="mt-1 text-xs text-slate-500">${[h.chapter, h.page ? `page ${h.page}` : "", h.taken_at ? h.taken_at.slice(0, 10) : ""].filter(Boolean).map(esc).join(" · ")}</p></li>`).join("")}</ul>
    <button type="button" data-copy-hl class="btn-secondary mt-3 py-1 text-sm">Copy all</button>
    <p class="mt-2 text-xs text-slate-500">Sent from your e-reader. To remove one, delete it in KOReader.</p></details>`;
  host.querySelector("[data-copy-hl]").onclick = () => copyText(list.map((h) => [h.text && `“${h.text}”`, h.note && `Note: ${h.note}`].filter(Boolean).join("\n")).join("\n\n"));
}
