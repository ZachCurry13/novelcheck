// Settings → Developer (admins): "Check my servers" runs harmless read-only checks of Calibre, Audiobookshelf and
// Calibre-Web, shows what they answered, and (being in the server report) lets a bug report carry it.
import { post } from "./api.js";
import { attempt } from "./ui.js";
import { copyText } from "./copy.js";

export const serverCheckHTML = () => `<div class="card space-y-2 lg:col-span-2"><h2 class="text-lg font-semibold">Check my servers</h2>
  <p class="text-sm text-slate-400">Asks your Calibre Content server, Audiobookshelf and Calibre-Web a few harmless questions (nothing is changed): the library names, whether NovelCheck's books are in the library Calibre serves, which features each one has, and what the Audiobookshelf token may do. The answer is added to the server report.</p>
  <button type="button" data-sc="run" class="btn-secondary self-start">Run the checks</button>
  <pre id="sc-out" class="hidden max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-3 text-xs text-slate-300"></pre>
  <button type="button" data-sc="copy" class="btn-ghost hidden self-start py-1 text-sm">Copy</button></div>`;

export function bindServerCheck(host) {
  const out = host.querySelector("#sc-out");
  const copy = host.querySelector("[data-sc=copy]");
  host.addEventListener("click", async (e) => {
    const k = e.target.closest("[data-sc]")?.dataset.sc;
    if (k === "copy") copyText(out.textContent);
    else if (k === "run") {
      e.target.disabled = true;
      out.classList.remove("hidden");
      out.textContent = "Checking… this can take up to a minute.";
      const r = await attempt(() => post("/api/admin/server-checks", {}));
      out.textContent = r?.text || "No answer.";
      copy.classList.toggle("hidden", !r);
      e.target.disabled = false;
    }
  });
}
