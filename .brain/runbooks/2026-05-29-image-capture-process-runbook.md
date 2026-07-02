# Runbook: Screenshot capture for daysuntil README

## When to use

Retaking screenshots after UI changes, or adding new ones to the README.

## Prerequisites

- `nix` available
- Working directory: `daysuntil/` (the public repo)

## Steps

### 1. Start a fresh app instance

```bash
# Kill any running instance on 8080
fuser -k 8080/tcp 2>/dev/null
sleep 2

# Start with a clean temp database
rm -f /tmp/daysuntil-screenshot.db
cd ~/Programming/programs/daysuntil/daysuntil
DB_PATH=/tmp/daysuntil-screenshot.db nix develop -c go run . &>/tmp/daysuntil.log &
sleep 4

# Confirm it's up
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/
```

### 2. Locate chromium

```bash
CHROMIUM_PATH=$(nix-shell -p chromium --run "which chromium")
echo $CHROMIUM_PATH
```

### 3. Set up the screenshot directory

```bash
mkdir -p /tmp/daysuntil-screenshots
cd /tmp/daysuntil-screenshots
nix-shell -p nodejs --run "npm install playwright"
```

### 4. Write / update capture.js

Key rules for the Playwright script:

- Use `page.request.post/get/put` for all authenticated API calls — **not** `page.evaluate` fetch
  and **not** `ctx.request`. Only `page.request` carries the session cookie reliably.
- Pass chromium via env var: `executablePath: process.env.CHROMIUM_PATH`
- Always include `--no-sandbox` in chromium args
- Register → login → create data → navigate → screenshot, in that order

Skeleton:

```js
const { chromium } = require('playwright');
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_PATH,
  args: ['--no-sandbox', '--disable-setuid-sandbox']
});
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();

// Register
await page.request.post('http://localhost:8080/api/register', {
  data: { email: 'demo@example.com', username: 'demo', password: 'demopassword123' }
});
// Login
await page.request.post('http://localhost:8080/api/login', {
  data: { email: 'demo@example.com', password: 'demopassword123' }
});
// Navigate and screenshot
await page.goto('http://localhost:8080');
await page.waitForLoadState('networkidle');
await page.screenshot({ path: 'screenshot.png' });
```

### 5. Run the script

```bash
cd /tmp/daysuntil-screenshots
CHROMIUM_PATH=$CHROMIUM_PATH nix-shell -p nodejs --run \
  "CHROMIUM_PATH=$CHROMIUM_PATH node capture.js"
```

### 6. Copy screenshots to the repo

```bash
cp /tmp/daysuntil-screenshots/*.png \
   ~/Programming/programs/daysuntil/daysuntil/docs/screenshots/
```

### 7. Stop the app

```bash
fuser -k 8080/tcp
rm -f /tmp/daysuntil-screenshot.db
```

## Known pitfalls

| Pitfall | Fix |
|---------|-----|
| `fuser -k` doesn't kill the nix dev shell's compiled binary | Use `pkill -9 -f "go-build"` as a fallback, or just let the old instance die naturally |
| Deleting the DB while the process holds it open causes SQLite write errors | Kill the process first, then delete the DB |
| `page.evaluate` fetch cookies not shared with `page.request` | Always use `page.request` for API calls |
| `ctx.request.post` for register + `page.request.post` for login loses the session | Use `page.request` for both |
| Playwright can't find chromium | Set `CHROMIUM_PATH` via `nix-shell -p chromium --run "which chromium"` |
