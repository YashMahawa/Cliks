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
- `server`: Fastify API/WebSocket relay currently deployed on a DigitalOcean Droplet. It stores teams in Supabase when configured, otherwise uses an in-memory local test store.
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

- `typ start`: tries `uiohook-napi` native/global capture first.
- `typ start --terminal --self`: local test mode. It captures keyboard bytes and terminal mouse-report events from the active terminal and plays self audio.
- `typ sound-test`: plays sample sounds without joining a room.

Important platform reality:

- Windows can use low-level hooks.
- macOS can use Event Tap APIs with Accessibility permission.
- Linux Xorg can use XRecord/XInput/native hooks.
- Linux Wayland intentionally blocks normal global input capture. Production support needs a permissioned helper, compositor-specific integration, portal support if it emerges, or a clearly limited capture mode.

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

The current DigitalOcean backend is a Droplet running `cliks-api` under systemd with Caddy in front for HTTPS. The bootstrap file is `deploy/droplet-cloud-init.yaml`.

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
