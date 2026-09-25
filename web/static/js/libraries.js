// 📚 Your libraries (on the Import page): rename a library, make it private
// (only you and the admins see its books) or shared, or delete it. Admins
// can also change who owns one.
import { get, patch, del } from "./api.js";
import { esc, attempt } from "./ui.js";

// canEdit mirrors the server: owners and admins; editors for the family's libraries.
export const canEditLibrary = (c, user) => c.source !== "calibre" && c.name !== "Looked up" &&
  (user.role === "admin" || c.owner_id === user.id || (c.owner_id == null && user.role === "editor"));

export async function renderLibraries(host, state) {
  const user = state.user;
  const admin = user.role === "admin";
  const [cats, users] = await Promise.all([attempt(() => get("/api/catalogs")),
    admin ? get("/api/admin/users").catch(() => []) : Promise.resolve([])]);
  if (!cats || !host.isConnected) return;
  const mine = cats.filter((c) => canEditLibrary(c, user));
  const adults = (users || []).filter((u) => u.role !== "restricted");
  host.innerHTML = `<div class="card space-y-3">
    <h2 class="text-lg font-semibold">📚 Your libraries</h2>
    <p class="text-sm text-slate-400">Libraries you imported are yours. <b>Private</b> ones are seen only by you and the admins; <b>shared</b> ones by everyone (kids still only see what their rules allow).</p>
    <ul class="space-y-2">${mine.map((c) => `<li data-cat="${c.id}" class="flex flex-wrap items-center gap-2 rounded-lg bg-slate-800/60 p-3">
      <span class="min-w-0 flex-1"><b>${esc(c.name)}</b> <span class="text-xs text-slate-400">${c.book_count.toLocaleString()} books ·
        ${c.owner ? `owned by ${esc(c.owner)}` : "the family's"}</span></span>
      ${c.owner_id != null ? `<label class="toggle text-sm"><input type="checkbox" data-private ${c.private ? "checked" : ""}> 🔒 Private</label>` : ""}
      ${admin ? `<select data-owner class="input w-auto py-1 text-xs" aria-label="Owner of ${esc(c.name)}">
        <option value="0">Family (no owner)</option>${adults.map((u) => `<option value="${u.id}" ${u.id === c.owner_id ? "selected" : ""}>${esc(u.username)}</option>`).join("")}</select>` : ""}
      <button data-rename class="btn-ghost py-1 text-sm">Rename</button>
      ${admin || c.owner_id === user.id ? `<button data-delete class="btn-ghost py-1 text-sm text-rose-300">Delete</button>` : ""}
    </li>`).join("") || `<li class="text-sm text-slate-500">No libraries yet. Import a Kindle or a list above and it'll show here.</li>`}</ul></div>`;
  const reload = () => renderLibraries(host, state);
  const catOf = (e) => mine.find((c) => c.id === Number(e.target.closest("[data-cat]")?.dataset.cat));
  host.onchange = async (e) => {
    const c = catOf(e);
    if (!c) return;
    if (e.target.matches("[data-private]")) {
      const priv = e.target.checked;
      if (!(await attempt(() => patch(`/api/catalogs/${c.id}`, { private: priv }), priv ? `${c.name} is now private` : `${c.name} is now shared`))) e.target.checked = !priv;
    } else if (e.target.matches("[data-owner]")) {
      await attempt(() => patch(`/api/catalogs/${c.id}`, { owner_id: Number(e.target.value) }), "Owner changed");
      reload();
    }
  };
  host.onclick = async (e) => {
    const c = catOf(e);
    if (!c) return;
    if (e.target.closest("[data-rename]")) {
      const name = prompt("New name for this library:", c.name);
      if (name && name.trim() !== c.name && (await attempt(() => patch(`/api/catalogs/${c.id}`, { name }), "Renamed"))) reload();
    } else if (e.target.closest("[data-delete]")) {
      if (!confirm(`Delete the library "${c.name}" from NovelCheck?\n\nIts ${c.book_count} books are taken off NovelCheck's list (books that are also in another library stay). Nothing is deleted from your devices or Calibre.`)) return;
      if (await attempt(() => del(`/api/catalogs/${c.id}`), "Library deleted")) reload();
    }
  };
}
