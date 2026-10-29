// 📤 Share card: your reading as a picture (a story-sized or square image, like
// a year-in-review), drawn in this browser from your stats. Pick a look, save
// it or share it from the phone. Nothing is uploaded anywhere.
import { esc } from "./ui.js";
import { byDay, byHourOfDay, fmtDur, streaks } from "./statscalc.js";
import { count, summary } from "./statsperiods.js";

const THEMES = {
  midnight: { label: "Midnight", bg: ["#1e1b4b", "#312e81", "#0f172a"], accent: "#a5b4fc", text: "#ffffff", panel: "rgba(255,255,255,0.10)" },
  sunrise: { label: "Sunrise", bg: ["#7c2d12", "#be185d", "#f59e0b"], accent: "#fde68a", text: "#ffffff", panel: "rgba(255,255,255,0.14)" },
  forest: { label: "Forest", bg: ["#064e3b", "#047857", "#134e4a"], accent: "#a7f3d0", text: "#ffffff", panel: "rgba(255,255,255,0.12)" },
};
const FORMATS = { story: ["Story (1080 × 1920)", 1080, 1920], square: ["Square (1080 × 1080)", 1080, 1080] };
const FONT = `system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;
const n = (x) => Number(x).toLocaleString();
const partOfDay = (h) => (h >= 5 && h < 12 ? "morning" : h >= 12 && h < 17 ? "afternoon" : h >= 17 && h < 22 ? "evening" : "night");
const cap = (s) => s.replace(/\b\w/g, (c) => c.toUpperCase());

// data collects what the card says from the prepared stats.
function data(m, s, title) {
  const sum = summary(m, s.period, s.offset);
  const books = sum.books.map((b) => ({ ...m.books[b.book], sec: b.sec }));
  const genre = count(books, (b) => b.genres || [])[0]?.label;
  const author = count(books, (b) => b.author || "")[0];
  const hours = byHourOfDay(sum.rows);
  return { title, books, sum, genre: genre && cap(genre), author: author && author.n > 0 ? author.label : "", streak: streaks(byDay(m.rows)).current,
    part: sum.sec ? partOfDay(hours.indexOf(Math.max(...hours))) : "" };
}

async function loadCover(b) {
  if (!b.book_id) return null;
  try {
    const r = await fetch(`/api/books/${b.book_id}/cover?size=l`);
    return r.ok ? await createImageBitmap(await r.blob()) : null;
  } catch {
    return null;
  }
}

function wrap(ctx, text, maxW) {
  const lines = [];
  let line = "";
  for (const w of text.split(" ")) {
    const t = line ? `${line} ${w}` : w;
    if (line && ctx.measureText(t).width > maxW) {
      lines.push(line);
      line = w;
    } else line = t;
  }
  return line ? [...lines, line] : lines;
}

function roundRect(ctx, x, y, w, h, r) {
  ctx.beginPath();
  ctx.roundRect(x, y, w, h, r);
}

// draw paints the card on the canvas.
function draw(canvas, d, o, covers) {
  const [, W, H] = FORMATS[o.format];
  const th = THEMES[o.theme];
  canvas.width = W;
  canvas.height = H;
  const ctx = canvas.getContext("2d");
  const g = ctx.createLinearGradient(0, 0, W * 0.4, H);
  th.bg.forEach((c, i) => g.addColorStop(i / (th.bg.length - 1), c));
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, W, H);
  ctx.fillStyle = "rgba(255,255,255,0.06)";
  ctx.beginPath();
  ctx.arc(W * 0.9, H * 0.12, W * 0.36, 0, Math.PI * 2);
  ctx.fill();

  const story = o.format === "story", M = 80;
  ctx.textBaseline = "top";
  ctx.fillStyle = th.accent;
  ctx.font = `700 ${story ? 38 : 32}px ${FONT}`;
  ctx.fillText("📖 NovelCheck", M, story ? 110 : 70);

  ctx.fillStyle = th.text;
  ctx.font = `800 ${story ? 88 : 66}px ${FONT}`;
  const titleLines = wrap(ctx, d.title, W - 2 * M);
  let y = story ? 200 : 140;
  for (const l of titleLines.slice(0, 3)) {
    ctx.fillText(l, M, y);
    y += story ? 100 : 76;
  }

  const stats = [[n(d.books.length), d.books.length === 1 ? "book" : "books"], [n(d.sum.pages), "pages"], [fmtDur(d.sum.sec), "of reading"]];
  y += story ? 40 : 24;
  if (story) {
    for (const [num, label] of stats) {
      ctx.fillStyle = th.text;
      ctx.font = `800 150px ${FONT}`;
      ctx.fillText(num, M, y);
      ctx.fillStyle = th.accent;
      ctx.font = `600 44px ${FONT}`;
      ctx.fillText(label, M, y + 150);
      y += 215;
    }
  } else {
    const colW = (W - 2 * M) / 3;
    stats.forEach(([num, label], i) => {
      ctx.fillStyle = th.text;
      ctx.font = `800 ${num.length > 6 ? 64 : 84}px ${FONT}`;
      ctx.fillText(num, M + i * colW, y);
      ctx.fillStyle = th.accent;
      ctx.font = `600 32px ${FONT}`;
      ctx.fillText(label, M + i * colW, y + 100);
    });
    y += 170;
  }

  const facts = [d.genre && `Top genre: ${d.genre}`, d.author && `Most read: ${d.author}`, d.part && `You read most in the ${d.part}`, d.streak > 1 && `🔥 ${d.streak}-day streak`].filter(Boolean);
  const shown = covers.filter(Boolean);
  if (o.covers && shown.length) {
    const per = story ? 4 : 5, cw = story ? 210 : 150, ch = cw * 1.5, gap = story ? 30 : 25;
    ctx.fillStyle = th.text;
    ctx.font = `600 ${story ? 40 : 30}px ${FONT}`;
    for (const f of facts.slice(0, story ? 2 : 1)) {
      ctx.fillText(f, M, y);
      y += story ? 56 : 46;
    }
    y += story ? 30 : 16;
    const rows = story ? 2 : 1;
    const startX = M + (W - 2 * M - (Math.min(per, shown.length) * cw + (Math.min(per, shown.length) - 1) * gap)) / 2;
    shown.slice(0, per * rows).forEach((img, i) => {
      const x = startX + (i % per) * (cw + gap), yy = y + Math.floor(i / per) * (ch + gap);
      ctx.save();
      roundRect(ctx, x, yy, cw, ch, 12);
      ctx.clip();
      const k = Math.max(cw / img.width, ch / img.height);
      ctx.drawImage(img, x + (cw - img.width * k) / 2, yy + (ch - img.height * k) / 2, img.width * k, img.height * k);
      ctx.restore();
    });
  } else {
    ctx.fillStyle = th.text;
    ctx.font = `600 ${story ? 46 : 36}px ${FONT}`;
    for (const f of facts.slice(0, story ? 4 : 2)) {
      roundRect(ctx, M, y - 10, W - 2 * M, story ? 90 : 70, 18);
      ctx.fillStyle = th.panel;
      ctx.fill();
      ctx.fillStyle = th.text;
      ctx.fillText(f, M + 30, y + (story ? 8 : 6));
      y += story ? 120 : 90;
    }
  }
  ctx.fillStyle = th.accent;
  ctx.font = `600 28px ${FONT}`;
  ctx.fillText("Made with NovelCheck", M, H - 80);
}

export async function openShareCard(m, s, title, name) {
  let d = document.getElementById("share-card-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "share-card-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  const info = data(m, s, title);
  const o = { format: "story", theme: "midnight", covers: true };
  const covers = await Promise.all(info.books.filter((b) => b.book_id).slice(0, 8).map(loadCover));
  const select = (key, opts) => `<select data-opt="${key}" class="input min-w-0 flex-1">${Object.entries(opts).map(([k, v]) => `<option value="${k}" ${o[key] === k ? "selected" : ""}>${esc(Array.isArray(v) ? v[0] : v.label)}</option>`).join("")}</select>`;
  d.innerHTML = `<div class="max-h-[90vh] space-y-3 overflow-y-auto p-5">
    <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">📤 Share your reading</h2>
      <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    <canvas class="mx-auto max-h-[55vh] w-auto max-w-full rounded-lg" aria-label="Preview of your share card"></canvas>
    <div class="flex flex-wrap gap-2">${select("format", FORMATS)}${select("theme", THEMES)}</div>
    <label class="toggle min-h-[2.5rem] text-sm"><input type="checkbox" data-opt="covers" checked> Show book covers</label>
    <p class="text-xs text-slate-500">It's drawn here on your device from the period you're looking at, and goes nowhere until you save or share it.</p>
    <div class="flex flex-wrap gap-2"><button type="button" data-save class="btn-primary">⬇ Save image</button>
      <button type="button" data-share class="btn-secondary">📤 Share…</button></div></div>`;
  const canvas = d.querySelector("canvas");
  const paint = () => draw(canvas, info, o, covers);
  const file = () => new Promise((res) => canvas.toBlob((b) => res(new File([b], `novelcheck-${(name || "reading").replace(/\W+/g, "-")}-${o.format}.png`, { type: "image/png" })), "image/png"));
  d.onchange = (e) => {
    const k = e.target.dataset.opt;
    if (k) {
      o[k] = k === "covers" ? e.target.checked : e.target.value;
      paint();
    }
  };
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    if (e.target.closest("[data-save]")) {
      const f = await file();
      const a = document.createElement("a");
      a.href = URL.createObjectURL(f);
      a.download = f.name;
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 10000);
    } else if (e.target.closest("[data-share]")) {
      const f = await file();
      if (navigator.canShare?.({ files: [f] })) navigator.share({ files: [f], title: info.title }).catch(() => {});
      else d.querySelector("[data-save]").click(); // this browser can't share files: save it instead
    }
  };
  paint();
  d.showModal();
}
