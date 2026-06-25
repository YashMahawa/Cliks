# AGENTS.md

This file is the durable project context for future coding agents working on Cliks. Keep it current whenever architecture, product behavior, deploy steps, protocol, capture behavior, or sound behavior changes.

## Product

Cliks is an ambient coworking tool for remote teams. It lets teammates hear realistic keyboard and mouse ambience from each other without sharing typed content.

The product CLI command is `cliks`. Do not reintroduce `typ` as a command alias.

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
- only activity event kind plus coarse timing offsets are sent

## Current Structure

- `site`: Next.js app intended for Vercel. It creates teams and displays copyable join/install commands. The landing page uses the "Warm Desk" design system (warm stone palette `#11100f`/`#1a1918`, bone text `#eae5d9`, ember accent `#d97746`; Geist + Geist Mono) and doubles as a live in-browser demo of the CLI ambience (see Sound). Brand assets: `site/public/images/cliks-keycap.png` (keycap logo/favicon) and `site/public/images/warm_desk_workspace.png` (hero photo).
- `server`: Fastify API/WebSocket relay currently deployed on a DigitalOcean Droplet. It stores teams in Supabase when configured, local Postgres when `CLIKS_LOCAL_POSTGRES=true` or `DATABASE_URL` is set, otherwise an in-memory local test store.
- `cli`: Go-based `cliks` command with Bubble Tea/Lip Gloss terminal interfaces. It joins a team, captures local activity, sends 500ms batches, receives teammate activity, and plays local sounds.
- `supabase/schema.sql`: minimal team table.
- `deploy/render.yaml`: starter Render config.
- `docs/architecture.md`: deeper architecture and scaling notes.
- `docs/capture-backends.md`: global input capture strategy and platform caveats.
- `shared/protocol.md`: WebSocket message shapes.

## Protocol

Activity batches preserve event kind and coarse timing offsets inside a 500ms window. Clients may send local millisecond offsets, but the relay rounds offsets into 50ms buckets before forwarding them to teammates. Do not reintroduce raw millisecond relay timing; it weakens the privacy promise by making keystroke rhythm fingerprinting easier.

Example:

```json
{
  "type": "activity_batch",
  "teamCode": "CLIK-842KQ9",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "mouse", "button": "left", "offsetMs": 150 },
    { "kind": "keyboard", "offsetMs": 500 }
  ]
}
```

The server only validates and relays these events. It does not assign 3D positions and does not store live event history.

## Team Codes And Data

Team codes use the `CLIK-XXXXXX` shape for newly created rooms. The in-memory local test room remains `CLIK-LOCAL`.

Stored data:

- team code
- team name
- delete password hash
- timestamps

Live presence is in memory. Rooms disappear from memory when empty. There is no membership list and no stored total member count. The relay sends WebSocket pings and removes peers that miss heartbeats, so half-open sockets should not leave ghost users.

Deleting a team requires its delete password. A successful delete marks the stored team row deleted and closes any live in-memory room for that team so connected peers cannot keep using a deleted code.

Teams can be created and deleted from the website, from `cliks create` / `cliks delete`, and from the bare `cliks` Bubble Tea interface. CLI/TUI delete-password prompts must not echo the password when a real terminal is available.

## Client-Side Placement

Do not move placement logic to the server.

Each listener locally assigns positions to teammates relative to themselves. The server sends presence with peer ids and joined timestamps; the CLI sorts peers and places them into expanding rings:

- first ring: 2m radius, 4 people
- second ring: 3m radius, 8 people
- third ring: 4m radius, 12 people
- capacity keeps growing by 4 per ring

When people leave, the local listener recomputes the arrangement, so far users move inward to fill gaps. Placement is deterministic per listener using peer ids as jitter seeds, but it is listener-relative and not a shared server truth.

Current audio playback stores pan/distance in placement and applies those values when the selected player supports them. `ffplay` is preferred for full stereo pan plus distance gain, then `mpv` for stereo pan plus volume, then native/basic players. Distance attenuation is applied with native player volume flags where supported (`afplay`, `paplay`, `pw-play`). `aplay` and Windows `Media.SoundPlayer` remain basic playback paths without gain/pan. The audio engine uses a bounded queue and caps concurrent player processes to avoid process storms during dense batches.

## Sound

The CLI uses bundled real WAV samples, not generated placeholder clicks.

Current pack:

- 5 keyboard samples in `cli/assets/sounds/keyboard`
- 2 mouse samples in `cli/assets/sounds/mouse` (real recorded clicks from Pixabay, trimmed to ~0.25s to match keyboard length)

