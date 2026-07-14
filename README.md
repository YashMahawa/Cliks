# Cliks

Cliks lets remote teammates hear the gentle background sound of each other working, without sharing what anyone is typing.

You create a team code, teammates join with the CLI, and Cliks turns anonymous keyboard/mouse activity into local ambience.

No login. No chat. No microphone. No keystrokes sent.

## Use The Hosted App

Open the Cliks website, create a team code, and copy the install or join command from the page:

[site-kappa-six-64.vercel.app](https://site-kappa-six-64.vercel.app)

The site is also a live preview: press any key or click on the page and it plays the same keyboard and mouse samples the CLI uses, so you can hear the ambience before you install anything.

Install the native CLI on macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

On Windows 10/11, open PowerShell and run:

```powershell
irm https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.ps1 | iex
```

The installers download a native release (no Go or Git required), prepare **spatial sound** and **background capture** access, then run `cliks setup`. Source compilation remains a fallback on unusual Unix architectures. Full per-OS notes: [docs/setup.md](docs/setup.md).

Then create or join a room:

```bash
cliks create
cliks join CLIK-XXXXXX
```

`cliks join` validates the code, saves it, and starts one background Cliks session automatically. Use `cliks join --no-start CLIK-XXXXXX` if you only want to save the team, or `cliks start CLIK-XXXXXX` if you want to select and start a code in one foreground command.

`cliks create` copies the new code to your clipboard when the platform has a clipboard command available. If you run bare `cliks`, it opens the interactive Bubble Tea control screen. On first run, Join Team, Create Team, Sound Check, and Setup Check are immediately visible. After joining, the compact home screen shows the current team name and code, active connection status, and a one-click keep-running toggle; deeper controls live under More. A small footer keeps the selected team, connection state, volume, and room count visible while you browse other views. If you run `cliks start` before joining a room, it prints the short setup checklist instead of failing with a stack trace.

## What It Sends

Cliks sends only tiny activity pulses:

- keyboard activity happened
- mouse click happened
- coarse timing between those activity pulses
- your optional 10-character plain-text display name, if you set one with `cliks nickname`
- your explicit presence state (`available`, `focus`, `break`, or `dnd`)
- allowlisted ephemeral reactions such as a wave or coffee — never arbitrary message text

Cliks does **not** send:

- actual keys
- key codes
- typed text
- mouse coordinates
- active app or window names
- clipboard data
- screenshots
- microphone audio

Remote timing is rounded into 50ms buckets by the relay before teammates receive it. This keeps the ambience rhythmic without exposing raw millisecond keystroke patterns.

Nicknames are stripped of terminal escape/control sequences by both the CLI and relay before they are displayed. A peer cannot use a styled nickname to recolor, move, or corrupt someone else's terminal UI.

Mouse activity means left/right clicks only. Cliks intentionally ignores cursor movement, scroll/wheel events, side buttons, app/window focus, and pointer coordinates. On Linux evdev, short stationary touchpad taps are treated as clicks: one-finger tap is left click and two-finger tap is right click; long presses, movement, and three-or-more-finger gestures are ignored.

## Quick Start

Clone and run locally:

```bash
git clone https://github.com/YashMahawa/Cliks.git
cd Cliks
npm install
npm run build
```

Start the backend:

```bash
npm run dev:server
```

Start the website in another terminal:

```bash
npm run dev:site
```

Open the site, create a team, then join it from the CLI:

```bash
cliks join CLIK-XXXXXX
```

**Prefer the easy path** — no OS-specific commands required for most people:

```bash
cliks setup          # one-time: sound + capture readiness
cliks sound-test     # hear sample clicks
cliks join CLIK-XXXXXX
```

- **macOS:** if capture is quiet, enable your Terminal app under System Settings → Privacy & Security → Accessibility (the installer opens this pane).
- **Windows:** capture works for normal apps with no extra permission dialog.
- **Linux:** the installer/`cliks setup` request input-device access; log out/in once only if permanent group membership was just added.

Cliks still does not send which key was pressed — only activity kind + coarse timing.

**Spatial sound:** best with **mpv** (stereo pan + distance). Installer installs it when possible. Without mpv/ffplay, Cliks falls back to basic system players (distance/volume only). Details: [docs/setup.md](docs/setup.md).

For a local self-test where you hear your own typing:

```bash
cliks sound-test
cliks start --terminal --self
```

To turn self-monitoring back off:

```bash
cliks set hear.self off
```

## CLI

The command is:

```bash
cliks
```

Useful commands:

```bash
cliks create
cliks delete [CLIK-XXXXXX]
cliks join CLIK-XXXXXX
cliks join --no-start CLIK-XXXXXX
cliks nickname "YourName"
cliks start
cliks start CLIK-XXXXXX
cliks settings
cliks setup
cliks preset deep
cliks teams
cliks switch CLIK-XXXXXX
cliks config
cliks set --list
cliks service start|stop|status
cliks service enable|disable
cliks set autostart on
cliks set audio.device default
cliks sound-test
cliks notification-test
cliks capture-test
cliks fix-terminal
cliks doctor
```

Bare `cliks` opens a full-terminal control screen with Open Live, Keep Running, Stop, More, and Quit. The selected team name and a one-click copyable code stay visible. A short launch animation runs on every start; the first launch teaches the spatial room with sound. More contains Preferences, Advanced, Team, Connection, and Diagnostics. Preferences includes direct row toggles for notifications, notification sound, sharing, listening, presence, and the Ember/Ocean/Mono themes. Advanced includes a confirmed Factory Reset that clears only this device, stops its session, and replays first-run onboarding without deleting the server room. Mouse hover/click and keyboard navigation operate the same actions; press `?` anywhere for the current shortcuts.

Cliks allows only one active local connection per config/device. If a foreground, background, launch-at-login, or older untracked session is already connected, `cliks start` refuses to create a second peer and tells you to use `cliks background status` or `cliks background stop`. The control screen also cleans up extra same-device copies left behind by older installs so you do not hear your own actions through a duplicate local client.

While `cliks start` is open, Cliks uses the terminal as a full spatial desk: you sit in the center, teammates occupy adaptive rings, active typers light up, recent reactions animate inside the circle, and large rooms collapse overflow into calm semantic dots. The action rail makes code copy, notifications, notification sound, mute, spatial audio, teammate selection, reactions, Preferences, Back, and Stop directly clickable. Keyboard equivalents remain visible in the footer and under `?`.

Rooms automatically expire after 48 hours without a live connection. A successful connection refreshes the 48-hour clock; a room that remains connected is kept alive.

The hosted Cliks relay is a shared free service: rooms are capped at 20 people and client batching is locked to 500 ms. To use a private backend, open `More → Server` and paste its HTTPS URL, or run `cliks set api.url https://your-cliks-server`. Self-hosted clients may then choose a 100–2000 ms batch window. Self-hosted servers can set `CLIKS_MAX_PEERS_PER_ROOM` from 2–200; larger rooms consume substantially more fan-out bandwidth and CPU.

Rooms are capped at 20 live people. Spatial placement pans teammates around your desk locally: 4 people in the first ring and 2 more seats per outer ring, with each ring rotated to avoid stacking teammates at the same angle. Dynamic circle placement is enabled for new installs and can be turned off; when enabled it reshuffles every 1-60 minutes so recently active teammates move closer locally. Fatigue fade softens long typing bursts with a room-aware gradual curve so busy rooms do not pump between loud and quiet. Density controls how many received sounds are played locally; it never changes what activity is sent.

Listening presets:

```bash
cliks preset deep
cliks preset balanced
cliks preset social
cliks preset quiet
```

Useful settings:

```bash
cliks set nickname "YourName"
cliks set keep.running on
cliks set autostart on
cliks set spatial.dynamic on
cliks set spatial.shuffleMinutes 10
cliks set audio.device default
```

`audio.device` is an optional advanced output identifier. It works with `mpv`, `paplay`, `pw-play`, and `aplay`; `cliks doctor` warns when the selected player cannot route to it. Use `default` to return to the system output. Environment overrides such as `CLIKS_API_URL` and `CLIKS_WS_URL` take precedence over saved URLs.

If teammates can hear you connect but cannot hear your keystrokes, run:

```bash
cliks setup
cliks capture-test
```

While `cliks start` is running, the status screen also shows connection state plus local captured and sent event counts. If captured stays at 0 while you type, fix capture permissions/settings. If captured increases but sent stays at 0, check whether the CLI is reconnecting to the backend.

If the WebSocket drops during a server restart or short network outage, `cliks start` stays open and retries automatically with exponential backoff plus bounded random jitter. This spreads reconnects after a shared outage instead of making every client retry together. The CLI and relay exchange frequent heartbeats and enforce read deadlines so half-open connections are cleaned up promptly instead of leaving stale teammates in the room. Slow relay listeners are isolated behind bounded per-connection queues, so one unhealthy device cannot pause everyone else's sounds. Activity captured while offline is best-effort and may be dropped until the connection is restored. Transient backend errors keep the saved team and retry; only an authoritative missing/deleted response removes the selected team from local config, disables launch-at-login, and stops reconnecting to it.

If a terminal tab feels stuck in a strange input mode after terminal-only testing, run:

```bash
cliks fix-terminal
```

The CLI defaults to the hosted Cliks backend. For local development, override it with:

```bash
CLIKS_API_URL=http://localhost:8787 cliks start
```

To reconnect automatically when you sign in:

```bash
cliks autostart enable
cliks autostart status
cliks autostart disable
```

To keep Cliks connected after closing the current terminal:

```bash
cliks background start
cliks background status
cliks background stop
```

Background mode writes status logs and a live connection state under the user state directory and uses the currently selected team unless you pass a team code. `cliks background status` also reports launch-at-login sessions because they share the same local session lock. Stopping a launch-at-login session stops the current process only; it does not delete the login launcher, so the next login or boot can connect again.

## Install Script

Install the CLI with:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

Designed for non-technical users. It:

- downloads the current native GitHub Release (source build only as a fallback)
- installs **mpv** for stereo spatial sound when a package manager is available
- adds `~/.local/bin` (or platform equivalent) to common shell PATH files
- prepares Linux input access automatically when possible
- opens macOS Accessibility settings when needed
- runs `cliks setup` and prints plain next steps

Windows users should use `cli/install.ps1` from PowerShell; Git Bash remains a supported fallback. Tagged releases are built and tested natively on Linux x64/arm64, macOS Intel/Apple Silicon, and Windows x64. In Termux, the source-build wrapper goes to `$PREFIX/bin` and desktop input-group steps are skipped.

See [docs/setup.md](docs/setup.md) for a full macOS / Windows / Linux walkthrough.

## Self-Hosting

Cliks is split into three parts:

- Website: deploy `site` to Vercel.
- Backend: deploy the Go `server` to DigitalOcean, Render, App Platform, or another host with WebSocket support.
- Database: use local Postgres on the same server, or use Supabase/Postgres elsewhere.

To build your own CLI pointed at your own backend:

```bash
git clone https://github.com/YashMahawa/Cliks.git
cd Cliks
npm install
npm run build
cliks set api.url https://your-backend-domain
```

For users installing from your fork, update `productionAPIURL` in `cli/config.go`, then commit and publish your fork. They can install from your repo by running:

```bash
curl -fsSL https://raw.githubusercontent.com/YOUR_USER/YOUR_FORK/main/cli/install.sh \
  | CLIKS_REPO_URL=https://github.com/YOUR_USER/YOUR_FORK.git bash
```

For the website, set:

```text
NEXT_PUBLIC_CLIKS_API_URL=https://your-backend-url
```

For the backend, set:

```text
CORS_ORIGIN=https://your-vercel-site
SUPABASE_URL=your-supabase-url
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key
```

On a single Droplet, the backend can store team records in local Postgres by setting:

```text
CLIKS_LOCAL_POSTGRES=true
```

Supabase is optional.

## Testing

Run the same checks used by CI:

```bash
npm run check
npm run build
npm run smoke:server
```

Run a safe live backend load test:

```bash
npm run load:server
```

For a larger explicit ramp:

```bash
CLIKS_LOAD_ROOMS=4 CLIKS_LOAD_PEERS=8 CLIKS_LOAD_BATCHES=10 npm run load:server
```

Docker backend smoke:

```bash
docker build -t cliks-server .
docker compose up
```

## Current Status

Cliks is an active prototype. The website, longer team codes, Go WebSocket relay, team deletion with live deleted-room signals, Go CLI config, event batching, reconnect loop, Bubble Tea terminal dashboard/control screen, single local session guard, autostart, spatial-capable CLI playback, and sample-based sounds are working.

Linux global capture has a `/dev/input` mode for Wayland and Xorg when permission is granted. macOS uses global hooks after Accessibility is granted to the terminal app launching Cliks; Windows global hooks work for normal-privilege applications, while capture may pause when an elevated window is focused. `cliks setup`, `cliks doctor`, and `cliks capture-test` provide platform-specific readiness checks and fixes.

Local configuration and session state are written atomically. Invalid saved JSON produces a visible warning instead of silently replacing settings with defaults, and one session lock prevents foreground, background, and login-started copies from connecting at the same time. WebSocket heartbeat writes are synchronized with other socket writes.

The hosted backend keeps `/health` public for uptime checks, but it returns only anonymous aggregate counts. It does not expose team codes, team names, or per-room snapshots. Team lookups and failed WebSocket joins are limited per IP, inbound WebSocket messages are size/rate guarded, a socket is allowed in only one room at a time, and recovered background/connection panics are logged with stack traces instead of taking down unrelated rooms.

## License

Cliks is released under the MIT License. Bundled sound sample attribution and licensing notes are in `cli/assets/sounds/NOTICE.md`.
