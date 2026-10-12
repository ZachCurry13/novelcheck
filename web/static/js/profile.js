// Profile: who you are. Your name and role, a few numbers from your reading,
// your reading taste and the content rules on this account. Everything you
// can set, your password included, lives under Settings.
import { $, esc, ageLabel } from "./ui.js";
import { RULES } from "./users.js";
import { openGuide } from "./guide.js";
import { on } from "./modules.js";
import { openTaste } from "./taste.js";
import { renderReadingSummary } from "./profilesummary.js";

const ROLES = { admin: "Admin", editor: "Editor", restricted: "Kid account", viewer: "Reader" };

export async function renderProfile(view, state) {
  const u = state.user;
  const active = RULES.filter(([k]) => u[k]).map(([, l]) => `<li>${esc(l)}</li>`).join("");
  const role = ROLES[u.role] || u.role;
  view.innerHTML = `
    <div class="mb-4 flex flex-wrap items-center gap-3"><span class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-indigo-700 text-2xl font-bold text-white" aria-hidden="true">${esc((u.username || "?").slice(0, 1).toUpperCase())}</span>
      <div class="min-w-0"><h1 class="break-words text-2xl font-bold">${esc(u.username)}</h1>
        <p class="text-sm text-slate-400">${esc(role)}${u.age_level ? ` · 👪 ${esc(ageLabel(u.age_level))}` : ""}</p></div>
      <a href="#/settings" class="btn-secondary ml-auto">⚙ Settings</a></div>
    <div class="grid gap-4 lg:grid-cols-2">
      <div id="reading-summary" class="card space-y-2${on(u, "koreader") ? "" : " module-off"} lg:col-span-2"></div>
      ${on(u, "queue") && on(u, "suggestions") && on(u, "taste") ? `<div class="card space-y-2">
        <h2 class="text-lg font-semibold">🎯 Your reading taste</h2>
        <p class="text-sm text-slate-400">Optional: mark books you know (want to read, read &amp; liked, …) so 💡 Suggested Reads on your Nightstand fit you better.</p>
        <button type="button" id="taste-open" class="btn-secondary">Rate some books</button></div>` : ""}
      <div class="card">
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

  $("#replay-guide", view).addEventListener("click", () => openGuide(state));
  $("#taste-open", view)?.addEventListener("click", () => openTaste());
  if (on(u, "koreader")) renderReadingSummary($("#reading-summary", view));
}
