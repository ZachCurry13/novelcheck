# Installing NovelCheck on TrueNAS SCALE (step by step)

This guide is for TrueNAS SCALE **24.10 (Electric Eel) or newer**. It takes about 10 minutes and happens entirely in web pages. You don't need a terminal or any command line.

You'll need:
- Your TrueNAS web address (for example `http://192.168.1.50`).
- To know roughly where your **Calibre library** is (for example somewhere inside your `plex` share). You'll pick the exact folder later, inside NovelCheck.

---

## Step 1: Make a folder for NovelCheck's own data

NovelCheck keeps its settings, users, and book ratings in one small file. It needs a home, ideally on your fast (SSD) pool.

1. In TrueNAS, open **Datasets** from the left menu.
2. Click the pool you want to use (your SSD pool if you have one), then **Add Dataset**.
3. Enter:
   - **Name:** `novelcheck`
   - **Dataset Preset:** `Apps`. This lets apps save files there.
4. Click **Save**.
5. Write down the path shown at the top of the dataset's details, for example `/mnt/ssd/novelcheck`. You'll need it in Step 3.

## Step 2: Note the folder that contains your books

You don't need the exact Calibre folder. A **parent** folder is fine, for example your whole `plex` share at `/mnt/red14/plex`. In Step 5 you'll click through it inside NovelCheck to the exact library (for example `books/Clean Library`), or let NovelCheck find it for you.

1. In **Datasets**, find the dataset that holds your books and write down its path, for example `/mnt/red14/plex`.
2. **Permissions:** NovelCheck only *reads* this folder, it never changes your books. If Plex or Calibre already run as TrueNAS apps, it's usually readable already. If NovelCheck later can't see it, see [Troubleshooting](#troubleshooting).

## Step 3: Install the app

1. Open **Apps** from the left menu, then click **Discover Apps** (top right).
2. Click the **⋮** (three dots) menu at the top right and choose **Install via YAML**.
3. **Name:** `novelcheck`
4. Paste the text below into the big box, then change the **two lines marked `👈 CHANGE`**:

```yaml
services:
  novelcheck:
    image: ghcr.io/zachcurry13/novelcheck:latest
    container_name: novelcheck
    restart: unless-stopped
    user: "568:568"
    ports:
      - "30080:8080"
    environment:
      TZ: America/Chicago
    volumes:
      - /mnt/ssd/novelcheck:/data            # 👈 CHANGE left side to your Step 1 path
      - /mnt/red14/plex:/calibre:ro          # 👈 CHANGE left side to your Step 2 path
```

   - Only change the part **before** the `:` on those lines. Leave `:/data` and `:/calibre:ro` exactly as they are.
5. Click **Save**. TrueNAS downloads NovelCheck, which takes a minute or two. Wait until the app shows **Running**.

## Step 4: Create your admin account

1. In your browser, go to `http://YOUR-TRUENAS-IP:30080`, for example `http://192.168.1.50:30080`.
2. The first time, NovelCheck shows **Welcome to NovelCheck**. Choose your admin username and password, then click **Create admin account**.
   - Do this right after installing. Until an admin account exists, anyone on your network who opens the page could create it.
3. A short **How to** guide opens. Click through it, or skip it. You can reopen it any time from **❔ Help** at the top or **How to use NovelCheck** at the bottom.

## Step 5: First-time setup (inside NovelCheck)

Open the **Admin** tab.

1. **Pick your Calibre library:** in the **Calibre Library** box, click **Find libraries automatically** and then **Use this** next to your library. Or click **Browse folders…**, click through to it (for example `books` → `Clean Library`), and click **Use this**. Folders that are Calibre libraries show a 📚 **Calibre library** badge. NovelCheck then loads your books, which takes a few seconds.
2. **AI settings** (the AI that rates books), in the **LLM Analysis Engine** box:
   - **Using OpenAI:** paste your API key. Leave the model as `gpt-4o-mini`.
   - **Using Gemini:** set the base URL to `https://generativelanguage.googleapis.com/v1beta/openai`, paste your Gemini API key, and set the model to a current Gemini **Flash** model name from Google AI Studio (for example `gemini-1.5-flash`).
   - Click **Save settings**.
3. Set **Batch size** to something small like `5`, then click **Analyze batch**. Watch "Tokens this hour" and "Spent to date" to see real costs before running bigger batches.
4. **Accounts:** under **Users & Content Rules**, add everyone else:
   - **Editor**: for a spouse or co-parent. They can check and correct ratings, run scans, import Kindles, and manage kids' accounts, but can't change AI or email settings, API keys, or backups.
   - **Restricted (Kid)**: starts with the strictest content rules on. Untick any you don't need.
5. **Send-to-Kindle (optional):** fill in the SMTP box. With Gmail, use host `smtp.gmail.com`, port `587`, your Gmail address, and a Gmail *App Password*. Click **Send test email**. Then add that Gmail address to Amazon's **Approved Personal Document E-mail List**.

## Step 6: Put it on your phone's home screen

- **iPhone (Safari):** open the NovelCheck address, tap **Share**, then **Add to Home Screen**.
- **Android (Chrome):** open the address and tap **Install app** (or ⋮ → **Add to Home screen**).

To use NovelCheck away from home, create a tunnel in **Cloudflare Zero Trust → Networks → Tunnels**, then install the **Cloudflared** app from TrueNAS **Discover Apps** and paste the tunnel token into it. In Cloudflare, point your tunnel's public hostname to `http://YOUR-TRUENAS-IP:30080`.

---

## Updating NovelCheck

When a new version comes out, admins and editors see a green **"NovelCheck x.y.z is available"** banner in the app, and **What's new** (bottom of every page) shows the release notes.

To update: go to **Apps**, click **novelcheck**, and click **Update** if TrueNAS offers it. If it doesn't, click **Edit** and then **Save** without changing anything. TrueNAS re-downloads the `latest` image. Your books, ratings, and users are kept.

Admins can turn the update check off under **Admin → App Updates**.

## Backups

In NovelCheck: **Admin → Download novelcheck.db**. Keep that file somewhere safe. It contains passwords (scrambled) and your API keys.

## Troubleshooting

| Problem | Fix |
|---|---|
| App won't start, logs say **permission denied** on `/data` | The Step 1 dataset must use the **Apps** preset. Or: **Datasets → novelcheck → Permissions → Edit** and give the **apps** user (568) *Modify* access. |
| **Browse folders…** shows nothing, or says **permission denied** | Give the **apps** user *Read* access to the Step 2 dataset: **Datasets → that dataset → Permissions → Edit → Add Item → User: apps → Read**, and tick **Apply permissions recursively**. |
| **Find libraries automatically** finds nothing | Check the Step 3 path points at the folder that *contains* your library somewhere inside it. The search goes up to 5 folders deep; use **Browse folders…** for anything deeper. |
| Can't open `http://…:30080` | Another app may already use port 30080. Edit the app and change `30080` to another number like `30081`. |
| Forgot a password | Another admin can reset it under **Admin → Users & Content Rules → Reset password**. Editors can reset kids' passwords the same way under **Manage**. |
| Locked out of the only admin account | Last resort, which erases all NovelCheck data: stop the app, delete `novelcheck.db` from the Step 1 dataset, start the app, and create a new admin in the browser (Step 4). |
| See what's going on | **Apps → novelcheck → Logs** (the icon on the container row). |
