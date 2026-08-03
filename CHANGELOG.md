# Changelog

All notable changes to this project should be documented in this file.

The format is based on Keep a Changelog, and this project follows Semantic Versioning.

## [Unreleased]

### Added

- Magic link sign-in — passwordless email-based login alongside the existing email + password flow
- Share groups — replace the old account-wide public profile with named groups; each group gets a human-readable public link and shows only the intervals assigned to it
- Public link rotation — regenerate a share group's slug without losing the group or its intervals
- Manual interval ordering — drag or move intervals up and down on the dashboard
- Mobile hamburger menu in the header with per-card overflow menus for touch devices
- Interval progress refreshes automatically at midnight without a page reload
- Incremental build number in the version label for non-tagged CI builds
- Email verification for password registration — when SMTP is configured, new accounts must verify their email before first sign-in; magic link and GitHub OAuth accounts are auto-verified
- `/help` page with user-facing documentation and screenshots, linked from the header
- Reminders per interval — one-time or repeating (daily/weekly/monthly/yearly), emailed via the homelab SMTP relay by an in-process dispatcher; the destination address is fetched from profile-service at send time and never stored in daysuntil's own database
- Recurring intervals (weekly/monthly/yearly) — the current/next occurrence is computed on read, never mutating the original stored dates
- Icon (emoji) and optional background photo URL per interval
- Multi-unit countdown display (seconds/minutes/hours/days/weeks/months/years/"sleeps", plus an auto mode), computed client-side; intervals also support an optional specific time-of-day instead of all-day-only
- Personal access tokens (`/api/tokens`) — bearer-token auth alongside the existing cookie session, so a future native client can authenticate without a browser cookie jar
- `email` field on profile-service's `Profile`, captured/synced from the OIDC `email` claim on every login

### Changed

- Local authentication now uses email + password instead of username + password; username is kept as the public display identity
- Public sharing routes moved from `/p/{username}` to `/g/{groupSlug}` via share groups
- Share-group slugs use three words plus a five-character base36 suffix for stronger uniqueness
- Account deletion available from the account settings panel
- Auth rate-limit buckets persisted to SQLite so state survives container restarts

### Fixed

- Inclusive interval length calculation was off by one day
- Email validation now rejects addresses without a dot in the domain
- Rate-limit IP detection corrected for deployments behind a reverse proxy
- Mobile header logout button visibility
- Version label computation handles the no-release-tag case in CI

### Security

- Cookie `Secure` flag auto-enables when `BASE_URL` or `GITHUB_CALLBACK_URL` uses `https://`
- Rate limiting on login and registration endpoints with `429` and `Retry-After` responses
- Rate limiting on public share-group lookup endpoint (60 req/min per IP)
- Security headers added: `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`
- HSTS (`Strict-Transport-Security`) header emitted on HTTPS deployments
- Query strings stripped from request logs, preventing OAuth `code` and `state` leakage
- Auth failure responses normalized to prevent email enumeration
- HTTP server read, write, header, and idle timeouts now configured explicitly

## [v0.1.0] - 2026-05-16

### Added

- Multi-user accounts with local username/password authentication
- Optional GitHub OAuth alongside local login
- Public profile pages with per-interval public/private visibility
- In-app date picker and expanded color palette
- Development shell utilities for local debugging
- Semantic versioning policy, changelog, and build-version UI label
- Local version fallback to the latest git tag, with `-dirty` for modified worktrees
