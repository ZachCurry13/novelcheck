// "Try the new rules on 20 books": rates a small sample with the current rules
// and shows old versus new, saving nothing.
import { get, post } from "./api.js";
import { esc, attempt, toast } from "./ui.js";

function table(rows) {
  const n = (d) => rows.filter((r) => r.difference === d).length;
  const mark = { same: "=", higher: "▲ higher", lower: "▼ lower" };
  return `<p class="mb-2 text-sm"><b>${n("same")}</b> same, <b>${n("higher")}</b> higher, <b>${n("lower")}</b> lower. Nothing was saved.</p>
  <div class="max-h-80 overflow-auto"><table class="w-full text-left text-xs"><tbody>${rows
    .map((r) => `<tr class="border-t border-slate-700"><td class="py-1 pr-2">${esc(r.title)}<br><span class="opacity-60">${esc(r.author)}</span></td>
      <td class="pr-2 whitespace-nowrap">${r.old_level ?? "?"} → <b>${r.new_level ?? "?"}</b></td>
      <td class="whitespace-nowrap">${mark[r.difference] || ""}</td></tr>`)
    .join("")}</tbody></table></div>`;
}

export async function tryNewRules(count = 20) {
  const dlg = document.createElement("dialog");
  dlg.className = "rounded-lg bg-slate-900 p-4 text-slate-100 w-[min(32rem,92vw)]";
  dlg.innerHTML = `<h3 class="mb-2 font-semibold">Try the new rules</h3><div data-body>Starting…</div>
    <div class="mt-3 text-right"><button class="btn-secondary py-1" data-close>Close</button></div>`;
  dlg.querySelector("[data-close]").onclick = () => dlg.close();
  dlg.addEventListener("close", () => { clearInterval(timer); dlg.remove(); });
  document.body.append(dlg);
  dlg.showModal();
  const body = dlg.querySelector("[data-body]");
  let timer;
  const r = await attempt(() => post("/api/admin/rerate/trial", { count }));
  if (!r) { dlg.close(); return; }
  const poll = async () => {
    const s = await get("/api/admin/rerate/trial").catch(() => null);
    if (!s) return;
    if (s.running) body.textContent = `Rating ${s.results?.length ?? 0} of ${s.total || count}… (about 20 seconds each)`;
    else { clearInterval(timer); body.innerHTML = s.error ? esc(s.error) : table(s.results || []); }
  };
  timer = setInterval(poll, 3000);
  poll();
  toast("Trial started");
}
