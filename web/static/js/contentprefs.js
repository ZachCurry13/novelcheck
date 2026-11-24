// Profile → "What to hide or flag": for each content label, Hide (the book stays out of your pages), Heads-up (it
// shows, with a notice at the top of its page; for spice, where a Deep Scan found it) or Don't mention. Only for you;
// kids' accounts keep the rules a parent sets (api/contentprefs.go).
import { get, put } from "./api.js";
import { esc, attempt } from "./ui.js";

const CHOICES = [["hide", "Hide"], ["warn", "Heads-up"], ["", "Don't mention"]];
const BOOK_FLAGS = [["nudity", "Nudity"], ["solo_acts", "Solo acts"], ["heavy_innuendo", "Heavy innuendo"], ["dark_occult", "Dark occult or demonic"]];

export async function renderContentPrefs(host) {
  const r = await get("/api/me/content-prefs").catch(() => null);
  if (!r) { host.innerHTML = ""; return; }
  const p = r.prefs, choice = (k) => (p.hide.includes(k) ? "hide" : p.warn.includes(k) ? "warn" : "");
  const row = (k, label, sub = false) => `<li class="flex flex-wrap items-center justify-between gap-2 py-1 ${sub ? "pl-4 text-slate-400" : ""}">
    <span class="min-w-0">${esc(label)}</span><span class="flex gap-1" role="radiogroup" aria-label="${esc(label)}">${CHOICES.map(([v, l]) =>
      `<button type="button" data-k="${esc(k)}" data-v="${v}" role="radio" aria-checked="${choice(k) === v}" class="${choice(k) === v ? "btn-secondary" : "btn-ghost"} px-2 py-0.5 text-xs">${l}</button>`).join("")}</span></li>`;
  const levels = (sel, none) => [`<option value="-1">${none}</option>`, ...[0, 1, 2, 3, 4, 5].map((n) => `<option value="${n}" ${sel === n ? "selected" : ""}>${n}</option>`)].join("");
  host.innerHTML = `<h2 class="text-lg font-semibold">What to hide or flag</h2>
    <p class="text-sm text-slate-400">Only for you. <b>Hide</b> keeps a book out of your pages; <b>Heads-up</b> shows it with a note at the top of its page; <b>Don't mention</b> leaves it alone.</p>
    <div class="grid gap-2 sm:grid-cols-2 text-sm">
      <label>Hide books above spice level <select data-spice="hide_above" class="input mt-1">${levels(p.hide_above, "no limit")}</select></label>
      <label>Heads-up from spice level <select data-spice="warn_from" class="input mt-1">${levels(p.warn_from, "never")}</select></label></div>
    <p class="text-xs text-slate-500">For spice, a book a Deep Scan has read also says where (“around 40% in”).</p>
    <ul class="divide-y divide-slate-800 text-sm">${BOOK_FLAGS.map(([k, l]) => row(k, l)).join("")}${row("lgbtq", "LGBTQ+ characters or themes")}</ul>
    ${r.groups.map((g) => `<details class="text-sm"><summary class="cursor-pointer py-1 font-medium">${esc(g.icon)} ${esc(g.label)}</summary>
      <ul class="divide-y divide-slate-800">${row("g:" + g.key, `All of ${g.label}`)}${g.items.filter((it) => it.key !== "lgbtq").map((it) => row(it.key, it.label, true)).join("")}</ul></details>`).join("")}
    ${r.flags.length ? `<details class="text-sm"><summary class="cursor-pointer py-1 font-medium">The family's own filters</summary>
      <ul class="divide-y divide-slate-800">${r.flags.map((f) => row("flag:" + f.key, f.label)).join("")}</ul></details>` : ""}
    <label class="toggle text-sm"><input type="checkbox" data-unrated ${p.hide.includes("unrated") ? "checked" : ""}> Hide books not rated yet</label>
    <button type="button" data-save class="btn-primary">Save</button>`;
  host.onclick = async (e) => {
    const b = e.target.closest("[data-k]");
    if (b) {
      const k = b.dataset.k, v = b.dataset.v;
      p.hide = p.hide.filter((x) => x !== k);
      p.warn = p.warn.filter((x) => x !== k);
      if (v) p[v === "hide" ? "hide" : "warn"].push(k);
      b.parentElement.querySelectorAll("[data-k]").forEach((x) => {
        const on = x === b;
        x.setAttribute("aria-checked", String(on));
        x.className = `${on ? "btn-secondary" : "btn-ghost"} px-2 py-0.5 text-xs`;
      });
    }
    if (e.target.closest("[data-save]")) {
      const unrated = host.querySelector("[data-unrated]").checked;
      p.hide = p.hide.filter((x) => x !== "unrated").concat(unrated ? ["unrated"] : []);
      host.querySelectorAll("[data-spice]").forEach((s) => { p[s.dataset.spice] = Number(s.value); });
      await attempt(() => put("/api/me/content-prefs", p), "Saved. Your pages follow your choices now.");
    }
  };
}
