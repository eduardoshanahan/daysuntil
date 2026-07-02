# Security Next Steps

Date: 2026-05-17

This document captures the main security-related follow-up work remaining after the recent hardening pass.

## Update — 2026-06-04 (public deployment hardening)

Completed in preparation for internet deployment:

- email verification for password registration (SMTP required; magic link + GitHub OAuth auto-verify)
- HSTS header on HTTPS deployments
- rate limit on public share-group lookup endpoint (60 req/min per IP)
- auth rate-limit buckets persisted to SQLite (survive restarts)

**Design decision:** open registration is intentional — the deployment is an experiment in running a real public service. No invite gate was added.

Infrastructure still needed before going live: see `DEPLOY.md`.

---

## Original status (2026-05-17)

The application had already addressed:

- secure cookie defaults for HTTPS deployments
- basic auth rate limiting
- email-based local login with usernames separated from authentication
- normalized login failures for local and OAuth-backed accounts
- removal of basic public-profile enumeration by username
- explicit HTTP server timeouts
- human-readable share-group slugs instead of username-based public URLs
- share-group link rotation
- browser-side hardening headers and no-store API responses
- authenticated self-service account deletion
- share-group-based public sharing with per-interval assignment

## Remaining items

### 1. Review the share-group privacy model

Priority: Medium

Why:

- a share group is intentionally public to anyone with its slug
- the current design allows one interval to belong to zero or one group
- future needs might include expiring links, many-to-many sharing, or stronger secrecy

Current status:

- the share-group model is implemented
- the design decision remains documented in `SHARE_GROUPS_DECISION.md`

Remaining scope:

- decide whether the current one-group-per-interval model is enough
- decide whether public share links need stronger entropy or expiry later

### 2. Logging discipline for future changes

Priority: Medium

Why:

- the app should continue avoiding sensitive data in logs
- future features could accidentally introduce logging of email addresses, cookies, or OAuth data

Current status:

- a project logging rule is documented in `SECURITY_LOGGING.md`
- request logs now avoid query strings, so OAuth callback parameters are no longer logged

Remaining scope:

- review any future auth/account changes against that rule

### 3. ~~Future password reset / email verification flows~~ — DONE (2026-06-04)

Email verification implemented. Design followed the suggested rules: generic responses,
no account existence confirmation, email treated as private throughout.

### 4. Rate limiting across multiple instances

Priority: Low for current deployment

Why:

- the current auth limiter is in-memory per process
- that is fine for a single small deployment, but it does not coordinate across multiple app instances

Possible future direction:

- move the limiter to shared storage or a reverse-proxy layer if the deployment model changes

## Recommended next task

The best next security improvement is:

1. review whether the current share-group model needs stronger privacy features

Reason:

- the account-wide sharing risk has been removed
- the remaining privacy tradeoff now sits inside the share-group model itself
- future hardening here is optional product design, not a clear vulnerability
