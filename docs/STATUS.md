# Status

- **2026-10-26 maintenance pass** (branch `maintenance/2026-10-26`, not released): stored the maintenance protocol in `CLAUDE.md`. `govulncheck` found `golang.org/x/net` v0.59.0 affected by five known issues (fixed in v0.60.0); updated it, build and tests unchanged (only the six known Windows test failures). CI now uses the newest patch of the pinned Go line.
- **Needs a decision:** bundled `cloudflared` 2026.9.3 has 2026.10.4 available (not applied: can't verify the image build locally); `anthropic-sdk-go` 1.75.0 to 1.84.0 and `chi` 5.3.2 to 5.5.0 are feature updates, not security fixes. No open GitHub issues or pull requests.
- **Next:** release the x/net fix when the owner agrees; decide about a weekly scheduled audit (needs an API key secret, or a key-free `govulncheck` workflow).
