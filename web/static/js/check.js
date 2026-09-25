// Check a book: the main screen for parents. Snap the cover (or type the
// title) in a shop and see the rating. Known books answer at once; new ones
// are looked up, rated by the AI, and kept under "Looked up".
import { get, post, qs } from "./api.js";
import { $, esc, attempt, classChip, flagChips, ageChip } from "./ui.js";
import { PEPPERS, whyChip, openPepperGuide } from "./peppers.js";
import { loadFlags, customChips } from "./customflags.js";
import { openBook } from "./bookdialog.js";

export async function renderCheck(view, state) {
  view.innerHTML = `
    <div class="mx-auto max-w-xl space-y-4">
      <div>
        <h1 class="text-2xl font-bold">Check a book</h1>
        <p class="text-sm text-slate-400">At the shop? Snap the cover, or type the title.</p>
      </div>
      <button id="snap" class="btn-primary w-full py-4 text-lg">📷 Take a photo of the cover</button>
      <button id="pick" class="btn-ghost w-full text-sm">🖼 Or choose a photo you already took</button>
      <input id="photo" type="file" accept="image/*" capture="environment" class="hidden">
      <input id="library-photo" type="file" accept="image/*" class="hidden">
      <form id="check-form" class="flex gap-2">
        <input name="q" type="search" enterkeyhint="search" placeholder="Title, author or ISBN" class="input min-w-0 flex-1" autocomplete="off">
        <button class="btn-secondary">Check</button>
      </form>
      <div id="check-result" aria-live="polite"></div>
      <section id="recent"></section>
    </div>`;
  await loadFlags(true);
  const out = $("#check-result", view);
  const form = $("#check-form", view);
  let run = 0; // a newer check cancels the polling of an older one

  const check = async (body, working) => {
    const mine = ++run;
    out.innerHTML = status(working);
    let res;
    try {
      res = await post("/api/check", body);
    } catch (e) {
      out.innerHTML = `<p class="card text-sm text-rose-300">${esc(e.message)}</p>`;
      form.q.focus();
      return;
    }
    const found = `Found <b>${esc(res.book.title)}</b>${res.book.author ? ` by ${esc(res.book.author)}` : ""}.`;
    let book = res.book;
    if (res.rating) {
      out.innerHTML = status(`${found} Rating it now… this usually takes 10–30 seconds.`);
      for (let i = 0; i < 90 && mine === run; i++) { // up to ~3 minutes
        await new Promise((r) => setTimeout(r, 2000));
        const d = await get(`/api/books/${book.id}`).catch(() => null);
        if (d && d.book.status !== "processing") {
          book = d.book;
          break;
        }
      }
      if (mine !== run) return;
    }
    out.innerHTML = resultCard(book, res);
    loadRecent(view, state);
  };

  $("#snap", view).addEventListener("click", () => $("#photo", view).click());
  $("#pick", view).addEventListener("click", () => $("#library-photo", view).click());
  const onPhoto = async (e) => {
    const file = e.target.files[0];
    e.target.value = "";
    if (!file) return;
    const isbn = await barcode(file); // Android can read the barcode on the back, free and instant
    if (isbn) return check({ query: isbn }, "Reading the barcode…");
    const image = await shrink(file).catch(() => "");
    if (!image) {
      out.innerHTML = `<p class="card text-sm text-rose-300">That photo couldn't be opened (some phones save photos in a format the browser can't read).
        Try <b>Choose a photo</b>, take the picture again, or type the title below.</p>`;
      return;
    }
    check({ image }, "📷 Reading the cover…");
  };
  $("#photo", view).addEventListener("change", onPhoto);
  $("#library-photo", view).addEventListener("change", onPhoto);
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    const q = form.q.value.trim();
    if (q) check({ query: q }, "Looking it up…");
  });
  out.addEventListener("click", (e) => {
    const act = e.target.closest("[data-act]")?.dataset.act;
    const id = Number(e.target.closest("[data-id]")?.dataset.id);
    if (act === "open") openBook(id, state);
    else if (act === "peppers") openPepperGuide();
    else if (act === "retry") check({ query: e.target.dataset.q }, "Trying again…");
    else if (act === "another") {
      out.innerHTML = "";
      form.q.value = "";
      form.q.focus();
    }
  });
  loadRecent(view, state);
}

const status = (text) => `<p class="card flex items-center gap-3 text-sm text-slate-300"><span class="animate-pulse text-xl">⏳</span><span>${text}</span></p>`;

