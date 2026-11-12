// A review window before a cleanup step changes anything: what will change, one line each, with a
// tick box to leave an item out. review() answers with the ticked items' ids, or null if cancelled.
import { esc } from "./ui.js";

function dialog() {
  let d = document.getElementById("cleanup-review");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "cleanup-review";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

// An item with warn starts unticked and shows the warning: it is only included when ticked on purpose.
// spec: { title, intro, items: [{ id, label, detail, warn }], confirm, fixed (true: no tick boxes, all or nothing), more }
export function review(spec) {
  const d = dialog();
  const items = spec.items || [];
  return new Promise((resolve) => {
    let done = false;
    const finish = (value) => {
      if (done) return;
      done = true;
      d.close();
      resolve(value);
    };
    d.innerHTML = `<div class="max-h-[88vh] space-y-3 overflow-y-auto p-5">
      <h2 class="text-lg font-semibold">Review: ${esc(spec.title)}</h2>
      <p class="text-sm text-slate-400">${esc(spec.intro || "")}</p>
      ${spec.fixed ? "" : `<div class="flex flex-wrap items-center gap-3 text-sm"><button type="button" data-all class="underline">${items.some((it) => it.warn) ? "Tick all without a warning" : "Tick all"}</button>
        <button type="button" data-none class="underline">Tick none</button><span data-count class="text-slate-400"></span></div>`}
      <ul class="max-h-[50vh] space-y-1 overflow-y-auto">${items.map((it, i) => `<li><label class="flex items-start gap-2 rounded-lg bg-slate-800/60 p-2 text-sm">
        ${spec.fixed ? "" : `<input type="checkbox" data-i="${i}" ${it.warn ? "" : "checked"} class="mt-1">`}
        <span class="min-w-0"><span class="break-words font-semibold">${esc(it.label)}</span>
          ${it.detail ? `<span class="block break-words text-xs text-slate-400">${esc(it.detail)}</span>` : ""}
          ${it.warn ? `<span class="block break-words text-xs text-amber-300">Not ticked: ${esc(it.warn)}</span>` : ""}</span></label></li>`).join("")}</ul>
      ${spec.more ? `<p class="text-xs text-slate-400">${esc(spec.more)}</p>` : ""}
      <div class="flex flex-wrap gap-2"><button type="button" data-ok class="btn-primary">${esc(spec.confirm || "Go ahead")}</button>
        <button type="button" data-cancel class="btn-ghost">Cancel</button></div></div>`;
    const boxes = () => [...d.querySelectorAll("input[data-i]")];
    const count = () => {
      const n = boxes().filter((b) => b.checked).length;
      const c = d.querySelector("[data-count]");
      if (c) c.textContent = `${n} of ${items.length} ticked`;
      d.querySelector("[data-ok]").disabled = !spec.fixed && n === 0;
    };
    d.onchange = count;
    d.onclick = (e) => {
      if (e.target.closest("[data-cancel]")) return finish(null);
      if (e.target.closest("[data-all]")) boxes().forEach((b) => (b.checked = !items[Number(b.dataset.i)].warn)); // never the doubtful ones
      if (e.target.closest("[data-none]")) boxes().forEach((b) => (b.checked = false));
      count();
      if (e.target.closest("[data-ok]")) finish(spec.fixed ? items.map((it) => it.id) : boxes().filter((b) => b.checked).map((b) => items[Number(b.dataset.i)].id));
    };
    d.oncancel = () => finish(null); // Esc
    count();
    d.showModal();
  });
}
