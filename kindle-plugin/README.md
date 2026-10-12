# NovelCheck plugin for KOReader

Step 1 of a KOReader plugin for NovelCheck: see your **Up Next** list on the
e-reader and tap a book to download it. Later steps add reporting what is on
the device. It needs NovelCheck 1.33 or newer; the plugin only needs your
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

Only books in your Up Next that have a file NovelCheck can send are listed;
kids' content rules apply as everywhere else.

## If something goes wrong

The messages say what failed: "did not accept the username or sync code" means a
typo in those two boxes; "Could not reach NovelCheck" with a number or text is
the address or the network. Tell whoever looks after NovelCheck the exact
message.
