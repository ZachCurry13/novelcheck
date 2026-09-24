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
* **Pepper scale:** Spice is rated 0–5 peppers: 0 No Romance, 1 Sweet Romance, 2 Romantic, 3 Steamy Closed-Door, 4 Explicit, 5 Very Explicit / Erotica-Level (full descriptions and examples in the prompt below and in the app's "What do the peppers mean?"). `spice_level` is stored per book; `classification` is derived for the Open Door filter and older rules (0–2 No Spice, 3 Closed Door, 4–5 Open Door). Books rated before the scale keep `spice_level` NULL and, for kids' limits, count as the highest level their old label allows (No Spice = 2, Closed Door = 3, Open Door = 4). Kid accounts have `max_spice` (−1 = no limit; age-group presets 0/1/2/3/none). Admins/editors can re-rate AI-rated older books in place (they stay visible; failures keep the old rating); hand-rated books are never re-rated.
* **Parent approval:** Admins/editors can mark a book "OK", which overrides hide filters and restricted accounts' content rules (e.g. Harry Potter's fantasy magic).
* **Calibre removal:** Admins can list the Calibre books their hide filters catch (never parent-approved ones) and either copy a Calibre search (`id:=N or …`) or, with the calibre Content server connected, remove them in one click. One-click removal goes through calibre's own remote interface (`/cdb/cmd/remove`, to calibre's recycle bin) after re-checking the list and verifying every id's title against calibre. NovelCheck's own access to the library stays read-only.
* **Feedback:** In-app links to GitHub issue forms for filter suggestions and general feedback.
* **Age groups & notes:** Books carry a parent-set `age_level` (1 Young kids ≤8, 2 Middle grade 9–12, 3 Teens 13–15, 4 Young adult 16–17, 5 Adults 18+). Restricted users have an `age_level` too; they never see books rated for an older group (this applies even to "OK"-marked books), and a parent-set age group satisfies "hide unrated". New kid accounts get content-rule presets per age group. Parents' notes (`book_notes`) are visible to `everyone` or `parents` only; authors (or admins) edit/delete them.
* **Backup AI:** optional second provider (`backup_llm_*`, `backup_price_*`; `Store.AIConfigs()` returns main then backup). The worker tries the main models, then the backup's; a connection-level failure (dial/DNS) skips the rest of that provider's models. Each call's cost is stored in `token_usage.cost` using that provider's prices (`Store.SpentUSD`). A `llm-backup` notification is raised when the backup rates a book and resolved when the main AI succeeds. Find Ollama can target the backup (`POST /api/admin/ollama/use` with `target: "backup"`).
* **GPU-aware recommendations:** `GET /api/admin/ollama/gpu?url=&probe=1|vram_gb=N` (`internal/ollama/gpu.go`). Ollama has no GPU-info API, so VRAM is inferred from `/api/ps` (`size_vram` vs `size` of the largest loaded model: 0 = CPU only, partial ≈ VRAM, full = at least); `probe` loads the largest downloaded model with `keep_alive` 1m via `/api/generate`, measures, then unloads it (`keep_alive` 0). A curated catalog is labelled best / powerful / fits / too_big (model size + 1.5 GB headroom), or cpu_ok / cpu_slow without a GPU.
* **Model chain:** `llm_model` is tried first, then each model in the comma-separated `llm_fallback_model`, in order, until one returns a valid verdict. The Ollama helper saves the chain from an ordered picker (`POST /api/admin/ollama/use` with `models`).
* **Duplicates & formats:** Book lists include `formats` (distinct file formats across copies) and `calibre_copies` (distinct Calibre ids). Calibre duplicates are Calibre entries that normalize to the same `norm_key`. The duplicates view suggests a keep (format score EPUB > AZW3/KFX > AZW > MOBI > PDF, then file count, size, lowest id). Admin removal re-validates that each id is still a duplicate, keeps at least one entry per book, verifies titles with calibre, and removes to the recycle bin.
* **Usage (admin):** Resource use of the NovelCheck container (CPU, memory, network rx/tx rates from `/proc/net/dev`, disk, DB size) with a 15-minute in-memory history sampled every 10 s, library progress, AI cost and 14-day daily token use, and Ollama loaded models (GPU vs RAM from `/api/ps`).
* **System & health (admin):** On-demand connection checks for every external dependency with fix hints, including each backup AI model and the public tunnel address (DNS lookup plus a request to `https://<hostname>/api/setup` through Cloudflare).
* **Diagnostics & AI diagnosis (admin):** `GET /api/admin/diagnostics` is a plain-text report (version, settings with secrets reduced to "(set)" and emails masked, library/worker state, resources, recent notifications, cloudflared log). `POST /api/admin/diagnose` sends it (plus an optional problem description) to the first configured model, which returns a diagnosis, fix steps, title, summary, suspected cause and bug/not-bug; the server always builds a GitHub issue body (AI reading + diagnostics, tunnel hostname replaced) even when the AI fails. The UI opens `.github/ISSUE_TEMPLATE/bug_report.yml` prefilled via the `report` field.
* **Notifications:** Persistent, de-duplicated admin/editor notifications (`notifications` table) raised by background failures and failed checks, auto-resolved where the problem clears; error toasts persist until dismissed.
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
- 2 = Romantic: More developed romance with stronger attraction and kissing, including passionate kissing or physical affection. No sexual activity, sexual desire, or implication of sex. The intimacy remains romantic rather than sexual. Example: My Phony Valentine (Courtney Walsh).
- 3 = Steamy Closed-Door: Strong sexual attraction and desire are present. May include heavy/passionate making out, sexual tension, and characters expressing or acting on sexual desire. Any sexual encounter occurs off-page or fades to black; no explicit sexual activity is described.
- 4 = Explicit: Sexual encounters occur on-page and include clear descriptions of sexual activity. Scenes contain meaningful sexual detail rather than simply implying what happens. There may be multiple or extended explicit scenes, but sex does not necessarily dominate the entire book. Examples: Fourth Wing (Rebecca Yarros); A Court of Thorns and Roses (Sarah J. Maas).
- 5 = Very Explicit / Erotica-Level: Frequent, extended, or highly graphic on-page sexual content with extensive detail. Sexual encounters are a major component of the book and may occupy a substantial portion of the story. Example: Fifty Shades of Grey (E. L. James).
If unsure between two levels, choose the higher one.

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
`users` (role `admin|editor|restricted` + `hide_*` content rules + `age_level` + delivery prefs + `guide_seen`) · `sessions` (token hash, expiry, `remember`) · `catalogs` (`calibre` | `drive` | `custom`) · `books` (one row per logical title, with `approved`/`approved_by` for parent "OK" marks and `age_level`/`age_set_by`, keyed by a normalized title + author-surname `norm_key` so the same book in Calibre and on a Kindle is shared; holds status `pending|queued|processing|analyzed|error` and the verdict flags) · `catalog_books` (copies/locations) · `queue_items` (per-user position + `queued|reading|finished`) · `settings` (admin-tunable key/values) · `book_notes` (book, author, body, visibility `everyone|parents`) · `notifications` (level, source, message, link, count, read) · `token_usage` (per-call prompt/completion tokens for caps and cost).

