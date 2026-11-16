// The Library's side panel pieces (the bookshop layout the owner's family picked): pepper levels as the coloured
// squares (any mix, with shortcuts), and short pick lists with counts in place of drop-downs. They keep the
// form's own field names (spice, genre, catalog, age), so links, Back and the chips under the search box work
// as before.
import { esc } from "./ui.js";
import { PEPPERS } from "./peppers.js";

const PRESETS = [["Clean", "0,1"], ["Up to romantic", "0,1,2"], ["Steamy", "3,4"], ["Spicy", "4,5"]];

const levels = (v) => String(v || "").split(",").filter((x) => /^[0-5]$/.test(x)).map(Number);

// peppersHTML is the Peppers section: six squares to tick on and off, a hidden field holding "0,1,2".
export function peppersHTML() {
  return `<section class="facet"><h3>Peppers <span data-pep-sum class="facet-sum"></span></h3>
    <input type="hidden" name="spice" value="">
    <div class="pep-squares" role="group" aria-label="Pepper levels">${PEPPERS.map((p) =>
      `<button type="button" data-pep="${p.n}" class="pep-sq" aria-pressed="false" title="Level ${p.n}: ${esc(p.name)}"><span class="lvl lvl-${p.n}">${p.n}</span></button>`).join("")}</div>
    <div class="pep-presets">${PRESETS.map(([l, v]) => `<button type="button" data-pep-preset="${v}" class="chip-filter" aria-pressed="false">${esc(l)}</button>`).join("")}</div>
    <button type="button" id="pepper-help" class="facet-link">What do the peppers mean?</button></section>`;
}

// pickHTML is a short list to pick one from (radio buttons), "Any" first, the first `few` shown and the
// rest behind "Show all".
export function pickHTML(title, name, anyLabel, items, few = 6) {
  const row = (v, label, n, more) => `<label class="facet-row${more ? " facet-more hidden" : ""}"><input type="radio" name="${name}" value="${esc(v)}"${v === "" ? " checked" : ""}>
    <span class="min-w-0 flex-1 truncate">${esc(label)}</span>${n === undefined ? "" : `<span class="facet-n">${Number(n).toLocaleString()}</span>`}</label>`;
  return `<section class="facet"><h3>${esc(title)}</h3>
    ${row("", anyLabel)}
    ${items.map((it, i) => row(it.value, it.label, it.count, i >= few)).join("")}
    ${items.length > few ? `<button type="button" data-facet-all class="facet-link">Show all ${items.length}</button>` : ""}</section>`;
}

// bindFacets keeps the squares in step with the hidden field (both ways) and runs the panel's buttons.
export function bindFacets(form) {
  const field = form.elements.spice;
  const paint = () => {
    const on = levels(field.value);
    form.querySelectorAll("[data-pep]").forEach((b) => b.setAttribute("aria-pressed", String(on.includes(Number(b.dataset.pep)))));
    form.querySelectorAll("[data-pep-preset]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.pepPreset === on.join(","))));
    const sum = form.querySelector("[data-pep-sum]");
    if (sum) sum.textContent = on.length ? on.join(", ") : "any";
  };
  const set = (v) => {
    field.value = v;
    paint();
    field.dispatchEvent(new Event("input", { bubbles: true }));
  };
  form.addEventListener("click", (e) => {
    const sq = e.target.closest("[data-pep]");
    if (sq) {
      const n = Number(sq.dataset.pep), on = levels(field.value);
      return set((on.includes(n) ? on.filter((x) => x !== n) : [...on, n]).sort().join(","));
    }
    const pr = e.target.closest("[data-pep-preset]");
    if (pr) return set(pr.dataset.pepPreset === levels(field.value).join(",") ? "" : pr.dataset.pepPreset);
    const all = e.target.closest("[data-facet-all]");
    if (all) {
      all.closest(".facet").querySelectorAll(".facet-more").forEach((r) => r.classList.remove("hidden"));
      return all.remove();
    }
    if (e.target.closest("[data-clear-all]")) { // every filter off; the search words and the order stay
      for (const el of form.elements) {
        if (!el.name || ["q", "sort"].includes(el.name) || el.type === "button") continue;
        if (el.type === "checkbox") el.checked = false;
        else if (el.type === "radio") el.checked = el.value === "";
        else el.value = "";
      }
      paint();
      form.dispatchEvent(new Event("input", { bubbles: true }));
    }
  });
  form.addEventListener("input", paint); // a chip's × or Back put a different value in the field
  paint();
  return paint;
}

// pepperLabel says "Peppers 0–2" or "Peppers 0, 3, 5" for a chip.
export function pepperLabel(v) {
  const on = levels(v);
  const run = on.every((x, i) => i === 0 || x === on[i - 1] + 1);
  return `Peppers ${on.length === 1 ? on[0] : run && on.length > 2 ? `${on[0]}–${on[on.length - 1]}` : on.join(", ")}`;
}
