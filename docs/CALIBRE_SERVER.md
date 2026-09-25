# One-click removal from Calibre

NovelCheck can remove the books your Hide filters catch straight from your Calibre library. It does this by asking **Calibre's Content server** to remove them, the same as selecting them in Calibre and pressing Delete. Calibre updates its own catalog and moves the files to its **recycle bin**, so a mistake can be undone in Calibre.

NovelCheck itself still only *reads* your library. It never deletes files directly.

## Step 1: Turn on Calibre's Content server with a login

Do this in the Calibre program that manages your library: Calibre on your computer, or the Calibre app on TrueNAS (open its web page to see the Calibre window).

1. In Calibre, click **Preferences → Sharing over the net** (or the **Connect/share** button, then **Start Content server**).
2. On the **Main** tab:
   - Note the **port** (for example `8080`, or `8081` in the TrueNAS Calibre app).
   - Tick **Require username and password to access the Content server**.
   - Tick **Run server automatically when calibre starts**.
3. On the **User accounts** tab, click **Add user**:
   - Username: `novelcheck`
   - Password: choose one and write it down.
   - Make sure the user is **allowed to make changes** (not read-only). Tick **Allow novelcheck to make changes (i.e. grant write access)** if you see it.
4. Click **Start server** (if it isn't already running), then **OK**.

## Step 2: Connect NovelCheck

1. In NovelCheck, open **Admin → Delivery & Services → Calibre Library → One-click removal**.
2. **Content server address:** your computer's or TrueNAS's IP plus the port, for example `http://192.168.1.50:8081`.
3. **Username** and **Password:** the ones from Step 1.
4. Click **Test & save**. NovelCheck shows your Calibre libraries. If you have more than one, pick the one NovelCheck reads.

## Step 3: Remove books

1. In the **Library**, tick the **Hide** boxes for the content you want gone.
2. Click **Remove hidden books from Calibre…** and **read the list carefully**. Books a parent marked OK are never included.
3. Click **Remove these books from Calibre** and confirm.

Before removing anything, NovelCheck checks each book's title with Calibre. If they don't match (for example, the Content server is serving a different library), it stops and removes nothing. Afterwards it re-syncs Calibre automatically.

To undo, open Calibre and use **Remove books → Restore recently deleted** (the recycle bin).

## Troubleshooting

- **"can't reach the calibre Content server":** Calibre isn't running, the server isn't started, or the address or port is wrong. If Calibre runs on a computer, that computer must be on (and Calibre open) when you remove books.
- **"calibre didn't accept the username or password":** re-enter them. Passwords are case-sensitive.
- **"does not have permission to make changes":** in Calibre's **User accounts**, allow that user to make changes.
- **"stopped: calibre's book #… is …":** the Content server is sharing a different library than the one NovelCheck reads. Pick the right library in **One-click removal**, or check the Calibre folder in NovelCheck's **Calibre Library** settings.
