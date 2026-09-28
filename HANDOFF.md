# NovelCheck handoff (2026-09-28, later)

## 1. Goal and active task
NovelCheck (Go + vanilla JS PWA, self-hosted on the user's TrueNAS) rates a family's Calibre library for romance ("peppers") and content. **v1.18.2 is finished on `feature/v1.18.2-mobile-peppers`** and waits for the user's OK to release. Next: **v1.19 = detailed content filters** (design below; write it up and get agreement before building). Discover moves to v1.20.

## 2. Decisions made (and why)
- **Pepper scale = the user's text, verbatim** (done in 1.18.2): 2 "Romantic" (no desire, no implication of sex), 3 "Steamy Closed-Door" (desire, heavy making out, sex only off-page / fade to black), 5 "Very Explicit / Erotica-Level". Twilight ≈ 2 and Dead Until Dark = 4 are in the quick prompt only (not the Deep Scan part prompt, where a book-level example would bias every part). Vampires/werewolves = standard fantasy.
- **v1.19 content filters (user's answers, 2026-09-28):**
  - Groups and items are the user's list: 🗣️ Language (profanity, strong profanity/F-word, religious profanity/blasphemy, sexual language, crude humor, slurs, insults), ⚔️ Violence (fights, weapons, gun violence, stabbing, murder, war/battles, torture, domestic violence, sexual violence, child violence, animal violence/death), 🩸 Gore (blood, graphic injuries, graphic deaths, dismemberment, mutilation, body horror, organs, corpses), 🍺 Substance Use (alcohol, underage drinking, tobacco, vaping, marijuana, illegal drugs, prescription misuse, drug dealing, addiction, overdose/withdrawal), 🧩 Other Content (suicide/self-harm, eating disorders, abuse, bullying, death/grief, religious themes, LGBTQ+ themes, pregnancy/childbirth, mental health).
  - 🌶️ peppers stay the sexual-content scale; ✨ Spiritual & Occult and the custom filters stay as they are.
  - Proposed (not yet confirmed by the user): merge "Sexual assault" (Other) into "Sexual violence"; the existing `lgbtq_content` flag becomes the Other Content item.
  - **Each item is yes/no. Deep-scanned books also get an amount per group** (A little / Some / A lot).
  - **Both** the quick rating (blurb) and Deep Scan fill the items in; the book window shows where each came from.
  - **Books not yet checked for the new items**: kids' accounts follow their existing **Hide unrated** setting (parent-approved books always show).
  - Proposed shape: the AI returns a short list of found keys (not 49 booleans); tables `book_content(book_id, key, source)` + per-group amounts for deep scans; `user_hidden_content(user_id, key)` replaces the fixed `hide_*` columns for new rules (kids' rules then also cover custom filters); a `RulesVersion` bump offers re-rating.
- **The user's Gemini "V2 manifest"** (a local file, deliberately not in the repo) is a wishlist, not a spec.
  - Already built: WAL, auth guard, GHCR/TrueNAS updates, roles, multi-genre + clickable chips, Up Next/Wishlist, phone tab bar, Deep Scan resume, Accept/Keep for big raises, model pulls, duplicates, cost logging.
  - Ignored: its "Pepper = violence" naming (peppers are romance), its shortened pepper wording, and the user's private setup details (hardware names, LAN IP, ports, electricity rate, time zone). Those must never go into the repo; build them as settings if wanted.
  - Its three small items shipped in 1.18.2: `<think>` stripping, the everyday-moments guard and `**` removal.
  - Big areas await the user's priorities: Stuff Your Kindle event pages; seasonal filters and AI collections with kid collection scoping; light/dark theme, dyslexia font and reduced motion; series page, box-set splitting and TV-title aliases; two-machine AI failover, power-cost labels and benchmarks; model-update alerts; format pruning with a 7-day trash and EPUB conversion; safe mode and a CLI; search history and "The"-stripping sort; read-status sync.
  - Note: real DeepSeek-R1 support also needs a bigger answer budget (`MaxTokens: 600` in `client.go` is too small for its thinking).
- **Earlier decisions still stand:** Deep Scan checks v2; private libraries are seen by the owner and admins only; 3 roles (admin, editor = "Manage" tab, restricted = kid); after Discover: physical libraries → parent profiles (optional 4-digit PIN).
- **Rules:**
  - Release only with the user's OK: fast-forward `main` to the branch, then `gh workflow run docker.yml --ref main -f version=X.Y.Z`. Every `main` push rebuilds `:latest`, which their TrueNAS pulls.
  - Neutral wording: no personal names or pronouns in the app or docs.
  - Never sign in or create accounts in the browser. Visual checks: stub `window.fetch`, then `import("/js/app.js?fixture=N")`, or import single modules (e.g. `/js/peppers.js`) on the login page.
  - Ask the user questions as they come up.

## 3. Files changed for 1.18.2
- Phone commit 8fd0d5d: `mobilenav.js`, `index.html`, `tailwind.input.css`/`app.css`, `app.js`, `admin.js`, `libraryselect.js`.
- Pepper commit:
  - `internal/llm/prompt.go`: wording, `EverydayGuard`, vampires.
  - `internal/llm/deep.go`: level-3 rule.
  - `internal/llm/deepconfirm.go`: `fade_to_black`, `sexual_desire`.
  - `internal/llm/client.go`: `stripThinking`, `ErrOnlyThinking`.
  - `internal/store/spice.go`: `RulesVersion` 3.
  - `internal/store/books.go`: `**` removal.
  - JS: `peppers.js`, `importlist.js`, `guide.js`; `sw.js` v50.
  - Tests: `deep_test.go`, `llm_test.go`, new `think_test.go`, `store_test.go`.
  - Docs: `CHANGELOG.md`, `README.md`, `NOVELCHECK_SPEC.md`.

## 4. Current status
- `main` = 4ef5470 (v1.18.1). The branch holds all of 1.18.2 and is pushed.
- Windows `go test ./...` fails only the known `internal/calibre` (3), `internal/db` (1) and `internal/tunnel` (2) tests. gofmt, `GOOS=linux go vet`, the JS module check and the Tailwind build are clean. Linux CI is the real gate.
- The pepper guide was checked at 375px in the local app; the list import strips `**` and keeps `M*A*S*H`.
- Known gap: Deep-scanned books rated Level 2 before 1.18.2 may deserve 3 under the new meaning (off-page sex). Deep ratings are never auto re-rated; an admin can Deep Scan them again.
- The user's server: v1.18.1 install unconfirmed. AI: `llm_model = llama3.2:latest`, fallback `qwen2.5:7b`, `deep_read_model` empty. Advised: qwen2.5:7b as the main and Deep Scan model.

## 5. Next immediate steps
1. Check the branch's CI run; ask the user to release **1.18.2** (see Rules).
2. Write the v1.19 content-filter design (data, prompt, filter UI, kids' rules, re-rate, Deep Scan amounts) and get the user's OK, including the two "proposed" points above.
3. Build v1.19; then Discover (v1.20).

## Waiting on the user
- The OK to release 1.18.2.
- Priorities among the big Gemini-manifest areas.
- If the app ever restarts on its own again, the TrueNAS app log (panics are now logged with a stack).
- Kindle `.kfx` file names (do store books carry titles?) and whether "user login information" meant a TrueNAS API key. Not built.

## Working on the Windows desktop (`C:\novelcheck`)
- Node.js LTS is installed (no `make`). In Git Bash, first `export PATH="/c/Program Files/nodejs:$PATH"`. Build CSS with `npx tailwindcss@3 -c tailwind.config.js -i web/tailwind.input.css -o web/static/css/app.css --minify`.
- For a visual check, run the app locally (`.claude/launch.json`, untracked; now port 18104). Use a fresh port each session to dodge the 1-hour static cache, and stub `fetch` (sign-in isn't possible).
- There's no Python. For multi-line or `#`-containing edits, use the Edit tool rather than `sed`.
- The clone uses LF endings (`core.autocrlf=false`). Use `GOOS=linux go vet ./...`.

## Conventions
- MIT license (`LICENSE`, © 2026 ZachCurry13); vendored libraries keep their own (listed at the end of the README).
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled and committed; strict CSP (no inline styles, no external scripts: vendor libraries into `web/static/vendor`).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done`.
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, and a bump of the `web/static/sw.js` cache name when JS changes.
- CI tests every branch push; images publish only from `main`, tags and manual runs. The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
