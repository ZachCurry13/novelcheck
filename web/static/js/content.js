// Detailed content items (🗣️ Language, ⚔️ Violence, 🩸 Gore, 🍺 Substance Use,
// 🧩 Other Content; the list lives in internal/content): icons on cards, the
// list in the book window, the Hide picker (Library and kids' accounts) and the
// ticks in "Edit rating".
import { get } from "./api.js";
import { $, $$, esc, attempt } from "./ui.js";

let cat = { groups: [], presets: {}, amounts: [] };

// loadContent fetches the catalog once (it only changes with an update).
export async function loadContent() {
  if (!loadContent.done) {
    const c = await attempt(() => get("/api/content"));
    if (c) {
      cat = c;
      loadContent.done = true;
    }
  }
  return cat;
}

export const contentPresets = () => cat.presets || {};

const SOURCES = { ai: "From the description", deep: "From the full text (Deep Scan)", parent: "Set by a parent" };

// found maps a book's items to who found them ("war:ai,blood:deep").
const found = (b) => new Map((b.content || "").split(",").filter(Boolean).map((s) => s.split(":")));

// amounts maps group keys to a Deep Scan amount (1 a little .. 3 a lot).
const amounts = (b) => Object.fromEntries((b.content_amounts || "").split(",").filter(Boolean)
  .map((s) => s.split(":")).map(([g, n]) => [g, Number(n)]));

// present lists the groups a book contains, each with its items.
function present(b) {
  const f = found(b);
  return cat.groups.map((g) => ({ g, items: g.items.filter((it) => f.has(it.key)).map((it) => ({ ...it, source: f.get(it.key) })) }))
    .filter((x) => x.items.length);
}

const groupLine = (g, items, amount) =>
  `${g.label}${amount ? ` (${amount})` : ""}: ${items.map((it) => it.label).join(", ")}`;

// contentIcons: a small chip per group a book contains; the book window lists the items.
export function contentIcons(b) {
  const amt = amounts(b);
  return present(b).map(({ g, items }) => {
    const tip = groupLine(g, items, cat.amounts[amt[g.key]]);
    return `<span class="chip-flag" title="${esc(tip)}" aria-label="${esc(tip)}">${g.icon}</span>`;
  }).join(" ");
}

// contentSection: the book window's list, with the Deep Scan amounts and where
// the items came from.
export function contentSection(b) {
  if (!cat.groups.length || b.status !== "analyzed") return "";
  const list = present(b);
  if (!list.length) {
    return `<p class="text-xs text-slate-500"><b>Content details:</b> ${b.content_version
      ? "none of the language, violence, gore, substance or other items were found." : "not checked yet."}</p>`;
  }
  const amt = amounts(b);
  const sources = [...new Set(list.flatMap(({ items }) => items.map((it) => SOURCES[it.source]).filter(Boolean)))];
  return `<div><span class="label">Content details</span>
    <ul class="space-y-1 text-sm text-slate-300">${list.map(({ g, items }) => {
      const a = cat.amounts[amt[g.key]];
      return `<li><b class="text-slate-200">${g.icon} ${esc(g.label)}${a ? ` · ${esc(a)}` : ""}:</b> ${esc(items.map((it) => it.label).join(", "))}</li>`;
    }).join("")}</ul>
    <p class="mt-1 text-xs text-slate-500">${esc(sources.join(" · "))}</p></div>`;
}

// contentLine: one "Also contains" line for Check a book.
export function contentLine(b) {
  const list = present(b);
  return list.length ? `<p class="text-sm"><b>Also contains:</b> ${list.map(({ g, items }) =>
    `${g.icon} ${esc(g.label)} (${esc(items.map((it) => it.label).join(", "))})`).join(" · ")}</p>` : "";
}

// hidePicker: per group, a box that hides the whole group (items added later
// included) and "Items" to pick single ones. attr is the checkboxes' marker,
// e.g. `name="hide"` (Library) or `data-hide` (kids' accounts).
export function hidePicker(selected = [], attr = `name="hide"`) {
  const on = new Set(selected);
  return `<div class="content-picker grid items-start gap-2 sm:grid-cols-2 lg:grid-cols-3">${cat.groups.map((g) => {
    const all = on.has("g:" + g.key);
    return `<div class="rounded-lg bg-slate-800/60 px-2 py-1" data-cgroup="${esc(g.key)}">
      <div class="flex items-center gap-2">
        <label class="toggle" title="Hide every ${esc(g.label)} item"><input type="checkbox" ${attr} value="g:${esc(g.key)}" data-group ${all ? "checked" : ""}> ${g.icon} ${esc(g.label)}</label>
        <button type="button" data-expand aria-expanded="false" class="ml-auto text-xs text-slate-400 underline"><span data-count>${count(g, on)}</span> ▸</button>
      </div>
      <div class="hidden grid gap-1 py-1 pl-6" data-items>${g.items.map((it) =>
        `<label class="toggle"${it.hint ? ` title="${esc(it.hint)}"` : ""}><input type="checkbox" ${attr} value="${esc(it.key)}" ${all || on.has(it.key) ? "checked" : ""} ${all ? "disabled" : ""}> ${esc(it.label)}</label>`).join("")}</div>
    </div>`;
  }).join("")}</div>`;
}

function count(g, on) {
  if (on.has("g:" + g.key)) return "All";
  const n = g.items.filter((it) => on.has(it.key)).length;
  return n ? `${n} of ${g.items.length}` : "Items";
}

// bindHidePicker wires "Items" and the group boxes inside root.
export function bindHidePicker(root) {
  root.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-expand]");
    if (!btn) return;
    const open = !$("[data-items]", btn.closest("[data-cgroup]")).classList.toggle("hidden");
    btn.setAttribute("aria-expanded", String(open));
    btn.lastChild.textContent = open ? " ▾" : " ▸";
  });
  root.addEventListener("change", (e) => {
    const box = e.target.closest("[data-cgroup]");
    if (!box) return;
    const group = $("[data-group]", box);
    if (e.target === group) {
      $$("[data-items] input", box).forEach((cb) => { cb.checked = group.checked; cb.disabled = group.checked; });
    }
    const on = new Set(pickedIn(box));
    $("[data-count]", box).textContent = count(cat.groups.find((g) => g.key === box.dataset.cgroup), on);
  });
}

// pickedIn returns the rules ticked inside root ("g:violence", "murder").
export const pickedIn = (root) => $$("[data-cgroup] input:checked:not(:disabled)", root).map((cb) => cb.value);

// contentTicks: the items in "Edit rating", one fold per group.
export function contentTicks(b) {
  const f = found(b);
  return `<div class="space-y-1"><span class="label">Content details</span>
    ${b.content_version ? "" : `<p class="text-xs text-slate-500">Not checked yet: tick what you know, or leave it for the AI.</p>`}
    ${cat.groups.map((g) => {
      const n = g.items.filter((it) => f.has(it.key)).length;
      return `<details class="rounded-lg bg-slate-900/60 px-2 py-1"><summary class="cursor-pointer py-1 text-sm">${g.icon} ${esc(g.label)}${n ? ` · ${n} ticked` : ""}</summary>
        <div class="grid gap-1 py-1 sm:grid-cols-2">${g.items.map((it) =>
          `<label class="toggle"><input type="checkbox" data-content="${esc(it.key)}" ${f.has(it.key) ? "checked" : ""}> ${esc(it.label)}</label>`).join("")}</div></details>`;
    }).join("")}</div>`;
}
