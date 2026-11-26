// Home (the start screen, 2.0): what is next for this person, new books, their asks and the answers, and for parents the asks waiting
// for an answer; for admins also how healthy NovelCheck is and the inbox of things that need a decision.
import { get, post, put } from "./api.js";
import { esc, attempt } from "./ui.js";

const LEVEL = { ok: "text-emerald-300", warn: "text-amber-300", bad: "text-rose-300" };
const WORDS = { pending: "waiting for a parent", approved: "approved", declined: "not this time", done: "ready" };

const coverTile = (b) => `<a href="#/library?book=${b.id}" class="block min-w-0" title="${esc(b.title)}">
  <img class="aspect-[2/3] w-full rounded object-cover" loading="lazy" alt="" src="/api/books/${b.id}/cover">
  <span class="mt-1 block truncate text-xs">${esc(b.title)}</span></a>`;

function healthHTML(h) {
  if (!h) return "";
  const calm = h.level === "ok";
  return `<section class="card space-y-2"><div class="flex items-center justify-between gap-2">
    <h2 class="font-semibold ${LEVEL[h.level]}">${calm ? "Everything is working" : h.level === "bad" ? "Something needs fixing now" : "A few things need a look"}</h2>
    <a href="#/health" class="btn-ghost py-1 text-sm">Health</a></div>
    ${h.items.map((i) => `<p class="text-sm"><b class="${LEVEL[i.level]}">${esc(i.title)}</b> <span class="text-slate-400">${esc(i.fix || i.detail)}</span> ${i.link ? `<a href="${esc(i.link)}" class="underline">Go</a>` : ""}</p>`).join("")}</section>`;
}

// "Getting started": what a new NovelCheck needs, in order, until it is all done (an admin can hide it for a week).
function setupHTML(steps) {
  if (!steps?.length) return "";
  const done = steps.filter((x) => x.done).length;
  return `<section class="card space-y-2"><div class="flex items-center justify-between gap-2"><h2 class="font-semibold">Getting started <span class="text-sm font-normal text-slate-400">${done} of ${steps.length} done</span></h2>
    <button type="button" data-snooze="setup" class="btn-ghost py-1 text-sm" title="Hide it for a week">Hide</button></div>
    <ul class="space-y-2">${steps.map((x) => `<li class="flex items-start gap-3 text-sm"><span class="${x.done ? "text-emerald-300" : "text-slate-500"}" aria-hidden="true">${x.done ? "✓" : "○"}</span>
      <span class="min-w-0 flex-1"><b>${esc(x.title)}</b>${x.optional ? ` <span class="text-xs text-slate-500">(optional)</span>` : ""}${x.done ? "" : `<span class="block text-slate-400">${esc(x.detail)}</span>`}</span>
      ${x.done ? "" : `<a href="${esc(x.link)}" class="btn-secondary shrink-0 py-1 text-sm">Do it</a>`}</li>`).join("")}</ul></section>`;
}

// Autopilot: the sure Hardcover fixes written into Calibre by themselves (off until turned on; Calibre has no undo).
function autopilotHTML(a) {
  if (!a) return "";
  return a.on
    ? `<section class="card flex items-center justify-between gap-3 text-sm"><span><b class="text-emerald-300">Autopilot is on.</b> <span class="text-slate-400">The sure fixes from Hardcover are written into Calibre by themselves${a.waiting ? ` (${a.waiting} waiting for the next round)` : ""}. ${esc(a.last || "")}</span></span>
      <button type="button" data-autopilot="off" class="btn-ghost shrink-0 py-1 text-sm">Turn off</button></section>`
    : `<section class="card space-y-2"><h2 class="font-semibold">Autopilot is off</h2>
      <p class="text-sm text-slate-400">Turn it on and NovelCheck writes the sure fixes into Calibre by itself, every half hour${a.waiting ? `: ${a.waiting} are waiting now` : ""}. “Sure” means titles, authors, series, ISBNs and descriptions Hardcover matched by ISBN or the exact title and author. Genres, series numbers, covers and anything doubtful stay on this page for you to decide. Calibre has no undo for these writes.</p>
      <button type="button" data-autopilot="on" class="btn-primary py-1 text-sm">Turn on Autopilot</button></section>`;
}

