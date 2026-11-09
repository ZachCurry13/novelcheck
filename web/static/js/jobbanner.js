// A banner at the top of Admin while a format job (converting to EPUB, removing formats, undoing) runs
// on the server. The job keeps going when you leave the page, and NovelCheck sends a notice when it ends.
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";

const KINDS = { convert: "Converting to EPUB", remove: "Removing other formats", restore: "Putting formats back" };

export function watchJob(box) {
  const tick = async () => {
    if (!box.isConnected) return; // the page was left
    const j = await get("/api/admin/formats/job").catch(() => null);
    if (j && j.running) {
      box.classList.remove("hidden");
      box.innerHTML = `⚙️ <b>${esc(KINDS[j.kind] || "Working on your library")}</b> in the background: ${j.done} of ${j.total}${j.failed ? `, ${j.failed} did not work` : ""}${j.current ? ` · ${esc(j.current)}${j.percent ? ` (${j.percent}%)` : ""}` : ""}
        <button type="button" data-job="stop" class="ml-2 underline">Stop after this one</button>
        <span class="block text-xs text-slate-400">You can leave this page. NovelCheck sends a notice when it is done.</span>`;
      setTimeout(tick, 4000);
    } else {
      box.classList.add("hidden");
      setTimeout(tick, 15000);
    }
  };
  box.onclick = async (e) => {
    if (e.target.closest("[data-job=stop]")) await attempt(() => post("/api/admin/formats/job/stop"), "Stopping after the current one");
  };
  tick();
}
