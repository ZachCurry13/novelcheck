// Profile: delivery preference, password change, and read-only content rules.
import { put } from "./api.js";
import { $, esc, attempt, ageLabel } from "./ui.js";
import { RULES } from "./users.js";
import { refreshUser } from "./app.js";
import { openGuide } from "./guide.js";
import { on, deliveryOptions } from "./modules.js";

export async function renderProfile(view, state) {
  const u = state.user;
  const active = RULES.filter(([k]) => u[k]).map(([, l]) => `<li>${esc(l)}</li>`).join("");
  view.innerHTML = `
    <h1 class="mb-4 text-2xl font-bold">Profile — ${esc(u.username)}</h1>
    <div class="grid gap-4 lg:grid-cols-2">
      <form id="delivery" class="card space-y-3${on(u, "queue") ? "" : " module-off"}">
        <h2 class="text-lg font-semibold">"Start Reading" delivery</h2>
        <select name="delivery_method" class="input">${deliveryOptions(u, u.delivery_method)}</select>
        <div><label class="label" for="kindle-email">Send-to-Kindle address</label>
          <input id="kindle-email" name="kindle_email" class="input" placeholder="name@kindle.com"></div>
        <p class="text-xs text-slate-500">Add the admin's sender address to your Amazon
          "Approved Personal Document E-mail List" first.</p>
        <button class="btn-primary">Save delivery settings</button>
      </form>
      <form id="password" class="card space-y-3">
        <h2 class="text-lg font-semibold">Change password</h2>
        <input name="current_password" type="password" required placeholder="Current password" class="input" autocomplete="current-password">
        <input name="new_password" type="password" required minlength="8" placeholder="New password (8+ chars)" class="input" autocomplete="new-password">
        <button class="btn-primary">Update password</button>
      </form>
      <div class="card lg:col-span-2">
        <h2 class="mb-2 text-lg font-semibold">Content rules on this account</h2>
        ${u.age_level ? `<p class="mb-2 text-sm text-slate-300">👪 Age group: <b>${esc(ageLabel(u.age_level))}</b>. Books a parent rated for older readers are hidden.</p>` : ""}
        ${active ? `<ul class="list-disc pl-5 text-sm text-slate-300">${active}</ul>`
          : `<p class="text-sm text-slate-400">No content is hidden for this account.</p>`}
        ${u.role === "restricted" ? `<p class="mt-2 text-xs text-slate-500">Only a parent (admin or editor) can change these rules.</p>`
          : u.role === "editor" ? `<p class="mt-2 text-xs text-slate-500">Only an admin can change these rules.</p>` : ""}
      </div>
      <div class="card lg:col-span-2 flex flex-wrap items-center gap-3">
        <p class="flex-1 text-sm text-slate-300">New here, or need a refresher?</p>
        <button type="button" id="replay-guide" class="btn-secondary">Show the How-to guide</button>
        <a href="#/whatsnew" class="btn-secondary">What's new</a>
      </div>
    </div>`;

  const dform = $("#delivery", view);
  dform.kindle_email.value = u.kindle_email;
  dform.addEventListener("submit", async (e) => {
    e.preventDefault();
    const ok = await attempt(() => put("/api/me/delivery", {
      delivery_method: dform.delivery_method.value,
      kindle_email: dform.kindle_email.value.trim(),
    }), "Delivery settings saved");
    if (ok) await refreshUser();
  });

  $("#replay-guide", view).addEventListener("click", () => openGuide(state));

  $("#password", view).addEventListener("submit", async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const ok = await attempt(() => put("/api/me/password", Object.fromEntries(fd)), "Password updated");
    if (ok) e.target.reset();
  });
}
