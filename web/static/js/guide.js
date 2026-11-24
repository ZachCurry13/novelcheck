// First-login "How to" guide. Opens automatically until the user finishes or
// skips it, and can be reopened any time from the Help link.
import { PEPPERS, pepperChip } from "./peppers.js";
import { put } from "./api.js";
import { $, esc, attempt } from "./ui.js";

const EVERYONE = [
  ["Welcome to NovelCheck",
    "NovelCheck rates every book for romance (\"spice\") and other content, so you know what's inside before you read."],
  ["The Library",
    "The <b>Library</b> shows every book. Search, or filter by genre, author, series and more. <b>Hide</b> boxes hide books with that content. <b>Select</b> works on several books at once."],
  ["Discover",
    "<b>Discover</b> suggests books: best sellers, kids' and teen picks, classics, and family favorites. Your content rules apply. <b>Wishlist</b> asks for a book you don't have."],
  ["Events",
    "Free-book days like <b>Stuff Your Kindle</b> live under <b>Discover → 🎉 Events</b>. Each book shows its rating and whether you have it. <b>Claim on Amazon</b>, then <b>✓ I claimed it</b> to add it to your library."],
  ["Collections & seasons",
    "<b>Collections</b> are shelves like \"Summer reading\". Seasonal ones (🎃 Halloween, 🕯️ Advent) sit at the top of the Library. <b>Series</b> lists each series in order, with what you've read and what's next."],
  ["What the ratings mean",
    `<p class="mb-2">Books get 0 to 5 peppers for romance and sexual content:</p>
    <ul class="space-y-1">${PEPPERS.map((p) => `<li>${pepperChip(p.n)}</li>`).join("")}
      <li><span class="chip-wait">Waiting</span> or <span class="chip-busy">Rating now…</span>: not rated yet; the rating shows up by itself.</li>
    </ul>
    <p class="mt-2"><b>What do the peppers mean?</b> in the Library has examples.</p>
    <p class="mt-2">Purple tags like <span class="chip-flag">Dark Occult</span> flag other content. Icons (⚔️ violence, 🗣️ language, 🩸 gore, 🍺 drinking, 🧩 other) mean the book has some; open it for details.</p>`],
  ["Book details",
    "Tap a book for its summary, rating and content. <b>＋ Nightstand</b> saves it for later."],
  ["Your Nightstand",
    `<p class="mb-2">The <b>Nightstand</b> is your reading list.</p>
    <ul class="list-disc space-y-1 pl-5">
      <li><b>Currently reading</b> shows how far you are. Books you start on your e-reader appear by themselves.</li>
      <li><b>On the pile</b> is what's next. Drag <b>⠿</b> to reorder.</li>
      <li><b>▶ Start Reading</b> sends the book to your e-reader, if one is set up.</li>
      <li><b>Suggested Reads</b> picks books for you. 👍 means more like this; 👎 hides one.</li>
      <li>Your <b>bookshelf</b> at the bottom shows what you finished this year.</li>
    </ul>`],
  ["Readers and sharing",
    "Your reading stats are private. To share them, use <b>Settings → Account → Sharing</b>; <b>Readers</b> then shows the family's reading. Highlights and notes are never shared. Parents can always see their kids' reading."],
  ["Your profile",
    "<b>Profile</b> has your reading and your content rules. <b>Settings</b> has the look, notifications, password, e-reader and accounts. Something wrong? <b>Report a problem</b> is at the bottom of every page."],
  ["Put it on your phone",
    "<b>iPhone:</b> tap <b>Share</b>, then <b>Add to Home Screen</b>.<br><b>Android:</b> tap <b>Install app</b> (or ⋮ → <b>Add to Home screen</b>). It then opens like a regular app."],
];

const MANAGER = [
  ["The Admin tab",
    "<b>Admin</b> (<b>Manage</b> for editors) shows rating progress and AI cost. Banners at the top flag anything waiting for you, like delete requests or problem reports."],
  ["Fixing a rating",
    "Open a book to set its <b>Age group</b> or add <b>Parents' notes</b>. Rating looks wrong? <b>Edit rating</b>. Fine for your family even though a filter catches it? <b>✓ Mark as OK</b> shows it to everyone."],
  ["Kids' accounts",
    "Add kids under <b>Admin → Users &amp; Rules</b>. Each sees only books for their age, minus anything you tick under <b>Hide content</b>. Hidden books never show up for them, even in search."],
  ["Importing a Kindle",
    "<b>Settings → Import books</b> brings in a Kindle's books, or a Goodreads or Amazon list, as a library you name. Tick <b>Private</b> to keep it to yourself."],
  ["Paper books",
    "Printed books count too. In <b>Settings → Paper books</b>, press <b>Scan barcodes</b> and hold up each book."],
];

const ADMIN = [
  ["Admin settings",
    "As main admin you also set the AI, spending limits, Send-to-Kindle email and Calibre folder, and what each other admin can reach (<b>Admin → People & Rules</b>)."],
  ["Start Reading delivery (optional)",
    `<p class="mb-2">When someone presses <b>▶ Start Reading</b> on your Nightstand, NovelCheck can put the book on their e-reader:</p>
    <ul class="list-disc space-y-1 pl-5">
      <li><b>Kindle</b>: fill in <b>Admin → E-readers & Delivery → SMTP / Send-to-Kindle</b> (for Gmail, an App Password) and press <b>Send test email</b>. Each reader adds that sender to their Amazon approved list; <b>Profile</b> shows how.</li>
      <li><b>KOReader</b>: nothing to set up here. Each reader opens <b>Settings → KOReader setup</b> for their private catalog address and QR code, and for reading sync, which marks books Reading and Finished on your Nightstand (parents can do it for kids from their account card).</li>
    </ul>
    <p class="mt-2">Don't use either? Switch them off under <b>Admin → System → Features</b>.</p>`],
];

// Admins other than the main admin: their areas instead of the setup steps.
const SUB_ADMIN = [["Your admin areas",
  "The main admin chose the admin areas you can reach. They're listed at the top of <b>Admin</b>."]];

const DONE = [["You're all set", "Reopen this guide any time from <b>Help</b>."]];

// Parents see this right after the welcome: it's the main feature.
const CHECK = ["Check a book",
  "At a shop? <b>Check a book</b>: photograph the cover or type the title to see its rating and why."];

// A friend's account has no admin side, no delivery and no sharing with a family: a shorter guide, with their reading.
const FRIEND_SKIP = new Set(["Readers and sharing", "Collections & seasons"]);
const READING = ["Your reading and bookshelf",
  "Import what you've read from <b>StoryGraph</b>, <b>Goodreads</b> or <b>Bookly</b> (<b>Settings → Import books</b>). <b>Stats</b> shows your year; <b>▶ Play Wrapped</b> on the Library banner plays it as a story. Your <b>bookshelf</b> is at the bottom of the <b>Nightstand</b>."];

function stepsFor(user) {
  if (user.role === "friend") {
    const f = EVERYONE.filter((e) => !FRIEND_SKIP.has(e[0]));
    f.splice(1, 0, CHECK);
    f.splice(f.findIndex((e) => e[0] === "Your profile"), 0, READING);
    return [...f, ...DONE];
  }
  const s = [...EVERYONE];
  if (user.role === "admin" || user.role === "editor") {
    s.splice(1, 0, CHECK);
    s.push(...MANAGER);
  }
  if (user.role === "admin") s.push(...(user.owner ? ADMIN : SUB_ADMIN));
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
        <h2 class="text-xl font-semibold">${esc(title)}</h2>
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