// How far the AI's spice levels can be trusted: how often it matched a person's own level on books they checked.
function accuracyHTML(a) {
  if (!a) return "";
  return `<section class="card flex items-center justify-between gap-3 text-sm"><span><b>How accurate are the ratings?</b> <span class="text-slate-400">On the ${a.n} books people checked, the AI matched their level exactly ${a.exact}% of the time and was within one level ${a.within1}%. Use your own judgment on the ones marked Not sure.</span></span>
    <a href="#/accuracy" class="btn-ghost shrink-0 py-1 text-sm">Details</a></section>`;
}

function askedHTML(list) {
  if (!list.length) return "";
  return `<section class="card space-y-2"><h2 class="font-semibold">Asks waiting for you</h2>${list.map((q) => `<div class="rounded-lg border border-slate-700 p-3" data-req="${q.id}">
    <p><b>${esc(q.title)}</b> <span class="text-slate-400">${esc(q.author)}</span></p>
    <p class="text-xs text-slate-400">${esc(q.user)} asked${q.note ? `: “${esc(q.note)}”` : ""}${q.book_id ? " · it is in the library" : " · not in the library yet"}</p>
    <div class="mt-2 flex flex-wrap gap-2">${q.status === "pending"
      ? `<button type="button" data-do="approve" class="btn-primary py-1 text-sm">${q.book_id ? "Yes, put it on their Nightstand" : "Yes"}</button><button type="button" data-do="decline" class="btn-secondary py-1 text-sm">Not this time</button>`
      : `<button type="button" data-do="done" class="btn-primary py-1 text-sm">Got it: it's in the library now</button>`}</div></div>`).join("")}
    <p class="text-xs text-slate-500">Yes marks the book OK for kids' accounts, the same as “Mark as OK” on its page.</p></section>`;
}

function inboxHTML(items) {
  if (!items.length) return "";
  return `<section class="card space-y-2"><h2 class="font-semibold">Needs a decision</h2>${items.map((c) => `<div class="flex items-center justify-between gap-3 rounded-lg border border-slate-800 p-3">
    <div class="min-w-0"><p><b>${c.count}</b> ${esc(c.title)}</p><p class="text-xs text-slate-400">${esc(c.detail)}</p></div>
    <div class="flex shrink-0 gap-2"><a href="${esc(c.link)}" class="btn-primary py-1 text-sm">Review</a><button type="button" data-snooze="${esc(c.id)}" class="btn-ghost py-1 text-sm" title="Hide it for a week">Not now</button></div></div>`).join("")}</section>`;
}

// Quick actions (admins): the three things most often done by hand.
const QUICK = {
  covers: ["/api/admin/covers/find", "The cover finder is looking now"],
  calibre: ["/api/admin/calibre-sync?wait=1", "Calibre was read again"],
  backup: ["/api/admin/backups/now", "A backup of NovelCheck's data was made"],
};
const quickHTML = (d) => (d.health ? `<section class="card space-y-2"><h2 class="font-semibold">Quick actions</h2><div class="flex flex-wrap gap-2">
  <button type="button" data-quick="covers" class="btn-secondary py-1 text-sm">Find covers now</button>
  <button type="button" data-quick="calibre" class="btn-secondary py-1 text-sm">Read Calibre again</button>
  <button type="button" data-quick="backup" class="btn-secondary py-1 text-sm">Back up now</button></div></section>` : "");

const askFormHTML =`<section class="card space-y-2"><h2 class="font-semibold">Ask for a book</h2>
  <form id="ask" class="grid gap-2 sm:grid-cols-2"><input name="title" class="input" placeholder="Title" aria-label="Title" maxlength="200" required>
    <input name="author" class="input" placeholder="Author (if you know)" aria-label="Author" maxlength="200">
    <input name="note" class="input sm:col-span-2" placeholder="Why this one? (optional)" aria-label="Why" maxlength="500">
    <button class="btn-primary sm:col-span-2">Ask</button></form></section>`;

