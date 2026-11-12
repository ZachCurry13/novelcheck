// The audiobook part of a book's window: listen, download, and asking the
// family for the other format (the audiobook of an e-book they have, or the e-book of an audiobook).
import { post, del } from "./api.js";
import { esc, attempt } from "./ui.js";
import { fmtLen } from "./audioui.js";

// renderAudioSection fills host; data is the book's reply from the server.
export function renderAudioSection(host, data, b, state) {
  const wishes = { audio: data.my_audio_wish, ebook: data.my_ebook_wish };
  const request = (fmt, what) => {
    if (wishes[fmt]) return `<button type="button" data-unwish-fmt="${fmt}" class="btn-ghost text-sm" title="Take it off your wishlist">${what} requested · Cancel</button>`;
    return `<button type="button" data-wish-fmt="${fmt}" class="btn-secondary text-sm">Request the ${what.toLowerCase()}</button>`;
  };
  const draw = () => {
    const wanted = [];
    if (data.owned && !data.has_audio) wanted.push(request("audio", "Audiobook"));
    if (data.owned && !data.has_ebook) wanted.push(request("ebook", "E-book"));
    if (!data.has_audio && !wanted.length) {
      host.innerHTML = "";
      return;
    }
    host.innerHTML = `<div class="space-y-2 rounded-lg bg-slate-800/60 p-3 text-sm">
      ${data.has_audio ? `<p class="label">Audiobook</p>
        <p>${b.duration_sec ? `<b>${fmtLen(b.duration_sec)}</b>` : "Audiobook"}${b.narrator ? ` · read by ${esc(b.narrator)}` : ""}</p>
        <div class="flex flex-wrap gap-2"><button type="button" data-listen class="btn-primary text-sm">▶ Listen</button>
          <a href="/api/books/${b.id}/download?format=audio" class="btn-secondary text-sm">⬇ Download audiobook</a></div>` : ""}
      ${wanted.length ? `<div class="flex flex-wrap gap-2">${wanted.join("")}</div>
        <p class="text-xs text-slate-500">A request goes on the family wishlist, tagged with the format.</p>` : ""}</div>`;
  };
  draw();
  host.onclick = async (e) => {
    if (e.target.closest("[data-listen]")) {
      const dlg = host.closest("dialog");
      const m = await import("./player.js");
      dlg?.close();
      return m.openPlayer(b.id, state);
    }
    const ask = e.target.closest("[data-wish-fmt]")?.dataset.wishFmt;
    const undo = e.target.closest("[data-unwish-fmt]")?.dataset.unwishFmt;
    if (ask) {
      const r = await attempt(() => post(`/api/books/${b.id}/wish`, { format: ask }), "Requested: it's on the family wishlist");
      if (r) wishes[ask] = r.wish;
    } else if (undo) {
      if (await attempt(() => del(`/api/books/${b.id}/wish?format=${undo}`), "Request cancelled")) wishes[undo] = null;
    } else return;
    draw();
  };
}
