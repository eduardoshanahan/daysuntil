# Auth: OIDC Migration (supersedes auth-decision.md, auth-migration.md)

Date: 2026-08-04 (documenting a migration that happened earlier; the local
email+password model described in `auth-decision.md`/`auth-migration.md` was
fully removed before this doc was written)

## Decision

`daysuntil` dropped local email+password authentication (and the
GitHub-OAuth / magic-link paths referenced in `security-next-steps.md`)
entirely in favor of **OIDC-only login against a self-hosted Zitadel
instance**.

There is no registration form, no password field, and no local credential
storage anywhere in the codebase. Verified directly against
`server/auth.go`, `server/oidc.go`, `server/handlers_oidc.go`,
`server/router.go` (2026-08-04) — the only auth routes are:

- `GET /api/oidc/start` — begins the OIDC flow (PKCE + state cookie)
- `GET /api/oidc/callback` — exchanges the code, creates/looks-up the local
  user by `oidc_sub`, starts a session
- `POST /api/logout`
- `GET/PUT /api/me`, `PUT /api/me/profile`, `PUT /api/me/username`

There is no `/api/login` or `/api/register` endpoint. Auth rate limiting
(`server/rate_limit.go`) now only guards `public_lookup` (the public
share-group endpoint) — the old per-IP login/register buckets described in
`security-next-steps.md`/`security-review.md` no longer exist because
there's nothing left to brute-force locally.

## Current model

- `users` table is just a local identity anchor: `id`, `oidc_sub`,
  `created_at`. No `email`, `username`, or `password` columns.
- All profile data (`username`, `display_name`, `first_name`, `last_name`,
  `avatar_url`, and — as of the 2026-08-03 reminders work — `email`) lives
  in **profile-service**, keyed by `oidc_sub`, fetched via `ProfileClient`.
  Session/JWT never carries a password because there is no password.
- Session cookie (`daysuntil_session`) and a bearer `Authorization` header
  (`api_tokens` table, added 2026-08-03 for future mobile-client use) are
  both accepted by `authenticatedUser` and populate the same
  `userContextKey` — every handler works unchanged either way.
- `COOKIE_SECURE` / HTTPS-detection logic in `auth.go` still exists and is
  still accurate, but it now only keys off `BASE_URL` — the old
  `GITHUB_CALLBACK_URL` signal referenced in `security-next-steps.md` is
  gone along with GitHub OAuth.

## What this makes stale in older decision docs

- `auth-decision.md` and `auth-migration.md`: describe the local
  `email + password` model in full. That model doesn't exist anymore —
  read as historical record only, not current architecture.
- `security-next-steps.md` item 3 ("password reset / email verification
  flows"): moot — there is no password to reset or reset flow to build.
- `security-review.md`'s "Login enumeration through account type" and
  auth-rate-limiting findings describe `/api/login`/`/api/register`
  behavior that no longer exists.
- `security-logging.md`'s password/OAuth-callback bullets are vestigial —
  still good general advice (don't log secrets), but there's no password
  field or GitHub OAuth callback left to worry about; the OIDC callback
  and bearer-token header are the things to keep out of logs now.

## Not affected by this migration

- `share-groups.md` — the public share-group model is orthogonal to how a
  user authenticates and is unchanged.
- Cookie security defaults, HTTP server timeouts, security headers,
  `public_lookup` rate limiting — all still in place as described in
  `security-review.md`'s "Previously Fixed And Still Valid" section.

## Related

- `~/Programming/brain/.brain/investigations/2026-08-03-daysuntil-feature-parity-rollout-*.md`
  — where the `email`-lives-in-profile-service decision was made, the most
  recent auth-adjacent change (bearer tokens for future mobile use).
