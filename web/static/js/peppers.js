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
  { n: 2, name: "Romantic", cls: "chip-none",
    desc: "More developed romance with stronger attraction and kissing, including passionate kissing or physical affection. No sexual activity, sexual desire, or implication of sex. The intimacy remains romantic rather than sexual.",
    ex: "My Phony Valentine (Courtney Walsh)" },
  { n: 3, name: "Steamy Closed-Door", cls: "chip-closed",
    desc: "Strong sexual attraction and desire are present. May include heavy/passionate making out, sexual tension, and characters expressing or acting on sexual desire. Any sexual encounter occurs off-page or fades to black; no explicit sexual activity is described.",
    ex: "A romance that is clearly sexually charged but remains true closed-door" },
  { n: 4, name: "Explicit", cls: "chip-open",
    desc: "Sexual encounters occur on-page and include clear descriptions of sexual activity. Scenes contain meaningful sexual detail rather than simply implying what happens. There may be multiple or extended explicit scenes, but sex does not necessarily dominate the entire book.",
    ex: "Fourth Wing (Rebecca Yarros); A Court of Thorns and Roses (Sarah J. Maas)" },
  { n: 5, name: "Very Explicit / Erotica-Level", cls: "chip-open",
    desc: "Frequent, extended, or highly graphic on-page sexual content with extensive detail. Sexual encounters are a major component of the book and may occupy a substantial portion of the story.",
    ex: "Fifty Shades of Grey (E. L. James)" },
];

// "🌶️🌶️" for 2; 0 peppers shows a single faded pepper so the scale reads.
export const pepperIcons = (n) => (n > 0 ? "🌶️".repeat(n) : `<span class="opacity-40">🌶️</span>`);

export function pepperChip(n) {
  const p = PEPPERS[n];
  return `<span class="${p.cls}" title="${esc(p.desc)}">${pepperIcons(n)} ${n} · ${esc(p.name)}</span>`;
}

// Options for a pepper <select>; selected may be null.
export function pepperOptions(selected) {
  return PEPPERS.map((p) => `<option value="${p.n}" ${selected === p.n ? "selected" : ""}>${p.n} 🌶️ ${esc(p.name)}</option>`).join("");
}

export function pepperScaleHTML() {
  return `<ul class="space-y-3">${PEPPERS.map((p) => `
    <li><p class="font-semibold">${pepperIcons(p.n)} ${p.n} Pepper${p.n === 1 ? "" : "s"} · ${esc(p.name)}</p>
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
    <p class="text-xs text-slate-500">Books rated before the pepper scale show their older label (No Spice, Closed Door, Open Door) until they're re-rated.</p>
  </div>`;
  dlg.showModal();
}
