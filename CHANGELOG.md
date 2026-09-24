# Changelog

All notable changes to NovelCheck. Newest first. Each `## [x.y.z]` section
becomes the release notes for that version and is shown in the app under
**What's new**.

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
