# NOVELCHECK_SPEC.md: NovelCheck

## 1. Project Overview
**NovelCheck** is a self-hosted, Dockerized web application designed to scan, analyze, and catalog e-book libraries for romantic/sexual content ("spice") and sensitive thematic elements (modeled after Common Sense Media ratings with specific Catholic moral discernment criteria).

It connects directly to a **Calibre Library** (via read-only SQLite database access), integrates a **Browser-based Drive Scanner** to index external e-reader drives (e.g., connected Kindles), features a **Netflix-style drag-and-drop Reading Queue** for managing device syncs, and is configured as a **Progressive Web App (PWA)** for home-screen mobile installation over a **Cloudflare Tunnel**.

---

## 2. Core Development Rules (Strict Enforcement)
1. **File Length Limit (Max ~300 Lines):** Keep code modular. No single file (Go, JavaScript, CSS, or SQL) should exceed ~300 lines of code. Split API routes, handlers, database services, and UI components into small, logical sub-modules.
2. **Model Delegation Strategy:** Default LLM calls to lightweight, high-efficiency models (e.g., `gpt-4o-mini`, `gemini-1.5-flash`, `llama3.2`) for fast structured JSON outputs. Only escalate to larger models if the primary model fails.
3. **Repository Source of Truth:** Keep GitHub repository files (`README.md`, `docker-compose.yml`, `NOVELCHECK_SPEC.md`) fully synchronized and up to date. Perform regular audits to ensure documentation matches codebase implementation without contradictions.

---

## 3. Technical Stack
* **Language/Backend:** Go 1.26 (using modular HTTP handlers)
* **Database:** SQLite for local caching, custom tags, user accounts, library catalogs, and user queues.
* **Authentication:** Cookie/Session-based Authentication with Role-Based Access Control (RBAC).
* **Frontend & Mobile:** HTML5, Tailwind CSS, Vanilla JavaScript (SortableJS for drag-and-drop queue management), and PWA Manifest/Service Worker for "Add to Home Screen" native app experience. Embedded directly into the Go binary using Go's `embed.FS`.
* **APIs & Integrations:**
  * Open Library API / Google Books API (Metadata & blurb lookups)
  * LLM providers selectable in the admin panel: OpenAI-compatible client (OpenAI, Google Gemini, Perplexity, local Ollama/vLLM) and Anthropic Claude via the official Go SDK (native Messages API)
  * Chromium File System Access API (`showDirectoryPicker()`) with fallback to `<input type="file" webkitdirectory>` for iOS/Safari drive scanning
  * SMTP Client (Amazon Send-to-Kindle email delivery)
* **Deployment & Remote Access:** Single-container Docker image optimized for TrueNAS SCALE / Linux hosts. Configured to store SQLite config databases on SSD datasets and read large media libraries from HDD pools. Ships the Cloudflare Tunnel connector (`cloudflared`) inside the image, configured from the admin panel; also works behind a separate `cloudflared` or reverse proxy with full HSTS, CORS, `Cache-Control: no-store` on API routes, and `X-Forwarded-For` header support.

---

## 4. Core Features & Functional Requirements