The audio engine randomly picks one sample per event. Mouse samples are real recorded click sounds and should remain audibly distinct from keyboard samples. Source/license details are in `cli/assets/sounds/NOTICE.md`. The website mirror in `site/public/sounds/` must stay in sync with both packs.

Audio playback auto-detects `ffplay`, `mpv`, `afplay`, `paplay`, `pw-play`, `aplay`, or Windows `Media.SoundPlayer` through PowerShell. Missing audio tools must be reported as a user-facing setup warning, not as an unhandled child-process crash. `ffplay` receives the bundled mono samples through an explicit stereo pan filter so left/right gain both apply. `cliks doctor` should show whether the active player has full stereo spatial support or only distance/basic playback.

The website mirrors this on the web. `site/components/AcousticProvider.tsx` preloads the keyboard and mouse WAVs from `site/public/sounds/` (a copy of the CLI pack) via the Web Audio API and plays a random sample on every `keydown`/`mousedown`, with randomized gain and playback-rate jitter to match the CLI's organic feel. Audio integrity rule: if the WAVs fail to load it must fail silently — never fall back to a synthesized oscillator beep. Keep `site/public/sounds/*` in sync with `cli/assets/sounds/*`.

## Capture

Current modes:

- Bare `cliks`: opens the Bubble Tea home/settings interface.
- `cliks create` / `cliks delete [CODE]`: create or delete teams from the CLI. The bare TUI also has in-app create/delete forms.
- `cliks start`: on Linux, tries `/dev/input` evdev capture first. Native macOS/Windows global capture hooks are still future work in the Go CLI.
- `cliks start --evdev`: Linux global capture through `/dev/input/event*`. This is intended to work across Wayland and Xorg when permission is granted.
- `cliks start --terminal --self`: local test mode. It captures keyboard bytes and terminal mouse-report events from the active terminal and plays self audio.
- `cliks start` before joining a room prints first-run setup steps instead of a raw error.
- `cliks sound-test`: plays sample sounds without joining a room.
- `cliks capture-test`: runs local capture for a short window and reports keyboard/mouse event counts plus fix commands when nothing is captured.
- `cliks doctor`: explains privacy, checks Go/audio/input-device readiness, and prints detected fix commands.
- `cliks settings` / `cliks ui`: opens the Bubble Tea settings TUI. It supports keyboard and mouse interaction for volume, density, mute, spatial audio, fatigue fade, self-monitoring, sharing toggles, and selected team.
- `cliks background start|stop|status [team-code]`: runs `cliks start` detached from the terminal for the selected team, reports the pid/log path, or stops it. Use this for "close the terminal but keep Cliks connected" behavior; `cliks autostart` is for login-time launch.
- `cliks preset deep|balanced|social|quiet`: applies listening presets for volume, density, spatial, and fatigue fade.
- `cliks autostart enable|disable|status`: manages login-time background autoconnect for the selected team through systemd user services, macOS LaunchAgents, or the Windows Startup folder.
- `cliks fix-terminal`: restores sane terminal input and disables terminal mouse reporting after interrupted terminal-mode tests.
- `cli/install.sh`: installs the CLI through a user-local wrapper, runs `cliks doctor`, gives macOS/Windows/Linux setup hints, and on desktop Linux offers to add the current user to the `input` group. Termux is allowed as a non-supported test shell and must not be sent through sudo/input-group setup. Keep this user-facing and never request or print backend provider tokens.

Important platform reality:

