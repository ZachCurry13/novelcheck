# NovelCheck

Self-hosted, Dockerized web app that scans and catalogs e-book libraries for romantic/sexual content ("spice") and sensitive themes, including Catholic-discernment occult criteria. It reads your **Calibre library** read-only, imports **Kindle / e-reader drives** from the browser, keeps a per-user **"Up Next" reading queue** with drag-and-drop, and installs to your phone's home screen as a **PWA**.

See [`NOVELCHECK_SPEC.md`](NOVELCHECK_SPEC.md) for the full specification, architecture, and build order.

## Features

| Area | What you get |
|---|---|
| Accounts | The first visit creates the admin account in the browser. Three roles: `Admin` (everything), `Editor` (ratings, scans, imports and kids' accounts, but no technical settings or secrets), and `Restricted` (kid). Cookie sessions; per-profile content rules (hide Open Door, Nudity, Solo Acts, Heavy Innuendo, Dark Occult / Demonic, LGBTQ+, or anything not yet analyzed), enforced in SQL so hidden titles can't be listed, searched, opened, queued or downloaded. |
| Calibre sync | Mount any parent folder at `/calibre`, then pick the exact library in **Admin → Calibre Library**, either by browsing or with **Find libraries automatically**. Opens `metadata.db` with `mode=ro` (it never writes to it), maps files to `/calibre/<library>/<relative path>`, and puts books in the **Calibre Main** catalog. Runs on a schedule (default every 6 h) or manually. Books deleted in Calibre are pruned. |
| Drive scanner | Uses `showDirectoryPicker()` on Chromium and falls back to `<input webkitdirectory>` on iOS Safari and Firefox. Reads EPUB OPF metadata and MOBI/AZW3 EXTH headers **in the browser**, and parses Kindle filenames (`Title - Author_B0XXXXXXXX_EBOK.azw`). Only metadata is uploaded. You pick an existing catalog or create one, such as "Jenna's Kindle". |
| Dashboard | Browse every catalog in one place. Filter by catalog, spice level, content flags, and cross-catalog overlap ("also in…" / "in 2+ catalogs"). |
| Reading queue | Personal "Up Next" list, reordered with SortableJS drag handles and saved as positions. **▶ Start Reading** emails the EPUB through Send-to-Kindle SMTP or flags it for KOReader, then moves it to *Currently Reading*. |
| LLM analysis | Pick a provider in **Admin**: OpenAI, **Anthropic Claude** (native Messages API via the official Go SDK), Google Gemini, Perplexity, Ollama, or any other OpenAI-compatible endpoint (vLLM, LM Studio). Presets choose a small model (for example `gpt-4o-mini` or `claude-haiku-4-5`), and an optional larger fallback model is used only when the small one fails. Blurbs come from Open Library, then Google Books, before the LLM runs. Syncing never triggers LLM calls: books wait in *Pending Analysis* until an admin runs a batch or a single scan. |
| Admin panel | Batch size, tokens/hour cap, scan delay, live token counter and cost estimator (spent and projected), SMTP settings with a test send, Calibre polling interval, one-click `novelcheck.db` backup, and a wipe of the pending analysis queue. |
| Ratings review | Admins and editors can correct any rating by hand (**Edit rating**). Manual ratings are labeled with who made them. |
| Help & updates | A step-by-step **How to** guide opens on each person's first login and can be reopened from **❔ Help**. Admins and editors see a banner when a new version is released, and **What's new** shows release notes (from `CHANGELOG.md` and GitHub Releases). |
| PWA | `manifest.json` (standalone, dark theme) and `sw.js` app-shell cache for offline launch and Add to Home Screen. |

## Install on TrueNAS (easiest)

Follow **[docs/TRUENAS.md](docs/TRUENAS.md)**. It's a click-by-click guide for either TrueNAS **Install Custom App** (a form) or **Install via YAML**, using the prebuilt image, so there's no command line and nothing to build. Set the image pull policy to **Always** so updates are actually downloaded.

## Quick start (Docker / Docker Compose)

A prebuilt image is published to GitHub Packages as `ghcr.io/zachcurry13/novelcheck` (`:latest` tracks `main`, and `:X.Y.Z` is published for each release tag), for `linux/amd64` and `linux/arm64`.

```bash
curl -O https://raw.githubusercontent.com/ZachCurry13/novelcheck/main/docker-compose.yml
# edit the two volume paths in docker-compose.yml, then:
docker compose up -d
```

To build from source instead, clone the repo, uncomment `build: .` in `docker-compose.yml`, and run `docker compose up -d --build`.

Open `http://<host>:8080` and create your admin account on the welcome page. Do this right away: until an admin exists, anyone who can reach the page can create one. (The TrueNAS guide maps port `30080` instead, to avoid clashing with other apps.)

Then, in **Admin**:
1. Under **Calibre Library**, pick your library folder.
2. Under **LLM Analysis Engine**, pick an AI provider and paste its API key. **Show setup steps** walks through each provider; the same guide is in [docs/AI_PROVIDERS.md](docs/AI_PROVIDERS.md). For local Ollama use `http://ollama:11434/v1` with `llama3.2`, and turn off JSON mode if your server rejects `response_format`.
3. Click **Analyze batch**. Pending books are processed within your token cap.
4. Add editor and restricted users, and set content rules.

### Volumes

| Container path | Put it on | Mode | Purpose |
|---|---|---|---|
| `/data` | SSD dataset | read-write | `novelcheck.db` (users, catalogs, verdicts, queues, settings) |
| `/calibre` | HDD pool | **read-only** | A folder that contains your Calibre library (the exact subfolder is chosen in the app) |

The container runs as UID/GID `568` (TrueNAS `apps`). That user needs read access to the Calibre dataset and write access to the data dataset.

### Optional profiles

- `docker compose --profile tunnel up -d`: runs `cloudflared` with `CLOUDFLARE_TUNNEL_TOKEN`. Route the tunnel to `http://novelcheck:8080`.
- `docker compose --profile ollama up -d`: runs a local Ollama with NVIDIA GPU passthrough.

## Configuration

Process-level settings are environment variables. Everything else is edited in the Admin panel and stored in the database.

| Variable | Default | Description |
|---|---|---|
| `NOVELCHECK_ADDR` | `:8080` | Listen address |
| `NOVELCHECK_DATA_DIR` | `/data` | Directory for `novelcheck.db` |
| `NOVELCHECK_CALIBRE_DIR` | `/calibre` | Mount point for the folder containing your Calibre library (read-only) |
| `NOVELCHECK_ADMIN_USER` | `admin` | Optional, for automated installs: username for a pre-created first admin |
| `NOVELCHECK_ADMIN_PASSWORD` | *(empty)* | Optional: if set and no users exist, creates that admin at startup. Leave empty to create the admin in the browser. |
| `NOVELCHECK_TRUST_PROXY` | `true` | Use `CF-Connecting-IP` / `X-Forwarded-For` / `X-Real-IP` for client IPs |
| `NOVELCHECK_CORS_ORIGINS` | *(empty)* | Comma-separated list of allowed cross-origin callers |
| `NOVELCHECK_SESSION_DAYS` | `30` | Default "keep me signed in" length. Admins can change it in **Admin → Sign-in & Updates**. |

## Security notes

- Every response carries HSTS, `nosniff`, `X-Frame-Options: DENY`, and a strict same-origin CSP. API responses also carry `Cache-Control: no-store`, so Cloudflare and the service worker never cache private data.
- Sign-ins with **Keep me signed in** last 30 days (configurable) and roll forward while the person keeps using the app. Without it, the sign-in ends when the browser closes or after 12 hours idle. Sessions use `HttpOnly`, `SameSite=Lax` cookies, and `Secure` is set when served over HTTPS or `X-Forwarded-Proto: https`. The database stores only a SHA-256 of each session token.
- State-changing API calls require an `X-NovelCheck: 1` header (CSRF guard). Login is rate-limited per IP.
- **The database backup contains password hashes and the LLM/SMTP secrets.** Store it like a credential.
- Book downloads are served only from paths recorded by the Calibre sync that sit inside `NOVELCHECK_CALIBRE_DIR`. The in-app folder browser is admin-only and can't leave that mount, even through symlinks.
- First-run setup (`POST /api/setup`) only works while the database has no accounts.
- Update checks call `api.github.com` about every 6 hours. Turn them off under **Admin → Sign-in & Updates**.

## Development

```bash
make test        # go test ./...
make run         # serves on :8080 with ./data and ./calibre
make css         # recompile Tailwind after changing classes (output is committed)
```

The version shown in the app comes from `-ldflags -X .../internal/version.Version=…`. The Makefile and CI set it from the git tag. Add a `## [x.y.z]` section to `CHANGELOG.md` before releasing, because it becomes the release notes.

CI (`.github/workflows/docker.yml`) runs `go vet` and `go test` on every push and pull request. On `main` it publishes `ghcr.io/zachcurry13/novelcheck:latest`. To cut a release, go to **Actions → Docker image → Run workflow** and enter a version such as `1.1.0`. That publishes `:1.1.0` and `:latest` and creates the `v1.1.0` tag and GitHub Release. Pushing a `vX.Y.Z` tag does the same.

Go 1.26+, no CGO (pure-Go `modernc.org/sqlite`). Front-end assets are embedded with `embed.FS`, so the binary is fully self-contained. Per the spec's rule, no source file is longer than about 300 lines.

```
cmd/novelcheck/        entrypoint (config, bootstrap, workers, HTTP server)
internal/config/       env configuration
internal/db/           SQLite open + embedded schema.sql
internal/store/        data access: users, sessions, catalogs, books, filters, queue, settings, usage
internal/auth/         bcrypt, cookie sessions, RBAC middleware, admin bootstrap
internal/calibre/      read-only metadata.db sync + polling scheduler
internal/enrich/       Open Library / Google Books blurb lookup
internal/llm/          OpenAI-compatible client, Claude client (anthropic-sdk-go), system prompt, verdict parser
internal/analyzer/     queued analysis worker, token-per-hour cap, small→large fallback
internal/delivery/     Send-to-Kindle SMTP + best-file picker
internal/api/          chi router, middleware, handlers
web/static/            index.html, Tailwind CSS, JS modules, manifest.json, sw.js, icons, vendored SortableJS/JSZip
```
