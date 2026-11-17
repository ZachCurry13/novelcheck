// After an update: a "What's new" window, once per device and version, with the release notes of the versions that
// device hasn't seen yet (the newest three; the rest are under More → What's new). Nothing shows on a first visit
// (a new person gets the guide instead), for kids' accounts, or for a version already seen. Where the last seen
// version is kept: this device's storage, so each phone gets it once after its own update.
import { esc } from "./ui.js";
import { renderMarkdown, changelogSections } from "./markdown.js";

const KEY = "novelcheck.seenVersion";
const MAX = 3;

const plain = (v) => {
  const m = String(v || "").match(/^v?(\d+)\.(\d+)\.(\d+)/);
  return m ? `${m[1]}.${m[2]}.${m[3]}` : "";
};
// newer: negative, 0 or positive like a comparison of two versions.
const newer = (a, b) => {
  const x = a.split(".").map(Number), y = b.split(".").map(Number);
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] - y[i];
  return 0;
};
const read = () => {
  try {
    return localStorage.getItem(KEY) || "";
  } catch {
    return "";
  }
};
const write = (v) => {
  try {
    localStorage.setItem(KEY, v);
  } catch {
    /* private mode: it may show again next time */
  }
};

// notesToShow picks the sections to show: those after `last` up to `now`; with no `last`, only the newest.
export function notesToShow(changelog, now, last) {
  const upTo = changelogSections(changelog).filter((s) => plain(s.version) && newer(plain(s.version), now) <= 0);
  const fresh = last ? upTo.filter((s) => newer(plain(s.version), last) > 0) : upTo.slice(0, 1);
  return { shown: fresh.slice(0, MAX), more: Math.max(fresh.length - MAX, 0) };
}

export function showNotesAfterUpdate(current, changelog, state) {
  const now = plain(current);
  if (!now) return; // a development build
  const last = plain(read());
  if (last === now) return;
  write(now); // seen from now on, whether or not it gets read
  const user = state?.user;
  if (!user || user.role === "restricted" || (!last && !user.guide_seen)) return;
  const { shown, more } = notesToShow(changelog, now, last);
  if (!shown.length) return;
  const dlg = document.createElement("dialog");
  dlg.className = "dialog";
  dlg.setAttribute("aria-label", "What's new");
  dlg.innerHTML = `<div class="space-y-4 p-6">
      <div><h2 class="text-xl font-semibold">What's new</h2>
        <p class="text-sm text-slate-400">NovelCheck updated to ${esc(now)}.</p></div>
      <div class="max-h-[55vh] space-y-5 overflow-y-auto pr-1">${shown.map((s) => `<section><h3 class="mb-1 font-semibold text-slate-100">Version ${esc(s.version)}</h3>
        <div class="text-sm leading-relaxed text-slate-300">${renderMarkdown(s.body)}</div></section>`).join("")}
        ${more ? `<p class="text-sm text-slate-400">${more} earlier update${more === 1 ? "" : "s"} are under More → What's new.</p>` : ""}</div>
      <div class="flex flex-wrap justify-end gap-2"><a href="#/whatsnew" data-close class="btn-secondary">All release notes</a>
        <button type="button" data-close class="btn-primary">Got it</button></div></div>`;
  dlg.addEventListener("click", (e) => {
    if (e.target === dlg || e.target.closest("[data-close]")) dlg.close();
  });
  document.body.append(dlg);
  dlg.showModal();
  // Gone from the page once closed. The open attribute is watched, as dialogback.js does: the "close" event is not
  // reliable in every browser.
  const watch = new MutationObserver(() => {
    if (!dlg.open) {
      watch.disconnect();
      dlg.remove();
    }
  });
  watch.observe(dlg, { attributes: true, attributeFilter: ["open"] });
}
