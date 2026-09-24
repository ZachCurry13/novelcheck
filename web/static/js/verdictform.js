// "Edit rating" form in the book dialog (admins and editors). Saves a manual
// verdict that is labelled with who made the change.
import { put } from "./api.js";
import { $, $$, esc, attempt } from "./ui.js";
import { pepperOptions } from "./peppers.js";

const FLAGS = [
  ["nudity", "Nudity"],
  ["solo_acts", "Solo Acts"],
  ["heavy_innuendo", "Heavy Innuendo"],
  ["lgbtq_content", "LGBTQ+ Content"],
  ["playful_fantasy", "Whimsical / standard fantasy magic"],
  ["dark_occult", "Dark Occult"],
  ["demonic_presence", "Demonic presence"],
];

export function verdictFormHTML(b) {
  // Older ratings preselect the closest pepper level.
  const guess = b.spice_level ?? { "No Spice": 0, "Closed Door": 3, "Open Door": 4 }[b.classification] ?? null;
  return `
    <form id="verdict-form" class="hidden space-y-3 rounded-lg bg-slate-800/60 p-4">
      <h3 class="font-semibold">Edit rating</h3>
      <label class="block"><span class="label">Peppers</span>
        <select name="spice_level" class="input" required>
          ${guess === null ? `<option value="" selected disabled>Choose 0–5 peppers</option>` : ""}${pepperOptions(guess)}
        </select></label>
      <input name="spice_reason" maxlength="80" class="input" value="${esc(b.spice_reason || "")}"
        placeholder="Why this many peppers? e.g. Kissing only · Heavy innuendo, on-page foreplay">
      <div class="grid gap-1 sm:grid-cols-2">
        ${FLAGS.map(([k, l]) => `<label class="toggle"><input type="checkbox" data-flag="${k}" ${b[k] ? "checked" : ""}> ${esc(l)}</label>`).join("")}
      </div>
      <textarea name="summary_verdict" rows="2" maxlength="1000" class="input" placeholder="1–2 sentence summary">${esc(b.summary_verdict)}</textarea>
      <div class="flex gap-2">
        <button class="btn-primary">Save rating</button>
        <button type="button" data-act="cancel-verdict" class="btn-ghost">Cancel</button>
      </div>
    </form>`;
}

export function bindVerdictForm(root, bookId, onSaved) {
  const form = $("#verdict-form", root);
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const body = {
      spice_level: Number(form.spice_level.value),
      spice_reason: form.spice_reason.value.trim(),
      summary_verdict: form.summary_verdict.value.trim(),
    };
    $$("[data-flag]", form).forEach((cb) => (body[cb.dataset.flag] = cb.checked));
    const ok = await attempt(() => put(`/api/books/${bookId}/verdict`, body), "Rating saved");
    if (ok) onSaved?.();
  });
  return form;
}