- Windows can use low-level hooks.
- macOS can use Event Tap APIs with Accessibility permission.
- Linux Xorg can use XRecord/XInput/native hooks.
- Linux Wayland intentionally blocks normal desktop global input APIs. The current practical path is evdev via `/dev/input`, which requires local input-device permission. The CLI must never send key codes even though evdev exposes them locally; it should emit only `keyboard` or `mouse` event kind and coarse timing.
- Mouse activity means left/right click only. Do not count middle clicks, side buttons, scroll/wheel events, touchpad movement, app/window hover, pointer coordinates, or generic gestures. Linux evdev touchpads use a conservative tap heuristic: short stationary one-finger tap is left click, short stationary two-finger tap is right click, physical `BTN_LEFT`/`BTN_RIGHT` clicks are emitted directly, movement/long-press/three-or-more-finger gestures are ignored, and physical button clicks suppress duplicate tap emission.
- Evdev mode should only be reported after readable event devices are confirmed. Do not count streams that later fail with async `EACCES`, because that creates a false "connected but not sending" state.
- Terminal mode must capture and restore the original `stty` state and disable mouse reporting on close/error/signals. It should never modify Caps Lock, Shift state, layout, or inject keyboard events. If a terminal tab is already corrupted, use `cliks fix-terminal`.
- The `cliks start` status screen shows local captured and sent event counters. For one-way reports, use them to split capture/config failures from connection/send failures.
- Terminal-mode state is registered with a process-wide restore registry. Top-level uncaught exceptions, unhandled rejections, and process exit restore tracked terminal state before exiting.
- `cliks start` no longer exits on ordinary WebSocket close/error. It keeps capture running, shows connection status, sends client pings, terminates heartbeat timeouts, and retries with exponential backoff. Offline activity pulses are best-effort and may be dropped until the socket is open again.
- `cliks start` uses a Bubble Tea live dashboard when stdin/stdout are TTYs. It shows room, capture, connection, local counters, peers, hints, and sound controls with keyboard and mouse support. Controls: Up/Down volume, Left/Right or `[`/`]` density, `m` mute, `s` spatial toggle, `f` fatigue fade toggle, mouse wheel for volume, and clickable controls. `Tab` or `Shift+S` opens live settings without disconnecting; `Tab`/`Esc`/`q` returns to the room. These settings are persisted. Non-TTY runs fall back to a plain text status renderer.
- TUI hotkeys only come from the focused terminal because Bubble Tea reads stdin. Detached `cliks background start` and login autostart run non-interactively and must not react to unrelated keyboard input.
- Fatigue protection fades dense audio bursts after sustained activity so long typing does not become harsh. Density controls randomly thin non-essential playback locally; it never changes what is sent to the relay.

## Commands

Useful local commands:

```bash
npm install
npm run check
npm run build
npm run smoke:server
npm run load:server
npm run dev:server
npm run dev:site
cliks sound-test
cliks capture-test
cliks fix-terminal
cliks create
cliks delete CLIK-LOCAL
cliks join CLIK-LOCAL
cliks start --terminal --self
cliks settings
cliks set hear.self off
cliks preset deep
cliks background status
cliks autostart status
```

CI lives in `.github/workflows/ci.yml` and runs install/check/build/server smoke across Ubuntu, macOS, and Windows, plus Docker image build on Ubuntu. Docker backend packaging is in `Dockerfile` and `docker-compose.yml`. `scripts/smoke-server.mjs` verifies health redaction, code shape, WebSocket relay, live room closure on delete, deleted-room lookup behavior, and 50ms timing quantization. `scripts/load-test.mjs` can safely exercise local or live backends with `CLIKS_LOAD_*` environment variables.

## Deploy

Vercel deploys the site. Set:

```text
NEXT_PUBLIC_CLIKS_API_URL=https://your-backend-url
```

Current production site alias is `https://site-kappa-six-64.vercel.app`. An attempt on 2026-06-20 to assign `https://cliks.vercel.app` failed because Vercel reported that alias was already in use.

The current DigitalOcean backend is a Droplet running `cliks-api` under systemd with Caddy in front for HTTPS. The bootstrap file is `deploy/droplet-cloud-init.yaml`. The live Droplet should run local Postgres and set `CLIKS_LOCAL_POSTGRES=true` so team codes survive service restarts.

The public `/health` route must stay unauthenticated for uptime checks, but it must not expose team codes, team names, peer ids, nicknames, or per-room snapshots. It returns only `ok`, `totalRooms`, and `totalPeers`.

Room creation and deletion routes have lightweight in-process per-IP rate limits before expensive bcrypt work. Delete attempts must run a dummy bcrypt comparison when a code is missing so timing does not reveal whether a room exists. Database uniqueness should apply only to active team codes (`deleted_at is null`) so soft-deleted rows do not permanently burn code namespace. Successful deletes should also close the live room if it is currently occupied.

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
- Full stereo pan requires `ffplay` or `mpv` on PATH. Basic/native players may only support distance-based gain or unspatialized playback.
- The command is `cliks`; product name is Cliks.

## README Policy

Keep `README.md` from the user point of view. It should explain what Cliks does, how to install/run it, privacy guarantees, and basic deploy steps. Do not overload it with internal details like ring math, protocol internals, or backend implementation notes. Put those details here or in `docs/`.

## Public Backend URL

`cli/config.go` currently points new installs at `https://139.59.29.207.sslip.io`. This is a public backend URL, not a secret. Never put the DigitalOcean API token, SSH private key, or service credentials into the CLI, website bundle, README, install script, or committed env files.
