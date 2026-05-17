# Security Review

Date: 2026-05-17

This document tracks the current security posture of the `daysuntil` codebase after the recent auth, sharing, and UI changes.

## Summary

No obvious remote code execution, SQL injection, stored XSS, or cross-user authorization bypass was found in this pass.

The earlier high-priority issues around secure cookie defaults, auth abuse controls, login error normalization, and missing HTTP server timeouts are still fixed.

The main current risk is now:

1. share-group links are readable but also enumerable

## Current Findings

### 1. Share-group slugs should be treated as discoverable, not secret

Severity: Medium

Relevant files:

- `public_slug.go`
- `models.go`
- `handlers.go`
- `main.go`

Problem:

- public share groups are exposed through `/g/{slug}` and `/api/public/groups/{groupSlug}`
- share-group slugs are generated from three words chosen from a fixed list of 93 words
- that gives `93^3 = 804,357` possible slugs, which is readable but not high-entropy
- there is no rate limiting on public group lookup

Impact:

- an attacker can enumerate a practical slug space and discover shared groups over time
- this means the current share-group links are privacy-friendly identifiers, not secret links
- if a user assumes “hard to guess” means “effectively private,” that assumption is too strong

Suggested fix direction:

- document clearly that share-group links are public if discovered
- if stronger privacy is wanted, increase slug entropy
- practical options:
  - add a short random suffix
  - use more words
  - use a separate opaque token for public lookup
- optional defense-in-depth:
  - add lightweight rate limiting on `/api/public/groups/{groupSlug}`

## Previously Fixed And Still Valid

### 1. Cookie security defaults

Status: Fixed

What remains true:

- secure cookies auto-enable when `BASE_URL` or `GITHUB_CALLBACK_URL` uses `https://`
- the app refuses to start if `COOKIE_SECURE=false` is combined with an HTTPS deployment signal

### 2. Auth endpoint abuse controls

Status: Fixed

What remains true:

- `/api/login` and `/api/register` use per-IP in-memory rate limiting
- the app returns `429 Too Many Requests` with `Retry-After` when the limit is exceeded

### 3. Login enumeration through account type

Status: Fixed

What remains true:

- local auth uses `email + password`
- login failures are normalized to `invalid email or password`
- OAuth-backed accounts do not reveal account type through login responses

### 4. Username-based public profile enumeration

Status: Fixed

What remains true:

- account-wide public profiles were removed
- public sharing now resolves through share groups instead of usernames
- empty public lookups return `404`

### 5. HTTP server timeouts

Status: Fixed

What remains true:

- the app uses an explicit `http.Server`
- read, write, header, and idle timeouts are configured

### 6. Browser-side hardening headers

Status: Fixed

What remains true:

- `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, and `Referrer-Policy` are set
- `/api` responses use `Cache-Control: no-store`

### 7. Request logging no longer includes OAuth callback query parameters

Status: Fixed

What remains true:

- request logs now include method, path, status, and duration without logging the query string
- OAuth callback requests no longer leak `code` or `state` into logs

## Notes

- no obvious cross-user authorization bug was found in interval or share-group operations
- interval assignment validates group ownership before associating an interval with a share group
- public share-group responses intentionally expose only the shared intervals plus the owner’s public identity fields
- email is used for local login and is not exposed in current public responses
- the local database file is ignored by git and is not tracked in the repository

## Verification

The following checks passed during this review cycle:

```bash
nix develop -c go test ./...
nix develop -c node --check static/app.js
```
