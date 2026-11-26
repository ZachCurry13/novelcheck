# NovelCheck plugin for KOReader

A KOReader plugin for NovelCheck: see your **Nightstand** list on the e-reader,
tap a book to download it, and report which books are on the device (NovelCheck
1.36 or newer for that part); the plugin only needs your
NovelCheck address, username and sync code.

## Install

1. Copy the folder `novelcheck.koplugin` (the one with `main.lua` in it) into
   KOReader's plugins folder:
   - Kindle: `/mnt/us/koreader/plugins/`
   - Kobo: `/mnt/onboard/.adds/koreader/plugins/`
   - Android: `<koreader folder>/plugins/`
2. Restart KOReader.

## Use

When a book leaves the device (plugin 1.63.0), the plugin asks: keep it in NovelCheck, remove it from my library, or remove it and ask for it to be deleted (the main admin decides). Its rating stays either way.

The menu (plugin 1.62.0): **Search**, **Nightstand**, **Download Nightstand**, **New**, **On this e-reader**, **Finish this book…**, **Sync now** (the book report, highlights, reading statistics and the tasks from NovelCheck at once), **Kindle care** (back up, tidy, delete finished books, crash logs), **Automatic** and **Settings** (set up, connect, device name, update, stop, developer tools, about).

1. In KOReader's top menu, tap the **NovelCheck icon** (a book with a check mark, near the end of
   the row of icons; the same tab is in the book's menu), then **Settings > Connect…** and fill in:
   - **Address**: where you open NovelCheck, like `https://novelcheck.example.com`.
   - **Username**: your NovelCheck name.
   - **Sync code**: from NovelCheck, **Settings > KOReader setup > Reading sync**.
2. Open **NovelCheck icon > Nightstand**. Tap a book to download it into a
   **NovelCheck** folder inside KOReader's home folder, next to your other books.

3. **NovelCheck icon > Sync now** (or the automatic report when online) sends NovelCheck the list of books
   in the home folder (including the NovelCheck folder) (file names, formats and sizes, never folder names).
   **Settings > Device name…** sets what NovelCheck calls this device. Settings > Reading
   sync shows the result.

Only books in your Nightstand that have a file NovelCheck can send are listed;
kids' content rules apply as everywhere else.

## If something goes wrong

The messages say what failed: "did not accept the username or sync code" means a
typo in those two boxes; "Could not reach NovelCheck" with a number or text is
the address or the network. Tell whoever looks after NovelCheck the exact
message.

## Easiest install (NovelCheck 1.38 or newer)

Open **Settings > KOReader plugin** in NovelCheck and press **Download the plugin**. The zip is already connected to your account, so there is nothing to type on the e-reader: unzip it, copy the `novelcheck.koplugin` folder into `koreader/plugins/`, and restart KOReader. The same folder works for updates (replace the old one). Do not share that zip: it holds your sync code.

## On this e-reader

- **NovelCheck icon > On this e-reader** lists the books with progress; tap one to delete it (it asks first).
- **Delete finished books...** lists the finished ones and deletes them after you confirm.
- **Download Nightstand** downloads every Nightstand book that is not on the device yet.
- **About** shows the plugin version.

## Automatic and updating (NovelCheck 1.39 or newer)

- **NovelCheck icon > More > Settings > Automatic jobs:** when the e-reader gets online (at most every 6 hours) the plugin reports its books, so they appear on your Nightstand without any menu (switch on **Download Nightstand** to also fetch new books). Turn either off there.
- **NovelCheck icon > More > Settings > Update** downloads the newest plugin files from your NovelCheck, checks them, swaps them in (old files are kept as `.bak`) and asks to restart KOReader. The plugin also mentions a new version once after connecting. If an update ever goes wrong, copy the folder from **Settings > KOReader plugin** over USB.

## One-tap setup (NovelCheck 1.40 or newer, plugin 1.40.0)

**NovelCheck icon > More > Settings > Set up** connects KOReader's Progress sync and Reading statistics sync to NovelCheck using the details already in the plugin. You do not need to add a Cloud storage (WebDAV) account or an OPDS catalog by hand. After it finishes, restart KOReader. **Sync reading statistics now** (same menu) runs the sync; **Automatic > Sync reading statistics when online** does it by itself.

## Matching and tidy names (NovelCheck 1.41 or newer, plugin 1.41.0)

The plugin now sends each book's real title and author, so NovelCheck matches your Kindle's books to your library far more often. In NovelCheck, **Settings > KOReader plugin > Match books on my e-readers** fixes the rest by hand. **NovelCheck icon > More > Organize > Show real titles** (in KOReader) then shows your library's titles and authors for the matched books (display only; the files are not touched).

On a Kindle, KOReader's own Restart closes KOReader instead of reopening it; reopen it from KUAL. **NovelCheck icon > More > Settings > Set up** usually needs no restart.

## Search and add books (NovelCheck 1.45 or newer, plugin 1.45.0)

- **NovelCheck > Search:** type any words of a title or author; tap a book to read what it is about (with its content rating), then **Download to this device** or **Add to Nightstand**.
- **NovelCheck > New:** the newest books, the same way.
- **Connect to NovelCheck...** has a fourth box for a **backup address**, tried when the main one can't be reached.

## Highlights and finishing a book (NovelCheck 1.83 or newer, plugin 1.54.0)

