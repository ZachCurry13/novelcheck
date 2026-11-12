// Admin settings, grouped by Admin tab into collapsible cards (the cards and their fields are listed in
// adminfields.js). Fields tied to a switch (for example the backup AI's details) stay hidden until it's on,
// rarely used ones sit under each card's "Advanced" line, and a Save bar appears once something changed.
import { put } from "./api.js";
import { $, $$, esc, attempt, can } from "./ui.js";
import { initProviderPicker } from "./llmpresets.js";
import { on } from "./modules.js";
import { refreshUser } from "./app.js";
import { SECTIONS, SECTION_MODULE, SECTION_AREA, NOTE, TOP, EXTRA, HELP } from "./adminfields.js";

// What each AI card is set to right now, shown beside its title while it is closed.
const STATE = {
  "Your AI": (v) => v.llm_model || "not set up yet",
  "Backup AI": (v) => (v.backup_llm_enabled === "true" ? v.backup_llm_model || "on" : "off"),
  "Deep Scans": (v) => (v.deep_llm_enabled === "true" ? "on a separate machine" : "on your AI"),
};

// renderSettingsTab puts one tab's settings (as collapsible cards) into host.
export function renderSettingsTab(host, tab, settings, user) {
  const sections = SECTIONS.filter(([t, title]) => t === tab && can(user, SECTION_AREA[title]));
  if (!sections.length) return;
  // Not a <form>: cards hold small forms of their own (custom filters), and forms can't be nested.
  const form = document.createElement("div");
  form.className = "space-y-3";
  const open = (title, i) => (tab === "ai" ? title === "Your AI" && !settings.llm_model : i === 0);
  form.innerHTML = sections.map(([, title, fields], i) => {
    const plain = fields.filter((f) => f[5] !== "adv");
    const adv = fields.filter((f) => f[5] === "adv");
    const draw = (list) => list.map(([k, label, ph, type, when]) => field(k, label, ph, type, settings[k], when)).join("");
    return `
    <details class="card min-w-0${on(user, SECTION_MODULE[title]) ? "" : " module-off"}" ${SECTION_MODULE[title] ? `data-module="${SECTION_MODULE[title]}"` : ""} ${open(title, i) ? "open" : ""}>
      <summary class="cursor-pointer text-lg font-semibold">${esc(title)}${STATE[title] ? ` <span data-state="${esc(title)}" class="set-state"></span>` : ""}
        ${NOTE[title] ? `<span class="set-note">${esc(NOTE[title])}</span>` : ""}</summary>
      <div class="mt-3 space-y-3">${TOP[title] || ""}${draw(plain)}${EXTRA[title] || ""}
        ${adv.length ? `<details class="set-adv"><summary class="cursor-pointer text-sm text-slate-400">Advanced</summary><div class="mt-3 space-y-3">${draw(adv)}</div></details>` : ""}</div>
    </details>`;
  }).join("") + `<div class="save-bar hidden" role="status"><span>You have unsaved changes.</span><button type="button" data-save class="btn-primary">Save</button></div>`;
  host.append(form);

  const llmHost = $("#llm-preset-host", form);
  if (llmHost) initProviderPicker(llmHost, form, settings);
  const autoHost = $("#auto-rate-host", form);
  if (autoHost) import("./autorateadmin.js").then((m) => m.renderAutoRate(autoHost, settings, $("#ideas-host", form))).then(() => (saved = read()));
  const absHost = $("#abs-admin", form);
  if (absHost) import("./absadmin.js").then((m) => m.renderABSAdmin(absHost, form));
  const discoverHost = $("#discover-admin", form);
  if (discoverHost) import("./discoveradmin.js").then((m) => m.renderDiscoverAdmin(discoverHost, form));
  for (const [id, prefix, key] of [["#backup-preset-host", "backup_", "backup_llm_enabled"], ["#deep-preset-host", "deep_", "deep_llm_enabled"]]) {
    const h = $(id, form);
    if (!h) continue;
    // Under the on/off switch: the note, then the provider menu.
    $(`[data-key="${key}"]`, form).closest("label").after(h.previousElementSibling, h);
    initProviderPicker(h, form, settings, prefix);
  }
  const toolsHost = $("#aitools-host", form);
  if (toolsHost) import("./aitoolsadmin.js").then((m) => m.renderAITools(toolsHost, $("#deep-status-host", form)));

  const read = () => {
    const body = {};
    $$("[data-key]", form).forEach((el) => (body[el.dataset.key] = el.type === "checkbox" ? String(el.checked) : el.value));
    return body;
  };
  const bar = $(".save-bar", form);
  const paint = (now) => {
    $$("[data-when]", form).forEach((el) => el.classList.toggle("hidden", !$(`[data-key="${el.dataset.when}"]`, form)?.checked));
    $$("[data-state]", form).forEach((el) => (el.textContent = STATE[el.dataset.state](now)));
  };
  let saved = read();
  paint(saved);
  const changed = () => {
    const now = read();
    paint(now);
    bar.classList.toggle("hidden", !Object.keys(now).some((k) => now[k] !== (saved[k] ?? "")));
  };
  form.addEventListener("input", changed);
  form.addEventListener("change", changed);
  form.addEventListener("click", (e) => { // menus that fill fields in (a model picked from the Ollama list)
    if (!e.target.closest("[data-save]")) setTimeout(changed, 0);
  });
  $("[data-save]", bar).addEventListener("click", async () => {
    const body = read();
    if (!(await attempt(() => put("/api/admin/settings", body), "Settings saved"))) return;
    saved = body;
    bar.classList.add("hidden");
    await refreshUser(); // shows/hides features right away
  });
}

function field(key, label, placeholder, type, value, when) {
  const wrap = (html) => (when ? `<div data-when="${when}">${html}</div>` : html);
  if (type === "hidden") return `<input type="hidden" data-key="${key}" value="${esc(value ?? "")}">`;
  if (type === "bool") {
    return wrap(`<label class="toggle"><input type="checkbox" data-key="${key}" ${value === "true" ? "checked" : ""}> ${esc(label)}</label>`);
  }
  if (type === "select") {
    const opts = placeholder.split("|"); // the choices; the first is the default
    return wrap(`<div><label class="label" for="s-${key}">${esc(label)}</label>
      <select id="s-${key}" data-key="${key}" class="input">${opts.map((o) => `<option ${o === (value || opts[0]) ? "selected" : ""}>${esc(o)}</option>`).join("")}</select>${HELP[key] || ""}</div>`);
  }
  const t = type === "password" ? "password" : type === "number" ? "number" : "text";
  return wrap(`<div><label class="label" for="s-${key}">${esc(label)}</label>
    <input id="s-${key}" data-key="${key}" type="${t}" ${t === "number" ? 'step="any" min="0"' : ""}
      value="${esc(value ?? "")}" placeholder="${esc(placeholder)}" class="input"
      autocomplete="${t === "password" ? "new-password" : "off"}" data-1p-ignore data-lpignore="true">${HELP[key] || ""}</div>`);
}
