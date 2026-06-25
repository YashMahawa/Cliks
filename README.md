# Cliks

Cliks lets remote teammates hear the gentle background sound of each other working, without sharing what anyone is typing.

You create a team code, teammates join with the CLI, and Cliks turns anonymous keyboard/mouse activity into local ambience.

No login. No chat. No microphone. No keystrokes sent.

## Use The Hosted App

Open the Cliks website, create a team code, and copy the install or join command from the page:

[cliks.agichaos.dev](https://cliks.agichaos.dev)

The site is also a live preview: press any key on the page and it plays the exact same keyboard samples the CLI uses, so you can hear the ambience before you install anything.

Install the CLI:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

Then join a room:

```bash
typ join CLIK-XXXXXX
typ start
```

If you run `typ` before joining a room, it prints the short setup checklist instead of failing with a stack trace.

## What It Sends

Cliks sends only tiny activity pulses:

- keyboard activity happened
- mouse click happened
- coarse timing between those activity pulses

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
typ join CLIK-XXXXXX
typ start
```

On Linux, for global capture across apps on both Wayland and Xorg, use:

```bash
typ doctor
typ start --evdev
```

If permission is needed, `typ doctor` shows the setup command. Cliks still does not send which key was pressed.

Local playback uses common system audio tools. `typ doctor` checks playback, reports whether full stereo spatial audio is available, checks `/dev/input` permissions on Linux, and prints commands to fix detected issues. For the best spatial sound, install `ffplay`/FFmpeg or `mpv`; otherwise Cliks falls back to volume-aware native players where available.

For local testing where you want to hear your own typing:

```bash
typ sound-test
typ start --terminal --self
```

To turn self-monitoring back off:

```bash
typ set hear.self off
```

## CLI

The temporary command name is:

```bash
typ
```

Useful commands:

```bash
typ join CLIK-XXXXXX
typ start
typ settings
typ preset deep
typ teams
typ switch CLIK-XXXXXX
typ config
typ autostart enable
typ sound-test
typ capture-test
typ fix-terminal
typ doctor
```

`typ settings` opens an interactive terminal UI for tuning Cliks with keyboard or mouse. It covers volume, density, mute, spatial audio, fatigue fade, self-monitoring, sharing toggles, nickname, selected team, backend URLs, and autostart. In non-interactive terminals it prints the current JSON config instead of trapping the shell.

While `typ start` is open, Cliks shows a live terminal dashboard with room, capture, connection, and sound controls. Use `Up/Down` to adjust volume, `Left/Right` or `[` and `]` to adjust sound density, `m` to mute, `s` to toggle spatial audio, and `f` to toggle fatigue fade. You can also click the on-screen controls in terminals with mouse reporting. Changes are saved automatically.

Listening presets:

```bash
typ preset deep
typ preset balanced
typ preset social
typ preset quiet
```

If teammates can hear you connect but cannot hear your keystrokes, run:

```bash
typ doctor
typ capture-test --evdev
```

While `typ start` is running, the status screen also shows connection state plus local captured and sent event counts. If captured stays at 0 while you type, fix capture permissions/settings. If captured increases but sent stays at 0, check whether the CLI is reconnecting to the backend.

If the WebSocket drops during a server restart or short network outage, `typ start` stays open and retries automatically with backoff. The CLI and relay exchange heartbeats so half-open connections are cleaned up instead of leaving stale teammates in the room. Activity captured while offline is best-effort and may be dropped until the connection is restored.

If a terminal tab feels stuck in a strange input mode after terminal-only testing, run:

```bash
typ fix-terminal
```

The CLI defaults to the hosted Cliks backend. For local development, override it with:

```bash
CLIKS_API_URL=http://localhost:8787 typ start
```

To reconnect automatically when you sign in:

```bash
typ autostart enable
typ autostart status
typ autostart disable
```

## Install Script

Install the CLI with:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

The installer points `typ` at the hosted Cliks backend by default and installs a user-local command wrapper instead of requiring global npm permissions. On Linux it also checks whether global input capture needs permission and shows the relevant setup step. On macOS it reminds you to grant Accessibility permission to your terminal for global capture. On Windows, run it from Git Bash or another MSYS-style shell and add the printed `bin` directory to PATH if needed.

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
typ set api.url https://your-backend-domain
```

For users installing from your fork, update `productionApiUrl` in `cli/src/config.ts`, then commit and publish your fork. They can install from your repo by running:

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

This is an early prototype. The website, longer team codes, WebSocket relay, team deletion, CLI config, event batching, reconnect loop, interactive terminal dashboard/settings UI, autostart, spatial-capable CLI playback, and sample-based sounds are working.

Linux global capture has a `/dev/input` mode for Wayland and Xorg when permission is granted. macOS and Windows still need more polish around native permission prompts and capture validation.

The hosted backend keeps `/health` public for uptime checks, but it returns only anonymous aggregate counts. It does not expose team codes, team names, or per-room snapshots.

## License

Cliks is released under the MIT License. Bundled sound sample attribution and licensing notes are in `cli/assets/sounds/NOTICE.md`.
