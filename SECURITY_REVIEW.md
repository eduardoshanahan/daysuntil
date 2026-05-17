# Security Review

Date: 2026-05-17

This document tracks the security review status of the `daysuntil` codebase after the hardening changes completed during this review cycle.

## Summary

No obvious remote code execution, SQL injection, or stored XSS issue was found in this review pass.

The main originally identified issues around cookie security, auth abuse, and basic user enumeration have been addressed.

The main remaining risk in the current code is:

1. intentional exposure of public intervals when a user chooses public visibility

## Fixed During This Review

### 1. Cookie security defaults

Status: Fixed

What changed:

- secure cookies now auto-enable when `BASE_URL` or `GITHUB_CALLBACK_URL` uses `https://`
- the app refuses to start if `COOKIE_SECURE=false` is combined with an HTTPS deployment signal
- the checked-in Compose config no longer forces `COOKIE_SECURE=false`

Files:

- `main.go`
- `auth.go`
- `docker-compose.yml`

### 2. Auth endpoint abuse controls

Status: Fixed

What changed:

- `/api/login` and `/api/register` now use basic in-memory per-IP rate limiting
- the app returns `429 Too Many Requests` with `Retry-After` when the limit is exceeded

Files:

- `main.go`
- `rate_limit.go`

### 3. Login account-type enumeration

Status: Fixed

What changed:

- local auth now uses `email + password`
- login failures are normalized to `invalid email or password`
- OAuth accounts remain passwordless by default and do not reveal their account type through login responses

Files:

- `auth.go`
- `handlers.go`

### 4. Public profile existence enumeration for users with no public data

Status: Fixed

What changed:

- public profile lookup now returns `404` unless the user has at least one public interval
- this removes the side channel where a username could be confirmed even when the account had nothing public to show

Files:

- `models.go`
- `handlers.go`

### 5. Private email exposure

Status: Fixed

What changed:

- email is now used as the private local login identifier
- username remains the public identity
- email is not exposed in current-user or public-profile JSON responses

Files:

- `auth.go`
- `models.go`
- `handlers.go`
- `static/app.js`
- `static/index.html`

### 6. Self-service account deletion

Status: Added as a safety/control improvement

What changed:

- authenticated users can now delete their own account
- deletion removes the user row, sessions, and intervals

Files:

- `main.go`
- `handlers.go`
- `models.go`
- `static/app.js`
- `static/index.html`

### 7. HTTP server timeouts

Status: Fixed

What changed:

- the app now uses an explicit `http.Server`
- read, write, header, and idle timeouts are configured

Files:

- `main.go`

## Remaining Findings

### 1. Public profile exposure is still intentional for public intervals

Severity: Low to Medium

Relevant files:

- `main.go`
- `models.go`

Problem:

- public intervals are intentionally exposed through `/u/{username}` and `/api/public/users/{username}`
- once a user marks an interval public, anyone who knows the username can fetch that public data

Impact:

- privacy still depends on correct user understanding of public visibility
- public sharing remains guessable by username, even though basic no-content enumeration was reduced

Suggested fix direction:

- keep this behavior if it matches product intent, but document it clearly
- consider optional unguessable share links if stronger privacy is wanted later

## Notes

- the frontend escapes interval names before injecting them into HTML
- SQL queries use parameterized placeholders for user-controlled values
- the local database file is ignored by git and is not tracked in the repository

## Verification

The following checks passed during this review cycle:

```bash
nix develop -c go test ./...
nix develop -c node --check static/app.js
```
