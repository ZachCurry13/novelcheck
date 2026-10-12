// Reading stats: the building blocks of the report. A card has a small label
// and a plain-sentence headline ("You read 8 books"), then its chart or list.
// Charts are SVG drawn with attributes and classes (the page's security rules
// block inline styles); bars are sized by setBars (data-pct).
import { esc } from "./ui.js";
import { coverImg } from "./covers.js";

export const card = (label, headline, inner) => `<section class="card space-y-3">
  <div><p class="text-sm text-slate-400">${esc(label)}</p><h2 class="text-xl font-semibold leading-snug">${headline}</h2></div>${inner}</section>`;

export const bigNum = (icon, n, label) => `<div class="flex items-start gap-3"><span class="text-xl" aria-hidden="true">${icon}</span>
  <div class="min-w-0"><p class="text-xl font-bold leading-tight">${n}</p><p class="text-sm text-slate-400">${esc(label)}</p></div></div>`;

const FILLS = ["bg-indigo-500", "bg-sky-500", "bg-pink-500", "bg-amber-500", "bg-emerald-500", "bg-violet-500", "bg-rose-500", "bg-orange-500"];
const TEXTS = ["text-indigo-400", "text-sky-400", "text-pink-400", "text-amber-400", "text-emerald-400", "text-violet-400", "text-rose-400", "text-orange-400"];

// barRows: a label, a bar and a value on each row. rows: [{ label, value, text, attrs }]; attrs makes the row a button.
export function barRows(rows, { colour = true } = {}) {
  const top = Math.max(...rows.map((r) => r.value), 1);
  return `<ul class="space-y-2">${rows.map((r, i) => {
    const inner = `<span class="flex items-baseline justify-between gap-2 text-sm"><span class="min-w-0 break-words ${colour ? TEXTS[i % TEXTS.length] : ""}">${esc(r.label)}</span>
      <span class="shrink-0 tabular-nums text-slate-300">${esc(r.text)}</span></span>
      <span class="mt-1 block h-2.5 overflow-hidden rounded-full bg-slate-800"><span class="block h-full rounded-full ${colour ? FILLS[i % FILLS.length] : "bg-indigo-500"}" data-pct="${Math.round((r.value / top) * 100)}"></span></span>`;
    return `<li>${r.attrs ? `<button type="button" ${r.attrs} class="block w-full text-left">${inner}</button>` : inner}</li>`;
  }).join("")}</ul>`;
}

// donut: parts [{ label, value }] as a ring with its legend.
export function donut(parts) {
  const total = parts.reduce((a, p) => a + p.value, 0) || 1;
  const R = 40, C = 2 * Math.PI * R;
  let used = 0;
  const STROKE = ["stroke-pink-500", "stroke-indigo-500", "stroke-slate-500", "stroke-emerald-500", "stroke-amber-500"];
  const arcs = parts.map((p, i) => {
    const len = (p.value / total) * C;
    const arc = `<circle cx="50" cy="50" r="${R}" fill="none" stroke-width="16" class="${STROKE[i % STROKE.length]}" stroke-dasharray="${len.toFixed(2)} ${(C - len).toFixed(2)}" stroke-dashoffset="${(-used).toFixed(2)}" transform="rotate(-90 50 50)"/>`;
    used += len;
    return arc;
  }).join("");
  const dots = ["bg-pink-500", "bg-indigo-500", "bg-slate-500", "bg-emerald-500", "bg-amber-500"];
  return `<div class="flex flex-wrap items-center gap-4"><svg viewBox="0 0 100 100" class="h-36 w-36 shrink-0" role="img" aria-label="${esc(parts.map((p) => `${p.label} ${p.value}`).join(", "))}">${arcs}</svg>
    <ul class="min-w-0 flex-1 space-y-1 text-sm">${parts.map((p, i) => `<li class="flex items-center justify-between gap-2"><span class="flex items-center gap-2"><span class="h-3 w-3 rounded-full ${dots[i % dots.length]}"></span>${esc(p.label)}</span>
      <span class="tabular-nums text-slate-300">${Math.round((p.value / total) * 100)}% (${p.value})</span></li>`).join("")}</ul></div>`;
}

// areaChart: points [{ label, value }] as a filled line with the first and last label.
export function areaChart(points, fmt) {
  const W = 300, H = 90, n = points.length;
  if (n < 2) return "";
  const top = Math.max(...points.map((p) => p.value), 1);
  const x = (i) => (i / (n - 1)) * W;
  const y = (v) => H - 6 - (v / top) * (H - 14);
  const line = points.map((p, i) => `${x(i).toFixed(1)},${y(p.value).toFixed(1)}`).join(" ");
  const peak = points.reduce((a, p, i) => (p.value > points[a].value ? i : a), 0);
  return `<div><svg viewBox="0 0 ${W} ${H}" class="h-28 w-full" role="img" aria-label="${esc(points.map((p) => `${p.label} ${fmt(p.value)}`).join(", "))}">
    <polygon points="0,${H} ${line} ${W},${H}" class="fill-indigo-500/20"/>
    <polyline points="${line}" fill="none" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" class="stroke-indigo-500"/>
    ${top > 0 && points[peak].value > 0 ? `<circle cx="${x(peak).toFixed(1)}" cy="${y(points[peak].value).toFixed(1)}" r="3.5" class="fill-white stroke-indigo-500" stroke-width="2"/>` : ""}</svg>
    <div class="flex justify-between text-xs text-slate-500"><span>${esc(points[0].label)}</span><span>${esc(points[n - 1].label)}</span></div></div>`;
}

// columns: a labelled column per point, the value on top.
export function columns(points, fmt, cls = "fill-indigo-500") {
  const W = 300, H = 90, n = points.length, top = Math.max(...points.map((p) => p.value), 1), bw = W / n;
  const bars = points.map((p, i) => {
    const h = p.value ? Math.max(3, (p.value / top) * (H - 18)) : 0;
    return `<rect x="${(i * bw + bw * 0.12).toFixed(1)}" y="${(H - h).toFixed(1)}" width="${(bw * 0.76).toFixed(1)}" height="${h.toFixed(1)}" rx="2" class="${cls}"><title>${esc(p.label)}: ${esc(fmt(p.value))}</title></rect>`;
  }).join("");
  return `<div><svg viewBox="0 0 ${W} ${H}" class="h-28 w-full" role="img" aria-label="${esc(points.map((p) => `${p.label} ${fmt(p.value)}`).join(", "))}">
    <line x1="0" x2="${W}" y1="${H}" y2="${H}" class="stroke-slate-700"/>${bars}</svg>
    <div class="flex justify-between text-[10px] text-slate-500">${points.map((p, i) => `<span class="${n > 12 && i % Math.ceil(n / 6) ? "invisible" : ""}">${esc(p.label)}</span>`).join("")}</div></div>`;
}

// coverTile: a book's cover (or its first letter) that opens the book's page.
export const coverTile = (b, i, size = "h-28 w-20") => `<button type="button" data-book="${i}" class="shrink-0 text-left" title="${esc(b.title)}">${b.book_id ? coverImg(b.book_id, size) : `<span class="flex ${size} items-center justify-center rounded bg-slate-800 p-1 text-center text-xs text-slate-400">${esc(b.title.slice(0, 24))}</span>`}</button>`;
