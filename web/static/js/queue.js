// Netflix-style "Up Next" reading queue with SortableJS drag-and-drop.
import { get, post, put, del } from "./api.js";
import { $, esc, attempt, toast, classChip } from "./ui.js";
import { on } from "./modules.js";
import { confirmKindleSend, openKOReaderSetup } from "./delivery.js";
import { renderSuggestions } from "./suggest.js";
import { coverImg } from "./covers.js";
import { openBook } from "./bookdialog.js";

export async function renderQueue(view, state) {
  view.innerHTML = `
    <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
      <h1 class="text-2xl font-bold">Up Next</h1>
      <p class="text-sm text-slate-400">Delivery: <span id="delivery-mode"></span> · <a href="#/profile" class="underline">change</a></p>
    </div>
    <section class="mb-8">
      <h2 class="label">Currently Reading</h2>
      <div id="reading" class="grid gap-3 sm:grid-cols-2"></div>
    </section>
    <section>
      <h2 class="label">Queued — drag to reorder</h2>
      <ol id="queued" class="space-y-2"></ol>
    </section>
    <div id="suggestions"></div>`;
  $("#delivery-mode", view).textContent = {
    email: `Send-to-Kindle (${state.user.kindle_email})`,
    koreader: "KOReader catalog",
  }[state.user.delivery_method] || "None";
  if (state.user.delivery_method === "koreader" && on(state.user, "koreader")) {
    $("#delivery-mode", view).insertAdjacentHTML("afterend", ` · <button id="ko-setup" class="underline">KOReader setup</button>`);
    $("#ko-setup", view).addEventListener("click", () => openKOReaderSetup());
  }

  const reading = $("#reading", view);
  const queued = $("#queued", view);
  let sortable;

  async function load() {
    const items = (await attempt(() => get("/api/queue"))) || [];
    const cur = items.filter((i) => i.status === "reading");
    const next = items.filter((i) => i.status === "queued");
    reading.innerHTML = cur.length ? cur.map(readingCard).join("")
      : `<p class="text-sm text-slate-500">Nothing in progress. Press ▶ Start Reading on a queued book.</p>`;
    queued.innerHTML = next.length ? next.map(queueRow).join("")
      : `<li class="text-sm text-slate-500">Your queue is empty. Add books from the Library with ＋.</li>`;
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
      const title = btn.closest("[data-item]").querySelector("[data-title]")?.textContent || "this book";
      if (state.user.delivery_method === "email" && on(state.user, "send_to_kindle") && !(await confirmKindleSend(state.user, title))) {
        btn.disabled = false;
        return;
      }
      const r = await attempt(() => post(`/api/queue/${id}/start`));
      if (r) toast(r.delivery_note);
    } else if (act === "finish") {
      await attempt(() => post(`/api/queue/${id}/finish`), "Marked as finished");
    } else if (act === "remove") {
      await attempt(() => del(`/api/queue/${id}`), "Removed from queue");
    }
    await load();
  });

  await load();
  if (on(state.user, "suggestions")) renderSuggestions($("#suggestions", view), state, load);
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

// On phones the row is the cover, two lines of title and small ▶ / ✕
// buttons; tapping the cover or title opens the book's window.
function queueRow(i, idx) {
  return `
    <li data-item="${i.id}" data-book="${i.book_id}" class="card flex items-center gap-2 p-2 sm:gap-3 sm:p-3">
      <span class="drag-handle px-1" title="Drag to reorder" aria-label="Drag to reorder">⠿</span>
      <button type="button" data-open class="flex min-w-0 flex-1 items-center gap-2 text-left sm:gap-3" title="Book details">
        ${coverImg(i.book_id, "h-16 w-11")}
        <span class="min-w-0 flex-1 space-y-0.5">
          <span data-title class="font-semibold leading-snug line-clamp-2">${esc(i.title)}</span>
          <span class="block truncate text-xs text-slate-400"><span data-pos>${idx + 1}</span> · ${esc(i.author || "Unknown author")}</span>
          ${deepBanner(i)}
          ${i.owned ? "" : `<span class="block text-xs text-sky-300">📦 Not in your library yet</span>`}
        </span>
      </button>
      <span class="hidden shrink-0 md:block">${classChip(i)}</span>
      <span class="flex shrink-0 flex-col items-center gap-1 sm:flex-row sm:gap-2">
        ${i.owned ? `<button data-act="start" class="btn-primary h-10 w-10 p-0 sm:w-auto sm:px-4" title="Start reading" aria-label="Start reading">▶&#xFE0E;<span class="hidden sm:inline"> Start Reading</span></button>`
          : `<a href="#/wishlist" class="btn-ghost h-10 w-10 p-0 sm:w-auto sm:px-3" title="Get a copy first" aria-label="Wishlist">⭐<span class="hidden sm:inline"> Wishlist</span></a>`}
        <button data-act="remove" class="btn-ghost h-9 w-10 p-0" title="Remove from Up Next" aria-label="Remove from Up Next">✕</button>
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
        ${i.delivery_note ? `<p class="text-xs text-slate-500">${esc(i.delivery_note)}</p>` : ""}
        <div class="flex gap-2"><button data-act="finish" class="btn-secondary">Finished</button>
          <button data-act="remove" class="btn-ghost">Remove</button></div></div>
    </div>`;
}
