# NovelCheck handoff (2026-09-24, moving from the cloud session to desktop)

## Where things are

- **Repo:** `github.com/ZachCurry13/novelcheck`. Default branch `main`. Working branch `claude/epic-newton-z23mz1`.
- **Released:** v1.14.1 is the latest published release. Every version up to v1.14.1 is built and published (GHCR image `ghcr.io/zachcurry13/novelcheck`).
- **v1.14.2** (duplicates: Check again, removal confirmation, file-less entries) is committed on `main` (7590b66). Its release run was started with `workflow_dispatch` (version `1.14.2`). **Check it finished** and that v1.14.2 is the latest release. If not, re-run **Actions → Docker image → Run workflow** with version `1.14.2`.
- **Unreleased, committed only on the working branch (not on `main`):** the **Delete requests** feature, described below. It's finished and tested but not yet released.

## Delete requests (next release, v1.15.0)

User request: "Users should be able to request to delete; the admin reviews that list and deletes as needed."

Done:
- `internal/db/schema.sql`: new `delete_requests` table. Title and author are copied into it, `book_id` uses `ON DELETE SET NULL`, and a unique pending request per (book, user).
- `internal/store/deletes.go`:
  - `RequestDelete`, `CancelDelete`, `MyDeleteRequest`, `PendingDeletes` (grouped per book, with Calibre ids and formats), `PendingDeleteCount`.
  - `PendingRequestIDs` and `DecideRequests`: request ids are captured **before** removal, because the Calibre re-sync deletes the book and nulls `book_id`.
  - `DecideDeletes`, `RecentDeleteDecisions`.
  - The book list gets a derived `delete_requests` count (`books.go` `derivedCols`, `models.go`).
- `internal/api/deletes_handlers.go`:
  - `POST|DELETE /api/books/{id}/delete-request` (any signed-in user).
  - `GET /api/admin/delete-requests` and `POST /api/admin/delete-requests/decide` with `{book_ids, action: delete|dismiss|done}` (admin). `delete` reuses `removeFromCalibre` (title check, recycle bin, synchronous re-sync). Books not in Calibre stay pending, to be marked `done`.
  - A `delete-requests` notification is raised on request and resolved when nothing is pending.
  - `GET /api/books/{id}` returns `my_delete_request`; admin status returns `pending_deletes`.
- UI:
  - Book window: **🗑 Request to delete** (optional reason prompt) / **Delete requested · Cancel**, plus an admin link **Review delete requests (N)**.
  - Library card chip **🗑 Delete requested**; admin link **Delete requests** in the Library toolbar.
  - New page `web/static/js/deletions.js` (route `#/deletions`, admin-only): tick books, then **Delete from Calibre**, **Keep**, **Mark done** or **Copy Calibre search**, plus a **Recently handled** history.
  - Admin page banner "N books are waiting for your delete review". Service-worker cache bumped to v22; CSS rebuilt.
- Tests: `internal/api/deletes_test.go` and `TestDeleteRequestSurvivesBookDeletion` in `internal/store/features_test.go`. All Go tests pass.
- Browser-tested end to end against a real `calibre-server`: the editor requested 2 deletions with reasons, the admin saw the banner and the review page, deleted one (gone from Calibre) and kept the other, and the history shows both.

Still to do before releasing:
1. **Bell badge:** in the browser test the admin's 🔔 badge showed 0 right after the requests. The notification is created server-side (the API test checks it). The bell probably just hadn't refreshed yet; it polls every 60 s. Verify, and optionally refresh the bell on page load.
2. Docs: add **Delete requests** rows/sections to `README.md`, `NOVELCHECK_SPEC.md` (API list, tables, build step 26) and `docs/TRUENAS.md` if relevant, plus a `CHANGELOG.md` `## [1.15.0]` entry in plain English.
3. Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `node --check` on `web/static/js/*.js`.
4. Merge the working branch into `main`, push, then run **Actions → Docker image → Run workflow** with version `1.15.0`, and confirm the release.

## Waiting on the user

- **Kindle / Amazon import (~500 store books):**
  - 4–5 file names from the Kindle's `documents` (and `documents/Downloads/Items01`) folder, to see whether `.kfx` names include titles.
  - A few books copied from Amazon **Manage Content and Devices → Books**, to teach **Paste a list** that format.
  - Optionally a sample of Amazon's **Request Your Data** Kindle order CSV (column names plus 1–2 rows).
  - I declined automatic Amazon login or scraping (against Amazon's terms, would need her password and 2FA stored).
- **TrueNAS / Ollama:** the earlier question of whether "user login information" means a TrueNAS API key, which would allow real per-app CPU/GPU stats. Not built.

## Project conventions (keep following)

- Files stay at roughly 300 lines or fewer. Go with chi, sqlx and modernc sqlite; vanilla ES modules; Tailwind is compiled (`make css`) and the CSS is committed.
- Strict CSP: no inline styles; set widths through the DOM.
- Every user-facing change needs:
  - a plain-English `CHANGELOG.md` entry (it becomes the release notes and the in-app **What's new**);
  - README, spec and docs kept in sync;
  - a service-worker cache bump in `web/static/sw.js` when JS files change.
- Releases: push to `main`, then run the `docker.yml` workflow with a `version` input. It creates the tag, the release and the image. Don't put model identifiers in commits.
- Secrets never go to the browser. The diagnostics report masks them.
- The user (and his wife, an **editor**) run this on TrueNAS. Explanations should be non-technical.

## Recent releases today (for context)

- **1.8.x:** age groups, notes, the Usage tab, the Ollama address fix and MTP Kindle help.
- **1.9.x:** duplicates and formats, Gmail App Password fixes.
- **1.10:** ordered Ollama models.
- **1.11.x:** pepper scale (0–5) and local AI timeouts.
- **1.12:** AI diagnosis and GitHub bug reports, copy buttons.
- **1.13:** backup AI and the public-address check.
- **1.14.x:** GPU model recommendations, rating errors with retry, duplicates Check again.
