// Book window extras for parents: the book's age group and parents' notes.
import { post, put, api } from "./api.js";
import { $, esc, attempt, AGE_GROUPS, ageLabel } from "./ui.js";

export function ageAndNotesHTML(b, notes, manager, viewer) {
  const ageSel = manager ? `
    <label class="block"><span class="label">Age group</span>
      <select data-age class="input">
        <option value="0">Not set</option>
        ${AGE_GROUPS.map(([l, n, r]) => `<option value="${l}" ${b.age_level === l ? "selected" : ""}>${esc(n)} (${esc(r)})</option>`).join("")}
      </select>
      <span class="text-xs text-slate-500">Kids see books rated for their age group or younger (even before the AI rates them); older ones stay hidden.${b.age_set_by ? ` Set by ${esc(b.age_set_by)}.` : ""}</span></label>`
    : b.age_level ? `<p class="text-sm">👪 Age group: <b>${esc(ageLabel(b.age_level))}</b></p>` : "";
  return `
    ${ageSel}
    <section data-notes-box>
      <span class="label">${manager ? "Parents' notes" : "Notes from your parents"}</span>
      <ul data-notes class="space-y-2">${notesList(notes, viewer)}</ul>
      ${manager ? `
      <form data-note-form class="mt-2 space-y-2">
        <textarea name="body" rows="2" maxlength="4000" class="input" placeholder="Read it? Leave a note for the family…"></textarea>
        <div class="flex flex-wrap items-center gap-2">
          <select name="visibility" class="input w-auto py-1 text-sm">
            <option value="everyone">Everyone can see (kids too)</option>
            <option value="parents">Parents only</option>
          </select>
          <button class="btn-secondary py-1">Add note</button>
        </div>
      </form>` : ""}
    </section>`;
}

function notesList(notes, viewer) {
  if (!notes.length) return `<li class="text-sm text-slate-500">No notes yet.</li>`;
  return notes.map((n) => {
    const mine = n.user_id === viewer.id || viewer.role === "admin";
    const d = new Date((n.created_at.includes("T") ? n.created_at : n.created_at.replace(" ", "T") + "Z"));
    return `<li class="rounded-lg bg-slate-800/60 p-3 text-sm" data-note="${n.id}">
      <p class="whitespace-pre-line text-slate-200">${esc(n.body)}</p>
      <p class="mt-1 text-xs text-slate-500">— ${esc(n.author || "former user")} · ${esc(isNaN(d) ? "" : d.toLocaleDateString())}
        ${n.visibility === "parents" ? ` · <span class="text-amber-300">🔒 parents only</span>` : ""}
        ${mine ? ` · <button data-note-edit class="underline">Edit</button> · <button data-note-del class="underline">Delete</button>` : ""}</p>
    </li>`;
  }).join("");
}

// bindAgeAndNotes wires the controls; onChange refreshes the Library behind.
export function bindAgeAndNotes(root, b, notes, viewer, onChange) {
  const render = (list) => {
    notes = list;
    $("[data-notes]", root).innerHTML = notesList(notes, viewer);
  };
  $("[data-age]", root)?.addEventListener("change", async (e) => {
    const ok = await attempt(() => put(`/api/books/${b.id}/age`, { age_level: Number(e.target.value) }),
      Number(e.target.value) ? `Age group set: ${ageLabel(Number(e.target.value))}` : "Age group cleared");
    if (ok) onChange?.();
  });
  $("[data-note-form]", root)?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = e.target;
    const r = await attempt(() => post(`/api/books/${b.id}/notes`, { body: f.body.value, visibility: f.visibility.value }), "Note added");
    if (r) {
      f.body.value = "";
      render(r.notes);
    }
  });
  $("[data-notes-box]", root)?.addEventListener("click", async (e) => {
    const li = e.target.closest("[data-note]");
    if (!li) return;
    const note = notes.find((n) => String(n.id) === li.dataset.note);
    if (e.target.closest("[data-note-del]")) {
      if (!confirm("Delete this note?")) return;
      const r = await attempt(() => api(`/api/notes/${note.id}`, { method: "DELETE" }), "Note deleted");
      if (r) render(r.notes);
    } else if (e.target.closest("[data-note-edit]")) {
      const body = prompt("Edit note:", note.body);
      if (body === null) return;
      const shareAll = confirm("Let everyone (kids too) see this note?\nOK = everyone, Cancel = parents only");
      const r = await attempt(() => put(`/api/notes/${note.id}`, { body, visibility: shareAll ? "everyone" : "parents" }), "Note updated");
      if (r) render(r.notes);
    }
  });
}
