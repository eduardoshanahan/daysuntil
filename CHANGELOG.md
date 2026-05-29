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

### Changed

- Local authentication now uses email + password instead of username + password; username is kept as the public display identity
- Public sharing routes moved from `/p/{username}` to `/g/{groupSlug}` via share groups
- Share-group slugs use three words plus a five-character base36 suffix for stronger uniqueness
- Account deletion available from the account settings panel

### Fixed

- Inclusive interval length calculation was off by one day
- Email validation now rejects addresses without a dot in the domain
- Rate-limit IP detection corrected for deployments behind a reverse proxy
- Mobile header logout button visibility
- Version label computation handles the no-release-tag case in CI

### Security

- Cookie `Secure` flag auto-enables when `BASE_URL` or `GITHUB_CALLBACK_URL` uses `https://`
- Rate limiting on login and registration endpoints with `429` and `Retry-After` responses
- Security headers added: `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`
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
