// Nightstand → Bookshelf: the books you finished in a year, standing on wooden shelves. Covers or spines; the season
// (wall, decorations, shelf extras, what falls) follows the date by itself and can be changed. The drawings of the
// seasons are in bookshelfart.js, the falling things in bookshelffall.js, the styles in css/bookshelf.css.
import { get } from "./api.js";
import { esc } from "./ui.js";
import { prepare, finishes } from "./statscalc.js";
import { addElsewhere } from "./statselsewhere.js";
import { THEMES, P } from "./bookshelfart.js";
import { startFalling } from "./bookshelffall.js";

const KEY = "novelcheck.bookshelf";
const load = () => {
  try {
    return JSON.parse(localStorage.getItem(KEY) || "{}");
  } catch {
    return {};
  }
};
const save = (v) => {
  try {
    localStorage.setItem(KEY, JSON.stringify(v));
  } catch {
    /* private mode: the choices are forgotten on reload */
  }
};

// seasonFor is the season of a date: winter December to February, spring, summer, then autumn with Halloween from the
// middle of October to the start of November.
export function seasonFor(d = new Date()) {
  const m = d.getMonth(), day = d.getDate();
  if (m === 11 || m <= 1) return "winter";
  if (m <= 4) return "spring";
  if (m <= 7) return "summer";
  if ((m === 9 && day >= 15) || (m === 10 && day <= 5)) return "halloween";
  return "autumn";
}

const COLORS = [["#2b4a6b", "#1c3350", "#fff2d6"], ["#7a2f2a", "#521d1b", "#ffe6c9"], ["#2f5d4c", "#1f4034", "#e4f5e8"], ["#c98a3a", "#9d6420", "#2b1a07"],
  ["#4b3a6e", "#32254f", "#ece4ff"], ["#a63f55", "#7a283b", "#ffe6ec"], ["#3f6f86", "#29495a", "#e6f6ff"], ["#d4b062", "#a98534", "#33250a"],
  ["#6b8f4e", "#48662f", "#f6ffe6"], ["#8e5f3a", "#63401f", "#ffeedd"], ["#2e2e4a", "#1d1d33", "#e6e6ff"], ["#c96a4a", "#9b4529", "#fff0e6"]];
const ART = [(c) => `radial-gradient(circle at 70% 62%,${c}55 0 22%,transparent 23%)`, (c) => `repeating-linear-gradient(135deg,${c}22 0 8px,transparent 8px 16px)`,
  (c) => `radial-gradient(circle at 30% 78%,${c}44 0 26%,transparent 27%),radial-gradient(circle at 72% 70%,${c}33 0 14%,transparent 15%)`,
  (c) => `repeating-radial-gradient(circle at 50% 130%,${c}30 0 6px,transparent 6px 14px)`];
const hash = (s) => [...s].reduce((h, ch) => (h * 31 + ch.charCodeAt(0)) >>> 0, 7);
const HEIGHTS = [168, 182, 160, 190, 175, 165, 186, 172], WIDTHS = [30, 38, 32, 44, 34, 40, 36, 31, 42, 35];

let coverBust = 0; // set when a cover was changed from the shelf, so the new one shows at once

function bookHTML(b, mode, i) {
  const h = hash(b.title || ""), [c1, c2, tc] = COLORS[h % COLORS.length];
  const vars = `--bs-b1:${c1};--bs-b2:${c2};--bs-t:${tc};${b.book_id ? "cursor:pointer;" : ""}`;
  const tip = esc(`${b.title}${b.author ? ` · ${b.author}` : ""}`);
  if (mode === "covers") {
    const plain = `<b>${esc(b.title)}</b><small>${esc(b.author || "")}</small>`;
    return `<div class="bs-cover" ${b.book_id ? `data-open="${Number(b.book_id)}" role="button" tabindex="0"` : ""} data-plain="${esc(plain)}" style="${vars}--bs-art:${ART[h % ART.length](tc)}" title="${tip}">${b.book_id
      ? `<img src="/api/books/${Number(b.book_id)}/cover${coverBust ? `?v=${coverBust}` : ""}" alt="" loading="lazy" style="position:absolute;inset:0;width:100%;height:100%;object-fit:cover">` : plain}</div>`;
  }
  const last = (b.author || "").trim().split(/\s+/).pop();
  return `<div class="bs-spine${i % 11 === 7 ? " bs-lean" : ""}" ${b.book_id ? `data-open="${Number(b.book_id)}" role="button" tabindex="0"` : ""} style="${vars}width:${WIDTHS[h % WIDTHS.length]}px;height:${HEIGHTS[(h >>> 3) % HEIGHTS.length]}px" title="${tip}"><i>${esc(b.title)}</i><em>${esc(last)}</em></div>`;
}

