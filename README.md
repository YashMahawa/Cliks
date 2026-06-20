# Cliks

Cliks lets remote teammates hear the gentle background sound of each other working, without sharing what anyone is typing.

You create a team code, teammates join with the CLI, and Cliks turns anonymous keyboard/mouse activity into local ambience.

No login. No chat. No microphone. No keystrokes sent.

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
```

## Install Script

Once this repo is public, the one-line installer is:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

## Hosting

Cliks is split into three parts:

- Website: deploy `site` to Vercel.
- Backend: deploy `server` to DigitalOcean or another Node host with WebSocket support.
- Database: run `supabase/schema.sql` in Supabase.

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

Global keyboard/mouse capture still needs proper production backends for Windows, macOS, Linux Xorg, and Linux Wayland.
