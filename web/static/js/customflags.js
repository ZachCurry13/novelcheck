// The family's own AI filters (Admin → Custom AI filters): chips on books,
// Hide checkboxes in the Library, ticks in "Edit rating", and the admin editor.
import { get, post, put, del } from "./api.js";
import { $, esc, attempt } from "./ui.js";

let flags = [];

// loadFlags fetches the filters once; force=true fetches them again.
export async function loadFlags(force = false) {
  if (force || !loadFlags.done) {
    flags = (await attempt(() => get("/api/flags"))) || [];
    loadFlags.done = true;
  }
  return flags;
}

const label = (key) => flags.find((f) => f.key === key)?.label;

// Chips for the filters a book matches (skips any since deleted).
export function customChips(b) {
  return (b.custom_flags || "").split(",").filter(label)
    .map((k) => `<span class="chip-flag" title="Your filter">${esc(label(k))}</span>`).join(" ");
}

// Hide checkboxes for the Library (value "flag:<key>").
export const hideBoxes = () => flags.map((f) =>
  `<label class="toggle" title="${esc(f.description)}"><input type="checkbox" name="hide" value="flag:${esc(f.key)}"> ${esc(f.label)}</label>`).join("");

// Ticks for "Edit rating".
export function verdictBoxes(b) {
  const on = new Set((b.custom_flags || "").split(","));
  return flags.map((f) => `<label class="toggle"><input type="checkbox" data-cflag="${esc(f.key)}" ${on.has(f.key) ? "checked" : ""}> ${esc(f.label)}</label>`).join("");
}

// Admin editor: list, add, change and delete filters.
export async function renderFlagsAdmin(host, onChange) {
  await loadFlags(true);
  host.innerHTML = `
    <section class="card mb-6 space-y-3">
      <h2 class="text-lg font-semibold">Custom AI filters</h2>
      <p class="text-sm text-slate-400">Add your own topics, like <i>Heavy swearing</i>, <i>Gore / violence</i> or <i>Substance abuse</i>.
        The AI checks every book for them, and each gets a <b>Hide</b> box in the Library. Books rated before a new filter
        can be re-rated from the banner above.</p>
      <ul class="space-y-2">${flags.map((f) => `
        <li data-flag="${f.id}" class="grid gap-2 rounded-lg bg-slate-800/60 p-3 sm:grid-cols-5">
          <input data-f="label" value="${esc(f.label)}" maxlength="40" class="input sm:col-span-1" aria-label="Name">
          <input data-f="description" value="${esc(f.description)}" maxlength="300" class="input sm:col-span-3" aria-label="What the AI looks for" placeholder="What should the AI look for?">
          <span class="flex gap-2"><button data-fact="save" class="btn-secondary">Save</button><button data-fact="delete" class="btn-ghost" aria-label="Delete">🗑</button></span>
        </li>`).join("") || `<li class="text-sm text-slate-500">No custom filters yet.</li>`}</ul>
      <form data-fact="add" class="grid gap-2 sm:grid-cols-5">
        <input name="label" required maxlength="40" placeholder="Name, e.g. Heavy swearing" class="input sm:col-span-1">
        <input name="description" maxlength="300" placeholder="What should the AI look for? e.g. Frequent strong profanity" class="input sm:col-span-3">
        <button class="btn-primary">Add filter</button>
      </form>
    </section>`;
  const done = async () => {
    await renderFlagsAdmin(host, onChange);
    onChange?.();
  };
  host.onsubmit = async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    if (await attempt(() => post("/api/admin/flags", { label: fd.get("label"), description: fd.get("description") }), "Filter added. Re-rate books from the banner to check them for it.")) done();
  };
  host.onclick = async (e) => {
    const act = e.target.closest("[data-fact]")?.dataset.fact;
    const row = e.target.closest("[data-flag]");
    if (!row || (act !== "save" && act !== "delete")) return;
    const id = row.dataset.flag;
    if (act === "delete") {
      if (!confirm("Delete this filter? Its marks on books go too.")) return;
      if (await attempt(() => del(`/api/admin/flags/${id}`), "Filter deleted")) done();
    } else {
      const body = { label: $("[data-f=label]", row).value, description: $("[data-f=description]", row).value };
      if (await attempt(() => put(`/api/admin/flags/${id}`, body), "Filter saved")) done();
    }
  };
}
