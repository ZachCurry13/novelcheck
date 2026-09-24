# Installing NovelCheck on TrueNAS SCALE (step by step)

This guide is for TrueNAS SCALE **24.10 (Electric Eel) or newer**. It takes about 10 minutes and happens entirely in the TrueNAS web page. You don't need a terminal or any command line.

You'll need:
- Your TrueNAS web address (for example `http://192.168.1.50`).
- The folder where your **Calibre library** lives, meaning the folder that contains a file called `metadata.db`.

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

## Step 2: Find your Calibre library path

1. Still in **Datasets**, find the dataset holding your Calibre library.
2. Write down the full path to the folder that contains `metadata.db`, for example `/mnt/tank/media/calibre`.
   - Not sure? It's the folder Calibre calls your "library". If Calibre runs as a TrueNAS app, open **Apps → calibre → Edit** and look at its storage settings for the library folder.
3. **Permissions:** NovelCheck only *reads* this folder. If Calibre already runs as a TrueNAS app, you're fine. If NovelCheck later says "Calibre: not mounted" or shows 0 books, see [Troubleshooting](#troubleshooting).

## Step 3: Install the app

1. Open **Apps** from the left menu, then click **Discover Apps** (top right).
2. Click the **⋮** (three dots) menu at the top right and choose **Install via YAML**.
3. **Name:** `novelcheck`
4. Paste the text below into the big box, then change the **three lines marked `👈 CHANGE`**:

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
      NOVELCHECK_ADMIN_USER: admin
      NOVELCHECK_ADMIN_PASSWORD: "pick-a-strong-password"   # 👈 CHANGE (8+ characters)
      TZ: America/Chicago
    volumes:
      - /mnt/ssd/novelcheck:/data                           # 👈 CHANGE left side to your Step 1 path
      - /mnt/tank/media/calibre:/calibre:ro                 # 👈 CHANGE left side to your Step 2 path
```

   - Only change the part **before** the `:` on the volume lines. Leave `:/data` and `:/calibre:ro` exactly as they are.
   - Keep the quotes around the password.
5. Click **Save**. TrueNAS downloads NovelCheck, which takes a minute or two. Wait until the app shows **Running**.

## Step 4: Sign in

1. In your browser, go to `http://YOUR-TRUENAS-IP:30080`, for example `http://192.168.1.50:30080`.
2. Sign in as **admin** with the password you picked in Step 3.
3. Optional: change your password under **Profile → Change password**. The password in the YAML is only used the very first time NovelCheck starts, so changing it here is safe.

## Step 5: First-time setup (inside NovelCheck)

Open the **Admin** tab.

1. **LLM Analysis Engine:** This is the AI that rates books.
   - **Using OpenAI:** paste your API key. Leave the model as `gpt-4o-mini`.
   - **Using Gemini:** set the base URL to `https://generativelanguage.googleapis.com/v1beta/openai`, paste your Gemini API key, and set the model to a current Gemini **Flash** model name from Google AI Studio (for example `gemini-1.5-flash`).
   - Click **Save settings**.
2. Click **Sync Calibre now**. After a few seconds the status line should show your book count.
3. Set **Books per batch** to something small like `5`, then click **Analyze batch**. Watch the "Tokens this hour" and "Spent to date" numbers to see real costs before running bigger batches.
4. **Kids' accounts:** under **Users & Content Rules**, add a user with role **Restricted (Kid)**. Restricted accounts start with the strictest rules on, and you can untick any you don't need.
5. **Send-to-Kindle (optional):** fill in the SMTP box. With Gmail, use host `smtp.gmail.com`, port `587`, your Gmail address, and a Gmail *App Password*. Click **Send test email**. Then add that Gmail address to Amazon's **Approved Personal Document E-mail List**.

## Step 6: Put it on your phone's home screen

- **iPhone (Safari):** open the NovelCheck address, tap **Share**, then **Add to Home Screen**.
- **Android (Chrome):** open the address and tap **Install app** (or ⋮ → **Add to Home screen**).

To use NovelCheck away from home, create a tunnel in **Cloudflare Zero Trust → Networks → Tunnels**, then install the **Cloudflared** app from TrueNAS **Discover Apps** and paste the tunnel token into it. In Cloudflare, point your tunnel's public hostname to `http://YOUR-TRUENAS-IP:30080`.

---

## Updating NovelCheck

When a new version is published, go to **Apps**, click **novelcheck**, and click **Update** if TrueNAS offers it. If it doesn't, click **Edit** and then **Save** without changing anything. TrueNAS re-downloads the `latest` image. Your data in the Step 1 folder is kept.

## Backups

In NovelCheck: **Admin → Download novelcheck.db**. Keep that file somewhere safe. It contains passwords (scrambled) and your API keys.

## Troubleshooting

| Problem | Fix |
|---|---|
| App won't start, logs say **permission denied** on `/data` | The Step 1 dataset must use the **Apps** preset. Or: **Datasets → novelcheck → Permissions → Edit** and give the **apps** user (568) *Modify* access. |
| **Calibre: not mounted** or 0 books | Check that the Calibre path in Step 3 is the folder that directly contains `metadata.db`. Then give the **apps** user *Read* access to that dataset (**Datasets → your Calibre dataset → Permissions → Edit → Add Item → User: apps → Read**). |
| Can't open `http://…:30080` | Another app may already use port 30080. Edit the app and change `30080` to another number like `30081`. |
| Forgot the admin password | Easiest: have another admin account reset it under **Admin → Users & Content Rules → Reset password**. Last resort (erases all NovelCheck data): stop the app, delete `novelcheck.db` from the Step 1 dataset, and start the app again. The password in the YAML is then used to create a fresh admin. |
| See what's going on | **Apps → novelcheck → Logs** (the icon on the container row). |

> Note: `NOVELCHECK_ADMIN_PASSWORD` is only used the very first time NovelCheck starts, to create the admin account. Changing it later doesn't change an existing password.
