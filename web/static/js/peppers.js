// The 0-5 pepper scale. The server's AI prompt (internal/llm/prompt.go) uses
// the same descriptions. Labels stay neutral: "Level N: name".
import { esc } from "./ui.js";

export const PEPPERS = [
  { n: 0, name: "No Romance", cls: "chip-none",
    desc: "No kissing. Crushes, attraction, flirting, holding hands, hugs and noticing that someone is good-looking all stay at 0, as do family affection, suggestive jokes that never get explicit, non-sexual nudity with no description of bodies (skinny-dipping, a bath), a prostitute or an affair only mentioned, and a sexual assault only stated, not shown (it shows in the content details).",
    ex: "Harry Potter and the Sorcerer's Stone, The Hobbit" },
  { n: 1, name: "Sweet Romance", cls: "chip-none",
    desc: "Sweet or brief kisses: one kiss at the end, a couple's quick kisses (married or not), a couple's kiss on the forehead, cheek or head. No passionate kissing, no sexual desire.",
    ex: "Seeking Persephone (Sarah M. Eden)" },
  { n: 2, name: "Romantic", cls: "chip-none",
    desc: "Passionate kissing, even once, or a lot of kissing and cuddling. No sexual desire and no implied sex.",
    ex: "My Phony Valentine (Courtney Walsh)" },
  { n: 3, name: "Steamy Closed-Door", cls: "chip-closed",
    desc: "Sexual desire and tension: characters wanting each other physically, with sex only implied off the page (a door closes, a fade to black, the morning after, \"they made love\" with no description). Also a brief, non-sexual description of a naked body, or undressing that stops before anything graphic.",
    ex: "A romance that is clearly sexually charged but remains true closed-door" },
  { n: 4, name: "Explicit / Open Door", cls: "chip-open",
    desc: "Anything sexual shown on the page, even briefly: a sex scene of any length; touching private parts, even over clothes; explicit talk or jokes that describe sex acts; describing a character's body in a sexual way; a sexual assault shown on the page, even if not graphic; graphic undressing and touching before a door closes.",
    ex: "Fourth Wing (Rebecca Yarros); A Court of Thorns and Roses (Sarah J. Maas)" },
  { n: 5, name: "Very Explicit / Erotica-Level", cls: "chip-open",
    desc: "Sex in most chapters, long or very graphic scenes: the physical relationship is a main part of the book (erotica).",
    ex: "Fifty Shades of Grey (E. L. James)" },
];

// "🌶️🌶️" for 2; 0 peppers shows a single faded pepper so the scale reads.
export const pepperIcons = (n) => (n > 0 ? "🌶️".repeat(n) : `<span class="opacity-40">🌶️</span>`);

export function pepperChip(n) {
  const p = PEPPERS[n];
  return `<span class="${p.cls}" title="${esc(p.desc)}">${pepperIcons(n)} Level ${n}: ${esc(p.name)}</span>`;
}

// "Why was this flagged?": books at Level 3 or higher show the AI's reason
// ("Heavy innuendo, on-page foreplay"), also as a tooltip. Older ratings
// without a reason fall back to their flags.
export function whyChip(b) {
  const lvl = b.spice_level;
  if (lvl === null || lvl === undefined || lvl < 3) return "";
  const why = b.spice_reason || (b.heavy_innuendo ? "Heavy innuendo" : b.nudity ? "Nudity" : lvl >= 4 ? "Explicit scenes" : "Heavy tension");
  return `<span class="${lvl >= 4 ? "chip-open" : "chip-closed"}" title="Why this is Level ${lvl}: ${esc(why)}">ⓘ ${esc(why)}</span>`;
}

// Options for a pepper <select>; selected may be null.
export function pepperOptions(selected) {
  return PEPPERS.map((p) => `<option value="${p.n}" ${selected === p.n ? "selected" : ""}>Level ${p.n}: ${esc(p.name)}</option>`).join("");
}

export function pepperScaleHTML() {
  return `<ul class="space-y-3">${PEPPERS.map((p) => `
    <li><p class="font-semibold">${pepperIcons(p.n)} Level ${p.n}: ${esc(p.name)}</p>
      <p class="text-sm text-slate-300">${esc(p.desc)}</p>
      <p class="text-xs text-slate-400">Examples: ${esc(p.ex)}</p></li>`).join("")}</ul>`;
}

// openPepperGuide shows the scale in a dialog.
export function openPepperGuide() {
  let dlg = document.getElementById("pepper-dialog");
  if (!dlg) {
    dlg = document.createElement("dialog");
    dlg.id = "pepper-dialog";
    dlg.className = "dialog";
    dlg.addEventListener("click", (e) => {
      if (e.target === dlg || e.target.closest("[data-close]")) dlg.close();
    });
    document.body.append(dlg);
  }
  dlg.innerHTML = `<div class="max-h-[85vh] space-y-4 overflow-y-auto p-6">
    <div class="flex items-start justify-between gap-4"><h2 class="text-xl font-semibold">What do the peppers mean?</h2>
      <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    ${pepperScaleHTML()}
    <p class="text-xs text-slate-500">Books rated before the pepper scale show their older label (No Spice, Closed Door, Open Door) until they're re-rated. The <b>Strict family</b> preset for kids' accounts allows up to 2 peppers.</p>
  </div>`;
  dlg.showModal();
}

// KIND says in a few words what kind of content a level is, with no names and nothing about the story, so a
// Deep Scan list can say why a part got its level without spoiling it. What actually happens is shown only
// when the parent opens the part in the book.
export const KIND = ["No romance", "Crushes, flirting or a sweet kiss", "Kissing and romantic affection",
  "Sexual desire or making out; any sex stays off the page", "Sex on the page, described", "Long, graphic sex scenes"];
export const kindOf = (level) => KIND[Math.max(0, Math.min(5, Number(level) || 0))];

// levelBadge is a part's level as a coloured square with the number in it, so a long list of parts (Deep Scan)
// can be read at a glance: grey 0, green 1, teal 2, amber 3, red 4, deep red 5.
export const levelBadge = (level) => `<span class="lvl lvl-${Math.max(0, Math.min(5, Number(level) || 0))}" aria-hidden="true">${Number(level) || 0}</span>`;

// levelTally counts the parts at each level, highest first: "Level 3 × 1 · Level 2 × 5 …" as badges.
export function levelTally(notes) {
  const by = new Map();
  for (const n of notes) by.set(n.level, (by.get(n.level) || 0) + 1);
  return `<p class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-400">${[...by].sort((a, b) => b[0] - a[0])
    .map(([level, k]) => `<span class="inline-flex items-center gap-1">${levelBadge(level)}<span>× ${k}<span class="sr-only"> at level ${level}</span></span></span>`).join("")}</p>`;
}
