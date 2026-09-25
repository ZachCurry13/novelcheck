// First-login "How to" guide. Opens automatically until the user finishes or
// skips it, and can be reopened any time from the Help link.
import { PEPPERS, pepperChip } from "./peppers.js";
import { put } from "./api.js";
import { $, esc, attempt } from "./ui.js";

const EVERYONE = [
  ["👋 Welcome to NovelCheck",
    "NovelCheck helps you pick books that fit your family. Each book gets a simple rating for romance (\"spice\") and a few content flags, so you know what's inside before you start reading."],
  ["📚 The Library",
    "The <b>Library</b> tab shows every book with its cover. Search by title, author, series or tag, or narrow it down by <b>genre</b>, <b>fiction or nonfiction</b>, <b>author</b>, <b>series</b> and more. Tick the <b>Hide</b> boxes (like <b>Nudity</b>) to hide books that include those things. <b>☑ Select</b> lets you pick several books at once, to add them to Up Next or ask to delete them."],
  ["🌶️ What the ratings mean",
    `<p class="mb-2">Books get 0 to 5 peppers for romance and sexual content:</p>
    <ul class="space-y-1">${PEPPERS.map((p) => `<li>${pepperChip(p.n)}</li>`).join("")}
      <li><span class="chip-pending">Pending Analysis</span>: not rated yet.</li>
    </ul>
    <p class="mt-2">Tap <b>🌶️ What do the peppers mean?</b> in the Library for the full descriptions and examples.</p>
    <p class="mt-2">Purple tags such as <span class="chip-flag">Dark Occult</span> point out other content you may want to know about.</p>`],
  ["🔎 Book details",
    "Tap any book to open it. You'll see its summary, the rating, and which libraries or devices it's on. Tap <b>＋</b> or <b>Add to Up Next</b> to save it for later."],
  ["▶️ Up Next",
    "The <b>Up Next</b> tab is your reading list. Drag the <b>⠿</b> handle to reorder books. When you're ready, press <b>▶ Start Reading</b>. If you've set up your Kindle email, the book is sent to your Kindle. Below the list, <b>💡 Suggested Reads</b> picks books for you: 👍 means more like this, 👎 hides one (and a quick \"Why not?\" helps it learn)."],
  ["👤 Your profile",
    "In <b>Profile</b> you can set how books reach your e-reader, change your password, and mark books you know under <b>🎯 Your reading taste</b> so your suggestions fit you better. Something not working, or an idea? <b>🐞 Report a problem or idea</b> at the bottom of any page tells your admin."],
  ["📱 Put it on your phone",
    "<b>iPhone:</b> tap <b>Share</b>, then <b>Add to Home Screen</b>.<br><b>Android:</b> tap <b>Install app</b> (or ⋮ → <b>Add to Home screen</b>). It then opens like a regular app."],
];

const MANAGER = [
  ["🛠️ The Admin tab",
    "Your account can manage NovelCheck. On the <b>Admin</b> tab, <b>AI &amp; Scans</b> shows how many books are rated and what the AI has cost so far. <b>Analyze batch</b> rates the next few unrated books, and <b>Sync Calibre now</b> picks up newly added books. Banners at the top point out anything waiting for you, such as delete requests or reported problems."],
  ["✏️ Fixing a rating",
    "Open any book to set its <b>Age group</b> (Young kids, Middle grade, Teens, Young adult, Adults) and to leave <b>Parents' notes</b> after you've read it, for everyone or for parents only. If a rating looks wrong, choose <b>Edit rating</b>. Your correction is saved with your name. If a book is fine for your family even though a filter catches it (Harry Potter's magic, say), choose <b>✓ Mark as OK</b>: it then shows for everyone, kids included. <b>Find duplicates</b> (in the Library) lists books that are in Calibre twice."],
  ["👧 Kids' accounts",
    "Under <b>Admin → Users &amp; Rules</b> you can add kid accounts by age group (they only see books rated for their age or younger), reset their passwords, and tick what each child should <b>not</b> see (for example Open Door or Dark Occult). Hidden books never show up for them, not even in search."],
  ["💾 Importing a Kindle",
    "Plug a Kindle into your computer, open <b>Import books</b>, and pick its <b>documents</b> folder (or bring in a list from Goodreads, Amazon and others). NovelCheck adds the books to a library you name, like \"Kids' Kindle\". It's yours: tick <b>🔒 Private</b> so only you and the admins see it, and manage it under <b>📚 Your libraries</b>."],
];

