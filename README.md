# NovelCheck

Self-hosted, Dockerized web app that scans and catalogs e-book libraries for romantic/sexual content ("spice") and sensitive themes, including Catholic-discernment occult criteria. It reads your **Calibre library** read-only, imports **Kindle / e-reader drives** from the browser, keeps a per-user **"Up Next" reading queue** with drag-and-drop, and installs to your phone's home screen as a **PWA**.

See [`NOVELCHECK_SPEC.md`](NOVELCHECK_SPEC.md) for the full specification, architecture, and build order.

## Features

| Area | What you get |
|---|---|
| Accounts | The first visit creates the admin account in the browser. Three roles: `Admin` (everything), `Editor` (ratings, scans, imports and kids' accounts, but no technical settings or secrets), and `Restricted` (kid, created per age group). Cookie sessions; per-profile content rules (hide Open Door, Nudity, Solo Acts, Heavy Innuendo, Dark Occult / Demonic, LGBTQ+, or anything not yet analyzed), enforced in SQL so hidden titles can't be listed, searched, opened, queued or downloaded. |
| Calibre sync | Mount any parent folder at `/calibre`, then pick the exact library in **Admin → Calibre Library**, either by browsing or with **Find libraries automatically**. Opens `metadata.db` with `mode=ro` (it never writes to it), maps files to `/calibre/<library>/<relative path>`, and puts books in the **Calibre Main** catalog. Runs on a schedule (default every 6 h) or manually. Books deleted in Calibre are pruned. |
| Import lists | **Import books** also takes a list with no cable: Goodreads, StoryGraph, Hardcover or LibraryThing exports, Amazon's data download, any spreadsheet saved as CSV, or text copied from Amazon's Content Library page. Columns are detected automatically, with shelf choices and plain step-by-step guides. |
| Delete requests | Anyone can **🗑 Request to delete** a book with a reason; admins review the list and delete from Calibre (recycle bin), keep, or mark done, with a history. |
| Drive scanner | Uses `showDirectoryPicker()` on Chromium and falls back to `<input webkitdirectory>` on iOS Safari and Firefox. Reads EPUB OPF metadata and MOBI/AZW3 EXTH headers **in the browser**, and parses Kindle filenames (`Title - Author_B0XXXXXXXX_EBOK.azw`). Only metadata is uploaded. You pick an existing catalog or create one, such as "Jenna's Kindle". Newer Kindles connect as an MTP "device" that browsers can't open: the page explains copying the `documents` folder off first, or you can **Paste a list** of titles instead. |
| Dashboard | Browse every catalog in one place. Filter by catalog, **peppers** (0–5) and cross-catalog overlap ("also in…" / "only books in 2+ catalogs"), and tick **Hide** boxes (Open Door, Nudity, Solo Acts, Heavy Innuendo, LGBTQ+, Dark Occult) to hide books with that content. |
| Parent "OK" | Admins and editors can **✓ Mark as OK** a book a filter catches by mistake (Harry Potter's magic, say). It then shows for everyone, overriding hide filters and kids' content rules, and is never offered for removal. |
| Remove from Calibre | Admins get **Remove hidden books from Calibre…**, which lists the Calibre books your Hide filters catch (never ones marked OK). With **One-click removal** connected to Calibre's Content server, one button removes them through Calibre, to its recycle bin, after checking every title against Calibre so the wrong library can't be touched. Without it, NovelCheck gives you a Calibre search to paste. Either way NovelCheck itself never writes to your library. Guide: [docs/CALIBRE_SERVER.md](docs/CALIBRE_SERVER.md). |
| Feedback | **Suggest a filter / feedback** (footer) and **Suggest one** (next to the Hide boxes) open GitHub issue forms. |
| Reading queue | Personal "Up Next" list, reordered with SortableJS drag handles and saved as positions. **▶ Start Reading** emails the EPUB through Send-to-Kindle SMTP or flags it for KOReader, then moves it to *Currently Reading*. |
| LLM analysis | Pick a provider in **Admin**: OpenAI, **Anthropic Claude** (native Messages API via the official Go SDK), Google Gemini, Perplexity, Ollama, or any other OpenAI-compatible endpoint (vLLM, LM Studio). Presets choose a small model (for example `gpt-4o-mini` or `claude-haiku-4-5`), and optional fallback models (several, in order) are used only when it fails. Blurbs come from Open Library, then Google Books, before the LLM runs. Syncing never triggers LLM calls: books wait in *Pending Analysis* until an admin runs a batch or a single scan. |
| Admin panel | Batch size, tokens/hour cap, scan delay, live token counter and cost estimator (spent and projected), SMTP settings with a test send, Calibre polling interval, one-click `novelcheck.db` backup, and a wipe of the pending analysis queue. |
| Ratings review | Admins and editors can correct any rating by hand (**Edit rating**). Manual ratings are labeled with who made them. |
| Remote access | Built-in Cloudflare Tunnel connector: paste a tunnel token in **Admin → Remote access** to get an `https://` address that works away from home, with no port forwarding. Guide: [docs/REMOTE_ACCESS.md](docs/REMOTE_ACCESS.md). |
| Ollama easy setup | With the Ollama provider selected, NovelCheck finds your Ollama server, measures what its GPU can hold and marks the best and most powerful models that fit, downloads a model with a progress bar, and lets you tick and order models (main first, then fallbacks), with no terminal needed. |
| 🌶️ Pepper scale | Books are rated **0–5 peppers**: 0 No Romance, 1 Sweet Romance, 2 Romantic, 3 Steamy Closed-Door, 4 Explicit, 5 Very Explicit / Erotica-Level. **What do the peppers mean?** in the Library gives the full descriptions and examples. Kid accounts have a **Most peppers allowed** limit (age groups start at 0, 1, 2, 3 or no limit). Older ratings can be re-rated in one click and stay visible meanwhile. |
| Duplicates & formats | Cards show each book's file formats (EPUB, AZW3, MOBI…). A **Format** filter finds a format, books with 2+ formats, duplicates, or books with no file. **Find duplicates** lists books that are in Calibre more than once with each copy's formats and size, suggests the copy to keep, and (admins, with one-click removal) moves the extras to Calibre's recycle bin, always keeping at least one. |
| Age groups & notes | Parents set each book's **age group** (Young kids, Middle grade, Teens, Young adult, Adults) and leave **notes** after reading it, visible to everyone or to parents only. Kid accounts are created per age group and only see books rated for their group or younger; a parent-set age group counts as a rating. |
| Usage page | Admins get a **Usage** tab: CPU, memory, network download/upload speed and disk with 15-minute charts, library progress, AI calls, cost and tokens per day, and the Ollama models currently loaded (GPU vs RAM). |
| Language | Summaries are written in the chosen language (default English (US)), whatever the book's language; summaries not in English can be re-rated in one click. |
| Backup AI | An optional second AI (another Ollama on your network, or a cloud AI) takes over when the main AI fails or its server is switched off, with a 🔔 notice and its own prices. |
| 🩺 AI diagnosis | **System → Diagnose with AI** has your connected AI read NovelCheck's diagnostics (never passwords or keys), explain what's wrong, and draft a GitHub bug report you can open in one click. Error pop-ups, notifications and failed books have a 🩺 shortcut, and logs have 📋 Copy / ⬇ Download buttons. |
| System page | Admins get a **System** tab with **🩺 Diagnose with AI** and **Check everything**, which tests every AI model (main, fallbacks and backup), Open Library, Google Books, the Calibre library and Content server, email login, remote access and whether the public address really opens from the internet, Ollama and the update check, each with a suggested fix. |
| Rating errors | Usage and Admin list books that failed to rate, grouped by reason with a plain explanation, and **Retry** them in one click. |
| Notifications | A 🔔 bell for admins and editors collects problems NovelCheck notices (failed ratings, Calibre sync errors, remote access dropping, Send-to-Kindle failures, Ollama download failures, failed checks), with **Fix it** links. Repeats are grouped, and some clear themselves when fixed. Error pop-ups stay until closed and are also listed there. |
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

- Remote access: easiest is the built-in tunnel (**Admin → Remote access**). Alternatively, `docker compose --profile tunnel up -d` runs a separate `cloudflared` with `CLOUDFLARE_TUNNEL_TOKEN`; route that tunnel to `http://novelcheck:8080`.
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
- The tunnel token is stored as a secret (never returned to the browser) and passed to `cloudflared` through its environment, not the command line.
- The Ollama helper (admin only) probes the container's gateway, `host.docker.internal`, and `ollama` on ports 11434/30068, plus any address the admin types.
- Update checks call `api.github.com` about every 6 hours. Turn them off under **Admin → Sign-in & Updates**.

## Development

```bash
make test        # go test ./...
make run         # serves on :8080 with ./data and ./calibre
make css         # recompile Tailwind after changing classes (output is committed)
```

`internal/calibresrv` has an optional test against a real `calibre-server`. Run one with `--enable-auth` and a user who can make changes (plus a read-only user `reader` / `readpass1`), then set `NOVELCHECK_TEST_CALIBRE_URL`, `NOVELCHECK_TEST_CALIBRE_USER` and `NOVELCHECK_TEST_CALIBRE_PASS`.

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
internal/calibresrv/   calibre Content server client (Digest/Basic login, list, remove to recycle bin)
internal/enrich/       Open Library / Google Books blurb lookup
internal/llm/          OpenAI-compatible client, Claude client (anthropic-sdk-go), system prompt, verdict parser
internal/analyzer/     queued analysis worker, token-per-hour cap, small→large fallback
internal/delivery/     Send-to-Kindle SMTP + best-file picker
internal/tunnel/       supervises the bundled cloudflared connector (remote access)
internal/ollama/       Ollama discovery, model downloads with progress, loaded models (ps)
internal/sysinfo/      container CPU / memory (cgroup v2), disk and DB size
internal/api/          chi router, middleware, handlers
web/static/            index.html, Tailwind CSS, JS modules, manifest.json, sw.js, icons, vendored SortableJS/JSZip
```
