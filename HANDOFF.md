# NovelCheck handoff

Latest state: everything is on `main` and released as **v1.15.0** (see CHANGELOG.md for what's in each version).

## Waiting on the user
- Kindle `.kfx` file names from the Kindle's `documents` folder, to check whether on-device store books carry titles in their names. The Amazon list import (paste or data download) covers her purchases in the meantime.
- Whether "user login information" for more Ollama detail meant a TrueNAS API key (real per-app CPU/GPU stats). Not built.

## Conventions
- Files ≤ ~300 lines; Go (chi, sqlx, modernc sqlite); vanilla ES modules; Tailwind compiled with `make css` and committed; strict CSP (no inline styles).
- Check JS as modules: `for f in web/static/js/*.js; do node --input-type=module --check < $f || echo BAD $f; done` (plain `node --check` misses some errors).
- Every user-facing change: plain-English CHANGELOG entry (release notes + in-app What's new), README/spec/docs in sync, bump the `web/static/sw.js` cache name when JS changes.
- Release: push to `main`, then run **Actions → Docker image → Run workflow** with the `version`. No model identifiers in commits.
- The users (a parent admin and an editor) run this on TrueNAS; keep explanations non-technical.
