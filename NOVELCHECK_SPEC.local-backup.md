# NOVELCHECK_SPEC.md: NovelCheck

## 1. Project Overview
**NovelCheck** is a self-hosted, Dockerized web application designed to scan, analyze, and catalog e-book libraries for romantic/sexual content ("spice") and sensitive thematic elements (modeled after Common Sense Media ratings with specific Catholic moral discernment criteria).

It connects directly to a **Calibre Library** (via read-only SQLite database access), integrates a **Browser-based Drive Scanner** to index external e-reader drives (e.g., connected Kindles), features a **Netflix-style drag-and-drop Reading Queue** for managing device syncs, and is configured as a **Progressive Web App (PWA)** for home-screen mobile installation over a **Cloudflare Tunnel**.

---

## 2. Core Development Rules (Strict Enforcement)

1. **File Length Limit (Max ~300 Lines):** Keep code modular. No single file (Go, JavaScript, CSS, or SQL) should exceed ~300 lines of code. Split API routes, handlers, database services, and UI components into small, logical sub-modules.
2. **Model Delegation Strategy:** Always delegate to smaller/faster models (e.g., `gpt-4o-mini`, `gemini-1.5-flash`, `llama3.2`) for classification, blurb summarization, and JSON parsing tasks. Save larger models only for complex edge-case fallbacks.
3. **Repository Source of Truth:** Keep GitHub repository files (`README.md`, `docker-compose.yml`, `NOVELCHECK_SPEC.md`) fully synchronized and up to date. Perform regular audits to ensure documentation matches codebase implementation without contradictions.

---

## 3. Technical Stack
* **Language/Backend:** Go (using `chi` or `fiber` HTTP router)
* **Database:** SQLite (GORM or `sqlx` driver) for local caching, custom tags, user accounts, library catalogs, and user queues.
* **Authentication:** JWT or Cookie-based Session Authentication with Role-Based Access Control (RBAC).
* **Frontend & Mobile:** HTML5, Tailwind CSS, Vanilla JavaScript (SortableJS for drag-and-drop queue management), and PWA Manifest/Service Worker for "Add to Home Screen" native app experience. Embedded directly into the Go binary using Go's `embed.FS`.
* **APIs & Integrations:**
  * Open Library API / Google Books API (Metadata & blurb lookups)
  * OpenAI-Compatible LLM Client (OpenAI, Anthropic, Gemini OpenAI-compatible URL, or local Ollama/vLLM instance)
  * Chromium File System Access API (`showDirectoryPicker()`) with fallback to `<input type="file" webkitdirectory>` for iOS/Safari drive scanning
  * SMTP Client (Amazon Send-to-Kindle email delivery)
* **Deployment & Remote Access:** Single-container Docker image optimized for TrueNAS SCALE / Linux hosts (supports GPU passthrough for local LLMs). Configured to store SQLite config databases on SSD datasets and read large media libraries from HDD pools. Designed to run behind a Cloudflare Tunnel (`cloudflared`) or Reverse Proxy with full HSTS, CORS, `Cache-Control: no-store` on API routes, and `X-Forwarded-For` header support.

---

## 4. Core Features & Functional Requirements

### Feature 1: Authentication & Parental Control Filters
* User login system supporting `Admin` and `Restricted` (Kid) accounts.
* Profile-level content rules (e.g., *Hide books flagged as Open Door, Nudity, or Dark Occult*).
* Database-level filtering ensures restricted accounts cannot view or search locked titles.

### Feature 2: Calibre Library Auto-Sync (Read-Only)
* Mount Calibre’s root storage directory into the Docker container.
* Read Calibre’s `metadata.db` SQLite file directly in **strict read-only mode** (`file:metadata.db?mode=ro`) to extract titles, authors, and existing metadata without database lock conflicts.
* Resolve internal file paths by prepending container mount prefix `/calibre/` to relative paths extracted from `metadata.db`.
* Assign synced entries to a default catalog named `Calibre Main`.

### Feature 3: Browser Drive & Kindle Scanner
* Front-end interface includes an "Import Local Drive / Kindle" feature using `window.showDirectoryPicker()`.
* Automatically falls back to standard `<input type="file" webkitdirectory>` on non-Chromium browsers (iOS Safari, mobile browsers).
* Recursively traverses selected directory folders (e.g., `documents/`) to read book filenames and metadata tags (`.epub`, `.mobi`, `.azw3`).
* Sends extracted metadata payloads to the Go backend API.
* Prompt user to create or select a destination catalog (e.g., `Jenna's Kindle`).

