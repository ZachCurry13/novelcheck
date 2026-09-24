// Ollama easy setup: tick the downloaded models to use and put them in order.
// #1 rates every book; #2 is tried only if #1 fails, then #3, and so on.
import { esc } from "./ui.js";

const bare = (m) => m.replace(/:latest$/, "");

// initialOrder puts the models already in use first (in their saved order),
// ticked, followed by the other downloaded models, unticked.
export function initialOrder(downloaded, current) {
  const used = current.map((c) => downloaded.find((d) => d === c || bare(d) === bare(c))).filter(Boolean);
  const rest = downloaded.filter((d) => !used.includes(d));
  return [...new Set(used)].map((name) => ({ name, on: true })).concat(rest.map((name) => ({ name, on: false })));
}

// fits (optional) maps a model name to its GPU fit (see ollamahelper.js).
export function orderHTML(list, fits = {}) {
  const fitOf = (name) => fits[name] ?? fits[name.replace(/:latest$/, "")];
  if (!list.length) return `<li class="text-slate-400">No models downloaded yet.</li>`;
  let n = 0;
  return list.map((m, i) => {
    const rank = m.on ? ++n : 0;
    const role = rank === 1 ? "main" : rank ? `backup ${rank - 1}` : "not used";
    return `<li class="flex flex-wrap items-center gap-2 rounded-lg bg-slate-900/60 px-2 py-1" data-i="${i}">
      <span class="w-6 text-center font-bold ${rank ? "text-indigo-300" : "text-slate-600"}">${rank || "–"}</span>
      <label class="toggle min-w-0 flex-1"><input type="checkbox" data-ord="on" ${m.on ? "checked" : ""}>
        <code class="break-all">${esc(m.name)}</code></label>
      <span class="text-xs ${rank === 1 ? "text-emerald-300" : "text-slate-500"}">${role}</span>
      ${["too_big", "cpu_slow"].includes(fitOf(m.name)) ? `<span class="text-xs text-amber-300" title="Too big to run fully on your GPU; it will be slow">⚠️ slow</span>` : ""}
      <button type="button" data-ord="up" class="btn-ghost px-2 py-0.5" ${i === 0 ? "disabled" : ""} aria-label="Move ${esc(m.name)} up">↑</button>
      <button type="button" data-ord="down" class="btn-ghost px-2 py-0.5" ${i === list.length - 1 ? "disabled" : ""} aria-label="Move ${esc(m.name)} down">↓</button>
    </li>`;
  }).join("");
}

// bindOrder wires ticks and ↑/↓ inside ul; getList/setList hold the state.
export function bindOrder(ul, state) {
  ul.addEventListener("click", (e) => {
    const b = e.target.closest("[data-ord]");
    const li = e.target.closest("[data-i]");
    if (!b || !li) return;
    const i = Number(li.dataset.i);
    const list = state.list;
    if (b.dataset.ord === "on") list[i].on = b.checked;
    else {
      const j = b.dataset.ord === "up" ? i - 1 : i + 1;
      if (j < 0 || j >= list.length) return;
      [list[i], list[j]] = [list[j], list[i]];
    }
    ul.innerHTML = orderHTML(list, state.fits);
    const focus = ul.querySelector(`[data-i="${b.dataset.ord === "up" ? i - 1 : b.dataset.ord === "down" ? i + 1 : i}"] [data-ord="${b.dataset.ord}"]`);
    focus?.focus();
  });
}
