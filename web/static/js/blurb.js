// "What's it about?" for cards: the AI's spoiler-free premise, or else the
// start of the book's own description with the marketing taken out (review
// quotes, best-seller and award lines). Never the rating note.
import { esc } from "./ui.js";

const HYPE = /best[- ]?sell|#\s?1\b|new york times|usa today|acclaimed|award|praise|review|book club|goodreads|must[- ]read|instant|“[^”]*”\s*[—–-]|"[^"]*"\s*[—–-]|\bpicks?\b|now a major|soon to be a/i;

// cardBlurb returns plain text: at most about two sentences.
export function cardBlurb(b, max = 240) {
  if (b.premise) return b.premise;
  const text = String(b.blurb || b.description || "").replace(/<[^>]*>/g, " ")
    .replace(/[“"][^”"]{1,300}[”"]\s*[—–-]+\s*[^.!?“"]{0,80}[.!?]?/g, " ") // “A thrill ride!” —People.
    .replace(/\s+/g, " ").trim();
  if (!text) return "";
  let out = "";
  for (const s of text.match(/[^.!?]+[.!?]+["”’)]*|[^.!?]+$/g) || []) {
    const sentence = s.trim();
    if (!sentence || HYPE.test(sentence) || sentence === sentence.toUpperCase()) continue;
    if (out && (out + " " + sentence).length > max) break;
    out = out ? `${out} ${sentence}` : sentence;
    if (out.length > max) return out.slice(0, max - 1).trimEnd() + "…";
  }
  return out;
}

// blurbHTML is the card paragraph, or "" when there's nothing to say yet.
export function blurbHTML(b, cls = "text-sm text-slate-300 line-clamp-3") {
  const t = cardBlurb(b);
  return t ? `<p class="${cls}">${esc(t)}</p>` : "";
}
