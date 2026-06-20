# AGENTS.md

This file is the durable project context for future coding agents working on Cliks. Keep it current whenever architecture, product behavior, deploy steps, protocol, capture behavior, or sound behavior changes.

## Product

Cliks is an ambient coworking tool for remote teams. It lets teammates hear realistic keyboard and mouse ambience from each other without sharing typed content.

The current command name is still `typ`.

Primary audiences:

- remote company teams
- college project groups
- hackathon/project rooms
- small friend/study groups

Core promise:

- no login required
- create a team code on the website
- join from the CLI
- no actual keystrokes, key codes, mouse coordinates, app names, window names, text, screenshots, clipboard data, microphone audio, or screen data are sent
- only activity event kind plus timing offsets are sent

## Current Structure

- `site`: Next.js app intended for Vercel. It creates teams and displays copyable join commands.
- `server`: Fastify API/WebSocket relay currently deployed on a DigitalOcean Droplet. It stores teams in Supabase when configured, local Postgres when `CLIKS_LOCAL_POSTGRES=true` or `DATABASE_URL` is set, otherwise an in-memory local test store.
- `cli`: `typ` command. It joins a team, captures local activity, sends 500ms batches, receives teammate activity, and plays local sounds.
- `supabase/schema.sql`: minimal team table.
- `deploy/render.yaml`: starter Render config.
- `docs/architecture.md`: deeper architecture and scaling notes.
- `docs/capture-backends.md`: global input capture strategy and platform caveats.
- `shared/protocol.md`: WebSocket message shapes.

## Protocol

Activity batches preserve exact event kind and timing offsets inside a 500ms window.

Example:

```json
{
  "type": "activity_batch",
  "teamCode": "CLIK-842K",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "mouse", "button": "left", "offsetMs": 173 },
    { "kind": "keyboard", "offsetMs": 499 }
  ]
}
```

The server only validates and relays these events. It does not assign 3D positions and does not store live event history.

## Team Codes And Data

Team codes use the `CLIK-XXXX` shape.

Stored data:

- team code
- team name
- delete password hash
- timestamps

Live presence is in memory. Rooms disappear from memory when empty. There is no membership list and no stored total member count.

## Client-Side Placement

Do not move placement logic to the server.

Each listener locally assigns positions to teammates relative to themselves. The server sends presence with peer ids and joined timestamps; the CLI sorts peers and places them into expanding rings:

- first ring: 2m radius, 4 people
- second ring: 3m radius, 8 people
- third ring: 4m radius, 12 people
- capacity keeps growing by 4 per ring

When people leave, the local listener recomputes the arrangement, so far users move inward to fill gaps. Placement is deterministic per listener using peer ids as jitter seeds, but it is listener-relative and not a shared server truth.

Current audio playback only uses distance as volume attenuation and stores pan/distance in placement. More realistic 3D processing is future work.

## Sound

The CLI uses bundled real WAV samples, not generated placeholder clicks.

Current pack:

- 5 keyboard samples in `cli/assets/sounds/keyboard`
- 5 mouse samples in `cli/assets/sounds/mouse`

The audio engine randomly picks one sample per event. Source/license details are in `cli/assets/sounds/NOTICE.md`.

Before public release, review the mouse samples because one OpenGameArt source has mixed license metadata. Prefer CC0-only or self-recorded samples for a clean launch.

## Capture

Current modes:

- `typ start`: on Linux, tries `/dev/input` evdev capture first; otherwise tries `uiohook-napi` native/global capture.
- `typ start --evdev`: Linux global capture through `/dev/input/event*`. This is intended to work across Wayland and Xorg when permission is granted.
- `typ start --terminal --self`: local test mode. It captures keyboard bytes and terminal mouse-report events from the active terminal and plays self audio.
- `typ sound-test`: plays sample sounds without joining a room.
- `typ doctor`: explains privacy and capture permission/setup.
- `cli/install.sh`: installs the CLI, runs `typ doctor`, and on Linux offers to add the current user to the `input` group. Keep this user-facing and never request or print backend provider tokens.

Important platform reality:

- Windows can use low-level hooks.
- macOS can use Event Tap APIs with Accessibility permission.
- Linux Xorg can use XRecord/XInput/native hooks.
- Linux Wayland intentionally blocks normal desktop global input APIs. The current practical path is evdev via `/dev/input`, which requires local input-device permission. The CLI must never send key codes even though evdev exposes them locally; it should emit only `keyboard` or `mouse` event kind and timing.

## Commands

Useful local commands:

```bash
npm install
npm run check
npm run build
npm run dev:server
npm run dev:site
typ sound-test
typ join CLIK-LOCAL
typ start --terminal --self
typ set hear.self off
```

## Deploy

Vercel deploys the site. Set:

```text
NEXT_PUBLIC_CLIKS_API_URL=https://your-backend-url
```

The current DigitalOcean backend is a Droplet running `cliks-api` under systemd with Caddy in front for HTTPS. The bootstrap file is `deploy/droplet-cloud-init.yaml`. The live Droplet should run local Postgres and set `CLIKS_LOCAL_POSTGRES=true` so team codes survive service restarts.

Security posture for the live Droplet:

- the DigitalOcean API token is not in the repo, website bundle, CLI, installer, or Droplet app env
- the CLI contains only the public backend URL, which is not secret
- UFW allows only OpenSSH, HTTP, and HTTPS
- Node listens on port 8787 behind Caddy, and direct public access to 8787 should be blocked by firewall
- SSH password and keyboard-interactive auth are disabled
- CORS is set to the production Vercel origin

If using App Platform or another managed Node host, set:

```text
CORS_ORIGIN=https://your-vercel-site
SUPABASE_URL=...
SUPABASE_SERVICE_ROLE_KEY=...
```

Supabase should run `supabase/schema.sql`.

## Known Issues

- `npm audit --omit=dev` currently reports moderate advisories through Next/PostCSS dependency metadata. Do not force downgrade to old Next; wait for a patched compatible release or reassess if Next dependency graph changes.
- Global capture is not production-ready across every OS yet.
- The `CLIKS_GAIN` and `CLIKS_PAN` environment values are not used by `paplay`; they are placeholders for a later player/mixer that can apply real gain/pan.
- The command is still `typ`; product name is Cliks.

## README Policy

Keep `README.md` from the user point of view. It should explain what Cliks does, how to install/run it, privacy guarantees, and basic deploy steps. Do not overload it with internal details like ring math, protocol internals, or backend implementation notes. Put those details here or in `docs/`.

## Public Backend URL

`cli/src/config.ts` currently points new installs at `https://139.59.29.207.sslip.io`. This is a public backend URL, not a secret. Never put the DigitalOcean API token, SSH private key, or service credentials into the CLI, website bundle, README, install script, or committed env files.
