// What a local model is good for in NovelCheck, judged from its family and size (not measured on your
// books): rating a book from its description, Deep Scan (reading the whole book), and reading cover photos.
// The same rules of thumb as the hints in Admin: small 1-3B models guess and over-rate on whole books, 7B and
// up read with more care, "thinking" models are slow and often break the answer format.
import { esc } from "./ui.js";

const DEFAULT_SIZE = { "llama3.2": 3, "llama3.1": 8, gemma3: 4, "qwen2.5": 7, mistral: 7, phi3: 3.8, phi4: 14, llama3: 8, gemma2: 9 };
const VISUAL = ["gemma3", "llava", "vision", "moondream", "minicpm-v", "qwen2.5vl", "qwen2-vl", "bakllava", "pixtral"];

// sizeOf is the model's size in billions of parameters (0 when the name doesn't say).
export function sizeOf(name) {
  const n = name.toLowerCase().replace(/:latest$/, "");
  const m = n.match(/(\d+(?:\.\d+)?)b\b/);
  if (m) return Number(m[1]);
  return DEFAULT_SIZE[n.split(":")[0]] || 0;
}

// jobsOf says how the model does at each job: "good", "ok", "weak" or "" (can't tell), with a few words.
export function jobsOf(name) {
  const n = name.toLowerCase();
  const size = sizeOf(name);
  const thinking = /deepseek-r1|qwq|-r1\b|reason|think/.test(n);
  const photos = VISUAL.some((v) => n.includes(v));
  const grade = (good, ok) => (!size ? "" : thinking ? "weak" : size >= good ? "good" : size >= ok ? "ok" : "weak");
  return { size, thinking, photos, rate: grade(7, 3), deep: grade(12, 7) };
}

const WORDS = {
  rate: { good: "careful ratings", ok: "quick ratings, less careful", weak: "too small for dependable ratings", "": "ratings: not known" },
  deep: { good: "careful Deep Scans", ok: "Deep Scan: the smallest size that reads well", weak: "Deep Scan: too small, tends to over-rate (7B or bigger)", "": "Deep Scan: not known" },
};
const MARK = { good: "⭐", ok: "✓", weak: "⚠️", "": "?" };
const TONE = { good: "text-emerald-300", ok: "text-slate-300", weak: "text-amber-300", "": "text-slate-500" };

// jobLine is one line under a model's name; job ("rate" or "deep") is the job this list is for, shown first.
export function jobLine(name, job = "rate") {
  const j = jobsOf(name);
  const part = (k) => `<span class="${TONE[j[k]]}">${MARK[j[k]]} ${esc(WORDS[k][j[k]])}</span>`;
  const out = [part(job), part(job === "rate" ? "deep" : "rate")];
  if (j.photos) out.push(`<span class="text-emerald-300">reads cover photos</span>`);
  if (j.thinking) out.push(`<span class="text-amber-300">a "thinking" model: slow, and it often breaks the answer format</span>`);
  return out.join(` <span class="text-slate-600">·</span> `);
}

// jobScore ranks models for a job: higher is better suited (a bigger model within the same grade wins).
export function jobScore(name, job = "rate") {
  const j = jobsOf(name);
  return { good: 300, ok: 200, "": 100, weak: 0 }[j[job]] + Math.min(j.size, 99);
}

// bestFirst says whether the first ticked model is the best suited of the ticked ones, and names a better one.
export function betterFirst(list, job = "rate") {
  const on = list.filter((m) => m.on);
  if (on.length < 2) return "";
  const best = on.reduce((a, b) => (jobScore(b.name, job) > jobScore(a.name, job) ? b : a));
  return jobsOf(best.name)[job] !== jobsOf(on[0].name)[job] && jobScore(best.name, job) > jobScore(on[0].name, job) ? best.name : "";
}