export async function renderBookshelf(host, state) {
  const data = await get("/api/me/reading-stats").catch(() => null);
  if (!data) return;
  const m = prepare(data);
  addElsewhere(m, data.hardcover_read || [], data.imported_read || []);
  const done = finishes(m);
  const years = [...new Set(done.map((d) => new Date(d.ms).getFullYear()))].sort((a, b) => b - a);
  const thisYear = new Date().getFullYear();
  if (!years.includes(thisYear)) years.unshift(thisYear);
  const pref = { mode: "covers", season: "auto", year: thisYear, lights: true, fall: true, hang: true, props: true, ...load() };
  if (!years.includes(pref.year)) pref.year = thisYear;
  let stopFalling = () => {};
  host.innerHTML = `<h2 class="label">Bookshelf</h2><div class="card mb-3 space-y-3" id="bs-ctl"></div><div id="bs-scene-host"></div>`;
  const ctl = host.querySelector("#bs-ctl"), sceneHost = host.querySelector("#bs-scene-host");
  const seg = (name, items) => `<div class="flex flex-wrap items-center gap-2"><span class="w-16 text-xs uppercase tracking-wide text-slate-400">${name}</span><span class="flex flex-wrap gap-1">${items}</span></div>`;

  function draw() {
    stopFalling();
    const season = pref.season === "auto" ? seasonFor() : pref.season, T = THEMES[season];
    const books = done.filter((d) => new Date(d.ms).getFullYear() === pref.year).map((d) => m.books[d.book]).filter(Boolean);
    const pages = books.reduce((n, b) => n + (b.pages || 0), 0);
    const btn = (key, v, label) => `<button type="button" data-set="${key}" data-v="${v}" class="${String(pref[key]) === String(v) ? "btn-primary" : "btn-secondary"} px-3 py-1 text-sm">${label}</button>`;
    const chk = (key, label) => `<label class="inline-flex items-center gap-2 text-sm"><input type="checkbox" data-chk="${key}" ${pref[key] ? "checked" : ""}> ${label}</label>`;
    ctl.innerHTML = `${seg("Show", btn("mode", "covers", "Covers") + btn("mode", "spines", "Spines"))}
      ${seg("Season", btn("season", "auto", `Automatic (${THEMES[seasonFor()].n})`) + Object.entries(THEMES).map(([k, t]) => btn("season", k, t.n)).join(""))}
      ${years.length > 1 ? seg("Year", years.map((y) => btn("year", y, y)).join("")) : ""}
      <div class="flex flex-wrap gap-x-4 gap-y-1"><span class="w-16 text-xs uppercase tracking-wide text-slate-400">Extras</span>
        ${T.lights ? chk("lights", esc(T.lightsName)) : ""}${chk("fall", esc(T.fall))}${chk("hang", esc(T.hangName))}${chk("props", esc(T.propsName))}</div>`;
    if (!books.length) {
      sceneHost.innerHTML = `<p class="text-sm text-slate-400">No finished books in ${pref.year} yet. They stand here once you finish them.</p>`;
      return;
    }
    const width = Math.max(240, (host.clientWidth || 360) - 80);
    const per = Math.max(2, Math.floor(width / (pref.mode === "covers" ? 103 : 41)));
    const extras = pref.props ? T.extras : [];
    let shelves = "";
    for (let s = 0; s * per < books.length; s++) {
      const ex = extras.length ? extras[s % extras.length] : [];
      const row = books.slice(s * per, s * per + per).map((b, i) => bookHTML(b, pref.mode, s * per + i) + (i === 2 && ex[0] ? `<div class="bs-extra">${P[ex[0]]()}</div>` : "")).join("");
      shelves += `${s ? '<div class="bs-gap"></div>' : ""}<div class="bs-shelf"><div class="bs-books">${row}${ex[1] ? `<div class="bs-extra">${P[ex[1]]()}</div>` : ""}</div><div class="bs-plank"></div></div>`;
    }
    const yAt = (x) => 10 + 34 * Math.sin(Math.PI * (((x % 200) + 200) % 200) / 200);
    let wire = "", bulbs = "";
    if (T.lights) {
      for (let x = -10; x <= 810; x += 8) wire += `${wire ? "L" : "M"}${x} ${yAt(x).toFixed(1)}`;
      for (let i = 0; i <= 15; i++) {
        const x = i * 53 + 10, y = yAt(x), c = T.lights[i % T.lights.length];
        bulbs += `<g class="bs-bulb" style="animation-delay:${(i * 0.37).toFixed(2)}s"><circle cx="${x}" cy="${y + 7}" r="9" fill="${c}" filter="url(#bs-g)" opacity=".85"/><rect x="${x - 1.6}" y="${y - 2}" width="3.2" height="5" fill="#10151c" opacity=".7"/><ellipse cx="${x}" cy="${y + 8}" rx="3.8" ry="5.5" fill="${c}"/></g>`;
      }
    }
    const vars = ["w1", "w2", "glow", "ink", "wood", "wood2", "wood3"].map((k) => `--bs-${k}:${T[k]}`).join(";");
    sceneHost.innerHTML = `<div class="bs-scene" style="${vars}">
      <div class="bs-wallpat" style="background-image:${T.pat};background-size:${T.patSize}"></div><div class="bs-sky">${T.sky}</div>
      ${pref.hang ? `<div class="bs-garland" style="height:90px"><svg viewBox="0 0 800 90" preserveAspectRatio="xMidYMin slice" style="overflow:visible;width:100%;height:100%">${T.hang()}</svg></div>` : ""}
      ${T.lights && pref.lights ? `<div class="bs-lights"><svg viewBox="0 0 800 70" preserveAspectRatio="xMidYMin slice"><defs><filter id="bs-g"><feGaussianBlur stdDeviation="3.5"/></filter></defs><path d="${wire}" fill="none" stroke="#10151c" stroke-width="1.6" opacity=".7"/>${bulbs}</svg></div>` : ""}
      <canvas class="bs-fall"></canvas>
      <div class="bs-title"><b>Books read in ${pref.year}</b><span>${books.length} book${books.length === 1 ? "" : "s"}${pages ? ` · ${pages.toLocaleString()} pages` : ""}</span></div>
      ${shelves}</div>`;
    if (pref.fall) stopFalling = startFalling(sceneHost.querySelector(".bs-fall"), sceneHost.querySelector(".bs-scene"), T);
  }

  // a book on the shelf opens its page (to change its cover, rate it…)
  sceneHost.addEventListener("click", (e) => {
    const b = e.target.closest("[data-open]");
    if (b) import("./bookdialog.js").then((m) => m.openBook(Number(b.dataset.open), state, () => { coverBust = Date.now(); draw(); }));
  });
  sceneHost.addEventListener("error", (e) => { // no cover image: the plain generated cover (title and author) takes its place
    const img = e.target;
    if (img.tagName !== "IMG" || !img.parentElement?.dataset.plain) return;
    img.parentElement.innerHTML = img.parentElement.dataset.plain;
  }, true);
  ctl.addEventListener("click", (e) => {
    const b = e.target.closest("[data-set]");
    if (!b) return;
    pref[b.dataset.set] = b.dataset.set === "year" ? Number(b.dataset.v) : b.dataset.v;
    save(pref);
    draw();
  });
  ctl.addEventListener("change", (e) => {
    const k = e.target.dataset.chk;
    if (!k) return;
    pref[k] = e.target.checked;
    save(pref);
    draw();
  });
  draw();
  return () => stopFalling();
}
