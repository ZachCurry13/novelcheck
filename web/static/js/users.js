// User management: accounts, roles and parental content rules.
// Admins manage everyone; editors see and manage only kid accounts.
import { PEPPERS } from "./peppers.js";
import { get, post, put, del } from "./api.js";
import { $, $$, esc, attempt, AGE_GROUPS } from "./ui.js";
import { on, deliveryOptions } from "./modules.js";
import { openKOReaderSetup } from "./delivery.js";

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
// kids=false (parent tools turned off) leaves out kid types, except on an
// existing kid's card, whose rules keep applying.
function typeOptions(role, age, isAdmin, kids = true) {
  const cur = `${role}:${role === "restricted" ? age || 0 : 0}`;
  const opts = [];
  if (kids || role === "restricted") {
    opts.push(...AGE_GROUPS.filter(([l]) => l < 5).map(([l, n, r]) => [`restricted:${l}`, `Kid · ${n} (${r})`]));
    opts.push(["restricted:0", "Kid · no age group (content rules only)"]);
  }
  if (isAdmin) opts.push(["editor:0", "Editor (parent, no technical settings)"], ["admin:0", "Admin"]);
  return opts.map(([v, l]) => `<option value="${v}" ${v === cur ? "selected" : ""}>${esc(l)}</option>`).join("");
}

// One-click household presets for kids' accounts. They set the pepper cap
// (and, for Young Reader, the age group) and switch the listed rules on;
// other rules, like LGBTQ+, stay as the parent set them.
const STRICT_RULES = ["hide_open_door", "hide_nudity", "hide_solo_acts", "hide_innuendo", "hide_unrated"];
const PRESETS = {
  strict: { label: "👪 Strict Family (Max Level 2)", max_spice: 2, rules: STRICT_RULES,
    title: "Up to Level 2; hides Open Door, nudity, solo acts, heavy innuendo and books not yet rated. Keeps the age group.",
    done: "Strict Family preset saved: up to Level 2, nothing explicit or unrated" },
  young: { label: "🧒 Young Reader (Level 1, ages 9–12)", type: "restricted:2", max_spice: 1, rules: [...STRICT_RULES, "hide_dark_occult"],
    title: "Middle grade (9–12), up to Level 1; also hides dark occult. Books a parent rated for older readers stay hidden.",
    done: "Young Reader preset saved: ages 9–12, up to Level 1" },
};

const parseType = (v) => {
  const [role, age] = String(v || "restricted:0").split(":");
  return { role, age_level: Number(age) || 0 };
};

