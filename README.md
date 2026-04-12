# binge-os-watch

A fast, minimal, self-hosted media tracker for movies and TV shows. Powered by [TMDB](https://www.themoviedb.org/). Zero JavaScript — pure HTML forms with server-side rendering. SQLite storage, single binary, runs anywhere.

## Features

- Track movies and TV shows with per-episode progress
- Star ratings (1-10) at every level: media, season, episode
- Watch history with custom watched dates
- Personal notes and reviews per item and per episode
- TMDB-powered search, discovery, trending, and recommendations
- Recommendation queue: add or dismiss one at a time
- Keyword watches with background scanning for new matches
- Release calendar with upcoming and recently-released sections
- Tags for custom organization
- Watched page with sort by watched date, rating, title, release
- Statistics: total movies, episodes, watch time, average rating, watch streak
- Webhooks with built-in presets (Discord, Slack, ntfy) and custom templates
- ICS calendar feed for subscribing in any calendar app
- Trakt JSON import
- Multi-user support with admin panel
- Three themes: OLED, dark, light
- Localization: English, Deutsch, Nederlands
- REST API with Bearer token auth
- No JavaScript required — works with JS disabled

## Quick Start

```bash
# Run directly
go run ./cmd/server

# Or build and run
make build
./build/binge-os-watch-linux-amd64
```

Open `http://localhost:8080` and create your account. To use TMDB search and discovery, add your TMDB API key to the config (see below).

To make yourself an admin, update the database directly:

```bash
sqlite3 ~/.binge-os-watch/data/binge-os-watch.db \
  "UPDATE users SET role='admin' WHERE username='your-username'"
```

## Configuration

On first run, a config file is created at `~/.binge-os-watch/server.toml`:

```toml
# All paths support ~ for the home directory.

[server]
# Address to listen on.
addr = ":8080"

# Public base URL — used for ICS feed links shown in settings.
base_url = "http://localhost:8080"

# Set to true to disable the web UI and only serve the API.
# disable_ui = false

# Set to true to disable user registration (existing accounts still work).
# disable_registration = false

# Directory where TMDB images are cached on disk.
image_cache_dir = "~/.binge-os-watch/cache/images"

[database]
path = "~/.binge-os-watch/data/binge-os-watch.db"

[session]
# Session cookie duration in hours (default: 720 = 30 days).
duration_hours = 720

# Set to true in production behind HTTPS.
secure_cookie = false

[argon2]
# Argon2id password hashing parameters.
time = 1
memory = 65536       # 64 MB
threads = 4
key_length = 32
salt_length = 16

[tmdb]
# Get a free API key at https://www.themoviedb.org/settings/api
api_key = ""

# Default region for watch provider lookups (ISO 3166-1).
default_region = "NL"

# How often background jobs run.
metadata_sync_interval = "24h"
keyword_scan_interval = "24h"
```

Use `--config /path/to/config.toml` to specify a custom config path.

## Production Deployment

1. Build a static binary: `make build`
2. Set `secure_cookie = true` and `base_url` in the config
3. Run behind a reverse proxy (nginx, Caddy) that terminates TLS
4. The binary is self-contained — no runtime dependencies

Example with Caddy:

```
binge.example.com {
    reverse_proxy localhost:8080
}
```

## Importing from Mediatracker

If you're coming from Mediatracker, the migration tool imports your library, watch history, ratings, runtime, and genres directly from a Mediatracker SQLite database — no TMDB calls needed.

```bash
# 1. Register your account in the BINGE web UI first
# 2. Run the migrate tool
go run ./cmd/migrate \
  -src /path/to/mediatracker.db \
  -db ~/.binge-os-watch/data/binge-os-watch.db \
  -user your-username \
  -mt-user 1
```

Use `-mt-user` to specify your Mediatracker user ID if it isn't 1. The tool clears existing media for the target user before importing, so it's safe to rerun.

## Webhooks

binge-os-watch can fire HTTP webhooks on events like new media, status changes, watched, episode watched, and released. Built-in presets for Discord, Slack, and ntfy work out of the box — just paste your webhook URL.

For other services, use the Custom mode with a Go template body and JSON headers:

```
Service: custom
URL: https://api.example.com/webhook
Body: {"text": "{{.Title}} is now {{.Status}}"}
Headers: {"Authorization": "Bearer xxx"}
```

Available template variables: `{{.Title}}`, `{{.Status}}`, `{{.MediaType}}`, `{{.MediaID}}`, `{{.Event}}`. Use `{{json .Title}}` for JSON-safe escaping.

Each webhook tracks its delivery history with status codes and errors.

## ICS Calendar Feed

Subscribe to your release calendar in any calendar app:

1. Go to Settings
2. Click "Regenerate Token" to create your ICS token
3. Copy the URL shown and add it as a calendar subscription

The feed includes upcoming and recently-released items from your library.

## API

Full OpenAPI 3.1 spec at `api/openapi.yaml`.

### Authentication

```bash
# Login — returns a session token
curl -X POST http://localhost:8080/api/v1/users:login \
  -H 'Content-Type: application/json' \
  -d '{"username": "you", "password": "your-password"}'

# Use the token for subsequent requests
curl http://localhost:8080/api/v1/media \
  -H 'Authorization: Bearer <token>'
```

### Key Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/users:register` | Create account |
| POST | `/api/v1/users:login` | Login (returns token) |
| GET | `/api/v1/search?q=` | TMDB search |
| GET | `/api/v1/media` | List library |
| POST | `/api/v1/media` | Add to library |
| POST | `/api/v1/media/{id}:set-status` | Change status |
| POST | `/api/v1/media/{id}:rate` | Rate (1-10) |
| POST | `/api/v1/episodes/{id}:watch` | Mark episode watched |
| GET | `/api/v1/calendar` | Release calendar |
| GET | `/api/v1/discover/trending` | Trending |
| GET | `/api/v1/discover/recommendations` | Personalized recommendations |
| GET | `/api/v1/keyword-watches` | Keyword watches |
| GET | `/api/v1/settings` | Get settings |

See `api/openapi.yaml` for the complete API reference.

## API-Only Mode

Set `disable_ui = true` in the config to run without the web UI. Only the REST API, health check, and ICS feed will be available.

## Building

```bash
make build          # Build for all platforms (linux, darwin, windows)
make test           # Run tests
make test-race      # Run tests with race detector
make clean          # Remove build artifacts
```

## Development

This project was developed using AI-assisted pair programming (Claude) to accelerate implementation and code review. All architecture, design decisions, and final implementation choices are human-made. Every line of code has been reviewed and tested by a person before being committed.

## Attribution

This product uses the [TMDB API](https://www.themoviedb.org/) but is not endorsed or certified by TMDB.

## License

AGPL-3.0 — see [LICENSE.md](LICENSE.md) for details. Commercial use requires permission.