### Feature 1: Authentication & Parental Control Filters
* User login system supporting `Admin`, `Editor`, and `Restricted` (Kid) accounts. Editors handle day-to-day management (rating corrections, scans, drive imports, kids' accounts) without access to technical settings, secrets, or backups.
* First-run setup: on a fresh install the web page asks for the admin username and password (no credentials in deployment YAML).
* First-login "How to" guide per user, reopenable from a Help link.
* Sessions: "Keep me signed in" (default on) gives a rolling session (admin-configurable, default 30 days, renewed at most daily while active); otherwise a browser-session cookie with a 12-hour idle limit.
* Profile-level content rules (e.g., *Hide Open Door, Nudity, Solo Acts, Heavy Innuendo, Dark Occult/Demonic, LGBTQ+, and Unanalyzed books*).
* Database-level filtering ensures restricted accounts cannot list, search, view, queue, or download hidden titles.

### Feature 2: Calibre Library Auto-Sync (Read-Only)
* Mount Calibre’s root storage directory into the Docker container.
* Read Calibre’s `metadata.db` SQLite file directly in **strict read-only mode** (`file:metadata.db?mode=ro`) to extract titles, authors, and existing metadata without database lock conflicts. Never writes to Calibre.
* Resolve internal file paths by prepending container mount prefix `/calibre/` to relative paths extracted from `metadata.db`.
* Automatically purge deleted Calibre titles during scheduled or manual sync runs.
* The container mounts a parent folder; the admin selects the exact library subfolder in the app (folder browser + automatic `metadata.db` search), restricted to the mount.
* Assign synced entries to a default catalog named `Calibre Main`.

### Feature 3: Browser Drive & Kindle Scanner
* Front-end interface includes an "Import Local Drive / Kindle" feature using `window.showDirectoryPicker()`.
* Automatically falls back to standard `<input type="file" webkitdirectory>` on non-Chromium browsers (iOS Safari, mobile browsers).
* MTP Kindles (shown as a "device", not a drive) can't be opened by browsers: the page explains copying `documents/` to the computer first, and offers **Paste a list** (one title per line, imported with format `list`).
* Recursively traverses selected directory folders to read book filenames and metadata tags (`.epub`, `.mobi`, `.azw3`), understanding Amazon naming conventions (`Title - Author_ASIN_EBOK.azw`).
* Sends extracted metadata payloads (never file bodies) to the Go backend API into a selected destination catalog.

### Feature 4: Multi-Catalog UI & Filtering
* **Unified Dashboard:** Browse all books across all catalogs simultaneously.
* **Filtering:** Filter by catalog and peppers (0–5, or "older rating"); content-flag checkboxes **hide** matching books.
* **Pepper scale:** Spice is rated 0–5 peppers: 0 No Romance, 1 Sweet Romance, 2 Mild / Closed Door, 3 Steamy Closed Door / Heavy Tension, 4 Explicit / Open Door, 5 Very Explicit / Erotica (full descriptions and examples in the prompt below and in the app's "What do the peppers mean?"). `spice_level` and a short `spice_reason` ("Heavy innuendo, on-page foreplay") are stored per book; cards at Level 3+ show an "ⓘ reason" chip with a "Why this is Level N" tooltip (falling back to the book's flags for older ratings). `classification` is derived for the Open Door filter and older rules (0–1 No Spice, 2–3 Closed Door, 4–5 Open Door). `books.rules_version` records `store.RulesVersion` (bumped whenever the prompt's rating rules change; 2 = the v1.16 wording), so AI ratings made under older rules are offered for re-rating; **Re-rate whole library** re-rates every AI-rated book. A kid account's **Strict family preset** sets `max_spice` 2 and hides Open Door, nudity, solo acts, heavy innuendo and unrated books. Books rated before the scale keep `spice_level` NULL and, for kids' limits, count as the highest level their old label allows (No Spice = 2, Closed Door = 3, Open Door = 4). Kid accounts have `max_spice` (−1 = no limit; age-group presets 0/1/2/3/none). Admins/editors can re-rate AI-rated older books in place (they stay visible; failures keep the old rating); hand-rated books are never re-rated.
* **Parent approval:** Admins/editors can mark a book "OK", which overrides hide filters and restricted accounts' content rules (e.g. Harry Potter's fantasy magic).
* **Calibre removal:** Admins can list the Calibre books their hide filters catch (never parent-approved ones) and either copy a Calibre search (`id:=N or …`) or, with the calibre Content server connected, remove them in one click. One-click removal goes through calibre's own remote interface (`/cdb/cmd/remove`, to calibre's recycle bin) after re-checking the list and verifying every id's title against calibre. NovelCheck's own access to the library stays read-only.
* **Calibre-Web:** optional setting `calibre_web_url` (admin; `192.168.1.10:8083` is normalized to `http://192.168.1.10:8083`). When set, `GET /api/books/{id}` returns it to admins and editors, and the book window links each Calibre entry to `<calibre_web_url>/book/<Calibre id>`. Calibre ids are shown next to each Calibre entry.
* **Feedback:** In-app links to GitHub issue forms for filter suggestions and general feedback.
* **Age groups & notes:** Books carry a parent-set `age_level` (1 Young kids ≤8, 2 Middle grade 9–12, 3 Teens 13–15, 4 Young adult 16–17, 5 Adults 18+). Restricted users have an `age_level` too; they never see books rated for an older group (this applies even to "OK"-marked books), and a parent-set age group satisfies "hide unrated". New kid accounts get content-rule presets per age group. Parents' notes (`book_notes`) are visible to `everyone` or `parents` only; authors (or admins) edit/delete them.
* **Backup AI:** optional second provider (`backup_llm_*`, `backup_price_*`; `Store.AIConfigs()` returns main then backup). The worker tries the main models, then the backup's; a connection-level failure (dial/DNS) skips the rest of that provider's models. Each call's cost is stored in `token_usage.cost` using that provider's prices (`Store.SpentUSD`). A `llm-backup` notification is raised when the backup rates a book and resolved when the main AI succeeds. Find Ollama can target the backup (`POST /api/admin/ollama/use` with `target: "backup"`).
* **GPU-aware recommendations:** `GET /api/admin/ollama/gpu?url=&probe=1|vram_gb=N` (`internal/ollama/gpu.go`). Ollama has no GPU-info API, so VRAM is inferred from `/api/ps` (`size_vram` vs `size` of the largest loaded model: 0 = CPU only, partial ≈ VRAM, full = at least); `probe` loads the largest downloaded model with `keep_alive` 1m via `/api/generate`, measures, then unloads it (`keep_alive` 0). A curated catalog is labelled best / powerful / fits / too_big (model size + 1.5 GB headroom), or cpu_ok / cpu_slow without a GPU.
* **Model chain:** `llm_model` is tried first, then each model in the comma-separated `llm_fallback_model`, in order, until one returns a valid verdict. The Ollama helper saves the chain from an ordered picker (`POST /api/admin/ollama/use` with `models`).
* **Delete requests:** `delete_requests` (title/author copied; `book_id` ON DELETE SET NULL; one pending request per book and user). Any user: `POST|DELETE /api/books/{id}/delete-request` (`GET /api/books/{id}` returns `my_delete_request`). Admin: `GET /api/admin/delete-requests` (pending grouped by book with Calibre ids, plus recent decisions), `POST /api/admin/delete-requests/decide` `{book_ids, action: delete|dismiss|done}`; `delete` removes the books' Calibre entries through the verified Content-server path (request ids captured before the re-sync) and leaves books not in Calibre pending. A `delete-requests` notification is raised and resolved when none are pending.
* **Language:** setting `language` (default "English (US)"; English UK, Spanish, French, German, Portuguese, Italian, Dutch). The analyzer prompt gets a language rule for `summary_verdict` (JSON keys unchanged); AI diagnosis writes its user-facing text in it. When the language is English, AI-written summaries failing a stop-word check can be re-rated in place (`POST /api/admin/rerate {"which":"language"}`; count in `non_english` on admin status).
* **List imports:** the browser parses CSV/TSV (Goodreads, StoryGraph, Hardcover, LibraryThing, Amazon data download, any sheet with a Title column) and text copied from Amazon's Content Library (title/author before each "Acquired on …" line), then posts rows with format `list` to `POST /api/import/drive` (ASIN used as the external id when present).
* **Duplicates & formats:** Book lists include `formats` (distinct file formats across copies) and `calibre_copies` (distinct Calibre ids). Calibre duplicates are Calibre entries that normalize to the same `norm_key`. The duplicates view suggests a keep (format score EPUB > AZW3/KFX > AZW > MOBI > PDF, then file count, size, lowest id). Admin removal re-validates that each id is still a duplicate, keeps at least one entry per book, verifies titles with calibre, and removes to the recycle bin.
* **Usage (admin):** Resource use of the NovelCheck container (CPU, memory, network rx/tx rates from `/proc/net/dev`, disk, DB size) with a 15-minute in-memory history sampled every 10 s, library progress, AI cost and 14-day daily token use, and Ollama loaded models (GPU vs RAM from `/api/ps`).
* **System & health (admin):** On-demand connection checks for every external dependency with fix hints, including each backup AI model and the public tunnel address (DNS lookup plus a request to `https://<hostname>/api/setup` through Cloudflare).
* **Diagnostics & AI diagnosis (admin):** `GET /api/admin/diagnostics` is a plain-text report (version, settings with secrets reduced to "(set)" and emails masked, library/worker state, resources, recent notifications, cloudflared log). `POST /api/admin/diagnose` sends it (plus an optional problem description) to the first configured model, which returns a diagnosis, fix steps, title, summary, suspected cause and bug/not-bug; the server always builds a GitHub issue body (AI reading + diagnostics, tunnel hostname replaced) even when the AI fails. The UI opens `.github/ISSUE_TEMPLATE/bug_report.yml` prefilled via the `report` field.
* **Notifications:** Persistent, de-duplicated admin/editor notifications (`notifications` table) raised by background failures and failed checks, auto-resolved where the problem clears; error toasts persist until dismissed. `token-cap` (warning) is raised when the worker first waits for the hourly token cap and resolved when it continues. Routine `info` events go through `Store.NotifyRoutine`, which respects the `notify_routine` setting: `calibre-new` (a sync created N books, counted by `books.created_at`), `batch-done` (a pass through the queue of 2+ books finished: rated, failed, tokens and cost from `token_usage` since the pass began; `internal/analyzer/batch.go`) and `reading` (who started which book, with the delivery note).
* **Feature switches:** settings `module_queue`, `module_send_to_kindle`, `module_koreader`, `module_import`, `module_parents` (default `true`; `internal/store/modules.go`). `GET /api/me` (and the login/setup responses) include `modules` (`{"queue": true, …}`). A switched-off queue or import answers 403; switched-off delivery methods make Start Reading just mark the book as reading. Parent tools are hidden in the UI only (kids' accounts, age groups, notes); restricted accounts keep their rules.
* **Check a book (main feature for parents):** `POST /api/check` (editors and admins) takes `{"query"}` (title/author/ISBN) or `{"image"}` (a `data:image/jpeg|png|webp;base64` photo, ≤ 5 MB; the app shrinks it to ~1280 px). A photo goes to the configured AIs in order through `llm.ImageReader` (OpenAI-compatible `image_url` parts or Claude image blocks, `llm.CoverPrompt`), then Open Library `FindTitle` for the catalogued spelling and ISBN; typed text goes to Open Library `Find` (an unknown non-ISBN query is rated as typed). `Store.MatchBook` looks for the book by `norm_key` (or a unique title). Unknown books are saved to the `Looked up` catalog (path `lookup:<title>`, format `list`). Anything not yet rated is rated at once in the background by `Worker.RateNow` (skips the queue, scan delay and hourly cap; status `processing`, failures saved as rating errors); the response says `rating: true` and the app polls `GET /api/books/{id}`, keeping each request under Cloudflare's 100 s limit. The app lands on `#/check` for parents; on Android the Barcode Detection API reads the ISBN before any AI is asked.
* **Deep Scan (full-text rating):** `internal/epub` reads an EPUB's spine in order (container.xml → OPF manifest/spine, XHTML to text, first h1–h3 as the section title; no DRM support). `internal/deepread` splits sections into parts (2,000 words for a local AI, 8,000 for cloud; long chapters cut into pieces), estimates tokens (words × 4/3 + 900 prompt + 150 answer per part + 700 wrap-up) and prices them at the main AI's rates. Each part is sent with `llm.DeepPartSystem` (the shared `PepperLevels`/`ContentGuide` text plus custom filters) and answered as `{"level", "note", flags…}`; the book's level is the highest part level, flags are OR-ed, and a wrap-up call (`DeepWrapUpSystem`) writes `spice_reason` and `summary_verdict` in the chosen language (fallback: the spiciest part's note). The rating is saved with `analysis_model = "deep: <model>"`, which re-rates, the language check and the rules banner skip. `deep_reads` (status `requested|queued|reading|done|error|declined|cancelled`, source `admin|request|batch|auto`, words, parts, estimate, model, notes JSON, `prev_level`/`new_level`) is also the audit log; one open scan per book. The runner (one scan at a time, resumed after a restart, cancel checked per part, panics contained) also queues new Up Next books of up to 3 users in `deep_scan_users`. A raised level raises a `deep-scan` warning notification (so phones on "Only problems" get it too) and queue items carry `deep_change` ("2→4"). API: `GET|POST /api/books/{id}/deep-scan` (any user: availability, estimate, latest scan; admins start, others request); admin `GET /api/admin/deep-scans`, `GET|POST /api/admin/deep-scans/next?n=` (estimate / queue the next Up Next books), `POST /api/admin/deep-scans/{id}/approve|decline|cancel`; settings `deep_read_model`, `deep_scan_users`, `deep_scan_top_n`; `GET /api/books?deep=1`.
* **Custom AI filters:** `custom_flags` (key derived from the label, label, description; at most 12) and `book_flags` (book ↔ filter). `llm.SystemPromptFor(lang, flags)` adds a "Custom Flags" section and asks for `"custom_flags": {"<key>": true|false}` (a list of keys is accepted too; unknown keys are ignored). `SaveAnalysis` replaces a book's marks and records `books.flags_version`; the setting `custom_flags_version` goes up when a filter is added or its description changes, so older AI ratings join the re-rate candidates. Book JSON carries `custom_flags` (comma-separated keys). Filters accept `flag:<key>` in `flags`, `exclude` and the Calibre removal list. API: `GET /api/flags` (any user); admin `POST /api/admin/flags`, `PUT|DELETE /api/admin/flags/{id}`; the manual verdict takes `custom_flags` (keys). Kids' content rules don't cover custom filters yet.
* **Phone notifications (Web Push):** `internal/push` implements RFC 8291 payload encryption (aes128gcm, one 4096-byte record) and RFC 8292 VAPID signing (ES256 JWT, `sub` = the project URL) with the standard library, verified against the RFC 8291 example. The VAPID key is generated on first use and stored as `push_vapid_private_key` (PKCS#8; a secret: masked in diagnostics, not editable, never sent to the browser). `push_subscriptions` holds each device (user, endpoint, p256dh, auth, scope `all|problems`, device label). `Store.OnNotify` fires for each new notification (not repeats of an unread one) and `push.Service.FromNotice` sends it to admins' and editors' devices (scope `problems` only gets warnings/errors); Start Reading pushes "Ready to read" to the reader's own devices. Endpoints must be `https` on the browsers' push services (`*.googleapis.com`, `*.mozilla.com`, `*.mozaws.net`, `*.push.apple.com`, `*.notify.windows.com`), so the server never posts into the home network; 404/410 answers drop the device. API (any signed-in user): `GET /api/push` (public key; own devices with `key` = SHA-256 of the endpoint, never the endpoint itself), `POST /api/push/subscribe`, `POST /api/push/unsubscribe`, `POST /api/push/test`. The service worker shows `push` events and opens the linked page on `notificationclick`; `web/static/js/push.js` is the Profile card (needs a secure context; on iOS the installed app).
* **Cross-Catalog Overlap:** Matches books present in both Calibre AND external drives as single entities to allow filtering for overlapping titles.

### Feature 5: Progressive Web App (PWA) "Install as App" Support
* Includes `manifest.json` metadata defining application icons, dark standalone theme colors, app name (`NovelCheck`), and `display: standalone`.
* Lightweight Service Worker (`sw.js`) enabling offline app shell caching and triggering native mobile "Install App" / "Add to Home Screen" prompts.

### Feature 6: Netflix-Style "Reading Queue" & Start Reading
* **Personal Queues:** Every logged-in user gets their own ordered queue ("Up Next") with persistent position ordering.
* **"Start Reading" Action Button:** Triggers immediate delivery based on user preference (emails `.epub` via Send-to-Kindle SMTP or flags for KOReader sync) and shifts status to `Currently Reading`.

### Feature 7: Flexible LLM Analysis Engine & Admin Control Panel
* **Metadata Enrichment Step:** Query Open Library / Google Books API for summary blurbs before LLM execution to prevent hallucinations on indie/self-published titles.
* **On-Demand & Queued Processing:** Syncing a library indexes metadata instantly without triggering LLM API calls for every book at once. Books display a `Pending Analysis` status until processed.
* **Manual rating corrections:** Admins and editors can override a verdict; it is recorded as `manual: <username>`.
* **Update notices:** The app checks GitHub Releases, shows admins/editors an "update available" banner, and displays release notes (from `CHANGELOG.md`) under "What's new".
* **Admin Control Panel:** Batch size limits, token/hourly rate caps, live cost estimator, test SMTP mailer, auto-sync schedules, 1-click SQLite database download (`novelcheck.db`), and queue clearing tools.

#### System Prompt Specification
```text
You are an expert book content analyzer. Your sole purpose is to determine the nature of romantic/sexual content ("spice") and specific thematic elements (including spiritual/occult themes) in a given book using the provided book title, author, and blurb metadata. The user wants to avoid specific types of content. You must be precise and objective while strictly adhering to the formatting and safety rules below.

STRICT RULES:
1. NO SPOILERS: Do not reveal major plot twists, endings, or critical character deaths.
2. NO EXPLICIT/GRAPHIC LANGUAGE: Do not use anatomically explicit terms, graphic descriptions, or vulgar words in your analysis. Use clinical or modest phrasing (e.g., "solo acts").
3. NO AFFIRMATIONS OR FILLER: Provide the output strictly matching the JSON payload format.

CATEGORIES & GUIDELINES:

1. Spice Level (0-5 peppers). Pick the single best fit:
- 0 = No Romance: No meaningful romantic or sexual content. No romantic subplot, kissing, sexual attraction, or romantic physical affection. Examples: Harry Potter and the Sorcerer's Stone; The Hobbit.
- 1 = Sweet Romance: Romance is present but mild and non-sexual. May include crushes, attraction, flirting, hand-holding, cuddling, and sweet/brief kisses. No sexual desire or sexualized physical intimacy. Examples: Uglies (Scott Westerfeld); Seeking Persephone (Sarah M. Eden).
- 2 = Mild / Closed Door: Romantic tension and kissing occur, including passionate kissing. Any physical intimacy beyond kissing cuts to black or happens strictly off-page; nothing sexual is shown or described on the page. Example: My Phony Valentine (Courtney Walsh).
- 3 = Steamy Closed Door / Heavy Tension: Heavy physical foreplay or suggestive on-page innuendo, such as heavy making out with clear sexual intent or sexually charged scenes that build toward intimacy, but it stops short of explicit sexual acts.
- 4 = Explicit / Open Door: Sexual encounters occur on-page and include clear descriptions of sexual activity. Scenes contain meaningful sexual detail rather than simply implying what happens. There may be multiple or extended explicit scenes, but sex does not necessarily dominate the entire book. Examples: Fourth Wing (Rebecca Yarros); A Court of Thorns and Roses (Sarah J. Maas).
- 5 = Very Explicit / Erotica: Frequent, extended, or highly graphic on-page sexual content with extensive detail. Sexual encounters are a major component of the book and may occupy a substantial portion of the story. Example: Fifty Shades of Grey (E. L. James).
If unsure between two levels, choose the higher one.
Also give spice_reason: 3-8 modest words naming what sets the level (e.g. "No romance", "Kissing only", "Fade-to-black intimacy", "Heavy innuendo, on-page foreplay", "Several explicit scenes").

2. Content Elements:
- Nudity: Presence of nudity in a romantic or intimate context.
- Solo Acts: Private, solo intimate acts by any character.
- Heavy Innuendo: Detailed physical foreplay or highly suggestive text.

3. Spiritual & Occult Classification:
- Whimsical / Standard Fantasy: Fictional fairy-tale magic, standard wizards (e.g., Merlin, Gandalf), or light YA fantasy (e.g., Harry Potter). (Mark dark_occult: false)
- Dark Occult / Demonic: Explicit real-world occult practices, black magic rituals, demonic possession, or active demonic themes. (Mark dark_occult: true)

OUTPUT FORMAT (JSON ONLY):
{
  "spice_level": 0 | 1 | 2 | 3 | 4 | 5,
  "spice_reason": "3-8 words",
  "content_elements": {
    "nudity": true | false,
    "solo_acts": true | false,
    "heavy_innuendo": true | false
  },
  "spiritual_elements": {
    "playful_fantasy": true | false,
    "dark_occult": true | false,
    "demonic_presence": true | false
  },
  "lgbtq_content": true | false,
  "summary_verdict": "1-2 sentence recommendation."
}
```

---

## 5. Architecture & Data Model (as implemented)

> Sections 5 and 6 describe the implementation and the order it was built in. Keep them in sync with the code (Rule 3).

### 5.1 Components
| Package | Responsibility |
|---|---|
| `cmd/novelcheck` | Loads env config, opens DB, bootstraps admin, resets interrupted analyses, starts the analysis worker + Calibre scheduler + HTTP server |
| `internal/db` | Opens `novelcheck.db` (WAL, foreign keys, single connection), applies the embedded idempotent `schema.sql`, and runs in-place migrations for older databases |
| `internal/store` | All SQL (via `sqlx`); SQL-level visibility clause for per-profile content rules |
| `internal/auth` | bcrypt passwords, random session tokens (only SHA-256 stored), `RequireUser` / `RequireManager` / `RequireAdmin` middleware |
| `internal/calibre` | `metadata.db?mode=ro` reader, library-folder selection/browse/find inside the mount, `/calibre/` path resolution, prune of deleted books, interval scheduler |
| `internal/calibresrv` | calibre Content server client: Digest/Basic login, library list, `list` (title verification) and `remove` (to recycle bin) commands |
| `internal/version` / `internal/updates` | Build-time version stamp, semver comparison, cached GitHub Releases check |
| `internal/enrich` | Open Library → Google Books → local description blurb lookup |
| `internal/llm` | Provider clients behind one `Completer` interface: OpenAI-compatible `/chat/completions` and Anthropic Claude (`anthropic-sdk-go`); Section 4 system prompt; tolerant JSON verdict parser |
| `internal/analyzer` | Single worker queue; token-per-hour cap on the trailing-hour window; scan delay; small-model-first with optional large fallback |
| `internal/delivery` | Send-to-Kindle SMTP (STARTTLS / implicit TLS) and best-format picker (EPUB > PDF) |
| `internal/api` | `chi` router, security middleware (HSTS, CSP, `no-store`, CORS allow-list, CSRF header, real IP), JSON handlers, embedded static serving |
| `web/static` | `index.html`, compiled Tailwind `css/app.css`, ES-module JS views, `manifest.json`, `sw.js`, icons, vendored SortableJS + JSZip |

### 5.2 Tables
`users` (role `admin|editor|restricted` + `hide_*` content rules + `age_level` + delivery prefs + `guide_seen`) · `sessions` (token hash, expiry, `remember`) · `catalogs` (`calibre` | `drive` | `custom`) · `books` (one row per logical title, with `approved`/`approved_by` for parent "OK" marks and `age_level`/`age_set_by`, `spice_level` + `spice_reason` + `rules_version`, keyed by a normalized title + author-surname `norm_key` so the same book in Calibre and on a Kindle is shared; holds status `pending|queued|processing|analyzed|error` and the verdict flags) · `catalog_books` (copies/locations) · `queue_items` (per-user position + `queued|reading|finished`) · `settings` (admin-tunable key/values) · `book_notes` (book, author, body, visibility `everyone|parents`) · `notifications` (level, source, message, link, count, read) · `push_subscriptions` (user, endpoint, keys, scope, device) · `custom_flags` (key, label, description) · `book_flags` (book, filter) · `deep_reads` (Deep Scans and their audit log) · `token_usage` (per-call prompt/completion tokens for caps and cost).

### 5.3 HTTP API
Public: `POST /api/auth/login`, `POST /api/auth/logout`, `GET|POST /api/setup` (first admin, only while no users exist), `GET /healthz`.
Any signed-in user: `GET /api/me` (the user plus `modules`), `PUT /api/me/password`, `PUT /api/me/delivery`, `PUT /api/me/guide-seen`, `GET /api/updates`, `GET /api/books` (filters: `q, catalog, overlap_with, multi, classification, flags, exclude, status, age, format, spice, sort, limit, offset`; `spice` = `0`–`5` or `old`; `format` = a file format, `multi`, `dupes` or `none`; `age` = `unset` or `1`–`5` for "suitable up to"), `GET /api/books/{id}`, `GET /api/books/{id}/download`, `GET /api/catalogs`, `GET /api/age-groups`, `GET|POST /api/queue`, `PUT /api/queue/order`, `DELETE /api/queue/{id}`, `POST /api/queue/{id}/start`, `POST /api/queue/{id}/finish`.
Editor or admin: `GET /api/notifications`, `POST /api/notifications/read`, `DELETE /api/notifications`, `POST /api/catalogs`, `PATCH /api/catalogs/{id}`, `POST /api/import/drive`, `POST /api/books/{id}/analyze`, `PUT /api/books/{id}/verdict` (`spice_level` 0–5), `PUT /api/books/{id}/approval`, `PUT /api/books/{id}/age`, `POST /api/books/{id}/notes`, `PUT|DELETE /api/notes/{id}`, `GET /api/admin/calibre/duplicates`, `POST /api/admin/rerate` (`{"which": "" | "language" | "all"}`: older rules, non-English summaries, or every AI-rated book; admin status reports `rerate_candidates` and `ai_rated`), `GET /api/admin/status`, `POST /api/admin/analyze-batch`, `GET /api/admin/errors` (failed books grouped by `analysis_error`), `POST /api/admin/retry-errors` (all, or `{"message"}` for one group), `POST /api/admin/calibre-sync`, `GET|POST /api/admin/users`, `PUT|DELETE /api/admin/users/{id}`, `PUT /api/admin/users/{id}/password` (editors: restricted accounts only).
Admin only: `DELETE /api/catalogs/{id}`, `GET|PUT /api/admin/settings`, `POST /api/admin/wipe-queue`, `GET /api/admin/calibre/browse`, `GET /api/admin/calibre/find`, `GET /api/admin/calibre/removal`, `POST /api/admin/calibre/remove`, `POST /api/admin/calibre/duplicates/remove`, `GET|PUT /api/admin/calibre/server`, `GET|PUT /api/admin/tunnel`, `GET /api/admin/system`, `POST /api/admin/health`, `GET /api/admin/diagnostics`, `POST /api/admin/diagnose`, `GET /api/admin/ollama/find`, `GET|POST /api/admin/ollama/pull`, `POST /api/admin/ollama/use`, `PUT /api/admin/calibre/library`, `POST /api/admin/smtp-test`, `GET /api/admin/backup`.
All non-GET API calls require the header `X-NovelCheck: 1`.

---

## 6. Step-by-Step Build Order

1. **Scaffold & config.** Go module, `chi`, `sqlx`, pure-Go `modernc.org/sqlite` (CGO-free static binary), env-based `internal/config`.
2. **Database layer.** Embedded `schema.sql` and the `store` package for users, sessions, catalogs, books, queue, settings, and token usage.
3. **Authentication & RBAC.** bcrypt, cookie sessions, admin bootstrap, `RequireUser` / `RequireAdmin`, and SQL-level content-rule filtering (Feature 1).
4. **Calibre auto-sync.** Read-only `metadata.db`, `/calibre/` path prefixing, the **Calibre Main** catalog, pruning, and the polling scheduler (Feature 2).
5. **Metadata enrichment + LLM engine.** Open Library / Google Books blurbs, an OpenAI-compatible client, the Section 4 system prompt, the verdict parser, and the queued worker with token caps, scan delay, and small→large fallback (Feature 7).
6. **Delivery.** Send-to-Kindle SMTP and the KOReader flag (Feature 6).
7. **HTTP API.** Security middleware (HSTS, CSP, CORS allow-list, `Cache-Control: no-store`, CSRF header, `X-Forwarded-For` / `CF-Connecting-IP`), handlers for books, catalogs, drive import, queue, admin, and users.
8. **Front-end.** Embedded HTML + Tailwind + vanilla JS views: library dashboard and filters (Feature 4), book dialog, Up Next queue with SortableJS (Feature 6), drive scanner with directory picker, `webkitdirectory` fallback, and EPUB/MOBI metadata parsing (Feature 3), admin panel, users, and profile.
9. **PWA.** `manifest.json`, `sw.js` app-shell cache, icons, and the install prompt (Feature 5).
10. **Tests.** Store filters and visibility, Calibre read-only sync, LLM client and parser, analyzer fallback and usage recording, API end-to-end (auth, CSRF, restricted filtering, import, queue).
11. **Packaging & docs.** Multi-stage `Dockerfile` (non-root UID 568), `docker-compose.yml` (prebuilt GHCR image, SSD `/data`, read-only HDD `/calibre`, port 8080, optional `cloudflared` and GPU `ollama` profiles), `README.md`, and this spec, kept in sync per Rule 3.
12. **Distribution.** GitHub Actions workflow (`.github/workflows/docker.yml`) that tests, builds multi-arch (`amd64`/`arm64`) images, publishes them to `ghcr.io/zachcurry13/novelcheck` (`:latest` from `main`, `:X.Y.Z` per release), and creates a GitHub Release from a `v*` tag or a manual **Run workflow** with a version input. `docs/TRUENAS.md` is a no-command-line TrueNAS install guide (either **Install Custom App** or **Install via YAML**, image pull policy **Always** so `:latest` updates are fetched) with `/data` (SSD), `/calibre` (HDD, read-only), and host port 30080.
13. **v1.1 usability.** Web-based first-run admin setup, Editor role with scoped permissions and manual rating corrections, in-app Calibre library folder picker, first-login How-to guide, and update notices with an in-app changelog (`CHANGELOG.md` also drives GitHub release notes).
14. **v1.3 AI providers.** Provider menu with presets (OpenAI, Claude, Gemini, Perplexity, Ollama, other), `llm_provider` setting, a native Claude client using Anthropic's official Go SDK, and per-provider setup guides (`docs/AI_PROVIDERS.md`, bundled and shown in the admin panel).
15. **v1.4 remote access & Ollama.** Bundled `cloudflared` supervised by `internal/tunnel` (token in Admin → Remote access, status + log, `docs/REMOTE_ACCESS.md` shown in-app) and an Ollama easy-setup flow (`internal/ollama`: discovery, model download with progress, one-click "use this model").
16. **v1.5 filters & feedback.** Hide-style filter checkboxes (incl. Open Door), parent "Mark as OK" (`PUT /api/books/{id}/approval`) respected by filters and content rules, Calibre removal helper (`GET /api/admin/calibre/removal`), GitHub issue forms and in-app feedback links.
17. **v1.6 one-click Calibre removal.** `internal/calibresrv` client (verified against calibre 7.6), Admin → Calibre Library → One-click removal (tested before saving, password secret), and `POST /api/admin/calibre/remove` with list-changed and title-mismatch guards; setup guide `docs/CALIBRE_SERVER.md` shown in-app.
18. **v1.7 system, health & notifications.** `internal/sysinfo`, Ollama `ps`, connection checks (`internal/api/health.go`), notifications store + bell UI, persistent error toasts, System tab.
19. **v1.8 age groups, notes & usage.** `age_level` on books and users (`internal/store/ages.go`, visibility clause), `book_notes` (`internal/store/notes.go`, `internal/api/notes_handlers.go`), account-type picker, network rates + history sampler (`internal/sysinfo/history.go`), SVG charts (`web/static/js/charts.js`) and the admin Usage tab.
20. **v1.9 duplicates & formats.** Derived `formats`/`calibre_copies` columns and the Format filter (`internal/store/filters.go`), `internal/store/duplicates.go` (groups + keep suggestion), `internal/api/duplicates_handlers.go` sharing the verified Calibre removal path, `web/static/js/duplicates.js`, format chips and per-entry copies in the book window.
21. **v1.10 model chain.** Ordered fallback models (`Store.LLMModels`), ordered Ollama picker (`web/static/js/ollamaorder.js`).
22. **v1.11 pepper scale.** `books.spice_level`, `users.max_spice` (`internal/store/spice.go`), prompt + parser (`spice_level`, legacy `classification` still accepted), in-place re-rating (`Worker.Rerate`, `POST /api/admin/rerate`), `web/static/js/peppers.js` (scale, chips, guide dialog), pepper filter/select/limits in the UI.
23. **v1.12 diagnostics.** `internal/api/diagnostics.go`, `internal/api/diagnose.go`, GitHub bug form, `web/static/js/diagnose.js` and `web/static/js/copy.js` (copy/download buttons, selection-safe live logs).
24. **v1.13 backup AI & public check.** `internal/store/ai.go`, `internal/analyzer/ai.go` (provider chain, `Unreachable`), per-call cost, backup settings UI (provider picker and Ollama helper take a `backup_` prefix), `internal/api/publiccheck.go`.
25. **v1.14 GPU recommendations.** `internal/ollama/gpu.go` (measure + catalog + labels), GPU line and labelled download menu in `web/static/js/ollamahelper.js`.
26. **v1.15 delete requests, language & list imports.** `internal/store/deletes.go`, `internal/api/deletes_handlers.go`, `web/static/js/deletions.js`; `internal/llm/language.go`; `web/static/js/importfile.js` (CSV + Amazon paste parsing) and `web/static/js/importguides.js`.
27. **v1.16 pepper wording & quick wins.** Revised levels 2–3 and `spice_reason` in the prompt/parser, `books.spice_reason` + `books.rules_version` (`store.RulesVersion`), `internal/api/rerate_handlers.go` (`which: "" | language | all`) and `internal/api/settings_handlers.go` (split from `admin_handlers.go`; `calibre_web_url` validation), gray-area chip (`web/static/js/peppers.js`), Strict family preset (`web/static/js/users.js`), Calibre-Web links in the book window, collapsible phone filters (plain CSS at the end of `web/tailwind.input.css`), footer credit; `internal/sysinfo/disk_*.go` so tests build on Windows.
28. **v1.16 feature switches & notices.** `internal/store/modules.go` (switch keys, `Modules()`), `internal/api/modules.go` (`/api/me` modules, `requireModule`), `web/static/js/modules.js` (`data-module` hiding, delivery options), Admin → Features; `Store.NotifyRoutine`, `internal/analyzer/batch.go` (batch summary, token-cap notice), new-book count after Calibre syncs, "started reading" notice.
29. **v1.16 phone notifications.** `internal/push` (`webpush.go`: encryption + VAPID + `Send`; `service.go`: key, `FromNotice`, `ToUser`, `SendTo`), `push_subscriptions` table and `internal/store/push.go`, `Store.OnNotify` wired in `cmd/novelcheck`, `internal/api/push_handlers.go`, service-worker `push`/`notificationclick`, `web/static/js/push.js` on the Profile page.
30. **v1.16 custom AI filters.** `internal/store/flags.go`, `custom_flags` + `book_flags` tables, `books.flags_version`, `internal/llm/flags.go` (prompt section, tolerant parsing), `internal/api/flags_handlers.go`, `web/static/js/customflags.js` (chips, Hide boxes, rating ticks, admin editor).
31. **v1.16 Check a book.** `internal/llm/vision.go` (`ImageReader`, cover prompt), `internal/enrich/find.go` (Open Library identification, `CleanISBN`), `internal/analyzer/check.go` (`ReadCover`, `RateNow`), `internal/store/lookup.go` (`MatchBook`, `Looked up`), `internal/api/check_handlers.go`, `web/static/js/check.js` (camera, barcode, polling, verdict card, recently checked), parents' landing page and guide step.
32. **v1.17 guardrails.** `web/static/js/api.js` retries reads (GET) up to 3 times on connection errors and tunnel 502/503/504/522/524/530, waiting for `online`; `internal/safe` recovers panics in the rating worker, Calibre sync loop, push sends and Check a book; `batch_size` 0 = all waiting (max 500), blank = default, validated 0–500; delete requests refused for books that are only in `Looked up` (`Store.Owned`); neutral "Level N: name" pepper labels and `whyChip` (reason + tooltip) for Level 3+.
33. **v1.17 Deep Scan.** `internal/epub`, `internal/deepread` (`chunk.go` split + estimate, `runner.go` queue/auto/EPUB lookup, `scan.go` reading, combining, audit notice), `internal/llm/deep.go` (part and wrap-up prompts; `prompt.go` now shares `PepperLevels`/`ContentGuide`), `deep_reads` table and `internal/store/deepreads.go`, `internal/api/deepscan_handlers.go`, `web/static/js/deepscan.js` (book window section, chip) and `deepscanadmin.js` (`#/deepscan`), Up Next banner, Library filter.