const ADMIN = [
  ["⚙️ Admin settings",
    "As an admin you also have the technical settings: the AI key and model, spending limits, the email account used for Send-to-Kindle, and which Calibre library folder to use. Editors can't see or change these."],
  ["📬 Start Reading delivery (optional)",
    `<p class="mb-2">When someone presses <b>▶ Start Reading</b> in Up Next, NovelCheck can put the book on their e-reader:</p>
    <ul class="list-disc space-y-1 pl-5">
      <li><b>Kindle</b>: fill in <b>Admin → Delivery & Services → SMTP / Send-to-Kindle</b> (for Gmail, an App Password) and press <b>Send test email</b>. Each reader adds that sender to their Amazon approved list; <b>Profile</b> shows how.</li>
      <li><b>KOReader</b>: nothing to set up here. Each reader opens <b>Profile → KOReader setup</b> for their private catalog address and QR code (parents can do it for kids from their account card).</li>
    </ul>
    <p class="mt-2">Don't use either? Switch them off under <b>Admin → System & Toggles → Features</b>.</p>`],
];

const DONE = [["✅ You're all set", "You can reopen this guide any time from the <b>Help</b> link at the top or bottom of the page."]];

// Parents see this right after the welcome: it's the main feature.
const CHECK = ["📷 Check a book",
  "At a shop, or deciding what to buy? Open <b>Check a book</b> (it's the first tab), tap <b>📷 Take a photo of the cover</b> or type the title, and you'll see its peppers, why, and a short summary. Books you already have answer at once; new ones take a few seconds and are kept under <b>Looked up</b>."];

function stepsFor(user) {
  const s = [...EVERYONE];
  if (user.role === "admin" || user.role === "editor") {
    s.splice(1, 0, CHECK);
    s.push(...MANAGER);
  }
  if (user.role === "admin") s.push(...ADMIN);
  return [...s, ...DONE];
}

export function openGuide(state) {
  const dlg = $("#guide-dialog");
  const steps = stepsFor(state.user);
  let i = 0;

  const render = () => {
    const [title, body] = steps[i];
    const last = i === steps.length - 1;
    dlg.innerHTML = `
      <div class="space-y-4 p-6">
        <div class="flex items-center justify-between gap-4">
          <p class="text-xs text-slate-500">Step ${i + 1} of ${steps.length}</p>
          <button data-g="skip" class="btn-ghost px-2 text-sm">Skip guide</button>
        </div>
        <h2 class="text-xl font-bold">${esc(title)}</h2>
        <div class="text-slate-300 leading-relaxed">${body}</div>
        <div class="flex gap-1">${steps.map((_, n) => `<span class="h-1.5 flex-1 rounded ${n <= i ? "bg-indigo-500" : "bg-slate-700"}"></span>`).join("")}</div>
        <div class="flex justify-between gap-2 pt-2">
          <button data-g="back" class="btn-secondary" ${i === 0 ? "disabled" : ""}>Back</button>
          <button data-g="${last ? "done" : "next"}" class="btn-primary">${last ? "Get started" : "Next"}</button>
        </div>
      </div>`;
  };

  const finish = async () => {
    dlg.close();
    if (!state.user.guide_seen) {
      await attempt(() => put("/api/me/guide-seen", { seen: true }));
      state.user.guide_seen = true;
    }
  };

  dlg.onclick = (e) => {
    const g = e.target.closest("[data-g]")?.dataset.g;
    if (g === "next") i = Math.min(i + 1, steps.length - 1);
    else if (g === "back") i = Math.max(i - 1, 0);
    else if (g === "skip" || g === "done") return finish();
    else return;
    render();
  };
  dlg.oncancel = (e) => {
    e.preventDefault(); // Esc counts as skipping, so it won't pop up again
    finish();
  };
  render();
  dlg.showModal();
}
