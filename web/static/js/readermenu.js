// The reader's panels (js/reader.js, js/readerbars.js): the book's contents, and the Aa settings: text size, page colour, font, line
// spacing, margins, text edges and turning pages by tapping or scrolling. Settings are kept on this device (they suit its screen) and apply
// to every book.
const KEY = "nc:reader";
const DEFAULTS = { font: "book", size: 100, spacing: 1.5, margin: "medium", theme: "night", justify: true, flow: "paginated" };
const FONTS = { book: ["The book's own", ""], serif: ["Serif", "Georgia, 'Iowan Old Style', 'Palatino Linotype', serif"],
  sans: ["Sans", "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"], dyslexic: ["OpenDyslexic", "'OpenDyslexic', sans-serif"] };
const MARGINS = { small: 16, medium: 40, large: 72 };
export const THEMES = { day: ["Day", "#ffffff", "#1a1a1a"], sepia: ["Sepia", "#f4ecd8", "#3b2f22"], night: ["Night", "#141415", "#d6d6d6"] };

export function readerSettings() {
  try {
    return { ...DEFAULTS, ...JSON.parse(localStorage.getItem(KEY) || "{}") };
  } catch {
    return { ...DEFAULTS };
  }
}

export const keepSettings = (s) => keep(s);

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
  r.setAttribute("flow", s.flow === "scrolled" ? "scrolled" : "paginated"); // turn pages, or scroll
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
  const walk = (list, depth = 0) => (list || []).forEach((t) => { // a chapter with no parts has subitems null
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
  const seg = (name, opts, cur) => `<div class="reader-seg" role="group">${opts.map(([v, l]) =>
    `<button type="button" data-set="${name}" data-value="${v}" aria-pressed="${String(cur) === String(v)}">${l}</button>`).join("")}</div>`;
  const row = (title, inner) => `<div class="reader-row"><span>${title}</span>${inner}</div>`;
  const paint = () => {
    panel().innerHTML = `<div class="reader-grab" aria-hidden="true"></div>
      ${row("Text size", `<div class="reader-seg"><button type="button" data-set="size" data-value="${s.size - 10}" aria-label="Smaller text">A−</button>
        <button type="button" data-set="size" data-value="100" aria-pressed="${s.size === 100}" title="Back to 100%">${s.size}%</button>
        <button type="button" data-set="size" data-value="${s.size + 10}" aria-label="Bigger text">A+</button></div>`)}
      ${row("Page", `<div class="reader-swatches">${Object.entries(THEMES).map(([k, [l, bg, fg]]) =>
        `<button type="button" data-set="theme" data-value="${k}" aria-pressed="${s.theme === k}" class="reader-swatch reader-swatch-${k}">${l}</button>`).join("")}</div>`)}
      ${row("Font", seg("font", Object.entries(FONTS).map(([k, [l]]) => [k, k === "book" ? "Book" : k === "dyslexic" ? "Dyslexic" : l]), s.font))}
      ${row("Line spacing", seg("spacing", [[1.2, "Tight"], [1.5, "Normal"], [1.8, "Loose"]], s.spacing))}
      ${row("Margins", seg("margin", Object.keys(MARGINS).map((k) => [k, k[0].toUpperCase() + k.slice(1)]), s.margin))}
      ${row("Text edges", seg("justify", [["true", "Even"], ["false", "Ragged"]], s.justify))}
      ${row("Turn pages", seg("flow", [["paginated", "Tap"], ["scrolled", "Scroll"]], s.flow))}
      <p class="mt-2 text-xs opacity-60">Kept on this device for every book.</p>`;
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
    document.dispatchEvent(new CustomEvent("nc:reader-settings")); // the bar's Day · Sepia · Night follows
  };
}
