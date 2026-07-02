# Agent Instructions — daysuntil

This file extends the global instructions at `~/Programming/brain/.brain/AGENT.md`.

---

## Project Context

- **BRAIN_CONTEXT**: dev
- **BRAIN_REPO**: daysuntil
- **Purpose**: Days Until — a Go web app for tracking countdowns to future events
- **Repo**: public, pushed to Gitea (`gitea.hhlab.home.arpa/eduardo/daysuntil`) + GitHub (`eduardoshanahan/daysuntil`)

---

## Brain

- Investigations go directly to the global brain (`~/Programming/brain`) — `BRAIN_ROOT` points there
- The `.brain/` here holds project-level decisions and runbooks only

---

## Deployment

- **rpi-box-01** (homelab): CI deploys on `main` branch push — builds multi-arch image, SSHs to pull + restart
- **vps-01** (public): CI deploys on `vps` branch push — bumps SHA in `deployments/vps-01/nixos/daysuntil.nix`, commits, runs `nixos-rebuild switch --target-host vps-01`
- Compose template: `~/Programming/docker-services/daysuntil/docker-compose.yml`
- Host config: `~/Programming/deployments/{rpi-box-01,vps-01}/nixos/daysuntil.nix`

## Repo-Specific Rules

- Always check which branch you are on before committing — `main` and `vps` diverge; committing to the wrong one means redoing the work
- Frontend and backend changes that alter API shape must be committed and deployed together
