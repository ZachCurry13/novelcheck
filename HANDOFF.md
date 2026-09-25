# NovelCheck handoff

Latest release: **v1.17.0** (2026-09-25; the V2 list: Deep Scan, barcode scanner + wishlist, KOReader catalog, kids' presets, changed-book re-rates, Admin tabs, chart readouts + Ollama cleanup, tidy titles). `feature/v2-updates` was fast-forwarded into `main`. Next: **§12 Suggested Reads** for **v1.18.0**, on a new branch from `main`. Commit per section; release when the user says so.

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
10. ⏭ **Next (v1.18)**: §12 suggested reads under Up Next (match library books to queue picks, read history and content filters; 👍 boosts similar authors/series/tropes with Add to Up Next / Wishlist, 👎 hides it and logs an exclusion so similar titles are suppressed).
- Already done before V2: §7 (switches, custom filters), §4 (scale, strict preset, reasons), §5 push, §9 diagnostics copy + Calibre-Web, §13 GPU detection.
- Dropped by the user: parent-child linking.

## Waiting on the user
- If the app ever restarts on its own again, the TrueNAS app log (panics are now logged with a stack).
- Kindle `.kfx` file names (do store books carry titles?) and whether "user login information" meant a TrueNAS API key. Not built.

## Working on the Windows desktop (`C:\novelcheck`)
- Node.js LTS is installed (no `make`): build CSS with `npx tailwindcss@3 -c tailwind.config.js -i web/tailwind.input.css -o web/static/css/app.css --minify`. For a visual check, run the app locally (`.claude/launch.json`, untracked) on a fresh port to dodge the 1-hour static cache, and stub `fetch` (sign-in isn't possible).
- The clone uses LF endings (`core.autocrlf=false`). `go test ./...` passes except Windows-only failures in `internal/calibre`, `internal/db` (temp-file lock at cleanup) and `internal/tunnel`; use `GOOS=linux go vet ./...`. CI on Linux is the real gate.

## Conventions
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled and committed; strict CSP (no inline styles, no external scripts: vendor libraries into `web/static/vendor`).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done`.
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, bump the `web/static/sw.js` cache name when JS changes. Neutral wording: no personal names or pronouns in the app or docs.
- Release: push to `main`, then run **Actions → Docker image → Run workflow** with the `version`.
- The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
