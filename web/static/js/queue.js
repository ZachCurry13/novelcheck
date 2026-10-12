// Netflix-style "Nightstand" reading queue with SortableJS drag-and-drop.
import { get, post, put, del } from "./api.js";
import { $, esc, attempt, toast, classChip } from "./ui.js";
import { on } from "./modules.js";
import { confirmKindleSend, openKOReaderSetup } from "./delivery.js";
import { renderSuggestions } from "./suggest.js";
import { coverImg } from "./covers.js";
import { openBook } from "./bookdialog.js";
import { progressBar, progressText, pct, ago, setBars } from "./progress.js";
import { renderKOReaderBooks } from "./koreaderbooks.js";

export async function renderQueue(view, state) {
  view.innerHTML = `
    <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
      <h1 class="text-2xl font-bold">Nightstand</h1>
      <p class="text-sm text-slate-400">Delivery: <span id="delivery-mode"></span> · <a href="#/settings" class="underline">change</a></p>
    </div>
    <p id="nightstand-intro" class="mb-4 hidden rounded-lg bg-slate-800/60 p-3 text-sm text-slate-300">🛏️ <b>Your Nightstand</b> (it used to be called Up Next): what you're reading now,
      then the pile you plan to read next. Books you open on your e-reader show up here by themselves.
      <button type="button" data-intro-ok class="ml-1 underline">Got it</button></p>
    <section class="mb-8">
      <h2 class="label">Currently Reading</h2>
      <div id="reading" class="grid gap-3 sm:grid-cols-2"></div>
      <div id="hidden" class="mt-2 text-sm"></div>
    </section>
    <section>
      <h2 class="label">On the pile — drag to reorder</h2>
      <ol id="queued" class="space-y-2"></ol>
    </section>
    <section id="koreader-books"></section>
    <div id="suggestions"></div>`;
  $("#delivery-mode", view).textContent = {
    email: `Send-to-Kindle (${state.user.kindle_email})`,
    koreader: "KOReader catalog",
  }[state.user.delivery_method] || "None";
  if (state.user.delivery_method === "koreader" && on(state.user, "koreader")) {
    $("#delivery-mode", view).insertAdjacentHTML("afterend", ` · <button id="ko-setup" class="underline">KOReader setup</button>`);
    $("#ko-setup", view).addEventListener("click", () => openKOReaderSetup());
  }

  const intro = $("#nightstand-intro", view);
  try {
    if (!localStorage.getItem("nc:nightstand-intro")) intro.classList.remove("hidden");
  } catch {
    /* private mode: just show it */
    intro.classList.remove("hidden");
  }
  intro.querySelector("[data-intro-ok]").onclick = () => {
    intro.classList.add("hidden");
    try {
      localStorage.setItem("nc:nightstand-intro", "1");
    } catch {
      /* private mode: it comes back next time */
    }
  };
  const reading = $("#reading", view);
  const hiddenEl = $("#hidden", view);
  const queued = $("#queued", view);
  let sortable;

  async function load() {
    const items = (await attempt(() => get("/api/queue"))) || [];
    const cur = items.filter((i) => i.status === "reading");
    const next = items.filter((i) => i.status === "queued");
    reading.innerHTML = cur.length ? cur.map(readingCard).join("")
      : `<p class="text-sm text-slate-500">Nothing in progress. Press ▶ Start Reading on a book from the pile.</p>`;
    queued.innerHTML = next.length ? next.map((i, idx) => queueRow(i, idx, state.user)).join("")
      : `<li class="text-sm text-slate-500">Your pile is empty. Add books from the Library with ＋.</li>`;
    setBars(reading);
    const hid = (await attempt(() => get("/api/queue?hidden=1"))) || [];
    hiddenEl.innerHTML = hid.length ? `<details class="rounded-lg bg-slate-800/40 p-2"><summary class="cursor-pointer text-slate-400">🙈 Hidden (${hid.length})</summary>
      <ul class="mt-2 space-y-1">${hid.map((i) => `<li data-item="${i.id}" data-book="${i.book_id}" class="flex items-center justify-between gap-2">
        <span class="min-w-0 truncate">${esc(i.title)}</span><button data-act="unhide" class="btn-ghost shrink-0 py-1 text-xs">Show again</button></li>`).join("")}</ul></details>` : "";
  }

  async function saveOrder() {
    const rows = [...queued.querySelectorAll("[data-item]")];
    rows.forEach((li, i) => (li.querySelector("[data-pos]").textContent = i + 1));
    await attempt(() => put("/api/queue/order", { ids: rows.map((li) => Number(li.dataset.item)) }));
  }

  view.addEventListener("click", async (e) => {
    const open = e.target.closest("[data-open]");
    if (open && open.closest("[data-item]")) return openBook(Number(open.closest("[data-item]").dataset.book), state, load);
    const btn = e.target.closest("[data-act]");
    if (!btn || !btn.closest("[data-item]")) return;
    const id = btn.closest("[data-item]").dataset.item;
    const act = btn.dataset.act;
    btn.disabled = true;
    if (act === "start") {
      // Send-to-Kindle: show who the email comes from (Amazon's approved list) first.
      const row = btn.closest("[data-item]");
      const title = row.querySelector("[data-title]")?.textContent || "this book";
      const sends = row.dataset.file === "send"; // nothing to email: it's just marked as reading
      if (sends && state.user.delivery_method === "email" && on(state.user, "send_to_kindle") && !(await confirmKindleSend(state.user, title))) {
        btn.disabled = false;
        return;
      }
      const r = await attempt(() => post(`/api/queue/${id}/start`));
      if (r) toast(r.delivery_note);
    } else if (act === "finish") {
      await attempt(() => post(`/api/queue/${id}/finish`), "Marked as finished");
    } else if (act === "hide") {
      await attempt(() => post(`/api/queue/${id}/hide`, {}), "Hidden from your Nightstand");
    } else if (act === "unhide") {
      await attempt(() => post(`/api/queue/${id}/hide`, { hidden: false }), "Back on your Nightstand");
    } else if (act === "remove") {
      await attempt(() => del(`/api/queue/${id}`), "Removed from queue");
    }
    await load();
  });

  await load();
  if (on(state.user, "koreader")) renderKOReaderBooks($("#koreader-books", view), state, load);
  if (on(state.user, "suggestions")) {
    // A suggestion added to your Nightstand grows the list above; keep the suggestions
    // where they were on screen (iPhones don't do this by themselves).
    const host = $("#suggestions", view);
    renderSuggestions(host, state, async () => {
      const before = host.getBoundingClientRect().top;
      await load();
      const moved = host.getBoundingClientRect().top - before;
      if (moved) window.scrollBy(0, moved);
    });
  }
  if (window.Sortable) {
    sortable = window.Sortable.create(queued, {
      handle: ".drag-handle",
      animation: 150,
      ghostClass: "sortable-ghost",
      onEnd: saveOrder,
    });
  }
  return () => sortable?.destroy();
}

