# NovelCheck plugin for KOReader

A KOReader plugin for NovelCheck: see your **Up Next** list on the e-reader,
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
   - **Sync code**: from NovelCheck, **Profile > KOReader setup > Reading sync**.
2. Open **NovelCheck > Up Next**. Tap a book to download it into KOReader's
   download folder (or home folder).

3. **NovelCheck > Report books on this device** sends NovelCheck the list of books
   in the download folder (file names, formats and sizes, never folder names).
   **Device name...** sets what NovelCheck calls this device. Profile > Reading
   sync shows the result.

Only books in your Up Next that have a file NovelCheck can send are listed;
kids' content rules apply as everywhere else.

## If something goes wrong

The messages say what failed: "did not accept the username or sync code" means a
typo in those two boxes; "Could not reach NovelCheck" with a number or text is
the address or the network. Tell whoever looks after NovelCheck the exact
message.

## Easiest install (NovelCheck 1.38 or newer)

Open **Profile > KOReader plugin** in NovelCheck and press **Download the plugin**. The zip is already connected to your account, so there is nothing to type on the e-reader: unzip it, copy the `novelcheck.koplugin` folder into `koreader/plugins/`, and restart KOReader. The same folder works for updates (replace the old one). Do not share that zip: it holds your sync code.

## On this device

- **NovelCheck > On this device** lists the books with progress; tap one to delete it (it asks first).
- **Delete finished books...** lists the finished ones and deletes them after you confirm.
- **Download all Up Next** downloads every Up Next book that is not on the device yet.
- **About** shows the plugin version.

## Automatic and updating (NovelCheck 1.39 or newer)

- **NovelCheck > Automatic:** when the e-reader gets online (at most every 6 hours) the plugin reports its books, so they appear in Up Next without any menu (switch on **Download Up Next when online** to also fetch new books). Turn either off there.
- **NovelCheck > Update plugin** downloads the newest plugin files from your NovelCheck, checks them, swaps them in (old files are kept as `.bak`) and asks to restart KOReader. The plugin also mentions a new version once after connecting. If an update ever goes wrong, copy the folder from **Profile > KOReader plugin** over USB.

## One-tap setup (NovelCheck 1.40 or newer, plugin 1.40.0)

**NovelCheck > Set up everything** connects KOReader's Progress sync and Reading statistics sync to NovelCheck using the details already in the plugin. You do not need to add a Cloud storage (WebDAV) account or an OPDS catalog by hand. After it finishes, restart KOReader. **Sync reading statistics now** (same menu) runs the sync; **Automatic > Sync reading statistics when online** does it by itself.