// The verdict first, big and plain; then the details.
function resultCard(b, res) {
  const where = res.in_library ? `<p class="text-sm text-emerald-300">✓ In your library: ${esc(res.catalogs.join(", "))}</p>`
    : `<p class="text-sm text-slate-400">Not in your library. It's saved under <b>Looked up</b>, so checking it again is instant.</p>`;
  if (b.status === "error" || b.spice_level === null || b.spice_level === undefined) {
    const why = b.status === "error" ? `Rating failed: ${esc(b.analysis_error)}` : b.status === "processing" ? "The AI is still working on it." : "It hasn't been rated yet.";
    return `<div class="card space-y-3" data-id="${b.id}">
      <h2 class="text-xl font-bold">${esc(b.title)}</h2><p class="text-sm text-slate-400">${esc(b.author || "")}</p>
      <p class="text-sm text-amber-200">${why}</p>${where}
      <div class="flex flex-wrap gap-2"><button data-act="retry" data-q="${esc(`${b.title} ${b.author || ""}`.trim())}" class="btn-primary">Try again</button>
        <button data-act="another" class="btn-ghost">Check another</button></div></div>`;
  }
  const p = PEPPERS[b.spice_level];
  // Green up to Level 2 (the Strict Family Preset's cap), amber at 3, red when explicit.
  const tone = b.spice_level <= 2 ? ["bg-emerald-950/60 text-emerald-200", "✓"]
    : b.spice_level === 3 ? ["bg-amber-950/60 text-amber-200", "⚠"] : ["bg-rose-950/60 text-rose-200", "✕"];
  const verdict = [tone[0], `${tone[1]} Level ${b.spice_level}: ${p.name}`];
  return `<div class="card space-y-3" data-id="${b.id}">
    <div><h2 class="text-xl font-bold">${esc(b.title)}</h2><p class="text-sm text-slate-400">${esc(b.author || "Unknown author")}</p></div>
    <p class="rounded-lg p-3 text-lg font-semibold ${verdict[0]}">${verdict[1]}</p>
    <div class="flex flex-wrap gap-1">${classChip(b)} ${whyChip(b)} ${ageChip(b)} ${flagChips(b)} ${customChips(b)}</div>
    ${b.spice_reason ? `<p class="text-sm"><b>Why:</b> ${esc(b.spice_reason)}</p>` : ""}
    ${b.summary_verdict ? `<p class="rounded-lg bg-slate-800 p-3 text-slate-200">${esc(b.summary_verdict)}</p>` : ""}
    <p class="text-xs text-slate-400">${esc(p.name)}: ${esc(p.desc)} <button data-act="peppers" class="underline">About peppers</button></p>
    ${where}
    <div class="flex flex-wrap gap-2"><button data-act="open" class="btn-secondary">Open details</button>
      <button data-act="another" class="btn-ghost">Check another</button></div>
  </div>`;
}

// The last few books checked that aren't in the library.
async function loadRecent(view, state) {
  const host = $("#recent", view);
  const cat = ((await attempt(() => get("/api/catalogs"))) || []).find((c) => c.name === "Looked up");
  if (!host || !cat) return;
  const data = await attempt(() => get("/api/books" + qs({ catalog: cat.id, sort: "recent", limit: 8 })));
  if (!data?.books.length) return;
  host.innerHTML = `<h2 class="mb-2 mt-4 text-sm font-semibold uppercase tracking-wide text-slate-400">Recently checked</h2>
    <ul class="space-y-2">${data.books.map((b) => `<li><button data-open="${b.id}" class="card flex w-full items-center justify-between gap-3 text-left">
      <span class="min-w-0"><span class="block truncate font-semibold">${esc(b.title)}</span><span class="block truncate text-xs text-slate-400">${esc(b.author || "")}</span></span>
      <span class="shrink-0">${classChip(b)}</span></button></li>`).join("")}</ul>`;
  host.onclick = (e) => {
    const id = e.target.closest("[data-open]")?.dataset.open;
    if (id) openBook(Number(id), state);
  };
}

// shrink turns a phone photo into a ~1280 px JPEG data: URL (small and quick to send).
async function shrink(file) {
  const bmp = await createImageBitmap(file);
  const scale = Math.min(1, 1280 / Math.max(bmp.width, bmp.height));
  const canvas = document.createElement("canvas");
  canvas.width = Math.round(bmp.width * scale);
  canvas.height = Math.round(bmp.height * scale);
  canvas.getContext("2d").drawImage(bmp, 0, 0, canvas.width, canvas.height);
  return canvas.toDataURL("image/jpeg", 0.85);
}

// barcode reads an ISBN barcode where the browser can (Chrome on Android).
async function barcode(file) {
  if (!("BarcodeDetector" in window)) return "";
  try {
    const codes = await new window.BarcodeDetector({ formats: ["ean_13"] }).detect(await createImageBitmap(file));
    return codes.map((c) => c.rawValue).find((v) => /^97[89]\d{10}$/.test(v)) || "";
  } catch {
    return "";
  }
}