### 5.3 HTTP API
Public: `POST /api/auth/login`, `POST /api/auth/logout`, `GET|POST /api/setup` (first admin, only while no users exist), `GET /healthz`.
Any signed-in user: `GET /api/me`, `PUT /api/me/password`, `PUT /api/me/delivery`, `PUT /api/me/guide-seen`, `GET /api/updates`, `GET /api/books` (filters: `q, catalog, overlap_with, multi, classification, flags, exclude, status, age, format, spice, sort, limit, offset`; `spice` = `0`–`5` or `old`; `format` = a file format, `multi`, `dupes` or `none`; `age` = `unset` or `1`–`5` for "suitable up to"), `GET /api/books/{id}`, `GET /api/books/{id}/download`, `GET /api/catalogs`, `GET /api/age-groups`, `GET|POST /api/queue`, `PUT /api/queue/order`, `DELETE /api/queue/{id}`, `POST /api/queue/{id}/start`, `POST /api/queue/{id}/finish`.
Editor or admin: `GET /api/notifications`, `POST /api/notifications/read`, `DELETE /api/notifications`, `POST /api/catalogs`, `PATCH /api/catalogs/{id}`, `POST /api/import/drive`, `POST /api/books/{id}/analyze`, `PUT /api/books/{id}/verdict` (`spice_level` 0–5), `PUT /api/books/{id}/approval`, `PUT /api/books/{id}/age`, `POST /api/books/{id}/notes`, `PUT|DELETE /api/notes/{id}`, `GET /api/admin/calibre/duplicates`, `POST /api/admin/rerate`, `GET /api/admin/status`, `POST /api/admin/analyze-batch`, `POST /api/admin/calibre-sync`, `GET|POST /api/admin/users`, `PUT|DELETE /api/admin/users/{id}`, `PUT /api/admin/users/{id}/password` (editors: restricted accounts only).
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
