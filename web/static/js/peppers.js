// The 0-5 pepper scale, in the family's own words. The server's AI prompt
// (internal/llm/prompt.go) uses the same descriptions.
import { esc } from "./ui.js";

export const PEPPERS = [
  { n: 0, name: "No Romance", cls: "chip-none",
    desc: "No meaningful romantic or sexual content. No romantic subplot, kissing, sexual attraction, or romantic physical affection.",
    ex: "Harry Potter and the Sorcerer's Stone, The Hobbit" },
  { n: 1, name: "Sweet Romance", cls: "chip-none",
    desc: "Romance is present but mild and non-sexual. May include crushes, attraction, flirting, hand-holding, cuddling, and sweet/brief kisses. No sexual desire or sexualized physical intimacy.",
    ex: "Uglies (Scott Westerfeld); Seeking Persephone (Sarah M. Eden)" },
  { n: 2, name: "Mild / Closed Door", cls: "chip-none", tag: "Jenna's limit",
    desc: "Romantic tension and kissing occur, including passionate kissing. Any physical intimacy beyond kissing cuts to black or happens strictly off-page; nothing sexual is shown or described.",
    ex: "My Phony Valentine (Courtney Walsh)" },
  { n: 3, name: "Steamy / Heavy Tension", cls: "chip-closed", tag: "Gray area",
    desc: "Heavy physical foreplay or suggestive on-page innuendo, such as heavy making out with clear sexual intent, but it stops short of explicit sexual acts. Cards show why a book got this rating.",
    ex: "A romance that is clearly sexually charged on the page but never explicit" },
  { n: 4, name: "Explicit / Open Door", cls: "chip-open",
    desc: "Sexual encounters occur on-page and include clear descriptions of sexual activity. Scenes contain meaningful sexual detail rather than simply implying what happens. There may be multiple or extended explicit scenes, but sex does not necessarily dominate the entire book.",
    ex: "Fourth Wing (Rebecca Yarros); A Court of Thorns and Roses (Sarah J. Maas)" },
  { n: 5, name: "Very Explicit / Erotica", cls: "chip-open",
    desc: "Frequent, extended, or highly graphic on-page sexual content with extensive detail. Sexual encounters are a major component of the book and may occupy a substantial portion of the story.",
    ex: "Fifty Shades of Grey (E. L. James)" },
];

// "🌶️🌶️" for 2; 0 peppers shows a single faded pepper so the scale reads.
export const pepperIcons = (n) => (n > 0 ? "🌶️".repeat(n) : `<span class="opacity-40">🌶️</span>`);

export function pepperChip(n) {
  const p = PEPPERS[n];
  return `<span class="${p.cls}" title="${esc(p.desc)}">${pepperIcons(n)} ${n} · ${esc(p.name)}</span>`;
}

// Level 3 is the gray area: say why at a glance ("Heavy innuendo, on-page
// foreplay"). Older ratings without a reason fall back to their flags.
export function grayAreaChip(b) {
  if (b.spice_level !== 3) return "";
  const why = b.spice_reason || (b.heavy_innuendo ? "Heavy innuendo" : b.nudity ? "Nudity" : "Heavy tension");
  return `<span class="chip-closed" title="Level 3 is the gray area: steamy, but not explicit">⚠ Gray area · ${esc(why)}</span>`;
}

// Options for a pepper <select>; selected may be null.
export function pepperOptions(selected) {
  return PEPPERS.map((p) => `<option value="${p.n}" ${selected === p.n ? "selected" : ""}>${p.n} 🌶️ ${esc(p.name)}</option>`).join("");
}

export function pepperScaleHTML() {
  return `<ul class="space-y-3">${PEPPERS.map((p) => `
    <li><p class="font-semibold">${pepperIcons(p.n)} ${p.n} Pepper${p.n === 1 ? "" : "s"} · ${esc(p.name)}${p.tag ? ` <span class="${p.cls}">${esc(p.tag)}</span>` : ""}</p>
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
    <div class="flex items-start justify-between gap-4"><h2 class="text-xl font-bold">What do the peppers mean?</h2>
      <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    ${pepperScaleHTML()}
    <p class="text-xs text-slate-500">Books rated before the pepper scale show their older label (No Spice, Closed Door, Open Door) until they're re-rated. The <b>Strict family</b> preset for kids' accounts allows up to 2 peppers.</p>
  </div>`;
  dlg.showModal();
}
