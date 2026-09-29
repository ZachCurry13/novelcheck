// A new event: paste the event's list (the Amazon links are kept when the
// page or email is pasted) or give its page's address; NovelCheck shows what
// it found, then saves it and rates the new books first in line.
import { post } from "./api.js";
import { esc, attempt } from "./ui.js";

export function openNewEvent() {
  let d = document.getElementById("event-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "event-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  d.innerHTML = `<form class="max-h-[85vh] space-y-3 overflow-y-auto p-5">
    <div class="flex items-center justify-between gap-3"><h2 class="text-lg font-bold">🎉 New event</h2>
      <button type="button" data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <input name="name" required maxlength="100" class="input" placeholder="e.g. Stuff Your Kindle – Fall">
    <label class="block"><span class="label">Paste the list</span>
      <textarea name="text" rows="6" class="input" placeholder="Open the event's page or email, select the book list, copy, and paste it here. The Amazon links come along."></textarea></label>
    <label class="block"><span class="label">…or the page's address</span>
      <input name="url" type="url" class="input" placeholder="https://…" autocomplete="off"></label>
    <button type="button" data-read class="btn-secondary w-full">Read the list</button>
    <div data-found class="space-y-2"></div>
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
  d.onclick = async (e) => {
    if (e.target === d || e.target.closest("[data-close]")) return d.close();
    if (e.target.closest("[data-read]")) {
      const r = await attempt(() => post("/api/events/preview", { html, text: form.text.value, url: form.url.value }));
      if (!r) return;
      books = r.books;
      const linked = books.filter((b) => b.asin || b.link).length;
      found.innerHTML = `<p class="rounded-lg bg-slate-800 p-3 text-sm">Found <b>${books.length} book${books.length === 1 ? "" : "s"}</b>${linked ? `, ${linked} with an Amazon link` : ""}:
        <span class="mt-1 block text-slate-400">${books.slice(0, 8).map((b) => esc(b.title)).join(" · ")}${books.length > 8 ? " …" : ""}</span></p>
        <p class="text-xs text-slate-400">Books NovelCheck hasn't rated yet are rated first in line, within your hourly AI limit.</p>
        <button data-save class="btn-primary w-full">Save the event</button>`;
      return;
    }
    if (e.target.closest("[data-save]")) {
      e.preventDefault();
      if (!form.name.value.trim()) return form.name.focus();
      const r = await attempt(() => post("/api/events", { name: form.name.value.trim(), source_url: form.url.value.trim(), books }),
        "Event saved");
      if (r) {
        d.close();
        location.hash = `#/events?id=${r.id}`;
      }
    }
  };
  form.onsubmit = (e) => e.preventDefault();
  d.showModal();
}
