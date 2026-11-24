# Backing up and restoring NovelCheck

NovelCheck keeps everything it knows (ratings, Nightstands, settings, reading, who may do what) in **one file**, `novelcheck.db`, in its data folder. Your books are not in it: they stay in Calibre, Audiobookshelf and on the e-readers.

## The daily copy

Every day (the first a few minutes after NovelCheck starts) it writes a clean copy to the `backups` folder of its data folder, named `novelcheck-YYYY-MM-DD.db`, and keeps the last seven. **Health** (in the menu) lists them, has **Back up now**, and lets the main admin download one.

A copy on the same disk does not help if the disk fails. Download one now and then (Health → Download) and keep it somewhere else.

## Restoring

1. Stop NovelCheck.
2. In its data folder, rename the current `novelcheck.db` (keep it), and put the backup there as `novelcheck.db`.
3. Start NovelCheck. Everything is as it was on that day. (Anything done since is lost: ratings made, books added to Nightstands.)

If NovelCheck is on TrueNAS, the data folder is the dataset you gave it for `/data`.

## What a backup holds

Everyone's settings and sign-in details (passwords are stored hashed, not readable). That is why only the main admin can download one.
