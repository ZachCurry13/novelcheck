// A book's page: the highlights and notes you made in it on your e-reader (the KOReader plugin and the Kindle app send
// them; api/plugin_highlights.go) and the words looked up in Amazon's reader (the Kindle app). Folded away, since a quote can give the story away; nothing shows when there are none.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { copyText } from "./copy.js";

export async function renderHighlights(host, b) {
  if (!host) return;
  const r = await get(`/api/books/${b.id}/highlights`).catch(() => null);
  const list = r?.highlights || [], words = r?.words || [];
  if ((!list.length && !words.length) || !host.isConnected) return;
  host.innerHTML = (list.length ? `<details class="rounded-lg bg-slate-800/60 p-3 text-sm"><summary class="cursor-pointer font-semibold text-slate-200">My highlights <span class="font-normal text-slate-400">(${list.length})</span></summary>
    <ul class="mt-3 space-y-3">${list.map((h) => `<li class="border-l-2 border-slate-600 pl-3">
      ${h.text ? `<p class="text-slate-200">“${esc(h.text)}”</p>` : ""}
      ${h.note ? `<p class="mt-1 text-slate-300"><span class="text-slate-400">Note:</span> ${esc(h.note)}</p>` : ""}
      <p class="mt-1 text-xs text-slate-500">${[h.chapter, h.page ? `page ${h.page}` : "", h.taken_at ? h.taken_at.slice(0, 10) : ""].filter(Boolean).map(esc).join(" · ")}</p></li>`).join("")}</ul>
    <button type="button" data-copy-hl class="btn-secondary mt-3 py-1 text-sm">Copy all</button>
    <p class="mt-2 text-xs text-slate-500">Sent from your e-reader. To remove one, delete it on the e-reader.</p></details>` : "")
    + (words.length ? `<details class="mt-2 rounded-lg bg-slate-800/60 p-3 text-sm"><summary class="cursor-pointer font-semibold text-slate-200">Words I looked up <span class="font-normal text-slate-400">(${words.length})</span></summary>
    <ul class="mt-3 space-y-2">${words.map((w) => `<li><b class="text-slate-200">${esc(w.word)}</b>${w.usage ? `<span class="text-slate-400">: “${esc(w.usage)}”</span>` : ""}</li>`).join("")}</ul>
    <p class="mt-2 text-xs text-slate-500">From the Kindle's dictionary lookups (the Kindle app sends them).</p></details>` : "");
  const copy = host.querySelector("[data-copy-hl]");
  if (copy) copy.onclick = () => copyText(list.map((h) => [h.text && `“${h.text}”`, h.note && `Note: ${h.note}`].filter(Boolean).join("\n")).join("\n\n"));
}
