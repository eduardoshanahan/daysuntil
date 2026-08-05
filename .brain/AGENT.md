# Agent Instructions — daysuntil

This file extends the global instructions at `~/Programming/brain/.brain/AGENT.md`.

---

## Project Context

- **BRAIN_CONTEXT**: dev
- **BRAIN_REPO**: daysuntil
- **Purpose**: Days Until — a Go API for tracking countdowns to future events. The web UI lives in the sibling `daysuntil-web` repo (split out 2026-08-05; see `~/Programming/brain/.brain/decisions/2026-08-05-daysuntil-repo-split.md`) — this repo is server-only.
- **Repo**: public, pushed to Gitea (`gitea.mediumsizedrobots.com/eduardo/daysuntil`) + GitHub (`eduardoshanahan/daysuntil`)

---

## Brain

- Investigations go directly to the global brain (`~/Programming/brain`) — `BRAIN_ROOT` points there
- The `.brain/` here holds project-level decisions and runbooks only

---

## Deployment

- Branch-based: `main` (test-only) / `box` (build + deploy to rpi-box-01) / `vps` (deploy-only, promoted via `git push origin origin/box:refs/heads/vps`) — see `~/Programming/brain/docs/projects.md`
- Matching worktrees: `~/Programming/daysuntil-box`, `~/Programming/daysuntil-vps`
- Host config: `~/Programming/deployments/{rpi-box-01,vps-01}/nixos/daysuntil.nix`, env var `DAYSUNTIL_IMAGE`

## Repo-Specific Rules

- Always check which branch/worktree you are on before committing — `main`, `box`, `vps` diverge; committing to the wrong one means redoing the work
- API changes that alter the HTTP contract must be coordinated with `daysuntil-web` (no shared types between the repos to catch drift automatically)
