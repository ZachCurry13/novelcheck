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

1. In KOReader's top menu, open the tools tab (the wrench), then **NovelCheck >
   Connect to NovelCheck...** and fill in:
   - **Address**: where you open NovelCheck, like `https://novelcheck.example.com`.
   - **Username**: your NovelCheck name.
   - **Sync code**: from NovelCheck, **Settings > KOReader setup > Reading sync**.
2. Open **NovelCheck > Nightstand**. Tap a book to download it into a
   **NovelCheck** folder inside KOReader's home folder, next to your other books.

3. **NovelCheck > Report books on this device** sends NovelCheck the list of books
   in the home folder (including the NovelCheck folder) (file names, formats and sizes, never folder names).
   **Device name...** sets what NovelCheck calls this device. Settings > Reading
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

## On this device

- **NovelCheck > On this device** lists the books with progress; tap one to delete it (it asks first).
- **Delete finished books...** lists the finished ones and deletes them after you confirm.
- **Download all Nightstand** downloads every Nightstand book that is not on the device yet.
- **About** shows the plugin version.

## Automatic and updating (NovelCheck 1.39 or newer)

- **NovelCheck > Automatic:** when the e-reader gets online (at most every 6 hours) the plugin reports its books, so they appear on your Nightstand without any menu (switch on **Download Nightstand when online** to also fetch new books). Turn either off there.
- **NovelCheck > Update plugin** downloads the newest plugin files from your NovelCheck, checks them, swaps them in (old files are kept as `.bak`) and asks to restart KOReader. The plugin also mentions a new version once after connecting. If an update ever goes wrong, copy the folder from **Settings > KOReader plugin** over USB.

## One-tap setup (NovelCheck 1.40 or newer, plugin 1.40.0)

**NovelCheck > Set up everything** connects KOReader's Progress sync and Reading statistics sync to NovelCheck using the details already in the plugin. You do not need to add a Cloud storage (WebDAV) account or an OPDS catalog by hand. After it finishes, restart KOReader. **Sync reading statistics now** (same menu) runs the sync; **Automatic > Sync reading statistics when online** does it by itself.

## Matching and tidy names (NovelCheck 1.41 or newer, plugin 1.41.0)

The plugin now sends each book's real title and author, so NovelCheck matches your Kindle's books to your library far more often. In NovelCheck, **Settings > KOReader plugin > Match books on my e-readers** fixes the rest by hand. **NovelCheck > Tidy book names** (in KOReader) then shows your library's titles and authors for the matched books (display only; the files are not touched).

On a Kindle, KOReader's own Restart closes KOReader instead of reopening it; reopen it from KUAL. **NovelCheck > Set up everything** usually needs no restart.

## Search and add books (NovelCheck 1.45 or newer, plugin 1.45.0)

- **NovelCheck > Search library:** type any words of a title or author; tap a book to read what it is about (with its content rating), then **Download to this device** or **Add to Nightstand**.
- **NovelCheck > New in library:** the newest books, the same way.
- **Connect to NovelCheck...** has a fourth box for a **backup address**, tried when the main one can't be reached.

## Highlights and finishing a book (NovelCheck 1.83 or newer, plugin 1.54.0)

- **NovelCheck > Send highlights now** sends what you highlighted and the notes you wrote; **Automatic > Send highlights when online** does it by itself. They appear on the book's page in NovelCheck under **My highlights**. Only books NovelCheck knows are sent for; a book it doesn't have yet is tried again next time.
- At the end of a book a window asks for **stars**, marks the book finished (in NovelCheck and KOReader) and offers the **next book of the series** to download. **NovelCheck > Finish this book...** does it by hand while a book is open; **Automatic > Ask for stars at the end of a book** switches the window off.

## Copy my Kindle (NovelCheck 1.85 or newer, plugin 1.55.2)

- **NovelCheck > Copy my Kindle to NovelCheck now** looks through the whole Kindle (every folder, not only what KOReader opened) for e-book files (EPUB, PDF, MOBI, AZW, AZW3, KFX and more) and sends the ones NovelCheck doesn't have yet. **Automatic > Copy my Kindle's books when online** does it by itself.
- Files are copied exactly as they are, protected ones too; nothing is unlocked or changed. NovelCheck keeps every file in its `kindle-backup` folder and adds the ones that are not locked to Calibre (unless Calibre already has that title). Logs and other files that only look like books are left out.
- One book at a time; if the connection drops it carries on next time.

## Back up the whole Kindle (NovelCheck 1.87 or newer, plugin 1.56.0)

- **NovelCheck > Back up my Kindle now** copies the Kindle's user storage to NovelCheck: KOReader and its plugins and settings (your reading progress, highlights and statistics too), KUAL and other launchers, fonts, dictionaries and your books. Hidden and system folders, caches and update downloads are left out. **Automatic > Back up my whole Kindle when online** does it once a day.
- To restore on a new or reset Kindle (jailbroken, KOReader not needed first): in NovelCheck open **Library cleanup → Books copied from Kindles → Download everything (ZIP)**, plug the Kindle into a computer and unzip the ZIP into its top folder. The Kindle's own system and your Amazon account are not part of the backup.

## Crash logs (plugin 1.57.2)

- **NovelCheck > Delete the Kindle's crash logs...** deletes the error reports the Kindle's own software leaves in its top folder (names with `_crash_` in them, such as `pillowd_..._crash_...`). It asks first and touches nothing else. The backup skips them.

## Developer tools (NovelCheck 1.50 or newer, plugin 1.50.0)

**NovelCheck > Developer tools** helps when something doesn't work: **Run self-test** checks the connection, sign-in and the plugin's files; **View log** shows every menu tap, request and error; **Send log to NovelCheck** sends the log to Settings > Developer > Bug reports (your sync code is removed), **Send log with a note...** adds what you were doing, and **Send errors automatically** (on by default) sends the log by itself, at most every ten minutes, after an error. **Run self-test** sends its result too. Times in the log are UTC. If the plugin doesn't appear in KOReader at all, look for `LOAD_ERROR.txt` in the plugin's folder, or send KOReader's `crash.log`.
