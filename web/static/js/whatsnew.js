// "What's new": running version, update status, and release notes.
import { get } from "./api.js";
import { esc, attempt } from "./ui.js";
import { renderMarkdown, changelogSections } from "./markdown.js";

export async function renderWhatsNew(view, state) {
  const data = await attempt(() => get("/api/updates"));
  if (!data) return;
  const st = data.status || { current: "unknown", releases: [] };
  const manager = state.user.role === "admin" || state.user.role === "editor";
  const newer = st.releases.filter((r) => r.newer);

  let banner = "";
  if (st.update_available && manager) {
    banner = `<div class="card mb-6 ring-emerald-700">
      <p class="font-semibold text-emerald-300">Version ${esc(st.latest)} is available. You're on ${esc(st.current)}.</p>
      <p class="mt-2 text-sm text-slate-300">To update on TrueNAS: <b>Apps → novelcheck → Update</b>. If there's no Update button,
        click <b>Edit</b>, then <b>Save</b> without changing anything. Your books, ratings and users are kept.</p>
    </div>`;
  } else if (manager && data.checks_enabled && st.latest && !st.error) {
    banner = `<p class="mb-6 text-sm text-emerald-400">✓ You're on the latest version.</p>`;
  } else if (manager && st.error) {
    banner = `<p class="mb-6 text-sm text-amber-400">${esc(st.error)}. Showing the notes bundled with this version.</p>`;
  }

  // Newer releases come from GitHub; everything up to the installed version
  // comes from the changelog bundled in this build.
  const upcoming = newer.map((r) => section(`${r.tag} (new)`, r.notes, r.published_at, r.url)).join("");
  const seen = new Set(newer.map((r) => r.tag.replace(/^v/, "")));
  const bundled = changelogSections(data.changelog)
    .filter((s) => !seen.has(s.version))
    .map((s) => section(`v${s.version}${sameVersion(s.version, st.current) ? " (installed)" : ""}`, s.body)).join("");

  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">What's new</h1>
    <p class="mb-4 text-sm text-slate-400">You're running NovelCheck <b>${esc(st.current)}</b>.</p>
    ${banner}
    <div class="space-y-4">${upcoming}${bundled}</div>`;
}

function sameVersion(a, b) {
  return String(b || "").replace(/^v/, "").split("-")[0] === a;
}

function section(title, notes, date, url) {
  return `<article class="card">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <h2 class="text-lg font-bold">${esc(title)}</h2>
      <span class="text-xs text-slate-500">${date ? esc(new Date(date).toLocaleDateString()) : ""}
        ${url ? ` · <a href="${esc(url)}" target="_blank" rel="noopener noreferrer" class="underline">GitHub</a>` : ""}</span>
    </div>
    <div class="text-sm leading-relaxed text-slate-300">${renderMarkdown(notes)}</div>
  </article>`;
}
