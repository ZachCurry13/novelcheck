// 🧭 Discover: best sellers, classics and the family's own new and popular
// books, in rows to swipe through. Everyone's content rules apply; kids never
// see books the AI hasn't rated yet (the server leaves them out).
import { get, post } from "./api.js";
import { $, esc, attempt, toast, canManage } from "./ui.js";
import { openBook } from "./bookdialog.js";
import { coverImg } from "./covers.js";
import { PEPPERS, pepperIcons } from "./peppers.js";
import { loadContent, contentIcons } from "./content.js";
import { cardBlurb } from "./blurb.js";
import { on } from "./modules.js";
import { seasonChipsHTML, bindSeasonChips } from "./seasonchips.js";
import { discoverTabsHTML } from "./discovertabs.js";

const GUIDE_URL = "https://github.com/ZachCurry13/novelcheck/blob/main/docs/DISCOVER.md";
const NYT_URL = "https://www.nytimes.com/books/best-sellers/";
const NYT_CREDIT = `<a href="${NYT_URL}" target="_blank" rel="noopener noreferrer" class="underline">Data provided by The New York Times</a>`;
const NYT_LISTS = {
  "nyt:combined-print-and-e-book-fiction": "Combined Print & E-Book Fiction",
  "nyt:combined-print-and-e-book-nonfiction": "Combined Print & E-Book Nonfiction",
  "nyt:young-adult-hardcover": "Young Adult Hardcover",
  "nyt:childrens-middle-grade-hardcover": "Children's Middle Grade Hardcover",
  "nyt:picture-books": "Picture Books",
};

// The last Discover this person saw, shown at once while the fresh one loads.
const cacheKey = (user) => `nc:discover:${user.id}:${showOwned() ? 1 : 0}`;
function cached(user) {
  try {
    return JSON.parse(localStorage.getItem(cacheKey(user)) || "null");
  } catch {
    return null;
  }
}
function remember(user, data) {
  try {
    localStorage.setItem(cacheKey(user), JSON.stringify(data));
  } catch {
    /* private mode or full: just no head start next time */
  }
}

// Discover is for finding books: the lists leave out the ones the family
// already has, unless this device asked to see them.
const OWNED_KEY = "nc:discover-owned";
function showOwned() {
  try {
    return localStorage.getItem(OWNED_KEY) === "1";
  } catch {
    return false; // private mode
  }
}

export async function renderDiscover(view, state) {
  view.innerHTML = `
    ${discoverTabsHTML("discover", state.user)}
    <h1 class="mb-1 text-2xl font-bold">🧭 Discover</h1>
    <p class="mb-2 text-sm text-slate-400">Popular and classic books with their peppers and content. Tap a book for details.</p>
    <div id="discover-seasons"></div>
    <label class="toggle mb-4 min-h-[2.5rem]"><input type="checkbox" id="show-owned" ${showOwned() ? "checked" : ""}> Also show books we already have <span id="owned-count" class="text-slate-500"></span></label>
    <div id="rows" class="space-y-6"><p class="text-slate-400">Loading…</p></div>
    <div id="credit" class="mt-8 space-y-1 text-xs text-slate-500"></div>`;
  $("#show-owned", view).addEventListener("change", (e) => {
    try {
      localStorage.setItem(OWNED_KEY, e.target.checked ? "1" : "0");
    } catch {
      /* private mode: this visit only */
    }
    renderDiscover(view, state);
  });
  const kid = state.user.role === "restricted";
  const queueOn = on(state.user, "queue");
  const rows = $("#rows", view);
  let shown = "";
  const paint = (data) => {
    const json = JSON.stringify(data);
    if (json === shown) return; // unchanged: keep where each row was scrolled to
    shown = json;
    const scrolled = Object.fromEntries([...rows.querySelectorAll("[data-row]")].map((r) => [r.dataset.row, r.scrollLeft]));
    $("#owned-count", view).textContent = data.owned ? `(${data.owned} hidden)` : "";
    rows.innerHTML = data.rows.length ? data.rows.map((r) => `
    <section>
      <h2 class="mb-2 text-lg font-semibold">${r.icon} ${esc(r.title)} <span class="text-sm font-normal text-slate-500">${r.books.length}</span></h2>
      ${r.books.some((b) => b.list?.startsWith("nyt:")) ? `<p class="-mt-1 mb-2 text-xs text-slate-500">From The New York Times Best Sellers lists · ${NYT_CREDIT}</p>` : ""}
      <div data-row="${esc(r.key)}" class="flex snap-x gap-3 overflow-x-auto pb-2">${r.books.map((b) => card(b, kid, queueOn)).join("")}</div>
    </section>`).join("") : `<p class="text-slate-400">${emptyText(data, state.user)}</p>`;
    rows.querySelectorAll("[data-row]").forEach((r) => (r.scrollLeft = scrolled[r.dataset.row] || 0));
    credit(data);
  };
  rows.onclick = async (e) => {
    if (e.target.closest("a")) return; // "Get it" links open normally
    const cardEl = e.target.closest("[data-book]");
    if (!cardEl) return;
    const id = cardEl.dataset.book;
    const btn = e.target.closest("[data-act]");
    if (!btn) return openBook(id, state, () => renderDiscover(view, state));
    btn.disabled = true;
    const ok = btn.dataset.act === "wish"
      ? await attempt(() => post(`/api/books/${id}/wish`, {}), "Added to the wishlist")
      : await attempt(() => post("/api/queue", { book_id: Number(id) }), "Added to Up Next");
    btn.textContent = ok ? (btn.dataset.act === "wish" ? "⭐ On wishlist" : "✓ In Up Next") : btn.textContent;
    btn.disabled = Boolean(ok);
  };
  const fresh = Promise.all([attempt(() => get("/api/discover" + (showOwned() ? "?owned=1" : ""))), get("/api/collections").catch(() => null)]);
  await loadContent(); // the content icons (fetched once, then kept)
  const old = cached(state.user);
  if (old) paint(old);
  const [data, shelves] = await fresh;
  if (shelves) {
    $("#discover-seasons", view).innerHTML = seasonChipsHTML(shelves.seasons);
    bindSeasonChips($("#discover-seasons", view));
  }
  if (!data) return;
  remember(state.user, data);
  paint(data);

  function credit(data) {
    $("#credit", view).innerHTML = [
      data.nyt ? `Best-seller lists: ${NYT_CREDIT}.` : "",
      "Classics, covers and book pages: Open Library.",
      !data.has_key && canManage(state.user) ? `For real best-seller lists (adults, teens, kids), add a free New York Times key in Admin → Delivery &amp; Services → Discover (<a href="${GUIDE_URL}" target="_blank" rel="noopener noreferrer" class="underline">how</a>).` : "",
    ].filter(Boolean).map((t) => `<p>${t}</p>`).join("");
  }
}

