// Settings → Developer (admins): the secret link to recent bug reports, and the
// list of recent server errors (each with the E-xxxx code the app showed).
import { get, post, del } from "./api.js";
import { esc, attempt, toast } from "./ui.js";
import { copyText } from "./copy.js";

const nice = (t) => {
  if (!t) return "";
  const d = new Date(/[zZ]$|[+-]\d\d:?\d\d$/.test(t) ? t : t.replace(" ", "T") + "Z");
  return isNaN(d) ? t : d.toLocaleString();
};

// renderLink fills box with the link and its buttons; method is GET, POST, DELETE or "extend".
export async function renderLink(box, method = "GET") {
  const call = () => {
    if (method === "POST") return post("/api/admin/bug-link", {});
    if (method === "extend") return post("/api/admin/bug-link?extend=1", {});
    if (method === "DELETE") return del("/api/admin/bug-link");
    return get("/api/admin/bug-link");
  };
  const r = await attempt(call);
  if (!r) return;
  if (!r.enabled) {
    box.innerHTML = `${r.expired ? `<p class="text-sm text-amber-300">The link ran out. Extend it, or make a new one.</p>` : ""}
      <div class="flex flex-wrap gap-2">${r.expired ? `<button type="button" data-link="extend" class="btn-primary">Extend 7 days</button>` : ""}
        <button type="button" data-link="new" class="${r.expired ? "btn-secondary" : "btn-primary"}">Create link</button></div>`;
    return;
  }
  box.innerHTML = `<input class="input text-xs" readonly value="${esc(r.url)}" aria-label="Bug report link">
    <p class="text-xs text-slate-400">Works until ${esc(nice(r.expires))} · ${r.seen ? `last opened ${esc(nice(r.seen))}` : "not opened yet"}</p>
    <div class="flex flex-wrap gap-2"><button type="button" data-link="copy" data-url="${esc(r.url)}" class="btn-primary">📋 Copy link</button>
      <button type="button" data-link="extend" class="btn-secondary">Extend 7 days</button>
      <button type="button" data-link="new" class="btn-secondary">New link</button>
      <button type="button" data-link="off" class="btn-ghost">Turn off</button></div>`;
}

// renderErrors lists the recent failed requests.
export async function renderErrors(box) {
  const r = await attempt(() => get("/api/admin/server-errors"));
  if (!r) return;
  const list = r.errors || [];
  box.innerHTML = list.length ? `<ul class="max-h-72 space-y-1 overflow-auto text-xs">${list.map((e) => `<li class="rounded bg-slate-800/60 p-2">
      <b>${esc(e.code)}</b> · ${esc(nice(e.at))} · ${esc(e.method)} ${esc(e.path)} → ${e.status}${e.username ? ` · ${esc(e.username)}` : ""}
      <span class="block break-words text-slate-400">${esc(e.detail)}</span></li>`).join("")}</ul>
    <div class="flex gap-2"><button type="button" data-err="copy" class="btn-secondary">📋 Copy</button><button type="button" data-err="clear" class="btn-ghost">Clear list</button></div>`
    : `<p class="text-sm text-slate-400">No server errors. 🎉</p>`;
  box.dataset.text = list.map((e) => `${e.code} ${e.at} ${e.method} ${e.path} -> ${e.status} ${e.username}: ${e.detail}`).join("\n");
}

// handleDevLink runs a click on the link or errors card; it returns true when it handled it.
export async function handleDevLink(e, host) {
  const l = e.target.closest("[data-link]");
  if (l) {
    const box = host.querySelector("#dev-link");
    const k = l.dataset.link;
    if (k === "copy") copyText(l.dataset.url);
    else await renderLink(box, k === "new" ? "POST" : k === "off" ? "DELETE" : "extend");
    return true;
  }
  const er = e.target.closest("[data-err]");
  if (er) {
    const box = host.querySelector("#dev-errors");
    if (er.dataset.err === "copy") copyText(box.dataset.text || "");
    else if (await attempt(() => del("/api/admin/server-errors"), "Cleared")) renderErrors(box);
    else toast("That didn't work", true);
    return true;
  }
  return false;
}