// noFile says, before ▶ is pressed, that a book has nothing to deliver (a
// paper book, one from an imported list, AZW3/MOBI only), so ▶ only marks it
// as reading.
function noFile(i, user) {
  const method = user.delivery_method;
  if (!i.owned || i.file === "send" || !["email", "koreader"].includes(method)) return "";
  const why = i.file === "other" ? "AZW3/MOBI only, which Amazon won't take" : `No ${method === "email" ? "EPUB or PDF" : "file"} to send`;
  return `<span class="block text-xs text-slate-400" title="▶ marks it as reading without sending anything">${i.file === "other" ? "🔄" : "📕"} ${why}: ▶ marks it as reading</span>`;
}

// On phones the row is the cover, two lines of title and small ▶ / ✕
// buttons; tapping the cover or title opens the book's window.
function queueRow(i, idx, user) {
  return `
    <li data-item="${i.id}" data-book="${i.book_id}" data-file="${esc(i.file || "none")}" class="card flex items-center gap-2 p-2 sm:gap-3 sm:p-3">
      <span class="drag-handle px-1" title="Drag to reorder" aria-label="Drag to reorder">⠿</span>
      <button type="button" data-open class="flex min-w-0 flex-1 items-center gap-2 text-left sm:gap-3" title="Book details">
        ${coverImg(i.book_id, "h-16 w-11")}
        <span class="min-w-0 flex-1 space-y-0.5">
          <span data-title class="font-semibold leading-snug line-clamp-2">${esc(i.title)}</span>
          <span class="block truncate text-xs text-slate-400"><span data-pos>${idx + 1}</span> · ${esc(i.author || "Unknown author")}</span>
          ${deepBanner(i)}
          ${i.progress ? `<span class="block text-xs text-indigo-300">📖 ${pct(i.progress)}% read · ${ago(i.progress.at)}</span>` : ""}
          ${i.owned ? noFile(i, user) : `<span class="block text-xs text-sky-300">📦 Not in your library yet</span>`}
        </span>
      </button>
      <span class="hidden shrink-0 md:block">${classChip(i)}</span>
      <span class="flex shrink-0 flex-col items-center gap-1 sm:flex-row sm:gap-2">
        ${i.owned ? `<button data-act="start" class="btn-primary h-10 w-10 p-0 sm:w-auto sm:px-4" title="Start reading" aria-label="Start reading">▶&#xFE0E;<span class="hidden sm:inline"> Start Reading</span></button>`
          : `<a href="#/wishlist" class="btn-ghost h-10 w-10 p-0 sm:w-auto sm:px-3" title="Get a copy first" aria-label="Wishlist">⭐<span class="hidden sm:inline"> Wishlist</span></a>`}
        <button data-act="hide" class="btn-ghost h-9 w-10 p-0" title="Hide from your Nightstand (stays in your library)" aria-label="Hide from your Nightstand">🙈</button>
        <button data-act="remove" class="btn-ghost h-9 w-10 p-0" title="Remove from your Nightstand" aria-label="Remove from your Nightstand">✕</button>
      </span>
    </li>`;
}

// "2→4": a Deep Scan found more than the blurb suggested.
const deepBanner = (i) => (i.deep_change
  ? `<span class="block text-xs font-semibold text-amber-300" title="The full text was rated higher than the description">⚠️ Deep Scan: Level ${esc(i.deep_change.replace("→", " → "))}</span>` : "");

function readingCard(i) {
  return `
    <div data-item="${i.id}" data-book="${i.book_id}" class="card flex gap-3 ring-indigo-700">
      <button type="button" data-open class="shrink-0" title="Book details">${coverImg(i.book_id, "h-24 w-16")}</button>
      <div class="flex min-w-0 flex-1 flex-col gap-2">
        <button type="button" data-open class="text-left" title="Book details">
          <span data-title class="block font-semibold leading-snug">${esc(i.title)}</span>
          <span class="block text-sm text-slate-400">${esc(i.author || "Unknown author")}</span>
        </button>
        ${deepBanner(i)}
        <p class="text-xs font-semibold text-indigo-300">▶ Currently reading</p>
        ${i.progress ? `<div class="space-y-1">${progressBar(i.progress)}<p class="text-xs text-slate-400">📖 ${progressText(i.progress)}</p></div>`
          : i.delivery_note ? `<p class="text-xs text-slate-500">${esc(i.delivery_note)}</p>` : ""}
        <div class="flex flex-wrap gap-2"><button data-act="finish" class="btn-secondary">Finished</button>
          <button data-act="hide" class="btn-ghost whitespace-nowrap" title="Hide from your Nightstand (stays in your library)">🙈 Hide</button>
          <button data-act="remove" class="btn-ghost">Remove</button></div></div>
    </div>`;
}
