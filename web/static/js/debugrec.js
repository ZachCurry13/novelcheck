// A small record of what happens in this browser: pages opened, buttons pressed
// (their labels, never what is typed), failed requests and errors. It stays in
// this browser until someone copies it from Settings → Developer.
const KEY = "nc:debuglog";
const MAX = 250;
let lines = [];
try {
  lines = JSON.parse(sessionStorage.getItem(KEY) || "[]");
} catch {
  lines = [];
}

const stamp = () => new Date().toISOString().slice(11, 19);

export function note(kind, msg) {
  lines.push(`${stamp()} [${kind}] ${String(msg).replace(/\s+/g, " ").slice(0, 300)}`);
  if (lines.length > MAX) lines = lines.slice(-MAX);
  try {
    sessionStorage.setItem(KEY, JSON.stringify(lines));
  } catch {
    /* private mode: kept in memory only */
  }
}

export function clearNotes() {
  lines = [];
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    /* nothing to clear */
  }
}

// report is the text to copy: the app's version and device, then the record.
export function report(version = "") {
  return [`NovelCheck web report · ${new Date().toISOString()}`, `App ${version || "?"} · ${navigator.userAgent}`,
    `Screen ${innerWidth}x${innerHeight} · online ${navigator.onLine} · page ${location.hash || "/"}`, "", ...lines].join("\n");
}

const label = (el) => (el.getAttribute("aria-label") || el.dataset.act || el.id || el.textContent || el.tagName).trim().replace(/\s+/g, " ").slice(0, 60);

export function startRecording() {
  document.addEventListener("click", (e) => {
    const el = e.target.closest?.("button, a, summary, [data-act], [role=button]");
    if (el) note("tap", `${el.tagName.toLowerCase()} ${label(el)}`);
  }, true);
  window.addEventListener("hashchange", () => note("page", location.hash));
  window.addEventListener("error", (e) => note("error", `${e.message} (${(e.filename || "").split("/").pop()}:${e.lineno})`));
  window.addEventListener("unhandledrejection", (e) => note("error", `promise: ${e.reason?.message || e.reason}`));
  note("page", `opened ${location.hash || "/"}`);
}
