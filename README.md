# Cliks

Cliks lets remote teammates hear the gentle background sound of each other working, without sharing what anyone is typing.

You create a team code, teammates join with the CLI, and Cliks turns anonymous keyboard/mouse activity into local ambience.

No login. No chat. No microphone. No keystrokes sent.

## Use The Hosted App

Open the Cliks website, create a team code, and copy the install or join command from the page:

[cliks.agichaos.dev](https://cliks.agichaos.dev)

The site is also a live preview: press any key or click on the page and it plays the same keyboard and mouse samples the CLI uses, so you can hear the ambience before you install anything.

Install the CLI:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

Then create or join a room:

```bash
cliks create
cliks join CLIK-XXXXXX
cliks start
```

If you run bare `cliks`, it opens the interactive Bubble Tea control screen. It starts with a greeting, the current team, active connection status, and a one-click keep-running toggle; deeper team/preferences/diagnostic actions live under More. If you run `cliks start` before joining a room, it prints the short setup checklist instead of failing with a stack trace.

## What It Sends

Cliks sends only tiny activity pulses:

- keyboard activity happened
- mouse click happened
- coarse timing between those activity pulses
- your optional display name, if you set one with `cliks nickname`

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
cliks start
```

On Linux, for global capture across apps on both Wayland and Xorg, use:

```bash
cliks doctor
cliks start --evdev
```

If permission is needed, `cliks doctor` shows the setup command. Cliks still does not send which key was pressed.

Local playback uses common system audio tools. `cliks doctor` checks playback, reports whether full stereo spatial audio is available, checks `/dev/input` permissions on Linux, and prints commands to fix detected issues. For the best spatial sound, install `ffplay`/FFmpeg or `mpv`; otherwise Cliks falls back to volume-aware native players where available.

For local testing where you want to hear your own typing:

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
cliks nickname "Your Name"
cliks start
cliks settings
cliks preset deep
cliks teams
cliks switch CLIK-XXXXXX
cliks config
cliks autostart enable
cliks background start
cliks background status
cliks background stop
cliks sound-test
cliks capture-test
cliks fix-terminal
cliks doctor
```

Bare `cliks` opens the control screen. The home view intentionally stays small: Open Live, Keep Running, Stop, More, and Quit. It shows whether this device is already connected, including foreground/background/launch-at-login mode, pid, connection state, teammate count, and local captured/sent counters. The More menu contains Preferences, Team, Connection, and Diagnostics; Team includes an easy Nickname form for the display name teammates see. If a connection is already active, turning Keep Running off schedules it to stop when the control screen closes; Stop disconnects immediately. Mouse hover, wheel, clicks, and arrow keys move or activate rows, and actions such as sound test, doctor, background toggle, and launch-at-login toggle return inside the TUI instead of dropping you back to the shell.

Cliks allows only one active local connection per config/device. If a foreground, background, launch-at-login, or older untracked session is already connected, `cliks start` refuses to create a second peer and tells you to use `cliks background status` or `cliks background stop`. The control screen also cleans up extra same-device copies left behind by older installs so you do not hear your own actions through a duplicate local client.

While `cliks start` is open, Cliks shows a live terminal dashboard with room, display names for small rooms, a compact typing-now summary, capture, connection, and sound controls. Larger rooms collapse to people/typing counts so the panel stays readable. Use `Up/Down` to adjust volume, `Left/Right` or `[` and `]` to adjust sound density, `m` to mute, `s` to toggle spatial audio, and `f` to toggle fatigue fade. Press `Tab` or `Shift+S` to open live settings without disconnecting, then `Tab`/`Esc`/`q` to return. You can also click the on-screen controls in terminals with mouse reporting. Changes are saved automatically.

Listening presets:

```bash
cliks preset deep
cliks preset balanced
cliks preset social
cliks preset quiet
```

If teammates can hear you connect but cannot hear your keystrokes, run:

```bash
cliks doctor
cliks capture-test --evdev
```

While `cliks start` is running, the status screen also shows connection state plus local captured and sent event counts. If captured stays at 0 while you type, fix capture permissions/settings. If captured increases but sent stays at 0, check whether the CLI is reconnecting to the backend.

If the WebSocket drops during a server restart or short network outage, `cliks start` stays open and retries automatically with backoff. The CLI and relay exchange heartbeats so half-open connections are cleaned up instead of leaving stale teammates in the room. Activity captured while offline is best-effort and may be dropped until the connection is restored.

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

Background mode writes status logs and a live connection state under the user state directory and uses the currently selected team unless you pass a team code. `cliks background status` also reports launch-at-login sessions because they share the same local session lock.

## Install Script

Install the CLI with:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

The installer points `cliks` at the hosted Cliks backend by default and installs a user-local command wrapper. It builds the Go CLI from source and tries to install Go automatically with the system package manager when Go is missing. On desktop Linux it also checks whether global input capture needs permission and shows the relevant setup step. On macOS it reminds you to grant Accessibility permission to your terminal for global capture. On Windows, run it from Git Bash or another MSYS-style shell and add the printed `bin` directory to PATH if needed. In Termux, the wrapper is installed into `$PREFIX/bin` and desktop input-device permission prompts are skipped.

## Self-Hosting

Cliks is split into three parts:

- Website: deploy `site` to Vercel.
- Backend: deploy `server` to DigitalOcean or another Node host with WebSocket support.
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

This is an early prototype. The website, longer team codes, WebSocket relay, team deletion, Go CLI config, event batching, reconnect loop, Bubble Tea terminal dashboard/control screen, single local session guard, autostart, spatial-capable CLI playback, and sample-based sounds are working.

Linux global capture has a `/dev/input` mode for Wayland and Xorg when permission is granted. macOS and Windows still need more polish around native permission prompts and capture validation.

The hosted backend keeps `/health` public for uptime checks, but it returns only anonymous aggregate counts. It does not expose team codes, team names, or per-room snapshots.

## License

Cliks is released under the MIT License. Bundled sound sample attribution and licensing notes are in `cli/assets/sounds/NOTICE.md`.
