// "Start Reading" delivery help: the Send-to-Kindle confirmation with the
// approved-sender steps, and KOReader setup (the private OPDS feed address,
// a QR code and step-by-step instructions).
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

function koreaderHTML(url, username, own) {
  const qr = qrcode(0, "M");
  qr.addData(url);
  qr.make();
  return `<div class="grid gap-4 sm:grid-cols-[auto,1fr]">
    <img src="${qr.createDataURL(5, 2)}" alt="QR code of the KOReader address" class="h-40 w-40 rounded bg-white">
    <div class="min-w-0 space-y-2">
      <p class="text-sm">KOReader address for <b>${esc(username)}</b>:</p>
      <p class="break-all rounded bg-slate-800 p-2 font-mono text-xs">${esc(url)}</p>
      <div class="flex flex-wrap gap-2"><button data-ko="copy" class="btn-secondary py-1 text-sm">📋 Copy</button>
        ${own ? `<button data-ko="reset" class="btn-ghost py-1 text-sm" title="The old address stops working">New address</button>` : ""}</div>
    </div></div>
  <ol class="list-decimal space-y-1 pl-5 text-sm text-slate-300">
    <li>On the e-reader, open <b>KOReader</b>, tap the <b>🔍 search</b> icon in the top menu, then <b>OPDS catalog</b>.</li>
    <li>Tap <b>＋</b> (top left). Name: <b>NovelCheck</b>. URL: the address above (type it, or scan the QR code on a phone or tablet).</li>
    <li>Open <b>NovelCheck</b> in the list: the Up Next books appear. Tap one and choose <b>Download</b>.</li>
  </ol>
  <p class="text-xs text-slate-500">The e-reader must reach this address: the home Wi-Fi address works at home, the https:// remote address works anywhere.
    Keep it private: anyone with it can download these books.</p>`;
}

// openKOReaderSetup shows the address for the signed-in person (no userId)
// or for a kid's device (userId, parents only).
export async function openKOReaderSetup(userId) {
  const info = await attempt(() => get(userId ? `/api/admin/users/${userId}/opds` : "/api/me/opds"));
  if (!info) return;
  const d = dialog();
  const render = (i) => {
    const url = location.origin + i.path;
    d.innerHTML = `<div class="max-h-[85vh] space-y-4 overflow-y-auto p-5">
      <div class="flex items-start justify-between gap-3"><h2 class="text-lg font-bold">📖 Set up KOReader</h2>
        <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>${koreaderHTML(url, i.username, !userId)}</div>`;
    d.onclick = async (e) => {
      const ko = e.target.closest("[data-ko]")?.dataset.ko;
      if (e.target === d || e.target.closest("[data-close]")) d.close();
      else if (ko === "copy") copyText(url);
      else if (ko === "reset" && confirm("Make a new address? KOReader will need the new one.")) {
        const n = await attempt(() => post("/api/me/opds/reset"), "New address made");
        if (n) render(n);
      }
    };
  };
  render(info);
  d.showModal();
}
