# NovelCheck handoff

Latest release: **v1.15.0**. **v1.16.0** (pepper wording + quick wins, see CHANGELOG.md) is committed on the working branch `claude/epic-newton-z23mz1`. The user asked to **hold the release** until more batches are done; later batches can share the release or get their own version numbers when it's cut.

## Feature plan (from the user's NOVELCHECK_UPDATES list, agreed 2026-09-24)
1. ✅ v1.16 quick wins: pepper levels 2-3 reworded (+ `spice_reason`, `rules_version` re-rate banner), Strict family preset, gray-area chip, Re-rate whole library, Calibre-Web links + credit, phone filters.
2. ✅ Feature switches (Admin → Features: queue, Send-to-Kindle, KOReader, import, parent tools) + 🔔 events: `calibre-new`, `batch-done`, `reading`, `token-cap` (routine ones can be turned off with `notify_routine`). Also in the unreleased 1.16.0 notes.
3. ✅ Phone push: `internal/push` (stdlib RFC 8291 + VAPID, tested against the RFC example), Profile → Phone notifications, managers get 🔔 notices (all or problems only), everyone gets "Ready to read". Only the UI's off/blocked states were checked in a browser (the app's pane blocks notification permission); the first real subscribe/test should be done on a phone over the https address after release.
4. ✅ Custom AI filters (Admin → Custom AI filters, up to 12; `flag:<key>` Hide boxes; `custom_flags_version` drives re-rate offers). Not yet in kids' content rules.
5. ⏭ **Check a book** (user: "should be the main feature, super easy and straightforward"): at the store, snap the cover or type title/author → instant rating. First tab and parents' landing page. If the book is in the library show its rating; else enrich + rate now (vision AI reads the cover; typed search as fallback) and save it to a "Looked up" catalog so a second check is free. Parents (admins/editors) only.
6. Parent-child linking: kids linked to one or more parents; parents manage only their linked kids (user chose to build it).
- Skipped by the user's choice: a "Skipped (up to date)" badge (same as Analyzed). Not requested: re-rating when a Calibre file changes.

## Waiting on the user
- When to merge and release (held for now).
- Kindle `.kfx` file names from the Kindle's `documents` folder, to check whether on-device store books carry titles in their names. The Amazon list import (paste or data download) covers her purchases in the meantime.
- Whether "user login information" for more Ollama detail meant a TrueNAS API key (real per-app CPU/GPU stats). Not built.

## Working on the Windows desktop (`C:\novelcheck`)
- Node.js LTS is installed (no `make`): build CSS with `npx tailwindcss@3 -c tailwind.config.js -i web/tailwind.input.css -o web/static/css/app.css --minify`. Shells started before the install may need `C:\Program Files\nodejs` on PATH. For a visual check, run the app locally (`.claude/launch.json`, untracked) on a fresh port to dodge the 1-hour static cache.
- The clone uses LF endings (`core.autocrlf=false`). `go test ./...` passes except Windows-only failures in `internal/calibre`, `internal/db` (temp-file lock at cleanup) and `internal/tunnel`; use `GOOS=linux go vet ./...`. CI on Linux is the real gate.

## Conventions
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled with `make css` and committed; strict CSP (no inline styles).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done` (plain `node --check` misses some errors).
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, bump the `web/static/sw.js` cache name when JS changes.
- Release: push to `main`, then run **Actions → Docker image → Run workflow** with the `version`. No model identifiers in commits.
- The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
