# Changelog

All notable changes to NovelCheck. Newest first. Each `## [x.y.z]` section
becomes the release notes for that version and is shown in the app under
**What's new**.

## [1.14.2]

### Fixed
- **No way to confirm duplicates were removed**: after **Remove selected copies**, NovelCheck now re-reads your Calibre library before answering, reloads the list, and says what happened (for example "✓ Removed 12 copies. Calibre now has no duplicates"). A new **🔄 Check again** button re-reads Calibre and refreshes the list any time, with the time it last read Calibre.
- **Duplicates missed for entries without files**: two Calibre entries of the same book that had no book files weren't recognised as duplicates. They are now (after the next Calibre read, or **Check again**).

## [1.14.1]

### Fixed
- **Errors you couldn't see or retry**: the Errors count on Usage didn't say what went wrong, and failed books were never tried again. A new **Rating errors** list (on Usage, and on Admin for editors too) groups failed books by reason, shows example titles and a plain explanation of each common error, and has **Retry** buttons for one group or **Retry all**. You can also **📋 Copy** or **🩺 Diagnose** a group.
- **Library → Rating failed**: the pepper filter can now show just the books that failed; **Show in Library** in the errors list opens it.

## [1.14.0]

### New
- **Model recommendations for your GPU**: the Ollama easy setup now works out how much GPU memory your Ollama server has and labels every model: **⭐ Best for your GPU**, **💪 Most powerful that fits**, **✓ Fits**, or **⚠️ Too big (slow)**. Without a GPU it recommends the small models and tells you if Ollama is running on the CPU only.
  - **🎮 Check my GPU** loads your biggest downloaded model for a moment to measure (Ollama can't report the GPU directly), then unloads it. Or just pick your GPU's memory size from the menu.
  - Downloaded models that are too big for the GPU get a **⚠️ slow** tag in the model order list.
  - New larger choices in the download menu: Qwen 2.5 3B, Gemma 3 12B, Qwen 2.5 14B, Gemma 3 27B and Qwen 2.5 32B.

## [1.13.0]

### New
- **Backup AI**: a new **Admin → Backup AI (optional)** section sets up a second AI that takes over when the main one fails: another Ollama on your network (the **Find Ollama** helper works here too) or a cloud AI like OpenAI or Claude. If the main server is switched off, NovelCheck goes straight to the backup instead of waiting on each model. The 🔔 bell tells you when the backup was used, and each AI's cost is counted at its own prices.
- **Public address check**: **Check everything** now loads your remote-access address from the internet and confirms NovelCheck answers, not just that the tunnel is connected. It explains what to fix (missing Public Hostname, wrong service address), or tells you the problem is your device's network when the site works from outside.

### Changed
- **Clearer "AI server is off" message**: Check everything says "Can't reach the AI server" with what to check, instead of a technical "dial tcp" error. AI diagnosis also uses the backup AI when the main one is down.

## [1.12.0]

### New
- **🩺 Diagnose with AI** (System page, admins): your connected AI reads NovelCheck's diagnostics (settings, recent problems, the worker and the remote-access log, never passwords or keys) and explains in plain words what's wrong and how to fix it. It also tells you whether it looks like a bug or something you can fix yourself.
- **Bug reports for GitHub**: every diagnosis comes with a ready-made bug report. **Open GitHub issue** opens the NovelCheck bug form with it filled in (and copies it, in case it's too long). Your web address is swapped for a placeholder. If the AI itself is what's broken, you still get the report with the diagnostics.
- **🩺 Diagnose shortcuts**: error pop-ups, 🔔 notifications and failed books have a **🩺 Diagnose** button that opens the diagnosis with the problem filled in.
- **Easy copying**: **📋 Copy** and **⬇ Download** on the Cloudflare connector log (now bigger, and it no longer loses your selection when it refreshes), the **Check everything** results, each notification (**📋 Copy all** too), error pop-ups and failed books. **Copy diagnostics** / **Download diagnostics** give the raw report.

## [1.11.1]

### Fixed
- **"context deadline exceeded" with Ollama**: NovelCheck gave every book only 2 minutes, which is too short on your own hardware while Ollama loads a model or when it runs on the CPU. The limit is now automatic: **10 minutes** for AI on your own network (Ollama, LM Studio…) and 2 minutes for cloud services. You can set your own under **Admin → LLM Analysis Engine → AI time limit per book**.
- **Clearer message when the AI is too slow**: it now names the model and suggests what to check (Usage → Ollama should say *100% GPU*; otherwise try a smaller model).
- **Check everything** waits up to 90 seconds for a local model to load instead of 10.

## [1.11.0]

### New
- **🌶️ The pepper scale**: books are now rated **0 to 5 peppers**: 0 No Romance, 1 Sweet Romance, 2 Romantic, 3 Steamy Closed-Door, 4 Explicit, 5 Very Explicit / Erotica-Level. The AI uses these exact descriptions and examples. **What do the peppers mean?** in the Library (and **About peppers** in each book) shows them all.
- **Pepper filter**: the Library can show books with a given number of peppers, older ratings, or books not rated yet.
- **Most peppers allowed** for kid accounts: new kids start at 0 (Young kids), 1 (Middle grade), 2 (Teens), 3 (Young adult) or no limit (Adults), and you can change it under **Admin → Users**. Existing kid accounts start with no limit, so nothing changes for them until you set one.
- **Edit rating** now asks for peppers.
- **Re-rate older books**: books rated before the pepper scale keep their old label (No Spice, Closed Door, Open Door) and show a **Re-rate them on the pepper scale** button in Admin. They stay in the library while they're re-rated, and hand-rated books are left alone. Until then a kid's pepper limit plays it safe: an old "No Spice" counts as up to 2 peppers, "Closed Door" as 3 and "Open Door" as 4.

## [1.10.0]

### New
- **Put your Ollama models in order**: the Ollama easy setup lists every downloaded model. Tick the ones to use and move them with **↑ / ↓**. **#1** rates every book; if it fails on a book, **#2** tries, then **#3**, and so on. One click saves them all, with no copy-paste.
- **Several fallback models for any provider**: the **Fallback model(s)** box takes a comma-separated list, tried in order. **Check everything** tests each one.

## [1.9.1]

### Fixed
- **"Send test email" tested the old password**: it used the last *saved* settings, so a new password typed without clicking **Save settings** was never tried. It now tests exactly what's on screen.
- **Browser filling in the wrong password**: Chrome and password managers could fill your NovelCheck login into the email username/password fields. Those fields are now marked so browsers leave them alone.
- **Gmail App Passwords with spaces**: the spaces Google shows ("abcd efgh ijkl mnop") are removed automatically.
- **Clearer Gmail error**: when Gmail rejects the login, NovelCheck now explains that Gmail needs an App Password (not your normal password) and your full Gmail address as the username. The SMTP box links straight to Google's App Password page.

## [1.9.0]

### New
- **Find duplicates**: **Library → Find duplicates** lists books that are in Calibre more than once (same title and author, even if the author is spelled slightly differently). For each copy it shows the Calibre number, formats and file size, and suggests which copy to keep: the best formats (EPUB first), then the most files, then the largest.
- **Remove duplicates in one click** (admins, with **One-click removal** turned on): tick the extra copies and click **Remove selected copies**. They go to Calibre's recycle bin, and NovelCheck never lets you remove every copy of a book. Without one-click removal, **Copy Calibre search** gives you a search to paste into Calibre. Editors can view the list.
- **File formats everywhere**: book cards show their formats (EPUB, AZW3, MOBI…) and a **⚠ 2× in Calibre** tag for duplicates. The book window lists each Calibre copy with all of its formats.
- **Format filter**: the Library can show only EPUB, AZW3, MOBI, KFX or PDF books, books with **2+ formats**, **Duplicates**, or books with **No file**.

## [1.8.1]

### Fixed
- **Ollama address without `http://`**: typing an address like `10.13.6.41:30068` broke book rating ("first path segment in URL cannot contain colon"). NovelCheck now fills in `http://` and Ollama's `/v1` for you, including for an address you already saved.
- **Find Ollama button missing**: with the TrueNAS Ollama app's port (30068), the AI provider showed as "Other" and hid the Ollama easy setup. Port 30068 is now recognized, and your saved address is filled in for **Find Ollama**.

### New
- **Kindles that show up as a "device"**: newer Kindles connect like a phone, which browsers can't open. The Import page now explains how to copy the Kindle's **documents** folder to the Desktop and import that instead.
- **Paste a list**: on the Import page you can type or paste books, one per line ("Title by Author", "Title - Author", or just the title), with no cable needed.

## [1.8.0]

### New
- **Age groups**: open any book and set its **Age group**: Young kids (up to 8), Middle grade (9–12), Teens (13–15), Young adult (16–17) or Adults (18+). Choosing one also counts as rating the book, even before the AI gets to it. The Library has a **Suitable for…** filter, and book cards show the age group.
- **Kid accounts by age group**: when you add a kid, pick their age group ("Kid · Teens (13–15)", for example). Each group starts with sensible content rules, and kids only see books rated for their group or younger. Existing kid accounts can be switched under **Admin → Users**.
- **Parents' notes**: after reading a book, leave a note on it. Notes can be for **everyone** (kids see them as "Notes from your parents") or **parents only** 🔒. You can edit or delete your own notes.
- **Usage tab** (admins): CPU, memory, **network speed** (download and upload) and disk, with 15-minute charts, plus books rated, AI calls and cost, AI tokens per day for the last two weeks, and what Ollama has loaded. The **System** tab now holds just the connection checks.

## [1.7.0]

### New
- **System page** (admins): a new **System** tab shows NovelCheck's CPU, memory, disk space and database size, and, when you use Ollama, which models are loaded and how much sits in GPU memory versus system RAM.
- **Check everything**: one button tests every connection NovelCheck uses (your AI provider and fallback model, Open Library, Google Books, the Calibre library and Content server, email, remote access, Ollama, and the update check) and says how to fix anything that fails.
- **Notifications 🔔**: admins and editors get a bell with a count. It lists problems NovelCheck notices on its own (books failing to rate, Calibre sync errors, remote access dropping, Send-to-Kindle failures, Ollama downloads failing, failed checks) with **Fix it** links, **Mark read** and **Clear all**. Some clear themselves once the problem is fixed.
- **Google Books API key guide**: without a key, Google's shared free quota often runs out. The AI setup guide now explains how to get a free key.

### Fixed
- **Error pop-ups vanished too fast**: error messages now stay until you close them (✕), and every message is also listed under the bell in "Messages on this device".

## [1.6.0]

### New
- **One-click removal from Calibre** (admins): connect NovelCheck to Calibre's Content server under **Admin → Calibre Library → One-click removal** (**Show setup steps** explains the Calibre side). Then **Remove hidden books from Calibre…** has a **Remove these books** button. Books go to Calibre's recycle bin, and books marked OK are never included.
- **Safety checks**: before removing anything, NovelCheck re-checks the list and confirms every title with Calibre. If Calibre is serving a different library, it stops and removes nothing.

## [1.5.0]

### Changed
- **Filter boxes now hide books**: the Library's checkboxes are labeled **Hide:**. Ticking **Nudity**, for example, hides books with nudity instead of showing only those. There's also a new **Open Door** box.

### New
- **✓ Mark as OK**: if a filter catches a book that's fine for your family (Harry Potter's magic, say), an admin or editor can open it and click **Mark as OK**. It then shows for everyone, kids included, whatever the filters or content rules say. Click **Remove OK mark** to undo.
- **Remove hidden books from Calibre** (admins): in the Library, tick the Hide boxes, then click **Remove hidden books from Calibre…**. NovelCheck lists the matching Calibre books (never ones marked OK) and gives you a search to paste into Calibre, which then removes exactly those books using its own recycle bin.
- **Suggest a filter / feedback**: links at the bottom of every page and next to the Hide boxes open a short form on GitHub (a free GitHub account is needed).

## [1.4.0]

### New
- **Remote access built in**: use NovelCheck away from home through a free Cloudflare Tunnel, with no router changes. Paste the tunnel token from Cloudflare into **Admin → Remote access**, tick **Turn on remote access**, and you get an `https://` address that works anywhere. **Show setup steps** walks through the Cloudflare side.
- **Ollama easy setup**: pick **Ollama** as the AI provider, click **Find Ollama**, **Download** a model (with a progress bar), and **Use this model**. No terminal commands needed. Works great with GPU passthrough.

## [1.3.2]

### Fixed
- **Calibre sync failed with "no such column: b.isbn"** on libraries made by current Calibre versions, which no longer have that column. ISBNs are now read only from Calibre's identifiers, which every version has. After updating, click **Sync Calibre now**.

## [1.3.1]

### New
- **Setup guides for every AI provider**: in **Admin → LLM Analysis Engine**, click **Show setup steps** for click-by-click instructions for the selected provider (creating an account, getting an API key, what to paste where, typical costs), or **Which one should I pick?** for a quick comparison. The same guide is in `docs/AI_PROVIDERS.md` on GitHub.

## [1.3.0]

### New
- **Choose your AI provider**: **Admin → LLM Analysis Engine** has an **AI provider** menu with OpenAI, **Anthropic Claude**, Google Gemini, Perplexity, and Ollama (free, on your own server). Picking one fills in the address, a low-cost model, and prices, and says where to get an API key.
- **Claude support**: NovelCheck talks to Claude directly through Anthropic's official API. By default Claude Haiku 4.5 rates books, and Claude Sonnet 5 is only used when Haiku can't.
- **Perplexity**: its Sonar model can search the web, which helps with lesser-known and self-published books.

## [1.2.1]

### Fixed
- **Updates not arriving on TrueNAS**: the install guide and `docker-compose.yml` now set the image pull policy to **Always**. Without it, TrueNAS keeps reusing its old download of `latest`. If you installed earlier, edit the app once and set it (see "Updating NovelCheck" in the TrueNAS guide).

### New
- **TrueNAS "Install Custom App" steps**: the install guide now covers the TrueNAS form as well as YAML.

## [1.2.0]

### New
- **Stay signed in**: the sign-in screen has **Keep me signed in on this device** (on by default). You stay signed in as long as you use NovelCheck at least once every 30 days. Admins can change the number of days under **Admin → Sign-in & Updates**. Untick it on shared computers: you're signed out when the browser closes.
- **Version in the header**: the running version now shows at the top right next to **Sign out**. Click it to see what's new.

## [1.1.0]

### New
- **Create the admin account in the browser**: new installs open a "Welcome to NovelCheck" page to set the admin username and password. There's no password in the TrueNAS YAML any more (`NOVELCHECK_ADMIN_PASSWORD` still works for automated installs).
- **How-to guide**: a short step-by-step guide opens the first time each person logs in, tailored to their account type. Reopen it any time from **❔ Help** at the top or **How to use NovelCheck** at the bottom.
- **Editor role**: a middle account type between Admin and Kid. Editors can review the dashboard and costs, run scans and Calibre syncs, import drives, correct book ratings by hand, and manage kids' accounts. They can't change AI or email settings, API keys, the library folder, or backups, and can't touch admin accounts.
- **Edit rating**: admins and editors can correct a book's spice level, content flags and summary from the book window. Hand-made ratings are labeled with who changed them.
- **Choose the Calibre library folder in the app**: mount a parent folder (for example your whole media share), then in **Admin → Calibre Library** browse to the exact library or let NovelCheck find it automatically.
- **Update notices and What's new**: admins and editors see a banner when a new version is out, and everyone can read the release notes in the app. Admins can turn update checks off.

### Changed
- Existing databases upgrade automatically on first start. No action needed.
- The TrueNAS guide now mounts a parent folder (for example your whole `plex` share) and picks the library inside NovelCheck.
- Existing users will see the How-to guide once on their next login.

## [1.0.0]

First release: Calibre sync (read-only), browser drive/Kindle scanner, spice and content analysis with any OpenAI-compatible AI, parental content rules, Up Next reading queue with Send-to-Kindle, admin panel with cost controls, installable phone app, and a TrueNAS install guide.
