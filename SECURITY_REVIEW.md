# Security Review

Date: 2026-05-17

This document records the current security risks identified in the `daysuntil` codebase during a code review. It is intended as a backlog for future hardening work.

## Summary

No obvious remote code execution, SQL injection, or stored XSS issue was found in this review pass.

The main risks are:

1. insecure session cookie deployment defaults
2. missing login and registration abuse controls
3. user enumeration through auth and public profile behavior
4. missing HTTP server timeouts
5. predictable public profile exposure once content is marked public

## Findings

### 1. Insecure cookie default in deployment

Severity: High

Relevant files:

- `main.go`
- `auth.go`
- `docker-compose.yml`

Problem:

- Session and OAuth state cookies only get the `Secure` flag when `COOKIE_SECURE=true`.
- The checked-in Compose file defaults `COOKIE_SECURE` to `false`.
- If the app is exposed over plain HTTP, or behind a proxy with incorrect TLS handling, session cookies can be intercepted.

Relevant code:

- `main.go:32`
- `auth.go:471`
- `auth.go:497`
- `docker-compose.yml:10`

Impact:

- Session hijacking
- OAuth state cookie exposure
- Higher risk from network attackers and proxy misconfiguration

Suggested fix direction:

- Make secure cookies the default for non-local use.
- Fail closed when deployed with a public `BASE_URL` that is HTTPS but `COOKIE_SECURE` is false.
- Clearly separate local development defaults from production defaults.

### 2. No rate limiting or abuse controls on auth endpoints

Severity: Medium

Relevant files:

- `main.go`
- `handlers.go`
- `auth.go`

Problem:

- `/api/login` and `/api/register` are public and have no rate limiting, cooldown, or lockout.
- Password verification runs directly on every request.

Relevant code:

- `main.go:53`
- `main.go:54`
- `handlers.go:133`
- `handlers.go:155`
- `auth.go:207`

Impact:

- Brute force attacks
- Credential stuffing
- Automated account creation and storage abuse
- Avoidable resource consumption

Suggested fix direction:

- Add IP-based and possibly username-based rate limiting.
- Add registration throttling.
- Consider exponential backoff or temporary lockouts.
- Log suspicious auth abuse patterns.

### 3. User enumeration through auth and public profile behavior

Severity: Medium

Relevant files:

- `auth.go`
- `handlers.go`
- `models.go`

Problem:

- Login returns a distinct message for GitHub-only accounts: `"this account uses GitHub sign-in"`.
- The public profile endpoint reveals whether a username exists, even if that user has no public intervals.

Relevant code:

- `auth.go:226`
- `handlers.go:226`
- `models.go:180`

Impact:

- Attackers can confirm valid usernames.
- Confirmed usernames make credential attacks more effective.
- User discovery may expose more information than intended.

Suggested fix direction:

- Normalize login failures so they do not reveal account type.
- Decide whether profiles with zero public intervals should return `404`.
- Review whether public display name exposure matches the intended privacy model.

### 4. Missing HTTP server timeouts

Severity: Medium

Relevant files:

- `main.go`

Problem:

- The application uses `http.ListenAndServe` with default server settings.
- No read, write, header, or idle timeouts are configured.

Relevant code:

- `main.go:43`
- `main.go:44`

Impact:

- Higher exposure to slowloris-style and connection exhaustion attacks
- Easier denial of service against a small self-hosted service

Suggested fix direction:

- Replace `http.ListenAndServe` with an explicit `http.Server`.
- Set at least `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`.

### 5. Public profile exposure is easy to enumerate if usernames are known

Severity: Low to Medium

Relevant files:

- `main.go`
- `models.go`

Problem:

- Public intervals are intentionally exposed through `/u/{username}` and `/api/public/users/{username}`.
- If usernames are guessed or enumerated, an attacker can fetch any intervals marked public.

Relevant code:

- `main.go:59`
- `main.go:62`
- `models.go:190`

Impact:

- Privacy depends heavily on username secrecy and correct user understanding of visibility settings.

Suggested fix direction:

- Keep this behavior if it matches product intent, but document it clearly.
- Consider optional unguessable share links if stronger privacy is wanted later.

## Notes

- The frontend appears to avoid obvious stored XSS in interval rendering by escaping interval names before inserting them into HTML.
- SQL queries use parameterized placeholders for user-controlled values.
- The local database file is ignored by git and is not currently tracked in the repository.

## Verification

The following command passed during review:

```bash
nix develop -c go test ./...
```