function emptyText(data, user) {
  if (data.refreshing) return "Fetching the book lists… this takes about a minute.";
  if (user.role === "restricted") return "Nothing here yet. Books show up once they've been rated.";
  return canManage(user) ? "No lists yet. They load a few minutes after NovelCheck starts, or press Refresh in Admin → Delivery &amp; Services → Discover."
    : "No lists yet. Check back later.";
}

function card(b, kid, queueOn) {
  const lvl = b.spice_level;
  const rated = b.status === "analyzed" && lvl !== null && lvl !== undefined;
  const pepper = rated ? `<span class="chip-none" title="Level ${lvl}: ${esc(PEPPERS[lvl].name)}">${pepperIcons(lvl)}</span>`
    : `<span class="chip-pending" title="The AI rates a few dozen Discover books a day">Not rated yet</span>`;
  const q = encodeURIComponent(`${b.title} ${b.author || ""}`.trim());
  const olLink = b.link || (b.isbn ? `https://openlibrary.org/isbn/${encodeURIComponent(b.isbn)}` : `https://openlibrary.org/search?q=${q}`);
  let action = "";
  if (b.owned && queueOn) {
    action = b.queued ? `<span class="text-xs text-slate-400">✓ In Up Next</span>` : `<button data-act="queue" class="btn-secondary px-2 py-1 text-xs">＋ Up Next</button>`;
  } else if (!b.owned) {
    action = b.wished ? `<span class="text-xs text-slate-400">⭐ On wishlist</span>` : `<button data-act="wish" class="btn-secondary px-2 py-1 text-xs">⭐ Wishlist</button>`;
  }
  return `<article data-book="${b.id}" class="card flex w-44 shrink-0 snap-start cursor-pointer flex-col gap-2 p-3">
    <div class="relative">${coverImg(b.id, "h-56 w-full")}
      ${b.list?.startsWith("nyt:") && b.rank ? `<span class="absolute left-1 top-1 rounded bg-slate-950/80 px-1.5 text-xs font-bold">#${b.rank}</span>` : ""}</div>
    <h3 class="line-clamp-2 text-sm font-semibold leading-tight">${esc(b.title)}</h3>
    <p class="truncate text-xs text-slate-400">${esc(b.author || "")}</p>
    <div class="flex flex-wrap gap-1 text-xs">${pepper} ${contentIcons(b)}
      ${b.owned ? `<span class="chip-cat">📚 Yours</span>` : ""}${nytChip(b)}${b.weeks_on_list === 1 ? ` <span class="chip-open">New</span>` : ""}</div>
    <p class="line-clamp-4 text-xs text-slate-300">${esc(cardBlurb(b, 200))}</p>
    <div class="mt-auto space-y-1">${action}
      <p class="flex gap-3 text-xs">${kid ? "" : `<a href="https://www.amazon.com/s?k=${q}&i=digital-text" target="_blank" rel="noopener noreferrer" class="underline">Amazon</a>`}
        <a href="${esc(olLink)}" target="_blank" rel="noopener noreferrer" class="underline">Open Library</a></p></div>
  </article>`;
}

// nytChip marks a book on a New York Times Best Sellers list (the data's
// source, credited as the New York Times asks).
function nytChip(b) {
  if (!b.list?.startsWith("nyt:")) return "";
  const weeks = b.weeks_on_list > 1 ? ` · ${b.weeks_on_list} weeks` : "";
  const list = NYT_LISTS[b.list] || "Best Sellers";
  return ` <span class="chip-cat" title="On The New York Times Best Sellers list (${esc(list)})${b.rank ? `, #${b.rank}` : ""}${weeks}. Data provided by The New York Times.">📰 NYT list${weeks}</span>`;
}
