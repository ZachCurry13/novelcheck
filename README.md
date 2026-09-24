# NovelCheck

Self-hosted, Dockerized web app that scans and catalogs e-book libraries for romantic/sexual content ("spice") and sensitive themes, including Catholic-discernment occult criteria. It reads your **Calibre library** read-only, imports **Kindle / e-reader drives** from the browser, keeps a per-user **"Up Next" reading queue** with drag-and-drop, and installs to your phone's home screen as a **PWA**.

See [`NOVELCHECK_SPEC.md`](NOVELCHECK_SPEC.md) for the full specification, architecture, and build order.

## Features

| Area | What you get |
|---|---|
| Accounts | `Admin` and `Restricted` (kid) roles; cookie sessions; per-profile content rules (hide Open Door, Nudity, Solo Acts, Heavy Innuendo, Dark Occult / Demonic, LGBTQ+, or anything not yet analyzed), enforced in SQL so hidden titles can't be listed, searched, opened, queued or downloaded. |
| Calibre sync | Opens `metadata.db` with `mode=ro` (it never writes to it), maps files to `/calibre/<relative path>`, and puts books in the **Calibre Main** catalog. Runs on a schedule (default every 6 h) or manually. Books deleted in Calibre are pruned. |
| Drive scanner | Uses `showDirectoryPicker()` on Chromium and falls back to `<input webkitdirectory>` on iOS Safari and Firefox. Reads EPUB OPF metadata and MOBI/AZW3 EXTH headers **in the browser**, and parses Kindle filenames (`Title - Author_B0XXXXXXXX_EBOK.azw`). Only metadata is uploaded. You pick an existing catalog or create one, such as "Jenna's Kindle". |
| Dashboard | Browse every catalog in one place. Filter by catalog, spice level, content flags, and cross-catalog overlap ("also in…" / "in 2+ catalogs"). |
| Reading queue | Personal "Up Next" list, reordered with SortableJS drag handles and saved as positions. **▶ Start Reading** emails the EPUB through Send-to-Kindle SMTP or flags it for KOReader, then moves it to *Currently Reading*. |
| LLM analysis | Works with any OpenAI-compatible endpoint (OpenAI, Gemini, Anthropic compatibility layer, Ollama, vLLM). Defaults to a small model (`gpt-4o-mini`), and an optional larger fallback model is used only when the small one fails. Blurbs come from Open Library, then Google Books, before the LLM runs. Syncing never triggers LLM calls: books wait in *Pending Analysis* until an admin runs a batch or a single scan. |
| Admin panel | Batch size, tokens/hour cap, scan delay, live token counter and cost estimator (spent and projected), SMTP settings with a test send, Calibre polling interval, one-click `novelcheck.db` backup, and a wipe of the pending analysis queue. |
| PWA | `manifest.json` (standalone, dark theme) and `sw.js` app-shell cache for offline launch and Add to Home Screen. |

## Install on TrueNAS (easiest)

Follow **[docs/TRUENAS.md](docs/TRUENAS.md)**. It's a click-by-click guide that uses TrueNAS's **Install via YAML** screen and the prebuilt image, so there's no command line and nothing to build.

## Quick start (Docker / Docker Compose)

A prebuilt image is published to GitHub Packages as `ghcr.io/zachcurry13/novelcheck` (`:latest` tracks `main`, and `:X.Y.Z` is published for each release tag), for `linux/amd64` and `linux/arm64`.

```bash
curl -O https://raw.githubusercontent.com/ZachCurry13/novelcheck/main/docker-compose.yml
# edit the two volume paths in docker-compose.yml, then:
NOVELCHECK_ADMIN_PASSWORD='choose-a-strong-one' docker compose up -d
```

To build from source instead, clone the repo, uncomment `build: .` in `docker-compose.yml`, and run `docker compose up -d --build`.

