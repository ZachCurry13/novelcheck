# NovelCheck handoff (2026-09-28, night)

## 0. v1.20.0 released (2026-09-28)
- The user merged `feature/v1.20-discover` into `main` on GitHub (PRs #2 and #3), then asked for the release: GitHub release v1.20.0 and its image were built on 2026-09-28.
- PR #3 ("feature v1.20") also committed `.claude/launch.json` (local paths with the Windows username) and `NOVELCHECK_SPEC.local-backup.md` (an old spec copy). A follow-up commit removed both and added them to `.gitignore`; they remain in history (removing that would need a force-push to `main`, which the user hasn't asked for).
- Built and tested: the Discover tab, card premises (`books.premise`, `content.Version` 2, `blurb.js`), the phone sideways-scroll fix (`:where(.grid)`), the Deep Scan review fix (held scans first, `keep-all`) and title tidying. README, `NOVELCHECK_SPEC.md` (build item 50), `CHANGELOG.md` `[1.20.0]`, the guide and `docs/DISCOVER.md` are done.
- Next: the user's priorities from the Gemini manifest (section 2), or the planned order: physical libraries, then parent profiles.
- The user's *Bridget Jones's Diary* shows Level 0 from llama3.2 3B; advise switching the main model to qwen2.5:7b and re-rating (after 1.20, so one pass adds content details and premises).
- Release rules are allowed in `.claude/settings.local.json`; run each release command as its own call.

## 1. Goal and active task
NovelCheck (Go + vanilla JS PWA, self-hosted on the user's TrueNAS) rates a family's Calibre library for romance ("peppers") and content.
- **v1.18.2 was released** on 2026-09-28 (GitHub release v1.18.2, image built).
- **v1.19.0 (detailed content filters) is built** on `feature/v1.19-content-filters` and waits for the user's OK to release.
- Next: v1.20 Discover.

## 2. Decisions made (and why)
- **v1.19 content details (all agreed with the user, 2026-09-28):**
  - The user's list: 45 items in 5 groups (🗣️ Language, ⚔️ Violence, 🩸 Gore, 🍺 Substance Use, 🧩 Other Content). "Sexual assault" is merged into "Sexual violence / assault". The LGBTQ+ flag became the item `lgbtq`; a one-time migration moves the data.
  - 🌶️ peppers, nudity/solo/innuendo, ✨ spiritual flags and custom filters are unchanged.
  - **Each item is yes/no.** Deep Scans also give an amount per group (A little = 1–2 parts, Some, A lot = more than a third of the parts). The amount is **shown only**, never used to hide.
  - In a Deep Scan an item counts from **one part**.
  - **Both** the quick rating and Deep Scan fill the items; the book window shows the source.
  - Books not yet checked for content are hidden only from kids who have content rules **and** "Hide unrated". A parent's age group counts as checked, and a rule on LGBTQ+ alone is exempt, since older ratings covered it.
  - Presets tick starter sets: Strict Family = 12 items, Young Reader = 30. New kid accounts start with Young Reader's set (age groups 1–2) or Strict Family's (3, or no age group); ages 4–5 start with none. Existing kids don't change.
  - Re-rate: AI-rated books get a full re-rate as before. Deep-scanned and parent-rated books keep their rating and only get a content check.
  - Edit rating: ticking nothing on a never-checked book doesn't count as "checked, none found".
- **Order:** v1.19 content filters → v1.20 Discover → physical libraries → parent profiles (optional 4-digit PIN).
- **The Gemini "V2 manifest"** (a local file of the user's, deliberately not in the repo) is a wishlist. Its big areas await the user's priorities:
  - Stuff Your Kindle event pages
  - Seasonal filters and AI collections, with kids limited to assigned collections
  - Light/dark theme, dyslexia font and reduced motion
  - Series page, box-set splitting and TV-title aliases
  - Two-machine failover, power-cost labels and benchmarks
  - Model-update alerts
  - Format pruning with a 7-day trash, and EPUB conversion
  - Safe mode and a CLI
  - Search history and "The"-stripping sort
  - Read-status sync
  - Never put the user's own setup details (hardware, LAN IP, ports, electricity rate, time zone) in the repo.
- **DeepSeek-R1:** `<think>` notes are stripped (1.18.2), but `MaxTokens: 600` in `internal/llm/client.go` is too small for its thinking; real support is later work.
- **Rules:**
  - Releases need the user's OK. Claude Code's auto-mode classifier blocks Claude from pushing `main` and triggering the release workflow, so the user runs these themselves:
    - `git checkout main`, `git pull --ff-only`, `git merge --ff-only <branch>`, `git push origin main`
    - `gh workflow run docker.yml --ref main -f version=X.Y.Z`
    - `git checkout <branch>`
  - Every `main` push rebuilds `:latest`, which their TrueNAS pulls.
  - Neutral wording: no personal names or pronouns in the app or docs.
  - Never sign in or create accounts in the browser.
  - Ask the user questions as they come up.

## 3. Files changed for v1.19 (on the branch)
- **New:**
  - `internal/content/` (`content.go` catalog + `Normalize`; `rules.go` rules/presets/amounts/prompt list; tests)
  - `internal/store/content.go`, `internal/llm/content.go`, `internal/api/content_handlers.go`
  - `web/static/js/content.js`
  - Tests: `internal/store/content_test.go`, `internal/llm/content_test.go`, `internal/api/content_test.go`
- **Changed (backend):**
  - `schema.sql`, `migrate.go` (`moveLGBTQ`)
  - Store: `models.go`, `books.go`, `users.go`, `ages.go`, `filters.go`, `flags.go`, `spice.go`
  - AI: `llm/prompt.go` (`SystemPrompt` is now a `var`), `llm/deep.go`, `llm/parse.go`, `llm/flags.go` (custom flags are section 5)
  - `deepread/checks.go`, `deepread/chunk.go` (estimate 1,600 prompt + 180 answer tokens per part)
  - `analyzer/worker.go` (`rerateOne`), `api/verdict_handlers.go`, `api/users_handlers.go`, `api/server.go`, `suggest/ai.go`
- **Changed (app):** `library.js`, `bookdialog.js`, `check.js`, `verdictform.js`, `users.js`, `ui.js`, `adminstats.js`, `guide.js`; `sw.js` v51 (+ `/js/content.js`); `tailwind.input.css` + `app.css`.
- **Docs and tests:** `CHANGELOG.md` `[1.19.0]`, `README.md`, `NOVELCHECK_SPEC.md` (item 49 + the Content details section); test fixtures now set `ContentSource`.

## 4. Current status
- Windows: gofmt and `GOOS=linux go vet` are clean. `go test ./...` fails only the known Windows tests (calibre 3, db 1, tunnel 2). The JS module check and the Tailwind build pass. Linux CI runs on push.
- Checked in the local app with stubbed data (desktop and 375px):
  - Library: the Hide content picker, whole-group hiding (items disabled), the exclude query, 44px rows on phones, card icons.
  - Book window: the list with amounts and source; "not checked yet".
  - Edit rating: the ticks and the body they send.
  - Users: saved rules load; the Strict preset adds its set and sends `hidden_content`.
  - Not checked in the browser: Check a book's "Also contains" line (the code path is the same as the book window).
- A real AI hasn't answered the new prompt yet. Watch the first re-rate on the user's server (qwen2.5:7b / llama3.2) for sensible items.

## 5. Next immediate steps
1. Check the branch's CI; ask the user to release **1.19.0** (commands in Rules).
2. After release: suggest the user press the re-rate banner and look at a few books' content details (Hunger Games, Charlotte's Web, a romance) to judge the AI's accuracy.
3. Start v1.20 Discover (plan first, get the user's OK).

## Waiting on the user
- The OK to release 1.19.0.
- Priorities among the Gemini-manifest areas.
- Kindle `.kfx` file names and whether "user login information" meant a TrueNAS API key (not built).

## Working on the Windows desktop (`C:\novelcheck`)
- Node.js LTS is installed (no `make`). In Git Bash, first `export PATH="/c/Program Files/nodejs:$PATH"`. Build CSS with `npx tailwindcss@3 -c tailwind.config.js -i web/tailwind.input.css -o web/static/css/app.css --minify`.
- For a visual check, run the local app (`.claude/launch.json`, untracked; last port 18106).
  - Web files are embedded, so restart after changes, on a fresh port to dodge the 1-hour static cache.
  - Stub `window.fetch` (including `/api/content`), then `import("/js/app.js?fixture=N")`.
- There's no Python. **Never use perl `\x{…}` escapes in `-pi` edits**: they re-encoded a whole file as double UTF-8 once. Use the Edit tool for anything non-ASCII.
- The clone uses LF endings (`core.autocrlf=false`). Use `GOOS=linux go vet ./...`.

## Conventions
- MIT license (`LICENSE`, © 2026 ZachCurry13); vendored libraries keep their own (listed at the end of the README).
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled and committed; strict CSP (no inline styles, no external scripts).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done`.
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, and a bump of the `web/static/sw.js` cache name when JS changes.
- CI tests every branch push; images publish only from `main`, tags and manual runs. The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
