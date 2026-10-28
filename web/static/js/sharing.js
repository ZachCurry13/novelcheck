// Settings → Account → 👥 Sharing: who can see your reading profile. Private by
// default. A kid's is a parent's choice, made here by the parent.
import { get, put } from "./api.js";
import { esc, attempt, canManage } from "./ui.js";

const toggle = (label, attrs, on, note = "") => `<label class="toggle min-h-[2.5rem] text-sm"><input type="checkbox" ${attrs} ${on ? "checked" : ""}> ${label}${note ? ` <span class="text-xs text-slate-500">${note}</span>` : ""}</label>`;

export async function renderSharingCard(host, user) {
  const [me, kids] = await Promise.all([get("/api/me/sharing").catch(() => null), canManage(user) ? get("/api/admin/kids-sharing").catch(() => null) : null]);
  if (!me) return;
  const sh = me.sharing;
  host.innerHTML = `<h2 class="text-lg font-semibold">👥 Sharing</h2>
    <p class="text-sm text-slate-400">Your reading profile is <b>${sh.profile ? "shared with the family" : "private"}</b>. When it's shared, the family can see your reading time, books and charts on the <a href="#/readers" class="underline">Readers</a> page. Everyone only sees books they're allowed to see, never your highlights or notes, and not files that aren't in the library.</p>
    ${me.kid ? `<p class="text-sm text-slate-300">A parent decides whether your profile is shared.</p>` : `${toggle("Share my reading profile with the family", "data-me-profile", sh.profile)}
      ${toggle("Also show what I'm reading right now", "data-me-current", sh.current, "Only when my profile is shared.")}`}
    ${kids?.kids?.length ? `<div class="space-y-1 border-t border-slate-800 pt-2"><p class="label">Kids' profiles</p><p class="text-xs text-slate-500">You can always see your kids' reading. Sharing lets the other family members see it too.</p>
      ${kids.kids.map((k) => toggle(`Share ${esc(k.username)}'s reading profile`, `data-kid="${k.id}" data-kid-field="profile"`, k.sharing.profile)
        + toggle(`Also show what ${esc(k.username)} is reading now`, `data-kid="${k.id}" data-kid-field="current"`, k.sharing.current)).join("")}</div>` : ""}`;

  const save = async (url, body) => {
    const r = await attempt(() => put(url, body), "Sharing updated");
    if (r) renderSharingCard(host, user);
  };
  host.onchange = (e) => {
    const t = e.target;
    if (t.matches("[data-me-profile],[data-me-current]")) {
      const profile = host.querySelector("[data-me-profile]").checked;
      save("/api/me/sharing", { profile, current: profile && host.querySelector("[data-me-current]").checked });
    } else if (t.dataset.kid) {
      const id = t.dataset.kid;
      const profile = host.querySelector(`[data-kid="${id}"][data-kid-field=profile]`).checked;
      save(`/api/admin/users/${id}/sharing`, { profile, current: profile && host.querySelector(`[data-kid="${id}"][data-kid-field=current]`).checked });
    }
  };
}
