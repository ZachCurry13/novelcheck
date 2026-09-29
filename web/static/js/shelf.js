// 📚 Paper books: add the family's printed books to a physical library
// ("Living room shelf") by scanning barcodes one after another, typing a
// title or ISBN, or taking a photo of the cover. Each book shows its rating
// as soon as the AI has one.
import { get, post, del } from "./api.js";
import { $, esc, attempt, toast, classChip } from "./ui.js";
import { coverImg } from "./covers.js";
import { openBook } from "./bookdialog.js";
import { loadContent, contentIcons } from "./content.js";
import { scanContinuous, liveBlocker, cameraError } from "./barcode.js";
import { shrinkPhoto, barcodeInPhoto } from "./photo.js";
import { pickBook } from "./bookchoices.js";

const PICK_KEY = "nc:shelf";
const remember = (id) => {
  try {
    localStorage.setItem(PICK_KEY, String(id));
  } catch {
    /* private mode */
  }
};
const remembered = () => {
  try {
    return localStorage.getItem(PICK_KEY) || "";
  } catch {
    return "";
  }
};

export async function renderShelf(view, state) {
  view.innerHTML = `
    <h1 class="mb-1 text-2xl font-bold">📚 Paper books</h1>
    <p class="mb-4 text-sm text-slate-400">Add the printed books on your shelves: scan the barcodes one after another, type a title or ISBN,
      or take a photo of the cover. Each book is rated as it's added, and kids see it under their usual rules.</p>
    <div class="card mb-4 space-y-3">
      <label class="block"><span class="label">Add to</span><select id="shelf" class="input"></select></label>
      <div id="new-shelf" class="hidden gap-2"><input id="new-name" class="input min-w-0 flex-1" placeholder="e.g. Living room shelf" maxlength="80">
        <button id="create" type="button" class="btn-secondary">Create</button></div>
      <div class="grid grid-cols-2 gap-2">
        <button id="scan" type="button" class="btn-primary">▦ Scan barcodes</button>
        <button id="snap" type="button" class="btn-secondary">📸 Cover photo</button>
      </div>
      <input id="photo" type="file" accept="image/*" capture="environment" class="hidden">
      <form id="typed" class="flex gap-2"><input name="q" class="input min-w-0 flex-1" placeholder="Title, author or ISBN" autocomplete="off" enterkeyhint="done">
        <button class="btn-secondary">Add</button></form>
    </div>
    <section id="camera" class="card mb-4 hidden space-y-2">
      <video class="max-h-[50vh] w-full rounded-lg bg-black object-cover" playsinline muted></video>
      <p id="cam-msg" class="text-sm text-slate-400">Hold each barcode steady, about a hand's width away. Books are added as they're read.</p>
      <button id="stop" type="button" class="btn-secondary w-full">Done scanning</button>
    </section>
    <div class="mb-2 flex items-baseline justify-between gap-2">
      <h2 class="text-lg font-semibold">Added now <span id="added-count" class="text-sm font-normal text-slate-500"></span></h2>
      <a id="see-all" href="#/library" class="text-sm underline">See the library</a></div>
    <ul id="added" class="space-y-2"></ul>`;

  await loadContent();
  const pick = $("#shelf", view);
  const list = $("#added", view);
  const added = []; // {book, fresh, shelf} newest first
  const seen = new Set(); // ISBNs and titles added on this visit
  let cam = null;
  let chain = Promise.resolve();
  const shelfId = () => (pick.value && pick.value !== "new" ? pick.value : "");
  const shelfName = () => pick.selectedOptions[0]?.dataset.name || "";

  async function loadShelves(select = "") {
    const cats = ((await attempt(() => get("/api/catalogs"))) || []).filter((c) => c.physical);
    pick.innerHTML = cats.map((c) => `<option value="${c.id}" data-name="${esc(c.name)}">${esc(c.name)} (${c.book_count})</option>`).join("")
      + `<option value="new">＋ New paper library…</option>`;
    const want = String(select || remembered());
    pick.value = cats.some((c) => String(c.id) === want) ? want : cats.length ? String(cats[0].id) : "new";
    shelfChanged();
  }
  function shelfChanged() {
    const isNew = pick.value === "new";
    $("#new-shelf", view).classList.toggle("hidden", !isNew);
    $("#new-shelf", view).classList.toggle("flex", isNew);
    if (isNew) $("#new-name", view).focus();
    else remember(pick.value);
    $("#see-all", view).href = shelfId() ? `#/library?catalog=${shelfId()}` : "#/library";
  }
  pick.addEventListener("change", shelfChanged);
  $("#create", view).addEventListener("click", async () => {
    const name = $("#new-name", view).value.trim();
    if (!name) return $("#new-name", view).focus();
    const r = await attempt(() => post("/api/shelves", { name }), `Created ${name}`);
    if (r) {
      $("#new-name", view).value = "";
      loadShelves(r.id);
    }
  });

  const paint = () => {
    $("#added-count", view).textContent = added.length ? `(${added.length})` : "";
    list.innerHTML = added.map(({ book: b, fresh, shelf }) => `<li class="card flex items-center gap-3 p-2" data-book="${b.id}" data-shelf="${shelf}">
      <button type="button" data-open class="shrink-0" title="Book details">${coverImg(b.id, "h-16 w-11")}</button>
      <button type="button" data-open class="min-w-0 flex-1 space-y-1 text-left" title="Book details">
        <span class="font-semibold leading-snug line-clamp-2">${esc(b.title)}</span>
        <span class="block truncate text-xs text-slate-400">${esc(b.author || "Unknown author")}${fresh ? "" : " · was already there"}</span>
        <span class="flex flex-wrap gap-1">${classChip(b)} ${contentIcons(b)}</span>
      </button>
      <button type="button" data-undo class="btn-ghost h-9 w-9 shrink-0 p-0" title="Take it off this library" aria-label="Take it off this library">✕</button>
    </li>`).join("") || `<li class="text-sm text-slate-500">Nothing yet.</li>`;
  };

  async function add(body, key) {
    const shelf = shelfId();
    if (!shelf) {
      toast("Pick or create a paper library first", true);
      return;
    }
    if (key && seen.has(key)) {
      toast("Already added");
      return;
    }
    const r = await attempt(() => post(`/api/shelves/${shelf}/books`, body));
    if (!r) return;
    if (r.choices) { // not sure which book was meant: ask
      const pick = await pickBook(r.choices, r.query);
      return pick ? add({ ...body, pick }, key) : undefined;
    }
    if (key) seen.add(key);
    navigator.vibrate?.(40);
    added.unshift({ book: r.book, fresh: r.added, shelf });
    paint();
    toast(r.added ? `Added: ${r.book.title}` : `Already in ${shelfName()}: ${r.book.title}`);
  }
  // One lookup at a time, in the order the books were scanned.
  const queue = (body, key) => (chain = chain.then(() => add(body, key)).catch(() => {}));

  $("#typed", view).addEventListener("submit", (e) => {
    e.preventDefault();
    const q = e.target.q.value.trim();
    if (!q) return;
    e.target.q.value = "";
    queue({ query: q }, q.toLowerCase());
  });
  $("#snap", view).addEventListener("click", () => $("#photo", view).click());
  $("#photo", view).addEventListener("change", async (e) => {
    const file = e.target.files[0];
    e.target.value = "";
    if (!file) return;
    const isbn = await barcodeInPhoto(file);
    if (isbn) return queue({ query: isbn }, isbn);
    const image = await shrinkPhoto(file).catch(() => "");
    if (!image) return toast("That photo couldn't be opened; please take it again", true);
    toast("Reading the cover…");
    queue({ image });
  });

  const stopCamera = () => {
    cam?.stop();
    cam = null;
    $("#camera", view).classList.add("hidden");
  };
  $("#scan", view).addEventListener("click", async () => {
    const why = liveBlocker();
    if (why) return toast(why, true);
    if (!shelfId()) return toast("Pick or create a paper library first", true);
    stopCamera();
    $("#camera", view).classList.remove("hidden");
    $("#camera", view).scrollIntoView({ block: "start", behavior: "smooth" });
    const msg = $("#cam-msg", view);
    cam = scanContinuous($("#camera video", view), (isbn) => {
      msg.textContent = `Looking up ${isbn}…`;
      queue({ query: isbn }, isbn).then(() => (msg.textContent = "Ready for the next book."));
    });
    try {
      await cam.ready;
    } catch (err) {
      cam?.stop();
      msg.className = "text-sm text-rose-300";
      msg.innerHTML = cameraError(err);
    }
  });
  $("#stop", view).addEventListener("click", stopCamera);

  list.addEventListener("click", async (e) => {
    const li = e.target.closest("[data-book]");
    if (!li) return;
    if (e.target.closest("[data-open]")) return openBook(Number(li.dataset.book), state);
    if (e.target.closest("[data-undo]")) {
      const b = added.find((a) => String(a.book.id) === li.dataset.book);
      if (await attempt(() => del(`/api/catalogs/${li.dataset.shelf}/books/${li.dataset.book}`), `Took ${b?.book.title || "it"} off`)) {
        added.splice(added.indexOf(b), 1);
        paint();
      }
    }
  });

  // Ratings arrive one by one: refresh the books still waiting.
  const poll = setInterval(async () => {
    const waiting = added.filter((a) => !["analyzed", "error"].includes(a.book.status)).slice(0, 8);
    if (!waiting.length || !document.body.contains(view)) return;
    let changed = false;
    for (const a of waiting) {
      const d = await get(`/api/books/${a.book.id}`).catch(() => null);
      if (d && d.book.status !== a.book.status) {
        a.book = d.book;
        changed = true;
      }
    }
    if (changed) paint();
  }, 5000);

  await loadShelves();
  paint();
  return () => {
    stopCamera();
    clearInterval(poll);
  };
}
