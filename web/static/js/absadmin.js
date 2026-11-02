// Admin → Delivery & Services → Audiobookshelf: how the last read went, a test
// for the address and token as typed, and a "read it now" button.
import { get, post } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";

export async function renderABSAdmin(host, form) {
  const s = await attempt(() => get("/api/admin/audiobookshelf"));
  if (!s) return;
  const when = s.last_sync ? new Date(s.last_sync).toLocaleString() : "not yet";
  host.innerHTML = `
    <p class="text-xs text-slate-400">Your audiobook server. Create the token in Audiobookshelf under Settings → Users → your user → API token (an admin user sees every library).
      NovelCheck reads the audiobooks, their length and narrators, rates them for peppers like any other book, and plays and sends the audio itself, so the address and token never reach a browser.
      Each person can link their own Audiobookshelf account in Settings → Connections to keep their own place in each book.</p>
    <p class="text-sm">Last read: <b>${esc(when)}</b>${s.busy ? " (reading now…)" : ""}. ${s.audiobooks.toLocaleString()} audiobook${s.audiobooks === 1 ? "" : "s"} in the library.</p>
    ${s.result ? `<p class="text-xs text-slate-400">${esc(s.result)}</p>` : ""}
    <div class="flex flex-wrap gap-2">
      <button type="button" data-aact="test" class="btn-secondary">Test connection</button>
      <button type="button" data-aact="sync" class="btn-secondary" ${s.busy || !s.configured ? "disabled" : ""}>Read the library now</button>
    </div>
    ${s.configured ? "" : `<p class="text-xs text-slate-500">Save the address and token first, then read the library.</p>`}`;
  host.onclick = async (e) => {
    const act = e.target.closest("[data-aact]")?.dataset.aact;
    if (act === "test") {
      const r = await attempt(() => post("/api/admin/audiobookshelf/test", { url: $('[data-key="abs_url"]', form)?.value || "", token: $('[data-key="abs_api_token"]', form)?.value || "" }));
      if (r) toast(r.libraries.length ? `Connected: ${r.libraries.join(", ")}. Remember to save.` : "Connected, but there are no audiobook libraries yet.");
    } else if (act === "sync") {
      if (await attempt(() => post("/api/admin/audiobookshelf/sync"), "Reading the library; a big one takes a few minutes")) {
        e.target.disabled = true;
        setTimeout(() => renderABSAdmin(host, form), 20_000);
      }
    }
  };
}
