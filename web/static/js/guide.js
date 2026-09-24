// First-login "How to" guide. Opens automatically until the user finishes or
// skips it, and can be reopened any time from the Help link.
import { put } from "./api.js";
import { $, esc, attempt } from "./ui.js";

const EVERYONE = [
  ["👋 Welcome to NovelCheck",
    "NovelCheck helps you pick books that fit your family. Each book gets a simple rating for romance (\"spice\") and a few content flags, so you know what's inside before you start reading."],
  ["📚 The Library",
    "The <b>Library</b> tab shows every book. Use the search box and the drop-downs to narrow it down. Tick the <b>Hide</b> boxes (like <b>Nudity</b>) to hide books that include those things. Missing a filter? Use <b>Suggest one</b> next to the boxes."],
  ["🌶️ What the ratings mean",
    `<ul class="list-disc space-y-1 pl-5">
      <li><span class="chip-none">No Spice</span>: no physical intimacy.</li>
      <li><span class="chip-closed">Closed Door</span>: romance, but intimate scenes happen off the page.</li>
      <li><span class="chip-open">Open Door</span>: intimate scenes are described on the page.</li>
      <li><span class="chip-pending">Pending Analysis</span>: not rated yet.</li>
    </ul>
    <p class="mt-2">Purple tags such as <span class="chip-flag">Dark Occult</span> point out other content you may want to know about.</p>`],
  ["🔎 Book details",
    "Tap any book to open it. You'll see its summary, the rating, and which libraries or devices it's on. Tap <b>＋</b> or <b>Add to Up Next</b> to save it for later."],
  ["▶️ Up Next",
    "The <b>Up Next</b> tab is your reading list. Drag the <b>⠿</b> handle to reorder books. When you're ready, press <b>▶ Start Reading</b>. If you've set up your Kindle email, the book is sent to your Kindle."],
  ["👤 Your profile",
    "In <b>Profile</b> you can add your Send-to-Kindle email and change your password."],
  ["📱 Put it on your phone",
    "<b>iPhone:</b> tap <b>Share</b>, then <b>Add to Home Screen</b>.<br><b>Android:</b> tap <b>Install app</b> (or ⋮ → <b>Add to Home screen</b>). It then opens like a regular app."],
];

const MANAGER = [
  ["🛠️ The Manage tab",
    "Your account can manage NovelCheck. The <b>Manage</b> tab shows how many books are rated and what the AI has cost so far. <b>Analyze batch</b> rates the next few unrated books, and <b>Sync Calibre now</b> picks up newly added books."],
  ["✏️ Fixing a rating",
    "If a rating looks wrong, open the book and choose <b>Edit rating</b>. Your correction is saved with your name. If a book is fine for your family even though a filter catches it (Harry Potter's magic, say), choose <b>✓ Mark as OK</b>: it then shows for everyone, kids included."],
  ["👧 Kids' accounts",
    "At the bottom of <b>Manage</b> you can add kid accounts, reset their passwords, and tick what each child should <b>not</b> see (for example Open Door or Dark Occult). Hidden books never show up for them, not even in search."],
  ["💾 Importing a Kindle",
    "Plug a Kindle into your computer, open <b>Import Drive</b>, and pick its <b>documents</b> folder. NovelCheck lists the books and adds them to a catalog like \"Jenna's Kindle\"."],
];

const ADMIN = [
  ["⚙️ Admin settings",
    "As an admin you also have the technical settings: the AI key and model, spending limits, the email account used for Send-to-Kindle, and which Calibre library folder to use. Editors can't see or change these."],
];

const DONE = [["✅ You're all set", "You can reopen this guide any time from the <b>Help</b> link at the top or bottom of the page."]];

function stepsFor(user) {
  const s = [...EVERYONE];
  if (user.role === "admin" || user.role === "editor") s.push(...MANAGER);
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
