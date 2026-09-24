# NovelCheck handoff

Latest release: **v1.15.0**. **v1.16.0** (pepper wording + quick wins, see CHANGELOG.md) is committed on the working branch `claude/epic-newton-z23mz1`; it goes to `main` and gets released once the user says so.

## Feature plan (from the user's NOVELCHECK_UPDATES list, agreed 2026-09-24)
1. ✅ v1.16 quick wins: pepper levels 2-3 reworded (+ `spice_reason`, `rules_version` re-rate banner), Strict family preset, gray-area chip, Re-rate whole library, Calibre-Web links + credit, phone filters.
2. ⏭ Module toggles (hide Send-to-Kindle, KOReader, drive scanner, reading queue, parent tools) + new 🔔 events: Calibre sync finished, AI batch finished (books, tokens, cost), book sent, token cap reached.
3. Phone push (Web Push/VAPID) for those events. Needs HTTPS (remote access); iPhone needs the PWA installed.
4. Custom AI filters: admin-defined flags injected into the prompt, auto-generated Hide checkboxes. Bump `store.RulesVersion` when flags change.
5. Parent-child linking: kids linked to one or more parents; parents manage only their linked kids (user chose to build it).
- Skipped by the user's choice: a "Skipped (up to date)" badge (same as Analyzed). Not requested: re-rating when a Calibre file changes.

## Waiting on the user
- Go-ahead to merge and release v1.16.0.
- Kindle `.kfx` file names from the Kindle's `documents` folder, to check whether on-device store books carry titles in their names. The Amazon list import (paste or data download) covers her purchases in the meantime.
- Whether "user login information" for more Ollama detail meant a TrueNAS API key (real per-app CPU/GPU stats). Not built.

## Working on the Windows desktop (`C:\novelcheck`)
- No Node: Tailwind can't be rebuilt, so use classes already in `app.css`; hand-written rules go at the end of `web/tailwind.input.css` (plain CSS) and are appended to `app.css`. JS is checked by importing every module in a local run (preview server, fresh port to dodge the 1-hour static cache).
- The clone uses LF endings (`core.autocrlf=false`). `go test ./...` passes except Windows-only failures in `internal/calibre`, `internal/db` (temp-file lock at cleanup) and `internal/tunnel`; use `GOOS=linux go vet ./...`. CI on Linux is the real gate.

## Conventions
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled with `make css` and committed; strict CSP (no inline styles).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done` (plain `node --check` misses some errors).
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, bump the `web/static/sw.js` cache name when JS changes.
- Release: push to `main`, then run **Actions → Docker image → Run workflow** with the `version`. No model identifiers in commits.
- The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