export async function renderUsers(host, viewer) {
  const isAdmin = viewer?.role === "admin";
  const kids = on(viewer, "parents");
  const users = (await attempt(() => get("/api/admin/users"))) || [];
  // Fresh container on each render so click listeners never stack up.
  const root = document.createElement("div");
  host.replaceChildren(root);
  root.innerHTML = `
    <h2 class="mb-1 text-xl font-bold">${isAdmin ? "Users & Content Rules" : "Kids' Accounts & Content Rules"}</h2>
    <p class="mb-3 text-sm text-slate-400">${isAdmin
      ? "Admin: full control. Editor: manages books, scans and kids' accounts, but no technical settings. Restricted: kid account filtered by its rules."
      : "Add kid accounts, reset their passwords, and choose what each one can see."}</p>
    <form id="new-user" class="card mb-4 grid gap-3 md:grid-cols-4${kids || isAdmin ? "" : " module-off"}">
      <input name="username" required placeholder="Username" class="input">
      <input name="password" type="password" required minlength="8" placeholder="Password (8+ chars)" class="input" autocomplete="new-password">
      <select name="type" class="input" title="Kids start with content rules suited to their age group; you can change them after.">${
        kids ? typeOptions("restricted", 2, isAdmin) : typeOptions("editor", 0, isAdmin, false)}</select>
      <button class="btn-primary">Add user</button>
    </form>
    ${kids ? "" : `<p class="mb-3 text-xs text-slate-500">Parent tools are turned off (Admin → Features), so new kid accounts can't be added. Existing kids keep their rules.</p>`}
    <div class="grid gap-3 lg:grid-cols-2">${users.map((u) => userCard(u, isAdmin, viewer)).join("")}</div>`;

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
    const preset = PRESETS[act];
    if (preset) {
      if (preset.type) $("[name=type]", cardEl).value = preset.type;
      $("[name=max_spice]", cardEl).value = String(preset.max_spice);
      $$("[data-rule]", cardEl).forEach((cb) => (cb.checked = preset.rules.includes(cb.dataset.rule) || cb.checked));
    }
    if (act === "save" || preset) {
      const body = { ...parseType($("[name=type]", cardEl).value),
        delivery_method: $("[name=delivery_method]", cardEl).value,
        kindle_email: $("[name=kindle_email]", cardEl).value };
      $$("[data-rule]", cardEl).forEach((cb) => (body[cb.dataset.rule] = cb.checked));
      const ms = $("[name=max_spice]", cardEl);
      if (ms) body.max_spice = Number(ms.value);
      await attempt(() => put(`/api/admin/users/${id}`, body), preset ? preset.done : "User saved");
    } else if (act === "koreader") {
      openKOReaderSetup(id);
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

function userCard(u, isAdmin, viewer) {
  return `
    <div data-user="${u.id}" class="card space-y-3">
      <div class="flex items-center justify-between">
        <div class="min-w-0"><p class="font-semibold">${esc(u.username)}</p><p class="mt-1 flex flex-wrap gap-1">${badges(u)}</p></div>
        <select name="type" class="input w-auto max-w-[60%] py-1 text-sm">${typeOptions(u.role, u.age_level, isAdmin, on(viewer, "parents"))}</select>
      </div>
      ${u.role === "restricted" ? `<label class="block"><span class="label">Most peppers allowed</span>
        <select name="max_spice" class="input">
          <option value="-1" ${u.max_spice < 0 ? "selected" : ""}>No limit</option>
          ${PEPPERS.filter((p) => p.n <= 3 || u.max_spice === p.n).map((p) => `<option value="${p.n}" ${u.max_spice === p.n ? "selected" : ""}>Up to Level ${p.n}: ${esc(p.name)}</option>`).join("")}
        </select></label>
        <div class="grid gap-2 sm:grid-cols-2">${Object.entries(PRESETS).map(([k, p]) =>
          `<button data-uact="${k}" class="btn-secondary text-xs" title="${esc(p.title)} Saves right away.">${esc(p.label)}</button>`).join("")}</div>` : ""}
      <div class="grid grid-cols-1 gap-1 sm:grid-cols-2">
        ${RULES.map(([k, l]) => `<label class="toggle"><input type="checkbox" data-rule="${k}" ${u[k] ? "checked" : ""}> ${esc(l)}</label>`).join("")}
      </div>
      <div class="grid gap-2 sm:grid-cols-2">
        <select name="delivery_method" class="input">${deliveryOptions(viewer, u.delivery_method)}</select>
        <input name="kindle_email" value="${esc(u.kindle_email)}" placeholder="name@kindle.com" class="input">
      </div>
      <div class="flex flex-wrap gap-2">
        <button data-uact="save" class="btn-primary">Save</button>
        <button data-uact="password" class="btn-secondary">Reset password</button>
        ${(isAdmin || u.role === "restricted") && on(viewer, "koreader") ? `<button data-uact="koreader" class="btn-ghost" title="Set up this reader's KOReader">📖 KOReader</button>` : ""}
        <button data-uact="delete" class="btn-danger">Delete</button>
      </div>
    </div>`;
}

// badges sum an account up at a glance: role, and for kids their limits.
function badges(u) {
  const role = { admin: ["Admin", "chip-open"], editor: ["Parent (editor)", "chip-cat"], restricted: ["Kid", "chip-none"] }[u.role] || [u.role, "chip-pending"];
  const out = [`<span class="${role[1]}">${esc(role[0])}</span>`];
  if (u.role === "restricted") {
    out.push(`<span class="chip-closed">${u.max_spice < 0 ? "No pepper limit" : `🌶️ Max Level ${u.max_spice}`}</span>`);
    const age = AGE_GROUPS.find(([l]) => l === u.age_level);
    if (age) out.push(`<span class="chip-cat">👪 ${esc(age[1])}</span>`);
  }
  return out.join("");
}
