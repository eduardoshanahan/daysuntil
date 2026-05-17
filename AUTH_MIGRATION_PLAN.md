# Auth Migration Plan

Date: 2026-05-17

## Goal

Move local authentication from `username + password` to `email + password`, while keeping `username` as the public identity used for profile URLs and sharing.

## Why this helps

- public usernames stop being login identifiers
- guessing a public username no longer directly helps password attacks
- the app gets a cleaner model for future account-management work

## Planned steps

### 1. Schema migration

- add an `email` column to `users`
- keep `email` private and out of JSON responses
- add a uniqueness constraint for non-empty email values
- keep OAuth accounts allowed to have an empty email and no local password

### 2. Backend auth changes

- split local registration and login inputs more clearly
- registration will require:
  - `email`
  - `username`
  - `password`
- login will require:
  - `email`
  - `password`
- normalize local auth failures to a generic error

### 3. Public identity behavior

- keep `/u/{username}` unchanged
- keep public profile lookup by `username`
- do not expose email in public or authenticated JSON responses

### 4. Frontend changes

- login form uses email + password
- registration form uses email + username + password
- update labels, placeholders, and validation messages

### 5. Tests

- add tests for:
  - schema migration adding `email`
  - register with email
  - login with email
  - generic auth failure behavior
  - OAuth accounts remaining passwordless by default
  - no email exposure in JSON responses

## Compatibility note

This migration is aimed at the new account model going forward.

For existing rows created before the `email` field existed:

- the schema migration will add the column safely
- new local accounts will require email immediately
- existing accounts without email may need a follow-up migration path if they must continue using local password login

For this code change, the main objective is to establish the correct model for all new and updated auth behavior.

## Completion

Status: Completed on 2026-05-17

Implemented:

- added `users.email` with schema migration support
- added a unique index for non-empty email values
- switched local registration to `email + username + password`
- switched local login to `email + password`
- kept `username` as the public identity for sharing URLs
- kept OAuth accounts passwordless by default
- normalized local login failures to `invalid email or password`
- kept email out of current user and public profile JSON responses
- updated frontend auth fields and copy to match the new model
- added backend tests for migration, email login, generic failures, OAuth passwordless behavior, and email privacy

Verification completed:

- `nix develop -c go test ./...`
- `nix develop -c node --check static/app.js`

Remaining note:

- legacy local accounts that already exist without an email now have the correct schema, but they do not yet have a user-facing migration flow to attach an email address for future local login
