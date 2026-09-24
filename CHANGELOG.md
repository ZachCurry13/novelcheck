# Changelog

All notable changes to NovelCheck. Newest first. Each `## [x.y.z]` section
becomes the release notes for that version and is shown in the app under
**What's new**.

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
