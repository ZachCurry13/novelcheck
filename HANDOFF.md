# NovelCheck handoff (2026-09-28)

## 1. Goal and active task
NovelCheck (Go + vanilla JS PWA, self-hosted on the user's TrueNAS) rates a family's Calibre library for romance ("peppers") and content. Active: **v1.18.2** on branch `feature/v1.18.2-mobile-peppers`: phone fixes (done, committed) + the user's exact pepper wording + vampire calibration (not started). Then v1.19.

## 2. Decisions made (and why)
- **Pepper scale = the user's text, verbatim.** 0/1/4 already match. Changes:
  - **2 "Romantic"**: "More developed romance with stronger attraction and kissing, including passionate kissing or physical affection. No sexual activity, sexual desire, or implication of sex. The intimacy remains romantic rather than sexual." (was "Mild / Closed Door", which allowed off-page intimacy)
  - **3 "Steamy Closed-Door"**: "Strong sexual attraction and desire are present. May include heavy/passionate making out, sexual tension, and characters expressing or acting on sexual desire. Any sexual encounter occurs off-page or fades to black; no explicit sexual activity is described." Example: "A romance that is clearly sexually charged but remains true closed-door."
  - **5 name**: "Very Explicit / Erotica-Level".
- **Vampires:** *Twilight* ≈ 2; *Dead Until Dark* (Charlaine Harris, basis of *True Blood*) = 4; vampires/werewolves are standard fantasy (`dark_occult` false). AI prompt only; don't alter the user's displayed wording.
- **Deep Scan checks v2** (released in v1.18.1) because llama3.2 3B rated fights and tension as Level 4: romance-first answers ("none" = 0), a scale-copy guard, a second look at 3+ parts, flags need backing, 2+ level raises held for admin Accept/Keep, old deep ratings re-rated at start-up, and a warning for models under 7B.
- **Phones:** a bottom tab bar instead of wrapping the 9-link top menu; fields forced to 16px (iPhone zoom).
- **Private libraries:** seen by the owner and admins only. Only 3 roles exist: admin, editor ("Manage" tab), restricted (kid).
- **v1.19 order: Discover → physical libraries → profiles.** Parent profiles get an optional 4-digit PIN; scanned books are rated as they're added; Discover books are rated in the background (unrated ones hidden from kids) and show pepper levels.
- **Rules:**
  - Release only with the user's OK: fast-forward `main` to the branch, then `gh workflow run docker.yml --ref main -f version=X.Y.Z`. Every `main` push rebuilds `:latest`, which their TrueNAS pulls.
  - Neutral wording: no personal names or pronouns in the app or docs.
  - Never sign in or create accounts in the browser. Visual checks: stub `window.fetch`, then `import("/js/app.js?fixture=N")`.

## 3. Files changed on the branch (commit 8fd0d5d)
- `web/static/js/mobilenav.js` (new): phone tab bar (first 4 visible of Check/Library/Up Next/Wishlist/Profile) + a More sheet.
- `web/static/index.html`: `<nav id="mobile-nav">` at the end of `#app-view`.
- `web/tailwind.input.css` (+ rebuilt `web/static/css/app.css`): phone rules (hide `#nav`, tab bar, 16px fields, `.above-tabbar`, toast offset).
- `web/static/js/app.js`: `buildMobileNav()` in `showApp`/`refreshUser`; `markMobileNav(name)` in `route`.
- `web/static/js/admin.js`: 2×2 tab grid and two-per-row stat tiles on phones.
- `web/static/js/libraryselect.js`: `above-tabbar` class on the selection bar.
- `web/static/sw.js`: cache `novelcheck-shell-v49` + `/js/mobilenav.js`.

## 4. Current status
- `main` = 4ef5470 (v1.18.1, released 2026-09-25, + the previous handoff). The branch has 1 commit, pushed; its CI run was in progress when written.
- Nothing uncommitted except this file; `.claude/` and `NOVELCHECK_SPEC.local-backup.md` stay untracked.
- Phone layout checked at 375px with stubbed data (admin role); the kid-role tab bar has not been checked in the browser.
- On Windows, `go test ./...` fails only the known tests in `internal/calibre` (3), `internal/db` (1) and `internal/tunnel` (2). Linux CI is the real gate.
- The user's server showed v1.18.0 in diagnostics; v1.18.1 install is unconfirmed. AI: `llm_model = llama3.2:latest`, fallback `qwen2.5:7b`, `deep_read_model` empty. Advised: qwen2.5:7b as the main and Deep Scan model.

## 5. Next immediate steps
1. `gh run list --branch feature/v1.18.2-mobile-peppers --limit 2`: confirm the tests passed.
2. Pepper wording (decision 1): `PepperLevels` in `internal/llm/prompt.go` and `PEPPERS` (names, descriptions, examples) in `web/static/js/peppers.js`. Then `grep -rn "Mild / Closed Door\|Heavy Tension\|Erotica\b"` for other copies, tests and docs.
3. Vampire calibration (decision 2) in the `ContentGuide`/prompt text in `internal/llm/prompt.go`.
4. `RulesVersion` 2→3 in `internal/store/spice.go`, so the re-rate banner offers new ratings.
5. Deep Scan for the new level 3:
   - `internal/llm/deep.go`: the prompt rule "Level 3 or higher needs sexual content happening on the page" becomes 3 = desire, heavy making out or off-page sex; 4+ = on-page sex.
   - `internal/llm/deepconfirm.go`: add `sexual_desire` and `fade_to_black` answers, mapped to 3 in `ConfirmedLevel`.
   - Update `internal/llm/deep_test.go`.
6. `web/static/js/guide.js`: say "Admin (Manage on editor accounts)" in the Admin and Kids' accounts steps.
7. Docs: `CHANGELOG.md` `## [1.18.2]` above `[1.18.1]` (tab bar, no zoom, pepper wording, vampires, re-rate banner), plus README, `NOVELCHECK_SPEC.md` and this file.
8. Check:
   - `gofmt -l internal cmd`, `GOOS=linux go vet ./...`, `go test ./...`
   - the node `--check` loop and the Tailwind build (below)
   - bump `sw.js` to v50, then push.
9. Ask the user to release **1.18.2**, then start v1.19 with Discover.

## Waiting on the user
- If the app ever restarts on its own again, the TrueNAS app log (panics are now logged with a stack).
- Kindle `.kfx` file names (do store books carry titles?) and whether "user login information" meant a TrueNAS API key. Not built.

## Working on the Windows desktop (`C:\novelcheck`)
- Node.js LTS is installed (no `make`); in Git Bash, first `export PATH="/c/Program Files/nodejs:$PATH"`. Build CSS with `npx tailwindcss@3 -c tailwind.config.js -i web/tailwind.input.css -o web/static/css/app.css --minify`.
- For a visual check, run the app locally (`.claude/launch.json`, untracked) on a fresh port to dodge the 1-hour static cache, and stub `fetch` (sign-in isn't possible).
- There's no Python. For multi-line or `#`-containing edits, use the Edit tool rather than `sed` (sed `#` delimiters and `a\` inserts broke several times).
- The clone uses LF endings (`core.autocrlf=false`). Use `GOOS=linux go vet ./...`.

## Conventions
- MIT license (`LICENSE`, © 2026 ZachCurry13); vendored libraries keep their own (listed at the end of the README).
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled and committed; strict CSP (no inline styles, no external scripts: vendor libraries into `web/static/vendor`).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done`.
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, bump the `web/static/sw.js` cache name when JS changes.
- CI tests every branch push; images publish only from `main`, tags and manual runs. The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
