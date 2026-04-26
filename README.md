# DaysUntil

A small homelab web application to track named time intervals. For each interval it shows how many days have elapsed, how many remain, and a visual progress bar.

## Features

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

### Local development

Requires a Nix dev shell:

```bash
nix develop
go run .
```

The app will be available at `http://localhost:8080`.

## Stack

- **Backend**: Go with [chi](https://github.com/go-chi/chi) router
- **Database**: SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGo)
- **Frontend**: Vanilla HTML, CSS, and JavaScript — no framework
