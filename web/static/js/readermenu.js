// The reader's menu (js/reader.js): the book's contents, and KOReader's main reading settings: font, size, line
// spacing, margins and day, sepia or night. Settings are kept on this device (they suit its screen) and apply
// to every book.
const KEY = "nc:reader";
const DEFAULTS = { font: "book", size: 100, spacing: 1.5, margin: "medium", theme: "night", justify: true };
const FONTS = { book: ["The book's own", ""], serif: ["Serif", "Georgia, 'Iowan Old Style', 'Palatino Linotype', serif"],
  sans: ["Sans", "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"], dyslexic: ["OpenDyslexic", "'OpenDyslexic', sans-serif"] };
const MARGINS = { small: 16, medium: 40, large: 72 };
const THEMES = { day: ["Day", "#ffffff", "#1a1a1a"], sepia: ["Sepia", "#f4ecd8", "#3b2f22"], night: ["Night", "#141415", "#d6d6d6"] };

export function readerSettings() {
  try {
    return { ...DEFAULTS, ...JSON.parse(localStorage.getItem(KEY) || "{}") };
  } catch {
    return { ...DEFAULTS };
  }
}

function keep(s) {
  try {
    localStorage.setItem(KEY, JSON.stringify(s));
  } catch { /* private window: the settings last until the page closes */ }
}

// css is what goes into the book's pages: colours, the chosen font, size and spacing.
function css(s) {
  const [, bg, fg] = THEMES[s.theme] || THEMES.night;
  const family = FONTS[s.font]?.[1];
  return `@namespace epub "http://www.idpf.org/2007/ops";
    @font-face { font-family: "OpenDyslexic"; src: url("${location.origin}/fonts/OpenDyslexic-Regular.woff2") format("woff2"); }
    html { color-scheme: ${s.theme === "night" ? "dark" : "light"}; background: ${bg} !important; color: ${fg} !important; }
    body { background: transparent !important; color: inherit !important; font-size: ${s.size}% !important; }
    p, li, blockquote, dd { line-height: ${s.spacing} !important; text-align: ${s.justify ? "justify" : "start"}; hyphens: auto; }
    ${family ? `body, p, li, blockquote, dd, span, div { font-family: ${family} !important; }` : ""}
    a:any-link { color: ${s.theme === "night" ? "#a5b4fc" : "#4338ca"}; }
    img, svg { max-width: 100%; height: auto; }
    ${s.theme === "night" ? "img { filter: brightness(0.9); }" : ""}`;
}

export function applySettings(view, s) {
  const r = view.renderer;
  r.setAttribute("flow", "paginated");
  r.setAttribute("margin", `${MARGINS[s.margin] ?? 40}px`);
  r.setAttribute("gap", "6%");
  r.setAttribute("max-inline-size", "720px");
  r.setAttribute("max-column-count", "2");
  r.setStyles?.(css(s));
  document.body.dataset.theme = s.theme;
}

const panel = () => document.getElementById("reader-panel");
export const closePanel = () => (panel().hidden = true);
const esc = (t) => String(t).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

export function openContents(view) {
  const items = [];
  const walk = (list = [], depth = 0) => list.forEach((t) => {
    items.push(`<li><button type="button" data-href="${esc(t.href || "")}" class="reader-toc-item depth-${Math.min(depth, 3)}">${esc((t.label || "").trim())}</button></li>`);
    walk(t.subitems, depth + 1);
  });
  walk(view.book?.toc);
  const p = panel();
  p.innerHTML = `<h2 class="text-base font-medium">Contents</h2>
    <ul class="mt-2 space-y-1">${items.join("") || `<li class="text-sm text-slate-400">This book has no table of contents.</li>`}</ul>`;
  p.hidden = false;
  p.onclick = (e) => {
    const b = e.target.closest("[data-href]");
    if (!b) return;
    view.goTo(b.dataset.href);
    p.hidden = true;
    document.getElementById("reader-top").hidden = true;
  };
}

export function openSettings(view) {
  const s = readerSettings();
  const choice = (name, value, label, on) => `<button type="button" data-set="${name}" data-value="${value}" aria-pressed="${on}" class="chip-filter">${label}</button>`;
  const row = (title, inner) => `<div class="space-y-2"><p class="text-sm text-slate-400">${title}</p><div class="flex flex-wrap gap-2">${inner}</div></div>`;
  const paint = () => {
    panel().innerHTML = `<h2 class="text-base font-medium">Reading settings</h2><div class="mt-3 space-y-4">
      ${row("Font", Object.entries(FONTS).map(([k, [l]]) => choice("font", k, l, s.font === k)).join(""))}
      ${row(`Size · ${s.size}%`, `${choice("size", s.size - 10, "A−", false)}${choice("size", 100, "Reset", s.size === 100)}${choice("size", s.size + 10, "A+", false)}`)}
      ${row("Line spacing", [[1.2, "Tight"], [1.5, "Normal"], [1.8, "Loose"]].map(([v, l]) => choice("spacing", v, l, s.spacing === v)).join(""))}
      ${row("Margins", Object.keys(MARGINS).map((k) => choice("margin", k, k[0].toUpperCase() + k.slice(1), s.margin === k)).join(""))}
      ${row("Page", Object.entries(THEMES).map(([k, [l]]) => choice("theme", k, l, s.theme === k)).join(""))}
      ${row("Text", `${choice("justify", "true", "Justified", s.justify)}${choice("justify", "false", "Ragged right", !s.justify)}`)}
      <p class="text-xs text-slate-500">Kept on this device for every book.</p></div>`;
  };
  paint();
  panel().hidden = false;
  panel().onclick = (e) => {
    const b = e.target.closest("[data-set]");
    if (!b) return;
    const { set, value } = b.dataset;
    if (set === "size") s.size = Math.max(60, Math.min(250, Number(value)));
    else if (set === "spacing") s.spacing = Number(value);
    else if (set === "justify") s.justify = value === "true";
    else s[set] = value;
    keep(s);
    applySettings(view, s);
    paint();
  };
}
