# Runbook: Verifying a daysuntil deployment

## When to use

After deploying a new image — confirm the running container actually has the new code before
investigating bugs or assuming a fix is live.

## Why this matters

`docker pull` returning "up to date" and `docker inspect` container start time are both
unreliable signals. The only authoritative check is the version the app reports at runtime.

## Steps

### 1. Check the running version

```bash
curl -s https://<your-domain>/api/version
# or locally:
curl -s http://localhost:8888/api/version
```

Expected response:

```json
{"version": "abc1234"}   # commit SHA for main branch builds
{"version": "v0.3.0"}    # tagged release builds
{"version": "dev"}       # local go run
```

If the version does not match the expected commit SHA or tag, the new image is not running.

### 2. If version is stale — force pull and restart

```bash
docker compose pull
docker compose up -d --force-recreate
```

Then repeat step 1.

### 3. If version still stale — check the image digest

```bash
docker inspect daysuntil-app --format '{{.Image}}'
docker images --digests | grep daysuntil
```

Compare the digest with what the CI registry shows for the expected tag.

### 4. Check logs for startup errors

```bash
docker compose logs --tail=50 app
```

Look for missing env vars, DB errors, or port conflicts that may have prevented the new
container from starting.

## Known pitfalls

| Signal | Reliability | Notes |
|--------|------------|-------|
| `docker pull` "up to date" | Unreliable | Only means the local digest matches the remote tag — the tag may not have been updated yet |
| `docker inspect` start time | Unreliable | Does not change if `systemctl restart` reuses the existing container |
| `/api/version` response | Authoritative | Built into the binary at compile time via `-ldflags`; cannot be faked by a stale container |
