// "Start Reading" delivery help: the Send-to-Kindle confirmation with the
// approved-sender steps, and KOReader setup (the private OPDS feed address,
// a QR code and step-by-step instructions).
import { pluginStepsHTML } from "./pluginmanage.js";
import { get, post } from "./api.js";
import { esc, attempt } from "./ui.js";
import { copyText } from "./copy.js";
import qrcode from "/vendor/qrcode.esm.js";

function dialog() {
  let d = document.getElementById("delivery-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "delivery-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  return d;
}

// Amazon only delivers documents from addresses on the reader's approved list.
export const amazonStepsHTML = (from) => `<ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
  <li>Open <a href="https://www.amazon.com/myk" target="_blank" rel="noopener noreferrer" class="underline">amazon.com/myk</a>
    (Account &amp; Lists → Content Library → <b>Preferences</b>).</li>
  <li>Open <b>Personal Document Settings</b> and find <b>Approved Personal Document E-mail List</b>.</li>
  <li>Choose <b>Add a new approved e-mail address</b>, enter <b>${esc(from || "the sender address your admin set up")}</b> and save.</li>
</ol>`;

// confirmKindleSend asks before emailing a book, showing who it comes from.
export function confirmKindleSend(user, title) {
  const d = dialog();
  return new Promise((resolve) => {
    d.innerHTML = `<div class="space-y-3 p-5">
      <h2 class="text-lg font-bold">Send to your Kindle?</h2>
      <p class="text-sm">“${esc(title)}” goes to <b>${esc(user.kindle_email || "your Kindle address")}</b>
        from <b>${esc(user.delivery_from || "the admin's sender address")}</b>.</p>
      <p class="text-xs text-slate-400">If it never arrives, that sender isn't on your Amazon approved list yet.</p>
      <details class="rounded-lg bg-slate-800/60 p-3"><summary class="cursor-pointer text-sm font-semibold">How to add it to Amazon's approved senders</summary>
        <div class="mt-2">${amazonStepsHTML(user.delivery_from)}</div></details>
      <div class="flex justify-end gap-2"><button data-no class="btn-ghost">Cancel</button><button data-yes class="btn-primary">Send</button></div></div>`;
    const done = (v) => {
      d.close();
      resolve(v);
    };
    d.onclick = (e) => {
      if (e.target.closest("[data-yes]")) done(true);
      else if (e.target === d || e.target.closest("[data-no]")) done(false);
    };
    d.oncancel = () => resolve(false);
    d.showModal();
  });
}

function koreaderHTML(url, username, own, simple) {
  const qr = qrcode(0, "M");
  qr.addData(url);
  qr.make();
  const easy = simple ? `<div class="space-y-2 rounded-lg bg-slate-800/60 p-3 text-sm">
    <p class="font-semibold">✨ Easiest to type</p>
    <p>Address: <code class="rounded bg-slate-900 px-2 py-0.5">${esc(simple.url)}</code> <button type="button" data-ko="copy-short" class="btn-ghost py-0.5 text-xs">📋 Copy</button></p>
    <p>Username: <code class="rounded bg-slate-900 px-2 py-0.5">${esc(username)}</code> · Password: <code class="rounded bg-slate-900 px-2 py-0.5">${esc(simple.code)}</code> (the sync code below)</p>
    <p class="text-xs text-slate-400">Fill all three boxes when you add the catalog. No long address to type.</p></div>
    <p class="text-sm text-slate-400">Or use the private address below, which needs no login (scan the QR code on a phone or tablet):</p>` : "";
  return `${easy}<div class="grid gap-4 sm:grid-cols-[auto,1fr]">
    <img src="${qr.createDataURL(5, 2)}" alt="QR code of the KOReader address" class="h-40 w-40 rounded bg-white">
    <div class="min-w-0 space-y-2">
      <p class="text-sm">KOReader address for <b>${esc(username)}</b>:</p>
      <p class="break-all rounded bg-slate-800 p-2 font-mono text-xs">${esc(url)}</p>
      <div class="flex flex-wrap gap-2"><button data-ko="copy" class="btn-secondary py-1 text-sm">📋 Copy</button>
        ${own ? `<button data-ko="reset" class="btn-ghost py-1 text-sm" title="The old address stops working">New address</button>` : ""}</div>
    </div></div>
  <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
    <li>On the e-reader, open <b>KOReader</b>, tap the <b>🔍 Search</b> or <b>⚙ Settings</b> icon in the top menu (it's in one of them), then <b>OPDS catalog</b>.</li>
    <li>Tap <b>＋</b> (top left). Name: <b>NovelCheck</b>. Then either the short address with your username and sync code (the three boxes in <b>Easiest to type</b>), or the long private address with no login.</li>
    <li>Open <b>NovelCheck</b> in the list: the Up Next books appear. Tap one and choose <b>Download</b>.</li>
  </ol>
  <p class="text-xs text-slate-500">The e-reader must reach this address: the home Wi-Fi address works at home, the https:// remote address works anywhere.
    Keep it private: anyone with it can download these books.</p>`;
}

// openKOReaderSetup shows the address for the signed-in person (no userId)
// or for a kid's device (userId, parents only).
export async function openKOReaderSetup(userId) {
  const syncURL = userId ? `/api/admin/users/${userId}/kosync` : "/api/me/kosync";
  const [info, sync] = await Promise.all([attempt(() => get(userId ? `/api/admin/users/${userId}/opds` : "/api/me/opds")),
    get(syncURL).catch(() => null)]);
  if (!info) return;
  const d = dialog();
  const render = (i, s) => {
    const url = location.origin + i.path;
    d.innerHTML = `<div class="max-h-[85vh] space-y-4 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">📖 Set up KOReader</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>${koreaderHTML(url, i.username, !userId, s ? { url: location.origin + "/opds", code: s.code } : null)}
      ${s ? syncHTML(location.origin + s.path, s.username, s.code) + statsHTML(location.origin + s.dav, s.username, s.code) + pluginHTML(userId) : ""}</div>`;
    d.onclick = async (e) => {
      const ko = e.target.closest("[data-ko]")?.dataset.ko;
      if (e.target === d || e.target.closest("[data-close]")) d.close();
      else if (ko === "copy") copyText(url);
      else if (ko === "copy-short") copyText(location.origin + "/opds");
      else if (ko === "copy-sync") copyText(location.origin + s.path);
      else if (ko === "copy-dav") copyText(location.origin + s.dav);
      else if (ko === "reset" && confirm("Make a new address? KOReader will need the new one.")) {
        const n = await attempt(() => post("/api/me/opds/reset"), "New address made");
        if (n) render(n, s);
      } else if (ko === "new-code" && confirm("Make a new sync code? KOReader has to sign in again with it.")) {
        const n = await attempt(() => post(syncURL), "New sync code made");
        if (n) render(i, n);
      }
    };
  };
  render(info, sync);
  d.showModal();
}

// syncHTML: KOReader's progress sync pointed at NovelCheck, so Up Next
// follows what's being read.
function syncHTML(url, username, code) {
  return `<div class="space-y-2 border-t border-slate-800 pt-4">
    <h3 class="font-semibold">🔄 Reading sync</h3>
    <p class="text-sm text-slate-400">KOReader tells NovelCheck how far each book is: opening one marks it ▶ Reading in Up Next, and reaching the end marks it ✓ Finished. It also keeps your place between devices. It works for books from this catalog.</p>
    <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
      <li>In a book, open the top menu → <b>🛠 Tools</b> → <b>Progress sync</b> → <b>Custom sync server</b>, and enter:
        <span class="mt-1 flex items-center gap-2"><code class="min-w-0 break-all rounded bg-slate-800 px-2 py-1 text-xs">${esc(url)}</code>
        <button type="button" data-ko="copy-sync" class="btn-ghost shrink-0 py-1 text-xs">📋 Copy</button></span></li>
      <li>Choose <b>Register / Login</b>: username <b>${esc(username)}</b>, password <code class="rounded bg-slate-800 px-2 py-0.5">${esc(code)}</code>, then <b>Login</b>.</li>
      <li>Turn on <b>Auto sync</b>. Leave <b>Document matching method</b> on <b>Binary</b>.</li>
    </ol>
    <button type="button" data-ko="new-code" class="btn-ghost py-1 text-xs">Make a new sync code</button></div>`;
}

// statsHTML: KOReader's reading statistics synced to a private NovelCheck
// folder, so Up Next shows every book opened on the reader's devices.
function statsHTML(url, username, code) {
  return `<div class="space-y-2 border-t border-slate-800 pt-4">
    <h3 class="font-semibold">📊 Reading statistics</h3>
    <p class="rounded bg-slate-800/60 p-2 text-sm text-slate-300">Using the NovelCheck plugin? Skip these steps: <b>NovelCheck → Set up everything</b> does them for you.</p>
    <p class="text-sm text-slate-400">NovelCheck can also keep KOReader's reading statistics: then Up Next shows every book opened on your KOReader devices (any book, not only ones from NovelCheck), how far you are and how long you've read. With several devices, each one syncs to the same place.</p>
    <p class="text-sm font-medium">Part 1: add NovelCheck as Cloud storage (once per device)</p>
    <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
      <li>In KOReader's file browser, tap the <b>⚙ Settings</b> icon in the top menu, then <b>Cloud storage</b>, then <b>＋</b>, then <b>WebDAV</b>.</li>
      <li>Fill the five boxes in this order:
        <ul class="mt-1 list-disc space-y-1 pl-5">
          <li><b>Name:</b> NovelCheck</li>
          <li><b>Address:</b> <code class="break-all rounded bg-slate-800 px-2 py-0.5 text-xs">${esc(url)}</code> <button type="button" data-ko="copy-dav" class="btn-ghost py-0.5 text-xs">📋 Copy</button></li>
          <li><b>Username:</b> <code class="rounded bg-slate-800 px-2 py-0.5">${esc(username)}</code></li>
          <li><b>Password:</b> <code class="rounded bg-slate-800 px-2 py-0.5">${esc(code)}</code> (your sync code)</li>
          <li><b>Folder:</b> just <code class="rounded bg-slate-800 px-2 py-0.5">/</code> (one slash, nothing else)</li>
        </ul></li>
      <li>Tap <b>Save</b>, then tap <b>NovelCheck</b> in the list. An empty list is normal.</li>
    </ol>
    <p class="pt-1 text-sm font-medium">Part 2: tell Reading statistics to use it</p>
    <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300" start="4">
      <li>Open any book, then the top menu → <b>🛠 Tools</b> → <b>Reading statistics</b> → <b>Settings</b> → <b>Cloud sync</b>, and pick <b>NovelCheck</b>.</li>
      <li>In the folder list, <b>press and hold for 2 seconds</b> on the bold line <b>Long-press here to choose current folder</b>. A box <b>Choose this folder?</b> appears: tap <b>Choose</b>. Don't open folders named <b>dav</b>.</li>
      <li>Then <b>Reading statistics</b> → <b>Synchronize now</b>. Repeat it whenever you want NovelCheck to catch up.</li>
    </ol>
    <p class="text-xs text-slate-500">Not working? Profile → Reading sync lists the latest requests and why a sign-in failed. <b>🧹 Remove empty folders</b> there clears leftovers.</p>
    <p class="text-xs text-slate-500">Menu names can differ a little between KOReader versions.</p></div>`;
}

// pluginHTML: the KOReader plugin, already connected to this person's account.
function pluginHTML(userId) {
  return `<div class="space-y-2 border-t border-slate-800 pt-4"><h3 class="font-semibold">🧩 KOReader plugin</h3>
    ${pluginStepsHTML(userId ? `/api/admin/users/${userId}/koplugin.zip` : "/api/me/koplugin.zip")}</div>`;
}
