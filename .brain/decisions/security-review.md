# Security Review

Date: 2026-05-17

> **Partially superseded 2026-08-04**: login moved to OIDC-only (self-hosted
> Zitadel) — there is no `/api/login`, `/api/register`, local password, or
> GitHub OAuth anymore, so "Auth endpoint abuse controls" and "Login
> enumeration through account type" below describe removed code. See
> `auth-oidc-migration.md`. Everything about share-group slugs, cookie
> security, HTTP timeouts, and security headers is still current.

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
- share-group slugs are generated as three words plus a five-character base36 suffix
- that is much stronger than the earlier plain three-word scheme, but the route still represents public content rather than authenticated access
- there is no rate limiting on public group lookup

Impact:

- share-group links should still be treated as public links if disclosed
- the current slug format is suitable for readable, hard-to-guess sharing, but not for access control
- if the product ever needs stronger secrecy guarantees, link knowledge alone should not be the protection boundary

Suggested fix direction:

- document clearly that share-group links are public if discovered
- if stronger privacy is wanted later, move from “hard to guess” toward explicit secret tokens or expiring links
- practical options:
  - use a separate opaque token for public lookup
  - add expiry
- optional defense-in-depth:
  - add lightweight rate limiting on `/api/public/groups/{groupSlug}`

## Previously Fixed And Still Valid

### 1. Cookie security defaults

Status: Fixed

What remains true:

- secure cookies auto-enable when `BASE_URL` or `GITHUB_CALLBACK_URL` uses `https://`
- the app refuses to start if `COOKIE_SECURE=false` is combined with an HTTPS deployment signal

### 2. Auth endpoint abuse controls

Status: Removed (2026-08-04) — no longer applicable

`/api/login` and `/api/register` don't exist anymore; login is OIDC-only
against Zitadel, which handles its own abuse controls. The remaining
in-process rate limiter only guards the public share-group lookup endpoint
(`authActionPublicLookup` in `rate_limit.go`).

### 3. Login enumeration through account type

Status: Removed (2026-08-04) — no longer applicable

There's only one account type now (OIDC via Zitadel), so there's nothing
to enumerate between. Local `email + password` login and GitHub OAuth were
both removed — see `auth-oidc-migration.md`.

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
- email now lives in profile-service (not locally), fetched only at reminder-send time, and is not exposed in current public responses (see `~/Programming/brain/.brain/investigations/2026-08-03-daysuntil-feature-parity-rollout-*.md`)
- the local database file is ignored by git and is not tracked in the repository

## Verification

The following checks passed during this review cycle:

```bash
nix develop -c go test ./...
nix develop -c node --check static/app.js
```
