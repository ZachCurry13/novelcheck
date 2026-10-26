# Maintenance, security and update protocol (reusable)

Drop this file into a project as `CLAUDE.md` (or add it to an existing one) and fill in the **Project** section. It tells Claude Code how to keep a repo patched without surprising you: it checks, fixes what is clearly safe on a branch, reports everything else, and never ships anything by itself.

## Project (fill in)
- **What it is / how to build and test:** `<build command>`, `<test command>`
- **Package managers in use:** Go modules / npm / pip / Docker / ...
- **Components that can be updated on their own:** dependencies, pinned container images (base images, bundled helper binaries), vendored libraries, plugins or modules with their own version
- **Known test failures to ignore (if any):** ...
- **Release rule:** who approves a release, and the exact steps (Claude never releases by itself)

## 1. Modular components and isolated updates
List what can move independently of the app itself and check each against its upstream: dependencies, pinned images, vendored files, plugins. An update can ship on its own when it touches only that component and changes no interface the rest of the app relies on. Anything that changes an interface is written up for the owner, not applied.

## 2. Auto-patch boundary
Apply, on a branch and without asking, only **patch or minor releases that fix a known vulnerability**. Everything else (feature releases, major versions, base-image or runtime jumps, anything that changes behaviour) is reported with a plain explanation of what changed and why it matters. Never merge to the main branch or release; that is the owner's call.

## 3. Security audit
Scan the code and its dependencies for known vulnerabilities and propose a targeted patch for each finding that applies, highest severity first. Findings in the language's standard library depend on the toolchain version doing the scanning, so check which version the real build uses before treating them as real.

| Stack | Vulnerability scan | Newer dependency versions |
|---|---|---|
| Go | `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | `go list -m -u all` |
| Node | `npm audit` | `npm outdated` |
| Python | `pip-audit` | `pip list --outdated` |
| Docker | compare pinned tags with the upstream latest release | same |

## 4. Feedback from GitHub
Review open issues, pull requests and bug reports for the repo and the components it bundles. Turn what is actionable into small, focused changes; list the rest.

## 5. Verify, and undo only your own change
After any update run the build and the full test suite. If something that passed before now fails, undo **only what this run changed** (for example `git restore go.mod go.sum`, or the specific files), never other work, and report why. Do not push a failing branch.

## 6. Headless and scheduled runs
- **Key-free weekly audit (recommended):** the GitHub Actions workflow in the appendix runs the scan and the version check every Monday and keeps one open issue up to date (closing it when everything is clear). It needs no API key and changes nothing.
- **Claude-driven runs:** `claude -p "run the maintenance protocol"`, or a scheduled workflow that does it and opens a draft PR. That needs an Anthropic API key stored as a repository secret and a workflow with write access, so set it up deliberately, not by default.

## Where results go
Keep a short log in `docs/STATUS.md` (3 to 5 lines per run: what changed, what needs a decision, what is next), and tell the owner in plain words about anything that needs one.

---

## Appendix: weekly audit workflow (GitHub Actions, Go version)
Save as `.github/workflows/audit.yml`. For another stack, replace the scan and update steps with the commands from the table (keeping the scan's exit status in `steps.vuln.outputs.code` and the update list in `updates.txt`), and adjust the pinned-image step or delete it. Setting the Go version from `go.mod`'s line (for example `1.26`) with `check-latest` gets the newest patch; an exact `go 1.26.0` would stay at 1.26.0, which is how a scan ends up reporting fixed vulnerabilities. Scheduled workflows run only on the default branch, and GitHub pauses them after about 60 days without repository activity.

```yaml
# Weekly audit (no API key needed): known vulnerabilities in the code and its
# dependencies, newer versions of direct dependencies, and the pinned
# cloudflared image. Findings go into ONE open issue titled "Weekly audit",
# updated each run and closed when everything is clear. Nothing is changed or
# released by this workflow; it only reports.
#   Runs Mondays 06:17 UTC, or Actions > Weekly audit > Run workflow
#   (untick "Open or update the issue" for a dry run that only prints a summary).
name: Weekly audit

on:
  schedule:
    - cron: "17 6 * * 1"
  workflow_dispatch:
    inputs:
      report:
        description: "Open or update the issue (untick for a dry run)"
        type: boolean
        default: true

permissions:
  contents: read
  issues: write

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - id: gover
        run: echo "line=$(grep -m1 '^go ' go.mod | awk '{print $2}' | cut -d. -f1,2)" >> "$GITHUB_OUTPUT"
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ steps.gover.outputs.line }} # "1.26": the newest patch of that line, like the container build
          check-latest: true

      - name: Vulnerability scan (govulncheck)
        id: vuln
        run: |
          set +e
          go run golang.org/x/vuln/cmd/govulncheck@latest ./... > vuln.txt 2>&1
          echo "code=$?" >> "$GITHUB_OUTPUT"
          tail -n 40 vuln.txt

      - name: Newer versions of direct dependencies
        run: |
          go list -m -u -f '{{if and (not .Indirect) .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}' all > updates.txt
          cat updates.txt

      - name: Pinned container image (cloudflared)
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          pinned=$(grep -oP 'cloudflare/cloudflared:\K[^ ]+' Dockerfile | head -n1)
          latest=$(gh api repos/cloudflare/cloudflared/releases/latest -q .tag_name || true)
          : > images.txt
          if [ -n "$latest" ] && [ "$pinned" != "$latest" ]; then
            echo "cloudflared $pinned -> $latest" > images.txt
          fi
          cat images.txt

      - name: Report
        env:
          GH_TOKEN: ${{ github.token }}
          VULN_CODE: ${{ steps.vuln.outputs.code }}
          REPORT: ${{ github.event_name == 'schedule' || inputs.report }}
        run: |
          {
            echo "## Weekly audit $(date -u +%F)"
            echo
            if [ "$VULN_CODE" != "0" ]; then
              echo "### Known vulnerabilities (govulncheck)"
              echo '```'
              grep -E '^(Vulnerability|    Found in|    Fixed in|Your code|This scan)' vuln.txt | head -n 60
              echo '```'
              echo "Run \`govulncheck ./...\` locally for the call paths. A finding in the standard library goes away by building with a newer Go patch."
              echo
            fi
            if [ -s updates.txt ]; then
              echo "### Newer versions of direct dependencies"
              echo '```'; cat updates.txt; echo '```'
              echo "Patch and minor releases that fix a vulnerability are safe to take; feature updates need a look."
              echo
            fi
            if [ -s images.txt ]; then
              echo "### Pinned image"
              echo '```'; cat images.txt; echo '```'
              echo
            fi
          } > body.md
          cat body.md >> "$GITHUB_STEP_SUMMARY"

          clear=true
          if [ "$VULN_CODE" != "0" ] || [ -s updates.txt ] || [ -s images.txt ]; then clear=false; fi
          [ "$REPORT" = "true" ] || { echo "Dry run: no issue changed."; exit 0; }

          gh label create audit --color 5319e7 --description "Weekly audit report" 2>/dev/null || true
          number=$(gh issue list --state open --label audit --json number --jq '.[0].number // empty')
          if [ "$clear" = "true" ]; then
            if [ -n "$number" ]; then
              gh issue comment "$number" --body "All clear on $(date -u +%F): no known vulnerabilities, no newer direct dependencies, image up to date."
              gh issue close "$number"
            fi
          elif [ -n "$number" ]; then
            gh issue edit "$number" --body-file body.md
          else
            gh issue create --title "Weekly audit" --label audit --body-file body.md
          fi
```
