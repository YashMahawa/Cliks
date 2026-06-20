# Cliks

Cliks lets remote teammates hear the gentle background sound of each other working, without sharing what anyone is typing.

You create a team code, teammates join with the CLI, and Cliks turns anonymous keyboard/mouse activity into local ambience.

No login. No chat. No microphone. No keystrokes sent.

## Use The Hosted App

Open the Cliks website, create a team code, and copy the install or join command from the page:

[https://site-kappa-six-64.vercel.app](https://site-kappa-six-64.vercel.app)

Install the CLI:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

Then join a room:

```bash
typ join CLIK-XXXX
typ start
```

## What It Sends

Cliks sends only tiny activity pulses:

- keyboard activity happened
- mouse click happened
- timing between those activity pulses

Cliks does **not** send:

- actual keys
- key codes
- typed text
- mouse coordinates
- active app or window names
- clipboard data
- screenshots
- microphone audio

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
typ join CLIK-XXXX
typ start
```

On Linux, for global capture across apps on both Wayland and Xorg, use:

```bash
typ doctor
typ start --evdev
```

If permission is needed, `typ doctor` shows the setup command. Cliks still does not send which key was pressed.

On Linux, local playback needs one common audio CLI to be available: `paplay`, `pw-play`, or `aplay`. `typ doctor` checks this, checks `/dev/input` permissions, and prints the commands to fix detected issues.

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
typ join CLIK-XXXX
typ start
typ teams
typ switch CLIK-XXXX
typ config
typ sound-test
typ capture-test
typ fix-terminal
typ doctor
```

If teammates can hear you connect but cannot hear your keystrokes, run:

```bash
typ doctor
typ capture-test --evdev
```

While `typ start` is running, the status screen also shows local captured and sent event counts. If captured stays at 0 while you type, fix capture permissions/settings. If captured increases but sent stays at 0, check the connection/backend.

If a terminal tab feels stuck in a strange input mode after terminal-only testing, run:

```bash
typ fix-terminal
```

The CLI defaults to the hosted Cliks backend. For local development, override it with:

```bash
CLIKS_API_URL=http://localhost:8787 typ start
```

## Install Script

Install the CLI with:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

The installer points `typ` at the hosted Cliks backend by default. On Linux it also checks whether global input capture needs permission and shows the relevant setup step.

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

## Current Status

This is an early prototype. The website, team codes, WebSocket relay, CLI config, event batching, and sample-based sounds are working.

Linux global capture has a `/dev/input` mode for Wayland and Xorg when permission is granted. macOS and Windows still need more polish around native permission prompts and capture validation.

## License

Cliks is released under the MIT License. Bundled sound sample attribution and licensing notes are in `cli/assets/sounds/NOTICE.md`.