Open `http://<host>:8080` and sign in as `admin`. If you leave `NOVELCHECK_ADMIN_PASSWORD` blank, a random password is printed once to `docker logs novelcheck`.

Then, in **Admin**:
1. Set the LLM base URL, API key, and model. For local Ollama use `http://ollama:11434/v1` with `llama3.2` and turn off JSON mode if your server rejects `response_format`.
2. Click **Sync Calibre now**, or wait for the schedule.
3. Click **Analyze batch**. Pending books are processed within your token cap.
4. Add restricted users and set their content rules.

### Volumes (TrueNAS SCALE)

| Container path | Put it on | Mode | Purpose |
|---|---|---|---|
| `/data` | SSD dataset | read-write | `novelcheck.db` (users, catalogs, verdicts, queues, settings) |
| `/calibre` | HDD pool | **read-only** | Calibre library root containing `metadata.db` |

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
| `NOVELCHECK_CALIBRE_DIR` | `/calibre` | Calibre library mount (read-only) |
| `NOVELCHECK_ADMIN_USER` | `admin` | First admin, created only when no users exist |
| `NOVELCHECK_ADMIN_PASSWORD` | *(random)* | Password for the first admin |
| `NOVELCHECK_TRUST_PROXY` | `true` | Use `CF-Connecting-IP` / `X-Forwarded-For` / `X-Real-IP` for client IPs |
| `NOVELCHECK_CORS_ORIGINS` | *(empty)* | Comma-separated list of allowed cross-origin callers |
| `NOVELCHECK_SESSION_DAYS` | `30` | Session lifetime |

## Security notes

- Every response carries HSTS, `nosniff`, `X-Frame-Options: DENY`, and a strict same-origin CSP. API responses also carry `Cache-Control: no-store`, so Cloudflare and the service worker never cache private data.
- Sessions use `HttpOnly`, `SameSite=Lax` cookies, and `Secure` is set when served over HTTPS or `X-Forwarded-Proto: https`. The database stores only a SHA-256 of each session token.
- State-changing API calls require an `X-NovelCheck: 1` header (CSRF guard). Login is rate-limited per IP.
- **The database backup contains password hashes and the LLM/SMTP secrets.** Store it like a credential.
- Book downloads are served only from paths recorded by the Calibre sync that sit inside `NOVELCHECK_CALIBRE_DIR`.

## Development

```bash
make test        # go test ./...
make run         # serves on :8080 with ./data and ./calibre
make css         # recompile Tailwind after changing classes (output is committed)
```

CI (`.github/workflows/docker.yml`) runs `go vet` and `go test` on every push and pull request. On `main` it publishes `ghcr.io/zachcurry13/novelcheck:latest`. On a `vX.Y.Z` tag it publishes a versioned image and creates a GitHub Release. You can also cut a release without a tag: **Actions → Docker image → Run workflow** with a version such as `1.1.0`.

Go 1.26+, no CGO (pure-Go `modernc.org/sqlite`). Front-end assets are embedded with `embed.FS`, so the binary is fully self-contained. Per the spec's rule, no source file is longer than about 300 lines.

```
cmd/novelcheck/        entrypoint (config, bootstrap, workers, HTTP server)
internal/config/       env configuration
internal/db/           SQLite open + embedded schema.sql
internal/store/        data access: users, sessions, catalogs, books, filters, queue, settings, usage
internal/auth/         bcrypt, cookie sessions, RBAC middleware, admin bootstrap
internal/calibre/      read-only metadata.db sync + polling scheduler
internal/enrich/       Open Library / Google Books blurb lookup
internal/llm/          OpenAI-compatible client, system prompt, verdict parser
internal/analyzer/     queued analysis worker, token-per-hour cap, small→large fallback
internal/delivery/     Send-to-Kindle SMTP + best-file picker
internal/api/          chi router, middleware, handlers
web/static/            index.html, Tailwind CSS, JS modules, manifest.json, sw.js, icons, vendored SortableJS/JSZip
```
