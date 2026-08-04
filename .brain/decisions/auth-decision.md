# Authentication Decision

Date: 2026-05-17

> **Superseded 2026-08-04**: this local `email + password` model was fully
> removed. Login is now OIDC-only against a self-hosted Zitadel instance —
> see `auth-oidc-migration.md`. Kept below as historical record only.

## Decision

The application will move to this account model:

- `email` is the login identifier for local accounts
- `username` is a separate public identity used for in-app identity and display
- `public_slug` is a separate human-readable public sharing identifier
- users authenticate with `email + password`
- OAuth accounts do not get a local password by default
- auth error messages should be normalized so they do not reveal whether an email or account exists

## Why

The current model uses `username` for both authentication and public identity.

That creates two problems:

1. public usernames help attackers target authentication attempts
2. enumeration of usernames has direct login value

Separating `email` from `username` improves the security model:

- public usernames are no longer login names
- shared profile URLs stop exposing the auth identifier
- public sharing no longer depends on exposing the username
- the system is better positioned for future password reset and account management flows

## OAuth rule

For OAuth-based accounts:

- the account may exist without any local password
- local password login is not enabled automatically
- if a local password is ever added later, that should be an explicit account-management action

This keeps OAuth sign-in separate from local password authentication and avoids silently creating extra login paths.

## Email handling requirements

Email addresses are sensitive account data and should be treated as private.

Requirements:

- never expose email in public profile APIs
- never expose email in frontend public views
- store emails normalized for lookup, typically trimmed and lowercased
- require uniqueness on normalized email
- return generic auth responses so login does not confirm whether an email exists
- if password reset is added later, that flow must also use generic responses

## Scope for the implementation

Planned changes:

1. add `email` to the `users` schema
2. update registration to collect `email`, `username`, and `password`
3. update local login to use `email` and `password`
4. keep `username` as the in-app public identity, but use `public_slug` for public sharing routes
5. keep OAuth accounts passwordless by default
6. normalize auth failures to avoid email enumeration
7. update tests and frontend copy

## Notes on complexity

Protecting email is not especially complicated in this application.

The important parts are mostly discipline and API boundaries:

- keep email out of public responses
- avoid logging it unnecessarily
- use generic auth and recovery responses
- validate and normalize it consistently

This does not require encryption-at-rest or a major architecture change for the current scope.
The main risk is accidental exposure through APIs, logs, or future features.
