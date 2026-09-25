# NovelCheck handoff

Latest release: **v1.18.0** (2026-09-25; Suggested Reads, taste profile, covers, browse filters, AI genres, problem reports, library owners, multi-select). Before that **v1.17.0** (2026-09-25; the V2 list: Deep Scan, barcode scanner + wishlist, KOReader catalog, kids' presets, changed-book re-rates, Admin tabs, chart readouts + Ollama cleanup, tidy titles). `feature/v2-updates` was fast-forwarded into `main`. Next: **§12 Suggested Reads** for **v1.18.0**, on a new branch from `main`. Commit per section; release when the user says so.

## V2 list: status
1. ✅ §1 fixes: GET retry on resume (`api.js`), `internal/safe` panic guards, batch size 0 = all (max 500), no delete for looked-up books, neutral "Level N" labels (no nicknames/pronouns), photo fallback on Check a book.
2. ✅ §3 Deep Scan: `internal/epub`, `internal/deepread`, `deep_reads` (+ audit), Admin → 🧬 Deep Scan (next N with estimate, up to 3 auto users, approvals), escalation warning + Up Next banner, 🧬 filter/badge.
3. ✅ §2 live barcode scanner (`barcode.js`, vendored ZXing, camera allowed for self) + wishlist (`wishlist` table, ⭐ Wishlist tab, owned-aware actions, pending acquisition in Up Next).
4. ✅ §5 KOReader OPDS catalog (`/opds/<token>`, QR, steps, kids' devices), Send-to-Kindle confirmation with the approved-sender steps, admin guide step. (VAPID push was done in v1.16.)
5. ✅ §6 presets: Strict Family + Young Reader (age + pepper cap + rules in one click), kids' cap picker 0–3.
6. ✅ §8 delta scanning: Calibre last_modified / file mtime per copy vs the rating's `rated_modified`; changed books join the re-rate banner (never automatic).
7. ✅ §10 Admin tabs (AI & Scans, Users & Rules, Delivery & Services, System & Toggles), collapsible cards, per-tab save, backup fields shown only when enabled, user-card badges.
8. ✅ §9 tap-to-see values on Usage charts; §13 Ollama model list with disk use + delete (in-use models protected).
9. ✅ §11 tidy titles: `internal/titles` parser, series + number on books, sync follows Calibre renames (ratings kept, stale file links pruned), Admin 🏷️ Tidy in Calibre + per-book edit via the Content server (Calibre-Web fallback).
10. ✅ §12 suggested reads: moved to v1.18 (see below).
- Already done before V2: §7 (switches, custom filters), §4 (scale, strict preset, reasons), §5 push, §9 diagnostics copy + Calibre-Web, §13 GPU detection.
- Dropped by the user: parent-child linking.

## v1.18 list (branch `feature/v1.18-suggestions`), from the user on 2026-09-25
1. ✅ §12 Suggested Reads: `internal/suggest` free matching + daily AI picks (+ books you don't own), row under Up Next, 👍/👎 with an optional "Why not?" (story / author / series / too spicy / already read) that steers the matching. Admin: `suggest_mode` (free / AI / AI + outside) and the `module_suggestions` switch.
2. ✅ Taste profile: an optional "rate 20 books" list (want to read / don't want / read & liked / read & didn't like; "20 more"), a varied mix from the library (series firsts, different authors and pepper levels), saved per person and editable, feeding the same signals as 👍/👎. Its own admin switch. Marking books uses no AI.
3. ✅ Book covers everywhere (taste picker first, then library cards, suggestions, Up Next, the book window): Calibre's `cover.jpg` next to each book's files, served read-only and shrunk + cached under /data. Plus Open Library covers by ISBN for looked-up books, and "🖼️ Wrong cover?" reports reviewed on the Admin page.
4. ✅ Library filters: author, series, genre (from Calibre tags, which the sync doesn't read yet), fiction vs nonfiction, the basic categories. Show the user the category list before building. Done: genres from Calibre tags, fiction/nonfiction, author/series type-ahead, links from the book window; Suggested Reads "From" one library. AI fills genres for untagged books (Admin button with a cost estimate, 25 per call).
5. ✅ Bug reports: the user chose "family members tell their admin": anyone can press Report a problem; it lands in the admin's 🔔 and an Admin list, and the admin passes it on (GitHub via Diagnose as today).
6. ✅ Library multi-select: tap to tick books (and drag across the grid to tick a run), then one bar: ＋ Up Next, 🧬 Deep Scan (admins start, others request), 🗑 delete (request; admins review as usual).
7. ✅ Library ownership: whoever imports a library owns it (`catalogs.owner`); owners can remove books from their own library without a request, and mark it Private (only them) or Shared (everyone). Private = the owner and admins only (the user chose this; every adult is an editor or admin, and kids can't import).


## v1.18.1: Deep Scan fix (branch `fix/deep-scan-checks`)
The user's library showed Harry Potter 2 and 2001 as Level 4 from Deep Scans by llama3.2 3B: parts scored tension and violence as peppers and the old max-of-parts combine let one part decide. Fixed with checks v2 (romance-first answers, "none" = 0, scale-copy guard, second look for 3+, flags need backing, big jumps held for review, old deep ratings re-rated from the description at start-up, small-model warning). Their setup: main model llama3.2:latest, fallback qwen2.5:7b (suggest making qwen2.5:7b main and the Deep Scan model).

## v1.19 ideas from the user (2026-09-25), not started
1. One shared family login with several reading profiles (Netflix-style, e.g. each parent plus a young child), each with its own Up Next, taste and suggestions, instead of linking parent accounts.
2. Physical libraries: continuous barcode scanning (and title/author/ISBN or cover photo) to add books to a named physical library, with the same ratings and filters.
3. Discover tab: popular now, new releases, all-time classics, top teen and kids' books, new in a library, popular in the family's library; the family's content filters apply, and the pepper level shows on every book.
## Waiting on the user
- If the app ever restarts on its own again, the TrueNAS app log (panics are now logged with a stack).
- Kindle `.kfx` file names (do store books carry titles?) and whether "user login information" meant a TrueNAS API key. Not built.

## Working on the Windows desktop (`C:\novelcheck`)
- Node.js LTS is installed (no `make`): build CSS with `npx tailwindcss@3 -c tailwind.config.js -i web/tailwind.input.css -o web/static/css/app.css --minify`. For a visual check, run the app locally (`.claude/launch.json`, untracked) on a fresh port to dodge the 1-hour static cache, and stub `fetch` (sign-in isn't possible).
- The clone uses LF endings (`core.autocrlf=false`). `go test ./...` passes except Windows-only failures in `internal/calibre`, `internal/db` (temp-file lock at cleanup) and `internal/tunnel`; use `GOOS=linux go vet ./...`. CI on Linux is the real gate.

## Conventions
- MIT license (`LICENSE`, © 2026 ZachCurry13); vendored libraries keep their own (listed at the end of the README).
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled and committed; strict CSP (no inline styles, no external scripts: vendor libraries into `web/static/vendor`).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done`.
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, bump the `web/static/sw.js` cache name when JS changes. Neutral wording: no personal names or pronouns in the app or docs.
- Release: push to `main`, then run **Actions → Docker image → Run workflow** with the `version`.
- The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