function mineHTML(list) {
  if (!list.length) return "";
  return `<section class="card space-y-1"><h2 class="font-semibold">Your asks</h2>${list.map((q) => `<p class="text-sm"><b>${esc(q.title)}</b> · ${WORDS[q.status] || q.status}${q.answer ? ` <span class="text-slate-400">${esc(q.answer)}</span>` : ""}</p>`).join("")}</section>`;
}

export async function renderHome(view) {
  view.innerHTML = `<h1 class="page-title mb-3">Home</h1><div id="home" class="max-w-4xl space-y-4"><p class="text-slate-400">Loading…</p></div>`;
  const draw = async () => {
    const d = await attempt(() => get("/api/home"));
    if (!d) return;
    const next = d.next_up.length ? `<section class="card space-y-2"><div class="flex items-center justify-between"><h2 class="font-semibold">Next on your Nightstand</h2><a href="#/queue" class="btn-ghost py-1 text-sm">All</a></div>
      <div class="grid grid-cols-4 gap-2 sm:grid-cols-6 lg:grid-cols-8">${d.next_up.map(coverTile).join("")}</div></section>` : "";
    const fresh = d.new_books.length ? `<section class="card space-y-2"><div class="flex items-center justify-between"><h2 class="font-semibold">New in the library</h2><a href="#/library" class="btn-ghost py-1 text-sm">Library</a></div>
      <div class="grid grid-cols-4 gap-2 sm:grid-cols-6 lg:grid-cols-8">${d.new_books.map(coverTile).join("")}</div></section>` : "";
    const lost = d.unmatched ? `<section class="card text-sm"><b>${d.unmatched}</b> book${d.unmatched === 1 ? "" : "s"} on your e-reader couldn't be matched. <a href="#/settings" class="underline">Match them</a></section>` : "";
    const add = d.may_upload ? `<section class="card text-sm">Add books by dragging them onto the box at the top of the <a href="#/library" class="underline">Library</a>.</section>` : "";
    const host = view.querySelector("#home");
    host.innerHTML = [setupHTML(d.setup), healthHTML(d.health), autopilotHTML(d.autopilot), accuracyHTML(d.accuracy), askedHTML(d.requests || []), inboxHTML(d.inbox || []), quickHTML(d), next, fresh, mineHTML(d.my_requests), askFormHTML, lost, add].join("") || "";
    const want = new URLSearchParams(location.hash.split("?")[1] || "").get("ask"); // from "Ask a parent" in the Library
    if (want) host.querySelector("#ask").title.value = want;
    host.querySelector("#ask").onsubmit = async (e) => {
      e.preventDefault();
      const f = e.target;
      if (await attempt(() => post("/api/requests", { title: f.title.value, author: f.author.value, note: f.note.value }), "Asked. A parent will answer.")) draw();
    };
    host.onclick = async (e) => {
      const sn = e.target.closest("[data-snooze]");
      if (sn && await attempt(() => post("/api/home/snooze", { id: sn.dataset.snooze }), "Hidden for a week")) return draw();
      const ap = e.target.closest("[data-autopilot]");
      if (ap) {
        ap.disabled = true;
        if (await attempt(() => put("/api/admin/autopilot", { on: ap.dataset.autopilot === "on" }), ap.dataset.autopilot === "on" ? "Autopilot is on" : "Autopilot is off")) return draw();
        ap.disabled = false;
        return;
      }
      const quick = e.target.closest("[data-quick]");
      if (quick) {
        const [path, said] = QUICK[quick.dataset.quick];
        quick.disabled = true;
        await attempt(() => post(path), said);
        quick.disabled = false;
        return;
      }
      const act = e.target.closest("[data-do]");
      if (!act) return;
      const id = act.closest("[data-req]").dataset.req;
      act.disabled = true;
      const r = await attempt(() => post(`/api/requests/${id}/decide`, { action: act.dataset.do }), act.dataset.do === "decline" ? "Answered" : "Done");
      if (r) draw(); else act.disabled = false;
    };
  };
  await draw();
}