- **NovelCheck icon > Sync now** sends what you highlighted and the notes you wrote; **Automatic > Highlights** does it by itself. They appear on the book's page in NovelCheck under **My highlights**. Only books NovelCheck knows are sent for; a book it doesn't have yet is tried again next time.
- At the end of a book a window asks for **stars**, marks the book finished (in NovelCheck and KOReader) and offers the **next book of the series** to download. **NovelCheck > Finish this book...** does it by hand while a book is open; **Automatic > Ask for stars at the end of a book** switches the window off.

## Copy my Kindle (NovelCheck 1.85 or newer, plugin 1.55.2)

- The older books-only copy (no longer in the menu since plugin 1.62.0; **Back up** covers it, and NovelCheck's Kindle tasks can still ask for it) looks through the whole Kindle (every folder, not only what KOReader opened) for e-book files (EPUB, PDF, MOBI, AZW, AZW3, KFX and more) and sends the ones NovelCheck doesn't have yet. **Automatic > Copy my Kindle's books when online** does it by itself.
- Files are copied exactly as they are, protected ones too; nothing is unlocked or changed. NovelCheck keeps every file in its `kindle-backup` folder and adds the ones that are not locked to Calibre (unless Calibre already has that title). Logs and other files that only look like books are left out.
- One book at a time; if the connection drops it carries on next time.

## Back up the whole Kindle (NovelCheck 1.87 or newer, plugin 1.56.0)

- **NovelCheck > Kindle care > Back up now** copies the Kindle's user storage to NovelCheck: KOReader and its plugins and settings (your reading progress, highlights and statistics too), KUAL and other launchers, fonts, dictionaries and your books. Hidden and system folders, caches and update downloads are left out. **Automatic > Back up my whole Kindle when online** does it once a day.
- To restore on a new or reset Kindle (jailbroken, KOReader not needed first): in NovelCheck open **Library cleanup → Books copied from Kindles → Download everything (ZIP)**, plug the Kindle into a computer and unzip the ZIP into its top folder. The Kindle's own system and your Amazon account are not part of the backup.

## Tasks from the app (NovelCheck 1.91 or newer, plugin 1.59.0)

- In NovelCheck, **Library cleanup → Kindle tasks** asks this Kindle to back itself up, tidy its books, delete its crash logs, delete books (only ones NovelCheck holds a copy of), move books into a folder, or download library books. The plugin does them when the Kindle next connects (every 15 minutes at most) or when you press **NovelCheck > Do the tasks from NovelCheck now**, and reports each one back.

## Crash logs (plugin 1.57.2)

- **NovelCheck > Kindle care > Delete crash logs…** deletes the error reports the Kindle's own software leaves in its top folder (names with `_crash_` in them, such as `pillowd_..._crash_...`). It asks first and touches nothing else. The backup skips them. **Automatic > Delete the Kindle's crash logs when online** does it by itself, without asking (on by default; the crash logs are only error reports).

## Developer tools (NovelCheck 1.50 or newer, plugin 1.50.0)

**NovelCheck > Settings > Developer tools** helps when something doesn't work: **Run self-test** checks the connection, sign-in and the plugin's files; **View log** shows every menu tap, request and error; **Send log to NovelCheck** sends the log to Settings > Developer > Bug reports (your sync code is removed), **Send log with a note...** adds what you were doing, and **Send errors automatically** (on by default) sends the log by itself, at most every ten minutes, after an error. **Run self-test** sends its result too. Times in the log are UTC. If the plugin doesn't appear in KOReader at all, look for `LOAD_ERROR.txt` in the plugin's folder, or send KOReader's `crash.log`.


## The launcher card (1.72.0)

Tapping NovelCheck's icon opens one card: who you are and when it last synced, a **search** bar, the book that is next ("Reading: …"
or "Up next: …", and the next two titles) with one big button, **Read it** (or **Download** if it is not on the Kindle yet), then
**Nightstand** (with how many), **New** and **My Kindle** (the books on it with progress), the last backup, and **Sync now**,
**More…** and **Close**. **More** is the whole list: **Get my Nightstand
books**, **Keep safe** (Back up now, Restore), **Organize** (Move my books into one folder, Show real titles (display only),
Undo the last move), **Free up space** (Delete finished books, Delete crash logs) and **Settings**. Nothing under Organize
deletes anything: moving a book needs a backup of it first and can be undone, and real titles change only what is shown.
If the card cannot open, the icon shows that list instead.

## Where it lives (1.67.0)

NovelCheck has its own icon in KOReader's top bar. One tap shows one list: **Nightstand, New, Search, Download Nightstand,
On this e-reader, Finish this book, Sync now**, then two small menus, **Back up and tidy** and **Settings** (with
**Automatic jobs** inside). It is added by changing KOReader's top menu when the plugin loads (`tab.lua`). If that cannot be
done, or you keep your own top-menu order that leaves it out, the same list is in a **NovelCheck** entry in the Tools tab
(the wrench), as before. The icon file is written into `koreader/icons/` and `resources/icons/mdlight/`.

## Search outside the library (1.71.0)

Search lists library matches first. **Not here? Look it up online** (also shown alone when nothing matches) finds the book on
Open Library; pick the one you meant and NovelCheck rates it as Check a book does, which can take a minute. The book's page shows
the rating and summary, with **Download** and **Add to Nightstand** if the library has it, or "Not available in the library yet."
with **Request the ebook** and **Request the audiobook** (the family wishlist) if not. A kid needs a parent's OK for this (the AI
features switch).
