# Security Logging Rules

Date: 2026-05-17

This project should keep application logs useful for operations without turning them into a source of sensitive data leakage.

## Current baseline

The app currently uses request logging through Chi middleware and does not intentionally log:

- request bodies
- cookies or session tokens
- OAuth access tokens or callback payloads
- password fields
- private email addresses in normal success paths

That baseline should be preserved.

## Rules

When adding or changing logging in Go handlers, auth code, or frontend diagnostics:

- do log high-level events such as route, method, status code, and unexpected internal errors
- do not log passwords, session cookies, OAuth tokens, or raw authorization headers
- do not log full request bodies for auth, account, or profile endpoints
- do not log email addresses unless there is a strong operational reason and the value is masked
- do not include secrets in panic messages, wrapped errors, or debug output
- prefer user IDs over usernames or emails in server-side operational logs

## Practical guidance

Good examples:

- `login failed from remote address`
- `oauth callback failed: state cookie missing`
- `delete account failed for user id 42`

Bad examples:

- `login failed for alice@example.com with password hunter2`
- `session token abc123 expired`
- `oauth callback payload: {...}`

## Future review points

Review these areas carefully whenever they change:

- login and registration handlers
- password reset or email verification flows
- OAuth callback handling
- account deletion and public-link management
- any client-side debugging added to auth or profile views
