# Status

- **2026-11-09:** added to `feature/v1.48`: plugin Developer tools + safe module loading (plugin 1.48.0), device logs and Settings → Developer, same-book detection with automatic joining after syncs, and Calibre change detection. Next: owner tries the plugin on the Kindle (`LOAD_ERROR.txt` / crash.log if still unrecognised), then the 1.47.0 and 1.48.0 releases.

- **2026-11-02:** v1.48 audiobooks built on `feature/v1.48` (Audiobookshelf import, length sort/filter, browser download and player, other-format requests, Libby audiobooks); tests added in `abs_test.go`. Next: owner's OK, then release 1.47.0 and 1.48.0 (1.48 branches from 1.47). The real server isn't tried yet: only a fake Audiobookshelf.

- **2026-10-26:** stored the maintenance protocol in `CLAUDE.md` (a project-neutral copy is `docs/MAINTENANCE_PROTOCOL.md`); 1.45.1 updated `golang.org/x/net` to v0.60.0. Added `.github/workflows/audit.yml`: a key-free weekly audit (Mondays 06:17 UTC, or run it by hand; untick "Open or update the issue" for a dry run) that reports vulnerabilities, newer direct dependencies and the cloudflared pin in one issue titled "Weekly audit".
- **Correction:** CI had been using exactly Go 1.26.0 (so the scan showed 30 stdlib findings). Both workflows now ask for the `1.26` line (newest patch, 1.26.10 today); the audit then reports 0 vulnerabilities. The 1.45.1 note about CI was premature; it is true from this change on.
- **2026-10-26 (later):** applied `cloudflared` 2026.10.4 (Dockerfile pin; verified by the image build before release). Newer direct dependencies left for later (no known vulnerability): `anthropic-sdk-go` 1.84, `chi` 5.5, `x/crypto` 0.58, `x/net` 0.61, `modernc.org/sqlite` 1.61.

- 2026-11-09: v1.49 on branch feature/v1.49 (not released): "Add to NovelCheck" for unmatched Kindle books, plugin 1.49.0 shows the download folder. Next: owner OK to release; real-Kindle check of the add button.
- 2026-11-09: v1.49.0 adds the bug-report link, browser Send report, self-test auto-send, Add to NovelCheck, NovelCheck download folder, device-report 500 fix and path-loaded plugin files. Next: owner to update the Kindle plugin and re-test report books, Tidy names and New in library.
- 2026-11-09: v1.50.0 (feature/v1.50): server error log with E-codes, auto-send of reports, notes, UTC logs, delete/expire for bug reports and the link. Next: owner updates plugin to 1.50.0 and tests Send log with a note.
- 2026-11-09: v1.51.0 (feature/v1.51, on top of unreleased v1.50): Library cleanup flow, Nightstand same-book banner, 45 s list cache, slow-request report. Next: owner OK to release 1.50.0 + 1.51.0 together; then read the Slow requests section after a day of use.
- 2026-11-09: v1.52.0 (feature/v1.52): title saves answer at once (background Calibre re-read, one at a time, button shows progress). Next: owner OK to release; confirm Scarlet saves fast.
- 2026-11-09: v1.52 (feature/v1.52): same-book pairs carry evidence and a confidence; spin-offs no longer paired; auto-join only when sure. Next: owner OK to release 1.52.0; check the Listed twice list again.
- 2026-11-09: v1.52 also: Kindle finished books leave the Nightstand (plugin 1.51.0), genres auto-fill. Next: owner OK to release 1.52.0; then design the Wrapped / achievements page.
- 2026-11-09: v1.53.0 (feature/v1.53): Admin → Library cleanup tab. Next: owner OK to release; then Wrapped/achievements.
- 2026-11-09: v1.54.0 (feature/v1.54, on top of unreleased v1.53): Back button fixes. Next: owner OK to release 1.53.0 + 1.54.0 together; then Wrapped.
- 2026-11-09: v1.55.0 (feature/v1.55): review-first cleanup, keep choice for joins, stricter pair evidence, box sets join existing books, tidy reasons. Open: ask owner whether NovelCheck and the Content server read the same Calibre library (Clean Library vs config); read Last format job in the next report.
- 2026-11-09: v1.55.0 is the big release (everything on feature/v1.55). Before release: full test run. After: owner to try Wrapped, the Libby filters, Find the missing books, and read the next bug report for the conversion failures and the Clean Library vs config question.

- 2026-11-09: v1.56 on branch feature/v1.56 (not released): Libby search + Add to Nightstand, full-screen Wrapped story, compact filter popover, iPhone Settings/dialog history fix. Tests: only the six known Windows failures. Next: owner OK to release 1.56.0; sister to retry Settings on her iPhone.

- 2026-11-09: v1.56.1 on feature/v1.56.1 (not released): "new version ready" Reload bar (`appupdate.js`). Next: owner OK to release; owner reloads the phone to pick up 1.56.0 (the Settings fix).
