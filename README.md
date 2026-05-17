# DaysUntil

A small homelab web application to track named time intervals. For each interval it shows how many days have elapsed, how many remain, and a visual progress bar.

## Features

- Local email/password accounts with separate public usernames
- Human-readable public sharing links generated from a word-based slug
- Optional GitHub OAuth alongside local login
- Basic in-memory rate limiting on login and registration endpoints
- Add, edit, and delete named intervals (start date → end date)
- Progress bar showing current position within the interval
- Per-interval accent color with a color picker and presets
- Status badge: upcoming / in progress / ended
- Responsive layout — works on desktop and mobile
- Data persisted in SQLite

## Running

### With Docker Compose (recommended)

```bash
docker compose up --build -d
```

The app will be available at `http://localhost:8888`.

Data is stored in a named Docker volume (`daysuntil-data`) and survives container restarts.

Public sharing uses routes like:

```text
/p/forest-harbor-otter
```

These links are generated per account and are separate from the login email and username.

### Docker Compose environment

The Compose file forwards these runtime variables into the app:

- `BASE_URL`: public base URL of the deployment, for example `https://eduardoshanahan.com`
- `GITHUB_CALLBACK_URL`: optional explicit OAuth callback URL. If unset, the app derives it from `BASE_URL` as `/api/oauth/github/callback`
- `GITHUB_CLIENT_ID`: GitHub OAuth App client ID
- `GITHUB_CLIENT_SECRET`: GitHub OAuth App client secret
- `COOKIE_SECURE`: optional override for cookie security. Leave unset for auto mode. In auto mode, cookies become secure when `BASE_URL` or `GITHUB_CALLBACK_URL` uses `https://`

GitHub login is enabled only when these are set:

- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- either `BASE_URL` or `GITHUB_CALLBACK_URL`

Example:

```bash
export BASE_URL=https://eduardoshanahan.com
export GITHUB_CLIENT_ID=your_github_oauth_app_client_id
export GITHUB_CLIENT_SECRET=your_github_oauth_app_client_secret
docker compose up --build -d
```

If `BASE_URL=https://eduardoshanahan.com`, the GitHub OAuth callback becomes:

```text
https://eduardoshanahan.com/api/oauth/github/callback
```

That exact callback URL must also be configured in the GitHub OAuth App settings.

### Secret handling

Do not store `GITHUB_CLIENT_SECRET` in this repo.

Recommended approaches:

- export it in the shell before running `docker compose`
- load it from an untracked local env file
- inject it from your deployment system or secret manager
- use SOPS or an equivalent encrypted secret workflow in your private infrastructure repo

The app reads the secret only from environment variables at runtime.

### Cookie security behavior

By default, cookie security runs in auto mode:

- if `BASE_URL` uses `https://`, cookies are marked `Secure`
- if `GITHUB_CALLBACK_URL` uses `https://`, cookies are marked `Secure`
- if neither is set to HTTPS, cookies are not marked `Secure`

This keeps local `http://localhost` development working without extra configuration, while making HTTPS deployments safe by default.

If you explicitly set `COOKIE_SECURE=false` together with an HTTPS `BASE_URL` or `GITHUB_CALLBACK_URL`, the app will refuse to start.

Use `COOKIE_SECURE=true` only if you want to force secure cookies even outside auto-detected HTTPS mode.

## Development

### Local run

Requires a Nix dev shell:

```bash
nix develop
go run .
```

The app will be available at `http://localhost:8080`.

By default, local data is stored in:

```text
./daysuntil.db
```

If you want a separate database for a specific run:

```bash
DB_PATH=/tmp/daysuntil-dev.db nix develop -c go run .
```

### Local GitHub OAuth testing

Keep local email/password login enabled and add GitHub OAuth on top:

```bash
export BASE_URL=http://localhost:8080
export GITHUB_CLIENT_ID=your_github_oauth_app_client_id
export GITHUB_CLIENT_SECRET=your_github_oauth_app_client_secret
nix develop -c go run .
```

Then configure this callback URL in the GitHub OAuth App:

```text
http://localhost:8080/api/oauth/github/callback
```

### Verification

Run the backend and frontend checks from the dev shell:

```bash
nix develop -c go test ./...
nix develop -c go build ./...
nix develop -c node --check static/app.js
```

### Logging rules

Keep logs free of sensitive auth and account data.

Do not log:

- passwords
- session cookies or tokens
- OAuth tokens or raw callback payloads
- full request bodies for auth or account endpoints
- unmasked email addresses unless there is a strong operational need

Use high-level operational logs instead, and prefer internal user IDs over email addresses or usernames when possible.

Detailed guidance is documented in [SECURITY_LOGGING.md](/home/eduardo/Programming/programs/daysuntil/SECURITY_LOGGING.md).

### Useful dev tools

The Nix dev shell also includes a few local debugging tools:

- `sqlite3` for inspecting the local database
- `lsof` and `fuser` for checking or clearing busy ports
- `ss` and `ps` for socket/process inspection

Examples:

```bash
# See what is listening on port 8080
nix develop -c lsof -iTCP:8080 -sTCP:LISTEN -n -P

# Kill whatever is listening on port 8080
nix develop -c fuser -k 8080/tcp

# Inspect the local database
nix develop -c sqlite3 daysuntil.db
```

## Stack

- **Backend**: Go with [chi](https://github.com/go-chi/chi) router
- **Database**: SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGo)
- **Frontend**: Vanilla HTML, CSS, and JavaScript — no framework

## Versioning

This project uses Semantic Versioning.

Release source of truth:

- Git tags like `v0.1.0`, `v0.2.0`, `v1.0.0`

Current release workflow:

- pushes to `main` publish container images tagged with:
  - `latest`
  - commit SHA
- pushes of semantic version tags publish container images tagged with:
  - the exact version tag, for example `v0.3.0`
- the running app shows that build version in the UI header and exposes it at:
  - `/api/version`

Examples:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Local `go run .` builds show:

```text
dev
```

Container and CI builds stamp the binary with:

- `${CI_COMMIT_SHA}` for `main`
- `${CI_COMMIT_TAG}` for tagged releases

Document release notes in [CHANGELOG.md](/home/eduardo/Programming/programs/daysuntil/CHANGELOG.md).
