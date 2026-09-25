// Netflix-style "Up Next" reading queue with SortableJS drag-and-drop.
import { get, post, put, del } from "./api.js";
import { $, esc, attempt, toast, classChip } from "./ui.js";
import { on } from "./modules.js";
import { confirmKindleSend, openKOReaderSetup } from "./delivery.js";
import { renderSuggestions } from "./suggest.js";

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
    const btn = e.target.closest("[data-act]");
    if (!btn) return;
    const id = btn.closest("[data-item]").dataset.item;
    const act = btn.dataset.act;
    btn.disabled = true;
    if (act === "start") {
      // Send-to-Kindle: show who the email comes from (Amazon's approved list) first.
      const title = btn.closest("[data-item]").querySelector(".font-semibold")?.textContent || "this book";
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

function queueRow(i, idx) {
  return `
    <li data-item="${i.id}" class="card flex items-center gap-3 py-3">
      <span class="drag-handle" title="Drag to reorder">⠿</span>
      <span data-pos class="w-6 text-right text-slate-500">${idx + 1}</span>
      <div class="min-w-0 flex-1">
        <p class="truncate font-semibold">${esc(i.title)}</p>
        <p class="truncate text-sm text-slate-400">${esc(i.author)}</p>
        ${deepBanner(i)}
        ${i.owned ? "" : `<p class="text-xs text-sky-300">📦 Pending acquisition: not in your library yet</p>`}
      </div>
      <div class="hidden sm:block">${classChip(i)}</div>
      ${i.owned ? `<button data-act="start" class="btn-primary">▶ Start Reading</button>` : `<a href="#/wishlist" class="btn-ghost text-sm" title="Get a copy first">⭐ Wishlist</a>`}
      <button data-act="remove" class="btn-ghost px-2" title="Remove">✕</button>
    </li>`;
}

// "2→4": a Deep Scan found more than the blurb suggested.
const deepBanner = (i) => (i.deep_change
  ? `<p class="text-xs font-semibold text-amber-300" title="The full text was rated higher than the description">⚠️ Rating changed via Deep Scan: Level ${esc(i.deep_change.replace("→", " → Level "))}</p>` : "");

function readingCard(i) {
  return `
    <div data-item="${i.id}" class="card flex flex-col gap-2 ring-indigo-700">
      <p class="font-semibold">${esc(i.title)}</p>
      <p class="text-sm text-slate-400">${esc(i.author)}</p>
      ${deepBanner(i)}
      ${i.delivery_note ? `<p class="text-xs text-slate-500">${esc(i.delivery_note)}</p>` : ""}
      <div class="flex gap-2"><button data-act="finish" class="btn-secondary">Finished</button>
        <button data-act="remove" class="btn-ghost">Remove</button></div>
    </div>`;
}
