# NovelCheck

Self-hosted, Dockerized web app that scans and catalogs e-book libraries for romantic/sexual content ("spice") and sensitive themes, including Catholic-discernment occult criteria. It reads your **Calibre library** read-only, imports **Kindle / e-reader drives** from the browser, keeps a per-user **"Up Next" reading queue** with drag-and-drop, and installs to your phone's home screen as a **PWA**.

See [`NOVELCHECK_SPEC.md`](NOVELCHECK_SPEC.md) for the full specification, architecture, and build order.

## Features

| Area | What you get |
|---|---|
| 📷 Check a book | **The main screen for parents.** In a shop, snap the cover (or type the title, author or ISBN) and get a plain verdict: "✓ Level 0–2", "⚠ Level 3" or "✕ Level 4–5 (explicit)", with the reason, tags and summary. Books you already have answer at once; others are looked up (Open Library), rated on the spot and kept under **Looked up**. The AI reads the cover (vision models such as `gpt-4o-mini`, Claude or Gemini); Android phones read the ISBN barcode themselves. |
| 👥 Family devices | A parent can make a shared tablet or computer open on **Who's reading?**: kids tap their name, and anyone with a 4-digit PIN types it (parents without a PIN use their password). Wrong PINs lock the profile on that device for a while; parents list and remove family devices under Admin → Users. |
| Accounts | The first visit creates the admin account in the browser. Three roles: `Admin` (everything), `Editor` (ratings, scans, imports and kids' accounts, but no technical settings or secrets), and `Restricted` (kid, created per age group). Cookie sessions; per-profile content rules (hide Open Door, Nudity, Solo Acts, Heavy Innuendo, Dark Occult / Demonic, LGBTQ+, or anything not yet analyzed), enforced in SQL so hidden titles can't be listed, searched, opened, queued or downloaded. |
| Calibre sync | Mount any parent folder at `/calibre`, then pick the exact library in **Admin → Delivery & Services → Calibre Library**, either by browsing or with **Find libraries automatically**. Opens `metadata.db` with `mode=ro` (it never writes to it), maps files to `/calibre/<library>/<relative path>`, and puts books in the **Calibre Main** catalog. Runs on a schedule (default every 6 h) or manually. Books deleted in Calibre are pruned. Each book shows its Calibre ID, and if you use [Calibre-Web](https://github.com/janeczku/calibre-web), enter its address under **Admin → Delivery & Services → Calibre Library** to get an **Open in Calibre-Web** link on every book. |
| Tidy titles | Titles with track or series numbers ("01 - The Hobbit", "Book 2: Catching Fire", "Guards! Guards! (Discworld, #8)") show as the plain title with **📚 series and number** underneath; Calibre's own series wins, and titles like "1984" are left alone. **🏷️ Tidy in Calibre** (Admin) saves the tidy titles into Calibre through its Content server after checking each book matches; any book's title, series and number can also be edited from its window, with an **Open in Calibre-Web** fallback. Renaming a book in Calibre keeps its rating, notes and queue places. |
| Import lists | **Import books** also takes a list with no cable: Goodreads, StoryGraph, Hardcover or LibraryThing exports, Amazon's data download, any spreadsheet saved as CSV, or text copied from Amazon's Content Library page. Columns are detected automatically, with shelf choices and plain step-by-step guides. Books on a Goodreads or StoryGraph **read** shelf count as finished. |
| 📖 What it's about | Cards and the book window say what a book is about in a sentence or two, without spoilers: the AI writes it when it rates the book (in the chosen language), and until then the book's own description is used with review quotes and best-seller lines taken out. The rating note stays in small text underneath. |
| 📕 Paper books | **📚 Paper books** (parents) adds printed books to a named physical library ("Living room shelf", "Kids' room"): scan barcodes one after another with the phone camera, type a title or ISBN, or take a photo of the cover. Each book is looked up, added and rated first in line (within the hourly token limit), with the ratings showing as they arrive. Paper books appear in the Library (📕 Paper chip; **📕 Paper books** under Format), Discover, Up Next and suggestions under everyone's rules; **▶ Start Reading** just marks them as reading. |
| ⚡ Automatic rating | While the AI is idle, NovelCheck rates waiting books by itself (Up Next and wishlist books first, then the newest), right after a Calibre sync or an import, within the hourly token limit. On by default with a local AI, off with a paid one until turned on (**Admin → AI & Scans → Automatic rating**), with optional hours (e.g. 23:00–07:00, in your time zone). Unrated books show ⏳ Waiting / ⏳ In line / ⚡ Rating now… / ⚠ Rating failed; the Library updates cards live (🧬 Deep Scan progress too) and parents get an activity pill in the header (⚡ 38, ⏸, 🌙, 🧬 4/12, 🧬⏸ while the Deep Scan machine is off). |
| 📚 Collections | Shelves across libraries: made by hand (add from any book's window) or filled by the AI from a theme you describe (it keeps only books clearly about the theme, checks each pick a second time and gives a reason; untick and save; **Find more** later). Parents can take books off a shelf for good (**☑ Select → ✕ Not for this shelf**) or have the AI list the ones that don't fit (**🔍 Check these books**). About weekly the AI proposes up to 3 themed collections to keep or drop (on by default with a local AI). A kid's account can be limited to chosen collections (enforced everywhere, on top of their content rules). |
| 🗂 Series | **Collections → Series** lists every series with how many you've read. A series opens in order, showing the numbers you don't have, what you've read or are reading, and **Next up** with **＋ Up Next**. |
| 📦 Box sets | Books whose titles say they hold several ("Books 1–3", "Trilogy", "Box Set") get a banner in Admin. A parent checks what's inside (the AI can list it) and splits it: each book gets its own card and rating, counts as owned, and opens the box set's file. **Not a box set** and **Undo the split** are there too. |
| 🔗 Same book, other titles | TV and movie tie-in editions ("Title (Movie Tie-In)") count as the original book. **Same book as…** on a book (parents) joins two cards for one book: the better-rated one stays, everything else moves to it, and later imports under the other title find it. |
| 🗓️ Seasonal shelves | Halloween, Fall, Advent, Christmas, Winter, Valentine's, Lent & Easter, Spring, Summer, Back to school and Saints, in season by date (church seasons follow Easter and Advent). Chips at the top of the Library and Discover (in-season first) and a seasonal row on Discover; found by whole words in titles, tags and descriptions (with look-alikes such as "Saint Louis" skipped), or by a collection a parent builds with the AI. |
| 🧭 Menu & start page | The menu is Check a book, Library, Discover (with ⭐ Wishlist), Up Next, Collections, Admin (with Deep Scan, Usage and System checks as tabs) and Profile (with ➕ Add books: Import books and Paper books). Each person picks the page NovelCheck opens on (Profile → Start page). |
| 🤖 Kids and AI | A kid's AI features (AI-picked Suggested Reads, Deep Scan requests) are off until a parent ticks **Allow AI features** on their card; the server enforces it. |
| 🎉 Events | Stuff Your Kindle and other free-book days: a parent pastes the event's list (the Amazon links are kept) or its page's address (public addresses only; NovelCheck presses "Load more books" and follows next pages, and fills in the event's name and day). New books are rated first in line. An event opens in the Library with every filter and sort (plus Hide books we have and the event's order), with **Claim on Amazon**, **✓ I claimed it** (files it in a chosen library) and ⭐ Wishlist on each card. Family-wide (kids see rated books within their rules, no Amazon links). An event moves to the 🗄 Archive when its end passes (or after 30 days unless pinned); parents can edit, archive, restore or delete it. |
| 🤖 AI machines | **Admin → AI & Scans → AI machines:** Deep Scans can run on a separate, bigger machine and wait (🧬⏸) while it's off; a daily check for newer versions of the Ollama models in use, with a 1-click **Update**; a speed test on a sample book (Deep Scan estimates then include the time); electricity cost from a price per kWh and each machine's watts (in Usage and estimates); ratings the AI wasn't sure of get ⚠ Not sure, a ⚠ Needs review filter and **Re-rate with the big model**; reasoning models get room to think. |
| 🎨 Appearance | **Profile → Appearance**, per person on every device: theme (match the device, Dark or Light), the **OpenDyslexic** font, and **Reduce motion**. |
| 🔎 Search | The Library remembers each person's last 10 searches (under the search box, with Clear). Title sort ignores "The/A/An", author sort goes by last name, and **Fewest peppers** sorts the mildest first. |
| 🛟 Safe mode & commands | Safe mode starts the web app without background work (Calibre sync, rating, Deep Scans): set `NOVELCHECK_SAFE_MODE=true`, run `novelcheck safe-mode on`, or it turns on after 3 unexpected stops within a few minutes of starting; admins leave it from a banner. Commands for the container shell: `novelcheck users`, `reset-password <user> [password]`, `backup [file]`, `check`, `safe-mode on|off|status`, `version`. |
| Delete requests | Anyone can **🗑 Request to delete** a book with a reason; admins review the list and delete from Calibre (recycle bin), keep, or mark done, with a history. |
| Drive scanner | Uses `showDirectoryPicker()` on Chromium and falls back to `<input webkitdirectory>` on iOS Safari and Firefox. Reads EPUB OPF metadata and MOBI/AZW3 EXTH headers **in the browser**, and parses Kindle filenames (`Title - Author_B0XXXXXXXX_EBOK.azw`). Only metadata is uploaded. You pick an existing catalog or create one, such as "Kids' Kindle". Newer Kindles connect as an MTP "device" that browsers can't open: the page explains copying the `documents` folder off first, or you can **Paste a list** of titles instead. |
| Library | Browse every catalog in one place. Filter by catalog, **peppers** (0–5) and cross-catalog overlap ("also in…" / "only books in 2+ catalogs"), and tick **Hide** boxes (Open Door, Nudity, Solo Acts, Heavy Innuendo, Dark Occult) or **Hide content** groups and items to hide books with that content. |
| 🧩 Content details | Besides peppers, the AI marks 45 content items in five groups: 🗣️ **Language** (profanity, the F-word, blasphemy, sexual language, crude humor, slurs, insults), ⚔️ **Violence** (fights, weapons, guns, stabbing, murder, war, torture, domestic, sexual, against children or animals), 🩸 **Gore** (blood, injuries, deaths, dismemberment, mutilation, body horror, organs, corpses), 🍺 **Substance Use** (alcohol, underage drinking, smoking, vaping, marijuana, drugs, prescription misuse, dealing, addiction, overdose) and 🧩 **Other** (suicide/self-harm, eating disorders, abuse, bullying, death/grief, religious themes, LGBTQ+, pregnancy, mental health). Cards show an icon per group; the book window lists the items and says whether they came from the description, a Deep Scan or a parent; Deep Scans also say how much of each group there is (A little / Some / A lot). **Hide content** (Library, and each account under Users & Rules) hides whole groups or single items; kids' presets and new kid accounts start with a sensible set, and **Edit rating** lets parents correct the items. |
| ▦ Barcode & ⭐ Wishlist | Check a book scans the ISBN barcode live with the phone camera (the browser's own detector on Android, the bundled [ZXing](https://github.com/zxing-js/library) library elsewhere; needs https). Books you own offer **Add to Up Next**; others offer **⭐ Add to Wishlist** or **Up Next (to get)**, never delete or remove. **Discover → ⭐ Wishlist** lets parents approve & track, decline or mark wishes as got (automatic once the book arrives in the library). |
| 🧬 Deep Scan | For chosen books the AI reads the whole EPUB (from Calibre) in parts instead of rating from the blurb. Peppers count romance and sex only (Level 3 = desire, heavy making out or sex off the page; 4+ = sex on the page); parts are sized to fit a local AI's context window (a setting, 4,096 tokens by default), split again if the AI says one is too long, and retried up to 3 times; every part must name what it saw, and parts rated 3+ get a second look that says how far the scene goes: Level 4+ needs a sex scene described in detail, with sentences the AI names that really are in the book, and a second question that agrees (scenes that fade out stay at 3). A raise of 2+ levels, or to 4+ on one passage, waits for an admin. A 7B+ model is recommended (the page offers a bigger installed one): admins start one, others request one for approval, **Admin → 🧬 Deep Scan** scans the next 10/20/30 Up Next books after showing the cost estimate, and up to 3 readers' Up Next can be scanned automatically. Results list what was found where; raised ratings trigger a warning and a "Rating changed via Deep Scan" banner; **🧬 Deep Scanned only** filters the Library. **Admin → 🧬 Deep Scan** has four sections: **Review** (raises waiting for a check: each flagged part with a short scene description and **📖 Read this part in the book**, a reader that highlights the part in the book's text with **◀ Earlier / Later ▶**; Accept, Keep or **Or set Level…**, plus **Accept all** and **Keep all old ratings**; every decision leaves a parents-only note on the book), **Running** (with progress), **Results** and **Settings**; titles open the book. The window of any scanned book shows parents the same **What the AI found** list with scenes and the reader. |
| Custom AI filters | **Admin → AI & Scans → Custom AI filters**: add your own topics (for example *Heavy swearing* or *Gore / violence*) with a sentence of instructions. The AI checks every book for them, books get a tag, and the Library (and each kid's rules) gets a **Hide** box per filter. Adding one (or changing its instructions) offers to re-rate existing books. |
| Parent "OK" | Admins and editors can **✓ Mark as OK** a book a filter catches by mistake (Harry Potter's magic, say). It then shows for everyone, overriding hide filters and kids' content rules, and is never offered for removal. |
| Remove from Calibre | Admins get **Remove hidden books from Calibre…**, which lists the Calibre books your Hide filters catch (never ones marked OK). With **One-click removal** connected to Calibre's Content server, one button removes them through Calibre, to its recycle bin, after checking every title against Calibre so the wrong library can't be touched. Without it, NovelCheck gives you a Calibre search to paste. Either way NovelCheck itself never writes to your library. Guide: [docs/CALIBRE_SERVER.md](docs/CALIBRE_SERVER.md). |
| Feedback | **Suggest a filter / feedback** (footer) and **Suggest one** (next to the Hide boxes) open GitHub issue forms. |
| Reading queue | Personal "Up Next" list, reordered with SortableJS drag handles and saved as positions. **▶ Start Reading** emails the EPUB through Send-to-Kindle SMTP (after showing the sender and the Amazon approved-list steps) and moves it to *Currently Reading*; a book with no EPUB or PDF says so and is just marked as reading. Books show how far you are (from KOReader's progress sync or statistics: %, device, when), and parents see each kid's reading on their card. |
| 💡 Suggested Reads | Under **Up Next**, each person gets books from the library picked for them, each with a reason ("Next in Discworld after Mort", "More by …", "Like …"), based on their Up Next, what they're reading and what they've finished. Only books their rules allow. 👍 offers **＋ Up Next** / **⭐ Wishlist**; 👎 hides the book and asks an optional **Why not?** (story, author, series, too spicy, already read) that steers later suggestions. The admin picks **Free matching only**, **AI picks from your library** (at most once a day per person, within the token cap) or **AI picks + books you don't own** (never for kids' accounts). **🎯 Your taste** (optional, no AI): mark 20 books at a time as want to read / read & liked / read & didn't like / not for me, any time, to tune the suggestions. |
| 🖼️ Covers | Every list and book window shows the book's cover from Calibre (the `cover.jpg` Calibre keeps next to the files; shrunk once and cached under `/data/covers`, the library is only read), Open Library covers by ISBN for looked-up books, and a drawn stand-in otherwise. **🖼️ Wrong cover?** sends a report to the admins, who fix it in Calibre (or Calibre-Web) and mark it fixed. |
| 🔎 Browse | Filters for **genre** (about 20 categories from Calibre's tags, matched on whole words; **Fill in with AI** on the Admin page sorts untagged books, ~25 per call, with a cost estimate), **Fiction / Nonfiction**, **author** and **series** (type-ahead), besides catalog, peppers, age group and format. Search covers title, author, series and tags. Author, series and genre in a book's window link to the filtered library. |
| 🐞 Report a problem | **Report a problem or idea** (footer, everyone) sends a note to the admins with the page and device; admins review it on the Admin page and can run **🩺 Diagnose** to pass real bugs on. No GitHub account needed. |
| 📚 Own libraries | Whoever imports a library owns it and can make it **Private** (only them and the admins see its books) or **Shared**, rename it, delete it, or take single books out of it straight away (**Remove from this library**; nothing is deleted from devices or Calibre). Admins can reassign owners; older libraries belong to the family. |
| 🧭 Discover | A tab of books to find next: **Popular now**, **New on the best-seller lists**, popular nonfiction, **Top teen books**, **Top kids' books** and **All-time classics**, plus **New in your libraries** and **Popular in the family**. With a free New York Times Books key ([how to get one](docs/DISCOVER.md)) the rows are the real weekly best-seller lists (fetched once a week; each book tagged **📰 NYT list** and each row credited); without one they come from Open Library. The tab opens at once with the lists seen last time, then refreshes. Lists refresh daily and the AI rates a set number of list books a day (default 30), best-ranked first, so every card shows its peppers and content icons and everyone's content rules apply; kids never see unrated books and their rows follow their age group. Tap ⭐ Wishlist, ＋ Up Next (books you own), Amazon or Open Library. List books you don't own stay out of the Library, and books the family already has are left out of the lists unless **Also show books we already have** is ticked. |
| ☑ Select several | **Select** mode in the Library (tap, drag with a mouse, Shift-click, or select all shown), then **＋ Up Next**, **🧬 Deep Scan** (admins start, others request) or **🗑 Delete** (your own libraries at once, a delete request otherwise) for all of them in one go. |
| KOReader catalog | Each reader has a private OPDS 1.2 catalog address (`/opds/<token>`; **Profile → KOReader setup**, with a QR code and steps; parents open it for kids' devices). It lists their Up Next with download links, follows their content rules, and can be replaced with **New address**. **Reading sync:** KOReader's Progress sync can point at NovelCheck (`/kosync`, signed in with the reader's name and a sync code), so opening a book marks it Reading in Up Next and finishing it marks it Finished. **Reading statistics:** KOReader's statistics Cloud sync can use a private NovelCheck WebDAV folder (`/dav/`, same sign-in), and Up Next then lists **📱 On your KOReader**: every book opened on the reader's KOReader devices with %, reading time and last opened. Uses the [qrcode-generator](https://github.com/kazuhikoarase/qrcode-generator) library (MIT). |
| LLM analysis | Pick a provider in **Admin**: OpenAI, **Anthropic Claude** (native Messages API via the official Go SDK), Google Gemini, Perplexity, Ollama, or any other OpenAI-compatible endpoint (vLLM, LM Studio). Presets choose a small model (for example `gpt-4o-mini` or `claude-haiku-4-5`), and optional fallback models (several, in order) are used only when it fails. Blurbs come from Open Library, then Google Books, before the LLM runs. Syncing itself never calls the AI: new books wait as *Pending Analysis* until **⚡ Automatic rating** gets to them or a parent runs a batch or a single scan. |
| Admin panel | Settings in four tabs (AI & Scans, Users & Rules, Delivery & Services, System & Toggles) with collapsible cards, next to the Deep Scan, Usage and System tabs. Batch size, tokens/hour cap, scan delay, live token counter and cost estimator (spent and projected), SMTP settings with a test send, Calibre polling interval, one-click `novelcheck.db` backup, a wipe of the pending analysis queue, and **Re-rate whole library** (with a cost estimate). Books are never rated twice unless you ask: syncing only adds unrated books, and books edited in Calibre after rating are listed for an optional re-rate. |
| Ratings review | Admins and editors can correct any rating by hand (**Edit rating**). Manual ratings are labeled with who made them. |
| Remote access | Built-in Cloudflare Tunnel connector: paste a tunnel token in **Admin → Delivery & Services → Remote access** to get an `https://` address that works away from home, with no port forwarding. Guide: [docs/REMOTE_ACCESS.md](docs/REMOTE_ACCESS.md). |
| Ollama easy setup | With the Ollama provider selected, NovelCheck finds your Ollama server, measures what its GPU can hold and marks the best and most powerful models that fit, downloads a model with a progress bar, and lets you tick and order models (main first, then fallbacks), with no terminal needed. **🧹 Installed models** lists each model's disk space and deletes unused ones (models in use are protected). |
| 🌶️ Pepper scale | Books are rated **0–5 peppers**: 0 No Romance, 1 Sweet Romance, 2 Romantic, 3 Steamy Closed-Door, 4 Explicit / Open Door, 5 Very Explicit / Erotica-Level. Each rating says why in a few words, and cards at Level 3 or higher show the reason as a tag and a "why" tooltip (for example "Heavy innuendo, on-page foreplay"). **What do the peppers mean?** in the Library gives the full descriptions and examples. Kid accounts have a **Most peppers allowed** limit (age groups start at 0, 1, 2, 3 or no limit) and one-click presets: **Strict Family** (up to Level 2, nothing explicit or unrated) and **Young Reader** (ages 9–12, up to Level 1). When the rating rules change, older AI ratings can be re-rated in one click and stay visible meanwhile. |
| Duplicates & formats | Cards show each book's file formats (EPUB, AZW3, MOBI…). A **Format** filter finds a format, books with 2+ formats, duplicates, or books with no file. **Find duplicates** lists books that are in Calibre more than once with each copy's formats and size, suggests the copy to keep, and (admins, with one-click removal) moves the extras to Calibre's recycle bin, always keeping at least one. **Formats…** (admins, with one-click removal) keeps only the formats you tick in every book that has one, with a preview, the space saved and **Undo** for 7 days, and has Calibre **convert** books with no EPUB. |
| Age groups & notes | Parents set each book's **age group** (Young kids, Middle grade, Teens, Young adult, Adults) and leave **notes** after reading it, visible to everyone or to parents only. Kid accounts are created per age group and only see books rated for their group or younger; a parent-set age group counts as a rating. |
| Usage page | Admins get a **Usage** tab: CPU, memory, network download/upload speed and disk with 15-minute charts, library progress, AI calls, cost and tokens per day, and the Ollama models currently loaded (GPU vs RAM). Tap or hover a chart for exact values. |
| Language | Summaries are written in the chosen language (default English (US)), whatever the book's language; summaries not in English can be re-rated in one click. |
| Backup AI | An optional second AI (another Ollama on your network, or a cloud AI) takes over when the main AI fails or its server is switched off, with a 🔔 notice and its own prices. |
| 🩺 AI diagnosis | **System → Diagnose with AI** has your connected AI read NovelCheck's diagnostics (never passwords or keys), explain what's wrong, and draft a GitHub bug report you can open in one click. Error pop-ups, notifications and failed books have a 🩺 shortcut, and logs have 📋 Copy / ⬇ Download buttons. |
| System page | Admins get a **System** tab with **🩺 Diagnose with AI** and **Check everything**, which tests every AI model (main, fallbacks and backup), Open Library, Google Books, the Calibre library and Content server, email login, remote access and whether the public address really opens from the internet, Ollama and the update check, each with a suggested fix. |
| Rating errors | Usage and Admin list books that failed to rate, grouped by reason with a plain explanation, and **Retry** them in one click. |
| Features | **Admin → System & Toggles → Features** turns off parts your family doesn't use: the Up Next queue, Send-to-Kindle email, KOReader sync, Import books, Discover, Suggested Reads, the taste profile, or parent tools (kids' accounts, age groups, notes). They disappear for everyone and their API calls are refused; kids' content rules always keep applying. |
| Notifications | A 🔔 bell for admins and editors collects problems NovelCheck notices (failed ratings, Calibre sync errors, remote access dropping, Send-to-Kindle failures, Ollama download failures, failed checks, the hourly AI limit being reached, books waiting for delete review), each with a button that goes to the fix and a ✕ to dismiss it; Deep Scans waiting for review are one card with a **Review** button. It can also list everyday events: new books from a Calibre sync, a finished rating batch (books, tokens, cost) and who started reading what. Repeats are grouped, and some clear themselves when fixed. Error pop-ups stay until closed and are also listed there. |
| 📱 Phone notifications | **Profile → Phone notifications** turns on Web Push for a phone or computer: parents get the 🔔 notices (everything, or only problems), and everyone gets "Ready to read" when a book they started is sent. Needs the `https://` address from **Remote access**; on iPhone, add NovelCheck to the Home Screen first. Built on the standard (VAPID + RFC 8291 encryption) with no extra services: the server's key is created on first use and never leaves it, and subscriptions may only point at the browsers' own push services. |
| Help & updates | A step-by-step **How to** guide opens on each person's first login and can be reopened from **❔ Help**. Admins and editors see a banner when a new version is released, and **What's new** shows release notes (from `CHANGELOG.md` and GitHub Releases). |
| PWA | `manifest.json` (standalone, dark theme) and `sw.js` app-shell cache for offline launch and Add to Home Screen. On phones the menu is a bottom tab bar (with **More** for the rest, opening from the bottom), pop-up windows open as sheets from the bottom, nothing makes a page wider than the screen, and form fields never zoom the page. The app's files are served at addresses tied to the version (`/v/<build>/…`), so a phone never runs a mix of old and new files after an update. |

## Install on TrueNAS (easiest)

Follow **[docs/TRUENAS.md](docs/TRUENAS.md)**. It's a click-by-click guide for either TrueNAS **Install Custom App** (a form) or **Install via YAML**, using the prebuilt image, so there's no command line and nothing to build. The guide keeps TrueNAS's default pull policy (only pull the image if it isn't on the NAS yet), so NovelCheck starts from its own copy at boot and updates arrive through TrueNAS's **Update** button.

## Quick start (Docker / Docker Compose)

A prebuilt image is published to GitHub Packages as `ghcr.io/zachcurry13/novelcheck` (`:latest` is the newest release, `:X.Y.Z` each release, and `:main` the newest commit on `main`), for `linux/amd64` and `linux/arm64`.

```bash
curl -O https://raw.githubusercontent.com/ZachCurry13/novelcheck/main/docker-compose.yml
# edit the time zone and the two volume paths in docker-compose.yml, then:
docker compose up -d
```

To update later: `docker compose pull && docker compose up -d`.

To build from source instead, clone the repo, uncomment `build: .` in `docker-compose.yml`, and run `docker compose up -d --build`.

Open `http://<host>:8080` and create your admin account on the welcome page. Do this right away: until an admin exists, anyone who can reach the page can create one. (The TrueNAS guide maps port `30080` instead, to avoid clashing with other apps.)

Then, in **Admin**:
1. Under **Calibre Library**, pick your library folder.
2. Under **LLM Analysis Engine**, pick an AI provider and paste its API key. **Show setup steps** walks through each provider; the same guide is in [docs/AI_PROVIDERS.md](docs/AI_PROVIDERS.md). For local Ollama use `http://ollama:11434/v1` with `llama3.2`, and turn off JSON mode if your server rejects `response_format`.
3. Click **Analyze batch**. Pending books are processed within your token cap.
4. Add editor and restricted users, and set content rules.

### Volumes

| Container path | Put it on | Mode | Purpose |
|---|---|---|---|
| `/data` | SSD dataset | read-write | `novelcheck.db` (users, catalogs, verdicts, queues, settings) |
| `/calibre` | HDD pool | **read-only** | A folder that contains your Calibre library (the exact subfolder is chosen in the app) |

The container runs as UID/GID `568` (TrueNAS `apps`). That user needs read access to the Calibre dataset and write access to the data dataset.

### Optional profiles

- Remote access: easiest is the built-in tunnel (**Admin → Delivery & Services → Remote access**). Alternatively, `docker compose --profile tunnel up -d` runs a separate `cloudflared` with `CLOUDFLARE_TUNNEL_TOKEN`; route that tunnel to `http://novelcheck:8080`.
- `docker compose --profile ollama up -d`: runs a local Ollama with NVIDIA GPU passthrough.

## Configuration

Process-level settings are environment variables. Everything else is edited in the Admin panel and stored in the database.

| Variable | Default | Description |
|---|---|---|
| `NOVELCHECK_ADDR` | `:8080` | Listen address |
| `NOVELCHECK_DATA_DIR` | `/data` | Directory for `novelcheck.db` |
| `NOVELCHECK_CALIBRE_DIR` | `/calibre` | Mount point for the folder containing your Calibre library (read-only) |
| `NOVELCHECK_ADMIN_USER` | `admin` | Optional, for automated installs: username for a pre-created first admin |
| `NOVELCHECK_ADMIN_PASSWORD` | *(empty)* | Optional: if set and no users exist, creates that admin at startup. Leave empty to create the admin in the browser. |
| `NOVELCHECK_TRUST_PROXY` | `true` | Use `CF-Connecting-IP` / `X-Forwarded-For` / `X-Real-IP` for client IPs |
| `NOVELCHECK_CORS_ORIGINS` | *(empty)* | Comma-separated list of allowed cross-origin callers |
| `NOVELCHECK_SAFE_MODE` | *(empty)* | `true` starts without background work until the variable is removed (see Safe mode) |
| `NOVELCHECK_SESSION_DAYS` | `30` | Default "keep me signed in" length. Admins can change it in **Admin → System & Toggles → Sign-in & Updates**. |

## Security notes

- Every response carries HSTS, `nosniff`, `X-Frame-Options: DENY`, and a strict same-origin CSP. API responses also carry `Cache-Control: no-store`, so Cloudflare and the service worker never cache private data.
- Sign-ins with **Keep me signed in** last 30 days (configurable) and roll forward while the person keeps using the app. Without it, the sign-in ends when the browser closes or after 12 hours idle. Sessions use `HttpOnly`, `SameSite=Lax` cookies, and `Secure` is set when served over HTTPS or `X-Forwarded-Proto: https`. The database stores only a SHA-256 of each session token.
- State-changing API calls require an `X-NovelCheck: 1` header (CSRF guard). Login is rate-limited per IP.
- **The database backup contains password hashes and the LLM/SMTP secrets.** Store it like a credential.
- Book downloads are served only from paths recorded by the Calibre sync that sit inside `NOVELCHECK_CALIBRE_DIR`. The in-app folder browser is admin-only and can't leave that mount, even through symlinks.
- First-run setup (`POST /api/setup`) only works while the database has no accounts.
- The tunnel token is stored as a secret (never returned to the browser) and passed to `cloudflared` through its environment, not the command line.
- The Ollama helper (admin only) probes the container's gateway, `host.docker.internal`, and `ollama` on ports 11434/30068, plus any address the admin types.
- Update checks call `api.github.com` about every 6 hours. Turn them off under **Admin → System & Toggles → Sign-in & Updates**.

## Development

```bash
make test        # go test ./...
make run         # serves on :8080 with ./data and ./calibre
make css         # recompile Tailwind after changing classes (output is committed)
```

On Windows without `make`, run the same command directly: `npx tailwindcss@3 -c tailwind.config.js -i web/tailwind.input.css -o web/static/css/app.css --minify`. `go test ./...` also runs on Windows; a few Calibre, database and tunnel tests need Linux (symlinks, `/` paths, `cloudflared`) and pass in CI.

`internal/calibresrv` has an optional test against a real `calibre-server`. Run one with `--enable-auth` and a user who can make changes (plus a read-only user `reader` / `readpass1`), then set `NOVELCHECK_TEST_CALIBRE_URL`, `NOVELCHECK_TEST_CALIBRE_USER` and `NOVELCHECK_TEST_CALIBRE_PASS`.

The version shown in the app comes from `-ldflags -X .../internal/version.Version=…`. `make` sets it from `git describe`; a release build uses the version entered when the release is started. Add a `## [x.y.z]` section to `CHANGELOG.md` before releasing, because it becomes the release notes.

CI (`.github/workflows/docker.yml`) runs `go vet` and `go test` on every push and pull request. On `main` it publishes `ghcr.io/zachcurry13/novelcheck:main`; only releases move `:latest`. To cut a release, go to **Actions → Docker image → Run workflow** and enter a version such as `1.1.0`. That publishes `:1.1.0` and `:latest` and creates the `v1.1.0` tag and GitHub Release. Pushing a `vX.Y.Z` tag does the same.

Go 1.26+, no CGO (pure-Go `modernc.org/sqlite`). Front-end assets are embedded with `embed.FS`, so the binary is fully self-contained. Per the spec's rule, no source file is longer than about 300 lines.

```
cmd/novelcheck/        entrypoint (config, bootstrap, workers, HTTP server)
internal/config/       env configuration
internal/db/           SQLite open + embedded schema.sql and migrations
internal/store/        data access: users, sessions, catalogs, books, filters, queue, settings, usage, events…
internal/auth/         bcrypt, cookie sessions, RBAC middleware, admin bootstrap
internal/calibre/      read-only metadata.db sync + polling scheduler
internal/calibresrv/   calibre Content server client (Digest/Basic login, list, remove to recycle bin)
internal/enrich/       Open Library / Google Books lookups and blurbs
internal/llm/          OpenAI-compatible client, Claude client (anthropic-sdk-go), prompts, verdict parser
internal/analyzer/     rating worker, token-per-hour cap, model chain, automatic rating
internal/deepread/     Deep Scans (whole-book reading, checks, the Deep Scan machine)
internal/epub/         EPUB text in reading order
internal/content/      the content items, rules and presets
internal/aitools/      model-update checks and the speed test
internal/collections/  AI-filled collections and weekly ideas
internal/seasons/      seasonal shelves
internal/discover/     Discover lists (New York Times, Open Library)
internal/events/       free-book event lists (reading pages and "Load more"), archiving
internal/suggest/      Suggested Reads and the taste profile
internal/genres/       genres from Calibre tags; internal/genrefill/ fills missing ones with AI
internal/titles/       tidy titles and series numbers
internal/covers/       covers from Calibre and Open Library, thumbnails
internal/delivery/     Send-to-Kindle SMTP + best-file picker
internal/push/         phone notifications (Web Push)
internal/tunnel/       supervises the bundled cloudflared connector (remote access)
internal/ollama/       Ollama discovery, model downloads with progress, GPU fit, loaded models, updates
internal/sysinfo/      container CPU / memory (cgroup v2), disk and DB size
internal/updates/      NovelCheck's own update check; internal/version/ the build's version
internal/safe/         panic recovery for background work
internal/safemode/     safe mode (flag, crash-loop detection, leaving it)
internal/cli/          maintenance commands (novelcheck help)
internal/api/          chi router, middleware, handlers
web/static/            index.html, Tailwind CSS, JS modules, manifest.json, sw.js, icons, vendored SortableJS, JSZip, QR code and ZXing
```

## Thanks

NovelCheck is built around [Calibre](https://calibre-ebook.com) by Kovid Goyal: it reads Calibre's library and removes books through Calibre's own Content server. It works well alongside [Calibre-Web](https://github.com/janeczku/calibre-web) for reading and browsing that same library.

## License

NovelCheck is released under the [MIT License](LICENSE). The libraries bundled in `web/static/vendor` keep their own licenses: [SortableJS](https://github.com/SortableJS/Sortable) (MIT), [JSZip](https://github.com/Stuk/jszip) (MIT or GPLv3), [QR Code Generator](https://github.com/kazuhikoarase/qrcode-generator) by Kazuhiko Arase (MIT) and [ZXing](https://github.com/zxing-js/library) (Apache-2.0, see `zxing.LICENSE.txt`). The [OpenDyslexic](https://github.com/antijingoist/opendyslexic) font in `web/static/fonts` is under the SIL Open Font License 1.1 (`OFL.txt`).
