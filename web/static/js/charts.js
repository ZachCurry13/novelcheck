// Tiny dependency-free SVG charts for the Usage page: a line chart with a
// hover crosshair and a column chart with per-bar tooltips. Everything is
// drawn with attributes/classes (the page's security policy blocks inline styles).
import { esc } from "./ui.js";

const W = 600, H = 120, PAD = 4;

// series: [{ name, cls, values: [number] }]; labels: x labels (same length);
// fmt formats a value for the readout.
export function lineChart(el, { series, labels, fmt, max }) {
  const n = labels.length;
  if (n < 2) {
    el.innerHTML = `<p class="py-6 text-center text-xs text-slate-500">Collecting data… the chart fills in over the next minute.</p>`;
    return;
  }
  const top = Math.max(max ?? 0, ...series.flatMap((s) => s.values), 1e-9);
  const x = (i) => PAD + (i / (n - 1)) * (W - 2 * PAD);
  const y = (v) => H - PAD - (v / top) * (H - 2 * PAD);
  const paths = series.map((s) =>
    `<polyline class="${s.cls} fill-none" stroke-width="2" vector-effect="non-scaling-stroke" stroke-linejoin="round"
      points="${s.values.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ")}"/>`).join("");
  const legend = series.length > 1 ? `<div class="flex gap-4 text-xs text-slate-400">${series.map((s) =>
    `<span class="flex items-center gap-1"><span class="inline-block h-0.5 w-4 ${s.cls.replace("stroke-", "bg-")}"></span>${esc(s.name)}</span>`).join("")}</div>` : "";
  el.innerHTML = `${legend}
    <p class="readout h-4 text-xs text-slate-300" aria-live="polite"></p>
    <svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" class="h-28 w-full cursor-crosshair" role="img"
      aria-label="${esc(series.map((s) => `${s.name} now ${fmt(s.values[n - 1])}`).join(", "))}">
      <line x1="0" x2="${W}" y1="${H - PAD}" y2="${H - PAD}" class="stroke-slate-700" stroke-width="1" vector-effect="non-scaling-stroke"/>
      ${paths}
      <line class="crosshair hidden stroke-slate-400" y1="0" y2="${H}" stroke-width="1" vector-effect="non-scaling-stroke"/>
    </svg>
    <div class="flex justify-between text-[10px] text-slate-500"><span>${esc(labels[0])}</span><span>now</span></div>`;
  const svg = el.querySelector("svg"), hair = el.querySelector(".crosshair"), out = el.querySelector(".readout");
  const show = (i) => {
    hair.setAttribute("x1", x(i)); hair.setAttribute("x2", x(i));
    hair.classList.remove("hidden");
    out.textContent = `${labels[i]} · ${series.map((s) => `${s.name}: ${fmt(s.values[i])}`).join(" · ")}`;
  };
  const pick = (e) => {
    const r = svg.getBoundingClientRect();
    show(Math.min(n - 1, Math.max(0, Math.round(((e.clientX - r.left) / r.width) * (n - 1)))));
  };
  svg.addEventListener("pointermove", pick);
  svg.addEventListener("pointerdown", pick); // a tap on a phone
  // With a mouse the readout follows the pointer; after a tap it stays until the next tap.
  svg.addEventListener("pointerleave", (e) => {
    if (e.pointerType !== "mouse") return;
    hair.classList.add("hidden");
    out.textContent = "";
  });
}

// bars: [{ label, value, tip }]. Hover (or tap, on a phone) a bar to read its
// exact value; a tapped bar stays highlighted until the next tap.
export function columnChart(el, bars, cls = "fill-indigo-500") {
  const top = Math.max(...bars.map((b) => b.value), 1);
  const slot = W / bars.length, gap = 2;
  el.innerHTML = `<p class="readout h-4 text-xs text-slate-300" aria-live="polite"></p>
    <svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" class="h-28 w-full cursor-pointer" role="img" aria-label="Daily AI tokens">
    ${bars.map((b, i) => {
      const h = b.value ? Math.max(2, (b.value / top) * (H - PAD)) : 0;
      return `<g class="group"><rect data-i="${i}" x="${i * slot}" y="0" width="${slot}" height="${H}" class="fill-transparent"><title>${esc(b.tip)}</title></rect>
        <rect data-bar="${i}" x="${i * slot + gap}" y="${H - h}" width="${slot - 2 * gap}" height="${h}" rx="2" class="${cls} group-hover:opacity-80 pointer-events-none"/></g>`;
    }).join("")}
    <line x1="0" x2="${W}" y1="${H}" y2="${H}" class="stroke-slate-700" stroke-width="1" vector-effect="non-scaling-stroke"/></svg>
    <div class="flex justify-between text-[10px] text-slate-500"><span>${esc(bars[0]?.label || "")}</span><span>today</span></div>`;
  const svg = el.querySelector("svg"), out = el.querySelector(".readout");
  const show = (e) => {
    const i = e.target.closest?.("[data-i]")?.dataset.i;
    if (i === undefined) return;
    out.textContent = bars[i].tip;
    el.querySelectorAll("[data-bar]").forEach((r) => r.classList.toggle("opacity-60", r.dataset.bar !== i));
  };
  svg.addEventListener("pointermove", show);
  svg.addEventListener("pointerdown", show);
  svg.addEventListener("pointerleave", (e) => {
    if (e.pointerType !== "mouse") return;
    out.textContent = "";
    el.querySelectorAll("[data-bar]").forEach((r) => r.classList.remove("opacity-60"));
  });
}
