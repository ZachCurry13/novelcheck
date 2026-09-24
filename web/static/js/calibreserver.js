// Admin → Calibre Library → One-click removal: connect to calibre's Content
// server so NovelCheck can remove filtered books through calibre.
import { get, put } from "./api.js";
import { $, esc, attempt, toast } from "./ui.js";
import { renderMarkdown } from "./markdown.js";

export async function renderCalibreServer(host) {
  const data = await attempt(() => get("/api/admin/calibre/server"));
  if (!data) return;
  host.innerHTML = `
    <div class="space-y-3 border-t border-slate-800 pt-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span class="label mb-0">One-click removal (optional)</span>
        <span id="cs-state">${data.configured ? `<span class="chip-none">Connected</span>` : `<span class="chip-pending">Not set up</span>`}</span>
      </div>
      <p class="text-xs text-slate-400">Lets <b>Remove hidden books from Calibre</b> remove books for you, through Calibre's Content server (to Calibre's recycle bin).</p>
      <button type="button" data-cs="guide" class="btn-secondary py-1 text-xs">Show setup steps</button>
      <div id="cs-guide" class="hidden rounded-lg bg-slate-800/60 p-4 text-sm leading-relaxed text-slate-300"></div>
      <div class="grid gap-2">
        <input data-cs-f="url" class="input" placeholder="Content server address, e.g. http://192.168.1.50:8081" value="${esc(data.url)}">
        <input data-cs-f="user" class="input" placeholder="Calibre username (e.g. novelcheck)" autocomplete="off" value="${esc(data.user)}">
        <input data-cs-f="password" type="password" class="input" autocomplete="new-password"
          placeholder="${data.has_password ? "Password saved. Type a new one to change it" : "Calibre password"}">
        <select data-cs-f="library" class="input ${data.library ? "" : "hidden"}">${data.library ? `<option>${esc(data.library)}</option>` : ""}</select>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" data-cs="save" class="btn-secondary">Test &amp; save</button>
        ${data.configured ? `<button type="button" data-cs="off" class="btn-ghost">Turn off</button>` : ""}
      </div>
    </div>`;
  const field = (n) => $(`[data-cs-f="${n}"]`, host);

  host.addEventListener("click", async (e) => {
    const act = e.target.closest("[data-cs]")?.dataset.cs;
    if (act === "guide") {
      const g = $("#cs-guide", host);
      g.innerHTML = renderMarkdown(data.guide);
      g.classList.toggle("hidden");
    } else if (act === "save" || act === "off") {
      const body = act === "off" ? { url: "" } : {
        url: field("url").value, user: field("user").value,
        password: field("password").value, library: field("library").value,
      };
      const r = await attempt(() => put("/api/admin/calibre/server", body));
      if (!r) return;
      if (!r.configured) {
        toast("One-click removal turned off");
        return renderCalibreServer(host);
      }
      const sel = field("library");
      sel.innerHTML = Object.entries(r.libraries).map(([id, name]) =>
        `<option value="${esc(id)}" ${id === r.library ? "selected" : ""}>${esc(name)}</option>`).join("");
      sel.classList.toggle("hidden", Object.keys(r.libraries).length < 2);
      field("password").value = "";
      $("#cs-state", host).innerHTML = `<span class="chip-none">Connected</span>`;
      toast(`Connected to Calibre (library: ${r.library})`);
    }
  });
}
