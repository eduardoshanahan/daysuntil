# DaysUntil

A small homelab web application to track named time intervals. For each interval it shows how many days have elapsed, how many remain, and a visual progress bar.

## Features

- Local username/password accounts
- Optional GitHub OAuth alongside local login
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

### Docker Compose environment

The Compose file forwards these runtime variables into the app:

- `BASE_URL`: public base URL of the deployment, for example `https://eduardoshanahan.com`
- `GITHUB_CALLBACK_URL`: optional explicit OAuth callback URL. If unset, the app derives it from `BASE_URL` as `/api/oauth/github/callback`
- `GITHUB_CLIENT_ID`: GitHub OAuth App client ID
- `GITHUB_CLIENT_SECRET`: GitHub OAuth App client secret
- `COOKIE_SECURE`: set to `true` when the site is served over HTTPS

GitHub login is enabled only when these are set:

- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- either `BASE_URL` or `GITHUB_CALLBACK_URL`

Example:

```bash
export BASE_URL=https://eduardoshanahan.com
export COOKIE_SECURE=true
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

### Local development

Requires a Nix dev shell:

```bash
nix develop
go run .
```

The app will be available at `http://localhost:8080`.

For local GitHub OAuth testing:

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

## Stack

- **Backend**: Go with [chi](https://github.com/go-chi/chi) router
- **Database**: SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGo)
- **Frontend**: Vanilla HTML, CSS, and JavaScript — no framework
