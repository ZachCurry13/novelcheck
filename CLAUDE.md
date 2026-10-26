# NovelCheck: working rules for Claude Code

Go (chi, sqlx, pure-Go SQLite) + vanilla JS PWA, self-hosted. Read `HANDOFF.md` first for where things stand; `NOVELCHECK_SPEC.md` has the numbered feature list; `CHANGELOG.md` is the release notes.

## Standing rules
- **Releases only with the owner's OK**, from `main`, one command per call (merge `--ff-only`, push, `gh workflow run docker.yml --ref main -f version=X`, then watch the run). Update `HANDOFF.md` after each release.
- Never put the owner's own setup details (hardware, LAN addresses, ports, pool names, time zone, domain names) in the repo.
- Keep source files small (about 200 lines); docs (CHANGELOG, README, spec item) and the `web/static/sw.js` cache version change with every user-facing change.
- Test and check on a branch (`go vet`, `go test ./...`; six known Windows-only test failures: calibre x3, db x1, tunnel x2), then ask before merging and releasing.

## Maintenance, security and modular update protocol
Run it when asked (or on a schedule the owner has set up). It never releases by itself.

1. **Modular components.** List what can be updated on its own: Go modules (`go.mod`), the pinned container images in `Dockerfile` (Go, Alpine, the bundled `cloudflared` connector), vendored JS in `web/static/vendor`, and the KOReader plugin (`kindle-plugin`, versioned, updated from the app). Check each against its upstream. An update that touches only that component, with no interface change to the core, can ship on its own.
2. **Auto-patch boundary.** Apply on a branch, without asking, only patch or minor releases that fix a known vulnerability. Feature or major updates and anything that changes behavior (a bigger SDK jump, the tunnel connector, a base image) are written up for the owner instead, with what changed and why it matters.
3. **Security audit.** `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (standard library findings depend on the Go version doing the scanning; the container build uses the newest patch of its pinned Go line), plus a look at what the app exposes. Propose a targeted patch for each finding that applies, highest severity first.
4. **GitHub feedback.** Review open issues, pull requests and bug reports for the repo and its modules; turn what is actionable into focused changes.
5. **Verify, and roll back.** After any update run `go build ./...`, `go vet ./...` and `go test ./...`. If something new fails, undo only what this run changed (`git restore` the specific files such as `go.mod` and `go.sum`; never discard other work), and report why.
6. **Headless runs.** `claude -p "..."` or a scheduled GitHub Actions workflow can run this protocol weekly and open a draft PR. That needs an Anthropic API key stored as a repository secret and write access for the workflow, so it is set up only when the owner asks. A key-free alternative: a weekly workflow that runs `govulncheck` and `go list -m -u all` and reports.

Results of each run go to `docs/STATUS.md` (3 to 5 lines: what changed, what is next) and, for anything needing a decision, to the owner in plain words.
