// User management: accounts, roles and parental content rules.
// Admins manage everyone; editors see and manage only kid accounts.
import { PEPPERS } from "./peppers.js";
import { get, post, put, del } from "./api.js";
import { $, $$, esc, attempt, AGE_GROUPS } from "./ui.js";

export const RULES = [
  ["hide_open_door", "Hide Open Door"],
  ["hide_nudity", "Hide Nudity"],
  ["hide_solo_acts", "Hide Solo Acts"],
  ["hide_innuendo", "Hide Heavy Innuendo"],
  ["hide_dark_occult", "Hide Dark Occult / Demonic"],
  ["hide_lgbtq", "Hide LGBTQ+ Content"],
  ["hide_unrated", "Hide books not yet analyzed"],
];

// Account types: kid accounts by age group (they only see books rated for
// their age or younger), plus Editor and Admin. Values are "role:age".
function typeOptions(role, age, isAdmin) {
  const cur = `${role}:${role === "restricted" ? age || 0 : 0}`;
  const opts = AGE_GROUPS.filter(([l]) => l < 5).map(([l, n, r]) => [`restricted:${l}`, `Kid · ${n} (${r})`]);
  opts.push(["restricted:0", "Kid · no age group (content rules only)"]);
  if (isAdmin) opts.push(["editor:0", "Editor (parent, no technical settings)"], ["admin:0", "Admin"]);
  return opts.map(([v, l]) => `<option value="${v}" ${v === cur ? "selected" : ""}>${esc(l)}</option>`).join("");
}

const parseType = (v) => {
  const [role, age] = String(v || "restricted:0").split(":");
  return { role, age_level: Number(age) || 0 };
};

export async function renderUsers(host, viewer) {
  const isAdmin = viewer?.role === "admin";
  const users = (await attempt(() => get("/api/admin/users"))) || [];
  // Fresh container on each render so click listeners never stack up.
  const root = document.createElement("div");
  host.replaceChildren(root);
  root.innerHTML = `
    <h2 class="mb-1 text-xl font-bold">${isAdmin ? "Users & Content Rules" : "Kids' Accounts & Content Rules"}</h2>
    <p class="mb-3 text-sm text-slate-400">${isAdmin
      ? "Admin: full control. Editor: manages books, scans and kids' accounts, but no technical settings. Restricted: kid account filtered by its rules."
      : "Add kid accounts, reset their passwords, and choose what each one can see."}</p>
    <form id="new-user" class="card mb-4 grid gap-3 md:grid-cols-4">
      <input name="username" required placeholder="Username" class="input">
      <input name="password" type="password" required minlength="8" placeholder="Password (8+ chars)" class="input" autocomplete="new-password">
      <select name="type" class="input" title="Kids start with content rules suited to their age group; you can change them after.">${typeOptions("restricted", 2, isAdmin)}</select>
      <button class="btn-primary">Add user</button>
    </form>
    <div class="grid gap-3 lg:grid-cols-2">${users.map((u) => userCard(u, isAdmin)).join("")}</div>`;

  $("#new-user", root).addEventListener("submit", async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const body = { username: fd.get("username"), password: fd.get("password"), ...parseType(fd.get("type")) };
    const ok = await attempt(() => post("/api/admin/users", body), "User created");
    if (ok) renderUsers(host, viewer);
  });

  root.addEventListener("click", async (e) => {
    const act = e.target.closest("[data-uact]")?.dataset.uact;
    if (!act) return;
    const cardEl = e.target.closest("[data-user]");
    const id = cardEl.dataset.user;
    if (act === "save") {
      const body = { ...parseType($("[name=type]", cardEl).value),
        delivery_method: $("[name=delivery_method]", cardEl).value,
        kindle_email: $("[name=kindle_email]", cardEl).value };
      $$("[data-rule]", cardEl).forEach((cb) => (body[cb.dataset.rule] = cb.checked));
      const ms = $("[name=max_spice]", cardEl);
      if (ms) body.max_spice = Number(ms.value);
      await attempt(() => put(`/api/admin/users/${id}`, body), "User saved");
    } else if (act === "password") {
      const pw = prompt("New password (8+ characters):");
      if (pw) await attempt(() => put(`/api/admin/users/${id}/password`, { password: pw }), "Password reset");
    } else if (act === "delete") {
      if (!confirm("Delete this user and their queue?")) return;
      const ok = await attempt(() => del(`/api/admin/users/${id}`), "User deleted");
      if (ok) renderUsers(host, viewer);
    }
  });
}

function userCard(u, isAdmin) {
  return `
    <div data-user="${u.id}" class="card space-y-3">
      <div class="flex items-center justify-between">
        <p class="font-semibold">${esc(u.username)}</p>
        <select name="type" class="input w-auto max-w-[60%] py-1 text-sm">${typeOptions(u.role, u.age_level, isAdmin)}</select>
      </div>
      ${u.role === "restricted" ? `<label class="block"><span class="label">Most peppers allowed</span>
        <select name="max_spice" class="input">
          <option value="-1" ${u.max_spice < 0 ? "selected" : ""}>No limit</option>
          ${PEPPERS.map((p) => `<option value="${p.n}" ${u.max_spice === p.n ? "selected" : ""}>Up to ${p.n} 🌶️ ${esc(p.name)}</option>`).join("")}
        </select></label>` : ""}
      <div class="grid grid-cols-1 gap-1 sm:grid-cols-2">
        ${RULES.map(([k, l]) => `<label class="toggle"><input type="checkbox" data-rule="${k}" ${u[k] ? "checked" : ""}> ${esc(l)}</label>`).join("")}
      </div>
      <div class="grid gap-2 sm:grid-cols-2">
        <select name="delivery_method" class="input">
          ${["none", "email", "koreader"].map((m) => `<option value="${m}" ${u.delivery_method === m ? "selected" : ""}>${
            { none: "No delivery", email: "Send-to-Kindle email", koreader: "KOReader sync" }[m]}</option>`).join("")}
        </select>
        <input name="kindle_email" value="${esc(u.kindle_email)}" placeholder="name@kindle.com" class="input">
      </div>
      <div class="flex flex-wrap gap-2">
        <button data-uact="save" class="btn-primary">Save</button>
        <button data-uact="password" class="btn-secondary">Reset password</button>
        <button data-uact="delete" class="btn-danger">Delete</button>
      </div>
    </div>`;
}
