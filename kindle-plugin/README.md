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
