# Security Next Steps

Date: 2026-05-17

This document captures the main security-related follow-up work remaining after the recent hardening pass.

## Current status

The application has already addressed:

- secure cookie defaults for HTTPS deployments
- basic auth rate limiting
- email-based local login with usernames separated from authentication
- normalized login failures for local and OAuth-backed accounts
- removal of basic public-profile enumeration by username
- explicit HTTP server timeouts
- human-readable public sharing slugs instead of username-based public URLs
- public link rotation
- browser-side hardening headers and no-store API responses
- authenticated self-service account deletion
- account-level "make all intervals private" control

## Remaining items

### 1. Decide whether public sharing should stay account-wide

Priority: Medium

Why:

- one public slug still exposes all intervals marked `public`
- the new account-level "make all private" control helps with revocation, but it does not support selective sharing
- some users may want to share one interval without sharing every public interval on the account

Possible directions:

- keep the current model and document it clearly
- move to per-interval share links
- support both profile-wide and per-interval sharing

### 2. Logging review

Priority: Medium

Why:

- the app should continue avoiding sensitive data in logs
- future features could accidentally introduce logging of email addresses, cookies, or OAuth data

Current status:

- a project logging rule is now documented in `SECURITY_LOGGING.md`

Remaining scope:

- review any future auth/account changes against that rule

### 3. Future password reset / email verification flows

Priority: Medium

Why:

- the new email-based auth model is a good base for these features
- those flows can easily reintroduce account enumeration if implemented carelessly

Suggested rules:

- use generic responses
- avoid confirming whether an email exists
- treat email as private account data throughout the flow

### 4. Rate limiting across multiple instances

Priority: Low for current deployment

Why:

- the current auth limiter is in-memory per process
- that is fine for a single small deployment, but it does not coordinate across multiple app instances

Possible future direction:

- move the limiter to shared storage or a reverse-proxy layer if the deployment model changes

## Recommended next task

The best next security improvement is:

1. decide whether to keep account-wide public sharing or move to per-interval sharing

Reason:

- it is now the main remaining privacy tradeoff in the product
- it affects both the backend model and the user experience
- the smaller hardening items are mostly operational follow-up rather than core product risk
