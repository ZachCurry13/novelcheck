// View as: an admin looking at the app as someone else (read-only). A bar at the top says who, and takes
// them back to their own account.
import { del } from "./api.js";
import { esc, attempt } from "./ui.js";

export function showViewAs(user) {
  document.getElementById("viewas-banner")?.remove();
  if (!user?.viewed_by) return;
  const bar = document.createElement("div");
  bar.id = "viewas-banner";
  bar.className = "border-b bg-indigo-900 text-slate-100"; // the theme's tinted accent, so it reads as "not your usual view"
  bar.innerHTML = `<div class="mx-auto flex max-w-7xl flex-wrap items-center gap-2 px-4 py-2 text-sm">
    <span class="min-w-0 grow basis-60">You're seeing the app as <b>${esc(user.username)}</b> does. Looking only: nothing can be changed here.</span>
    <button type="button" data-back class="btn-primary py-1 text-sm">Back to my account</button></div>`;
  document.getElementById("update-banner").after(bar);
  bar.querySelector("[data-back]").addEventListener("click", async () => {
    if (await attempt(() => del("/api/me/view-as"))) {
      location.hash = "#/admin?tab=users";
      location.reload();
    }
  });
}
