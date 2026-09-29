// Reading progress from e-readers (KOReader's progress sync and reading
// statistics): "43% · Kindle · 2 hours ago", with a bar.
import { esc } from "./ui.js";

// ago is "just now", "5 min ago", "3 hours ago", "yesterday", "4 days ago"
// or a date.
export function ago(unix) {
  const s = Date.now() / 1000 - unix;
  if (s < 90) return "just now";
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  if (s < 86400) {
    const h = Math.round(s / 3600);
    return `${h} hour${h === 1 ? "" : "s"} ago`;
  }
  const d = Math.round(s / 86400);
  if (d === 1) return "yesterday";
  return d < 14 ? `${d} days ago` : new Date(unix * 1000).toLocaleDateString();
}

export const pct = (p) => Math.round((p?.percent || 0) * 100);

export const progressText = (p) => `${pct(p)}% · ${esc(p.device || "KOReader")} · ${ago(p.at)}`;

export const progressBar = (p) => `<div class="h-1.5 overflow-hidden rounded-full bg-slate-800">
  <div class="h-full rounded-full bg-indigo-500" data-pct="${pct(p)}"></div></div>`;

// setBars sizes the bars (the strict security rules allow no inline styles in HTML).
export const setBars = (host) => host.querySelectorAll("[data-pct]").forEach((e) => (e.style.width = `${e.dataset.pct}%`));

export const readTime = (sec) => (sec >= 3600 ? `${Math.round(sec / 360) / 10} h` : `${Math.max(1, Math.round(sec / 60))} min`);
