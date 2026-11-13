// A new event: paste the event's list (the Amazon links are kept when the
// page or email is pasted) or give its page's address (NovelCheck presses
// "Load more books" for you); NovelCheck shows what it found, then saves it
// and rates the new books first in line. openEditEvent renames one or
// changes when it ends.
import { post, patch } from "./api.js";
import { esc, attempt } from "./ui.js";

// A time as a date-and-time box shows it (the viewer's own time), and back.
const pad = (n) => String(n).padStart(2, "0");
const toBox = (iso) => {
  if (!iso) return "";
  const d = new Date(iso);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
const fromBox = (v) => (v ? new Date(v).toISOString() : "");

const endsField = (iso = "") => `<label class="block"><span class="label">Ends (optional)</span>
    <input name="ends" type="datetime-local" class="input" value="${esc(toBox(iso))}">
    <span class="mt-1 block text-xs text-slate-400">When it ends, the event moves to the archive, with its books and ratings.</span></label>`;

function dialog() {
  let d = document.getElementById("event-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "event-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

export function openNewEvent() {
  const d = dialog();
  d.innerHTML = `<form class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
    <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-semibold">New event</h2>
      <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <label class="block"><span class="label">The page's address</span>
      <input name="url" type="url" class="input" placeholder="https://…" autocomplete="off">
      <span class="mt-1 block text-xs text-slate-400">NovelCheck presses "Load more books" for you, so it finds the whole list. An Amazon wish list, or a Linktree of lists, works too.</span></label>
    <label class="block"><span class="label">…or paste the list</span>
      <textarea name="text" rows="5" class="input" placeholder="Open the event's page or email, select the book list, copy, and paste it here. The Amazon links come along."></textarea></label>
    <button type="button" data-read class="btn-secondary w-full">Read the list</button>
    <div data-found class="space-y-3"></div>
  </form>`;
  const form = d.querySelector("form");
  const found = d.querySelector("[data-found]");
  let html = "";
  let books = [];
  form.text.addEventListener("paste", (e) => {
    html = e.clipboardData?.getData("text/html") || ""; // keeps the links; the box shows the words
  });
  form.text.addEventListener("input", () => {
    if (!form.text.value.trim()) html = "";
  });
  // A page of lists (a Linktree of the day's free books): tick the ones to read.
  const showLists = (lists) => {
    found.innerHTML = `<p class="rounded-lg bg-slate-800 p-3 text-sm">This page links to <b>${lists.length} Amazon list${lists.length === 1 ? "" : "s"}</b>. Tick the ones you want in this event.</p>
      <div class="space-y-1">${lists.map((l) => `<label class="flex items-center gap-3 rounded-lg px-2 py-2 text-sm hover:bg-slate-800">
        <input type="checkbox" data-list value="${esc(l.url)}" data-name="${esc(l.name)}" ${l.picked ? "checked" : ""} class="h-5 w-5">
        <span>${esc(l.name)}</span></label>`).join("")}</div>
      <button type="button" data-read-lists class="btn-primary w-full">Read the chosen lists</button>`;
  };
  const showBooks = (r) => {
    books = r.books;
    const linked = books.filter((b) => b.asin || b.link).length;
    found.innerHTML = `<p class="rounded-lg bg-slate-800 p-3 text-sm">Found <b>${books.length} book${books.length === 1 ? "" : "s"}</b>${linked ? `, ${linked} with an Amazon link` : ""}:
      <span class="mt-1 block text-slate-400">${books.slice(0, 8).map((b) => esc(b.title)).join(" · ")}${books.length > 8 ? " …" : ""}</span></p>
      <label class="block"><span class="label">Name</span>
        <input name="name" required maxlength="100" class="input" placeholder="e.g. Stuff Your Kindle – Fall" value="${esc(r.name || "")}"></label>
      ${endsField(r.day ? new Date(`${r.day}T23:59`).toISOString() : "")}
      <p class="text-xs text-slate-400">Books NovelCheck hasn't rated yet are rated first in line, within your hourly AI limit.</p>
      <button data-save class="btn-primary w-full">Save the event</button>`;
  };
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    if (e.target.closest("[data-read]")) {
      const btn = e.target.closest("[data-read]");
      btn.disabled = true;
      btn.textContent = "Reading… (long lists take a moment)";
      const r = await attempt(() => post("/api/events/preview", { html, text: form.text.value, url: form.url.value }));
      btn.disabled = false;
      btn.textContent = "Read the list again";
      if (!r) return;
      if (r.lists?.length && !r.books.length) return showLists(r.lists);
      return showBooks(r);
    }
    if (e.target.closest("[data-read-lists]")) {
      const btn = e.target.closest("[data-read-lists]");
      const picked = [...found.querySelectorAll("input[data-list]:checked")].map((c) => ({ url: c.value, name: c.dataset.name }));
      if (!picked.length) return;
      btn.disabled = true;
      btn.textContent = `Reading ${picked.length} list${picked.length === 1 ? "" : "s"}… (about ${Math.max(5, Math.ceil(picked.length / 4) * 8)} seconds)`;
      const r = await attempt(() => post("/api/events/preview", { urls: picked.map((p) => p.url) }));
      btn.disabled = false;
      btn.textContent = "Read the chosen lists";
      if (!r) return;
      return showBooks({ ...r, name: picked.length === 1 ? r.name || picked[0].name : picked.map((p) => p.name).join(", ").slice(0, 100) });
    }
    if (e.target.closest("[data-save]")) {
      e.preventDefault();
      if (!form.name.value.trim()) return form.name.focus();
      const r = await attempt(() => post("/api/events", { name: form.name.value.trim(), source_url: form.url.value.trim(), ends_at: fromBox(form.ends.value), books }),
        "Event saved");
      if (r) {
        d.close();
        location.hash = `#/library?event=${r.id}`;
      }
    }
  };
  form.onsubmit = (e) => e.preventDefault();
  d.showModal();
}

// openEditEvent renames an event or changes when it ends.
export function openEditEvent(ev, onDone) {
  const d = dialog();
  d.innerHTML = `<form class="space-y-3 p-5">
    <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-semibold">Edit event</h2>
      <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <label class="block"><span class="label">Name</span>
      <input name="name" required maxlength="100" class="input" value="${esc(ev.name)}"></label>
    ${endsField(ev.ends_at)}
    <button class="btn-primary w-full">Save</button>
  </form>`;
  const form = d.querySelector("form");
  d.onclick = (e) => {
    if (e.target === d || e.target.closest("[data-close]")) d.close();
  };
  form.onsubmit = async (e) => {
    e.preventDefault();
    if (await attempt(() => patch(`/api/events/${ev.id}`, { name: form.name.value.trim(), ends_at: fromBox(form.ends.value) }), "Saved")) {
      d.close();
      onDone?.();
    }
  };
  d.showModal();
}