### Feature 4: Multi-Catalog UI & Filtering
* **Unified Dashboard:** Browse all books across all catalogs simultaneously.
* **Filtering:**
  * Filter by Catalog (`Calibre Main`, `Jenna's Kindle`, or Custom).
  * Filter by Spice Classification (`Closed Door`, `Open Door`, `No Spice`).
  * Filter by Content Flags (Nudity, Solo Acts, LGBTQ+ Content, Dark Occult / Demonic).
  * Filter Cross-Catalog Overlap (View books present in both Calibre AND Kindle).

### Feature 5: Progressive Web App (PWA) "Install as App" Support
* Includes `manifest.json` metadata defining application icons, dark standalone theme colors, app name (`NovelCheck`), and `display: standalone`.
* Lightweight Service Worker (`sw.js`) enabling offline app shell caching and triggering native mobile "Install App" / "Add to Home Screen" prompts on iOS Safari and Android Chrome.

### Feature 6: Netflix-Style "Reading Queue" & Start Reading
* **Personal Queues:** Every logged-in user gets their own ordered queue ("Up Next").
* **Drag-and-Drop Reordering:** Reorder books in the queue using interactive drag handles (persisted to SQLite via position integers).
* **"Start Reading" Action Button:**
  * Triggers immediate delivery based on user preference (e.g., emails `.epub` via Send-to-Kindle SMTP, or flags for KOReader wireless sync).
  * Moves book status from `Queued` to `Currently Reading`.

### Feature 7: Flexible LLM Analysis Engine & Admin Control Panel
* **Small Model Delegation:** Default LLM calls to lightweight, high-efficiency models (e.g., `gpt-4o-mini`, `gemini-1.5-flash`, `llama3.2`) for fast structured JSON outputs.
* **Metadata Enrichment Step:** Query Open Library / Google Books API for summary blurbs before LLM execution to prevent hallucinations on indie/self-published titles.
* **On-Demand & Queued Processing:** Syncing a library indexes metadata instantly without triggering LLM API calls for every book at once. Books display a `Pending Analysis` status until processed.
* **Comprehensive Admin Settings Panel:**
  * **Single & Batch Scanning:** On-demand scan button per book + batch size limits (e.g., *Analyze 20 books at a time*).
  * **Token / Hourly Rate Caps:** Configurable max token limits per hour (e.g., *Cap at 25,000 tokens/hr*) and scan delays.
  * **Cost & Token Estimator:** Real-time token counter and estimated API cost dashboard.
  * **SMTP / Send-to-Kindle Configuration:** Manage host, port, and approved sender email credentials.
  * **Calibre Polling Schedule:** Configurable background sync intervals (e.g., *Sync Calibre every 6 hours* or *Manual trigger only*).
  * **Database Backup & Maintenance:** 1-click SQLite database download (`novelcheck.db`) + wipe pending queue option.

#### System Prompt Specification
```text
You are an expert book content analyzer. Your sole purpose is to determine the nature of romantic/sexual content ("spice") and specific thematic elements (including spiritual/occult themes) in a given book using the provided book title, author, and blurb metadata. The user wants to avoid specific types of content. You must be precise and objective while strictly adhering to the formatting and safety rules below.

STRICT RULES:
1. NO SPOILERS: Do not reveal major plot twists, endings, or critical character deaths.
2. NO EXPLICIT/GRAPHIC LANGUAGE: Do not use anatomically explicit terms, graphic descriptions, or vulgar words in your analysis. Use clinical or modest phrasing (e.g., "solo acts").
3. NO AFFIRMATIONS OR FILLER: Provide the output strictly matching the JSON payload format.

CATEGORIES & GUIDELINES:

1. Classification:
- Closed Door: Romantic tension exists. Intimacy occurs off-page or cuts away.
- Open Door: Explicit sexual acts described on the page.
- No Spice: No physical intimacy or sexual activity occurs.

2. Content Elements:
- Nudity: Presence of nudity in a romantic or intimate context.
- Solo Acts: Private, solo intimate acts by any character.
- Heavy Innuendo: Detailed physical foreplay or highly suggestive text.

3. Spiritual & Occult Classification:
- Whimsical / Standard Fantasy: Fictional fairy-tale magic, standard wizards (e.g., Merlin, Gandalf), or light YA fantasy (e.g., Harry Potter). (Mark dark_occult: false)
- Dark Occult / Demonic: Explicit real-world occult practices, black magic rituals, demonic possession, or active demonic themes. (Mark dark_occult: true)

OUTPUT FORMAT (JSON ONLY):
{
  "classification": "Closed Door | Open Door | No Spice",
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