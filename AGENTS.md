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
- `server`: Go API/WebSocket relay currently deployed on a DigitalOcean Droplet. It stores teams in Supabase when configured, local Postgres when `CLIKS_LOCAL_POSTGRES=true` or `DATABASE_URL` is set, otherwise an in-memory local test store.
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

The server only validates and relays these events. It does not assign 3D positions and does not store live event history. New CLIs negotiate `compact-v1`; when present, the server sends compact activity frames (`type: "a"`) to that recipient while preserving verbose JSON for older clients. Incoming WebSocket frames are capped at 8 KiB so full 128-event verbose batches still work while oversized payloads are rejected. Each socket also has a local message-rate guard for floods; do not lower the size cap below the largest legitimate batch without changing the client protocol first. Outgoing frames use one 32-frame bounded queue and writer goroutine per peer. Writes have a 5-second deadline, reads have a 75-second rolling deadline, and a full queue closes only that slow peer instead of blocking room fan-out or the heartbeat loop. The relay heartbeat runs every 10 seconds; a peer that misses the next pong is evicted on the following tick, bounding stale presence to roughly 20 seconds even when TCP does not immediately report the disconnect.

## Team Codes And Data

Team codes use the `CLIK-XXXXXX` shape for newly created rooms. The in-memory local test room remains `CLIK-LOCAL`.

Stored data:

- team code
- team name
- delete password hash
- timestamps

Teams track the last successful live connection and automatically soft-delete after 48 hours without a connection. Successful joins refresh the deadline. The hourly cleanup refreshes rooms that are currently occupied before expiring inactive rows, so a continuously connected room stays alive. New store implementations must preserve this lifecycle and the join/expiry race protection.

Live presence is in memory. Rooms disappear from memory when empty. Live peers include peer id, optional nickname/display name, joined timestamp, socket, and last-seen timestamp. Nicknames are explicit, plain text, and capped at 10 Unicode characters. Both CLI and relay strip ANSI escape sequences, control characters, and Unicode formatting controls before whitespace normalization and truncation; keep the relay as the trust boundary for modified clients. There is no persisted membership list and no stored total member count. The relay sends WebSocket pings every 10 seconds and removes peers after a missed response cycle, so half-open sockets should not leave ghost users for more than roughly 20 seconds.

Presence includes an explicit local status: `available`, `focus`, `break`, or `dnd`; unknown values normalize to `available`. Ephemeral reactions are allowlisted to `wave`, `nice`, `coffee`, `focus`, `celebrate`, and `break` and are never persisted. Every reaction is room-wide, including Wave; legacy target fields are ignored. The relay accepts at most six reactions per peer per 10 seconds. Enabled clients may turn any incoming signal into a native notification whose title combines the sender's sanitized presence nickname with an allowlisted emoji and whose body contains the fixed phrase; reactions never carry arbitrary message text. Local mute suppresses remote reaction animation and notification delivery, focus and DND suppress native notifications, and notification sound has a separate switch. Linux background clients reconstruct the user's D-Bus socket address when needed.

The public/default room cap is 20 live peers. Self-hosted relays may set `CLIKS_MAX_PEERS_PER_ROOM` from 2–200; a peer over that server's limit receives a room-full error and is disconnected. Public-backend client configs are locked to exactly 500 ms batching. Advanced → Server or `cliks set api.url` can select a self-hosted HTTP(S) backend, which unlocks the 100–2000 ms batch range and derives the WebSocket URL automatically.

Deleting a team requires its delete password. A successful delete marks the stored team row deleted, sends `team_deleted` to all live peers, and closes any live in-memory room for that team so connected peers cannot keep using a deleted code. Join and delete operations share a short per-team lifecycle gate around the store lookup/update and room mutation; this prevents a concurrent join from recreating a room with team data fetched just before deletion without blocking unrelated rooms. Joining a missing/deleted code sends `team_unavailable`; CLIs must remove that team from device storage, disable launch-at-login, and stop retrying it. Store or network errors send a generic retryable error and must never remove local configuration. Plain HTTP team-code lookups are rate-limited per source IP before database work to reduce code-scanning risk.

Teams can be created and deleted from the website, from `cliks create` / `cliks delete`, and from the bare `cliks` Bubble Tea interface. CLI/TUI delete-password prompts must not echo the password when a real terminal is available. CLI/TUI create should try to copy the new team code to the local clipboard and fail softly if no clipboard command exists.

## Client-Side Placement

Do not move placement logic to the server.

Each listener locally assigns positions to teammates relative to themselves. The server sends presence with peer ids, optional nicknames, and joined timestamps; the CLI sorts peers and places them into expanding rings:

- first ring: 2m radius, 4 people
- second ring: 3m radius, 6 people
- third ring: 4m radius, 8 people
- capacity keeps growing by 2 per ring

When people leave, the local listener recomputes the arrangement, so far users move inward to fill gaps. Placement is deterministic per listener using peer ids as jitter seeds, but it is listener-relative and not a shared server truth. Adjacent rings use cumulative half-seat rotations, and peer-id jitter spans one seat width, so outer rings do not begin on the same radial line.

Dynamic circle placement is optional and enabled by default for new configurations. When enabled, the listener counts received activity per peer and, every configured interval (default 10 minutes), locally places more active peers closer than inactive peers. Existing users who turned it off stay off. This remains listener-relative and must not move placement to the server.

Current audio playback stores pan/distance in placement and applies those values locally. macOS and Windows release binaries include an in-process Oto stereo PCM path with pan, gain, and embedded WAV playback; ordinary desktop installs do not require mpv, PowerShell audio, or another player. Linux/Termux prefer `mpv` (lavfi stereo pan + volume; never the invalid `--audio-pan` flag), then `ffplay`, then native/basic players (`paplay`/`pw-play`/`aplay` or Termux media player). When `audio.device` is configured, a routing-capable installed player is preferred: `mpv`, `paplay`, `pw-play`, then `aplay`; the built-in desktop path uses the system default output. The audio engine uses a bounded 96-job queue and four context-bound workers. Every external playback process has a 2-second timeout, and ending a session cancels active players and waits for all four workers to exit. Keyboard events that land within a 20ms local playback window are merged before queueing, using the latest event's offset/pan/gain, while mouse clicks stay distinct. Above 50% queue pressure it progressively sheds keyboard playback while preserving click events; at capacity it discards stale work in favor of recent work. Fatigue gain normalizes its five-second event threshold across the full 20-peer room capacity, then approaches its 35% floor through a smoothed nonlinear curve only under extreme sustained aggregate activity instead of muting ordinary busy-room ambience.

## Sound

The CLI uses bundled real WAV samples, not generated placeholder clicks.

Current pack:

- 5 keyboard samples in `cli/assets/sounds/keyboard`
- 1 mouse sample in `cli/assets/sounds/mouse` (real recorded click from Pixabay, trimmed; higher-pitch sample removed)

The audio engine randomly picks one sample per event. Mouse samples are real recorded click sounds and should remain audibly distinct from keyboard samples. Source/license details are in `cli/assets/sounds/NOTICE.md`. The website mirror in `site/public/sounds/` must stay in sync with both packs.

Audio playback uses the built-in stereo path first on macOS/Windows and auto-detects `ffplay`, `mpv`, `paplay`, `pw-play`, `aplay`, or Termux `termux-media-player` elsewhere. Release binaries embed every WAV and extract them to a versioned cache on first playback; do not return to source-tree-relative-only lookup. Missing Linux/Termux audio tools must be reported as a user-facing setup warning, not as an unhandled child-process crash. `ffplay` receives the bundled mono samples through an explicit stereo pan filter so left/right gain both apply. `cliks doctor` should show whether the active player has full stereo spatial support or only distance/basic playback.

The website mirrors this on the web. `site/components/AcousticProvider.tsx` preloads the keyboard and mouse WAVs from `site/public/sounds/` (a copy of the CLI pack) via the Web Audio API and plays a random sample on every `keydown`/`mousedown`, with randomized gain and playback-rate jitter to match the CLI's organic feel. Audio integrity rule: if the WAVs fail to load it must fail silently — never fall back to a synthesized oscillator beep. Keep `site/public/sounds/*` in sync with `cli/assets/sounds/*`.

Personal room tones are separate from teammate activity samples. `off`, `rain`, `fire`, `cafe`, `cloud`, `contemplation`, and `downtempo` select six CC0 tracks embedded as normalized MP3 assets and never synchronized or sent. Their level is adjustable from 5-100%; selecting `off` is the zero-volume state. macOS and Windows decode and loop them through the built-in Oto context. Linux/Termux decode them to a versioned cached WAV, then use the first available mpv, ffplay, PulseAudio, PipeWire, ALSA, or Termux player. Muting Cliks also pauses the room tone. Source and license details live in `cli/assets/ambient/NOTICE.md`.

## Capture

Current modes:

- Bare `cliks`: opens the full-terminal Bubble Tea control screen. The greeting/home view shows selected team name plus a directly clickable code, active local connection state, teammate count, local captured/sent counters, `Open Live`, `Solo Desk`, `Keep Running`, optional `Stop`, `More`, and `Quit`. Normal launches show a centered 3-second animation with bundled sound; the first launch and post-reset launch show a skippable 10-second spatial welcome with extra sound bursts, followed by a full-terminal one-decision-at-a-time setup for nickname/generated name, listening mix, OS permission, notifications, background behavior, launch at login, theme, and private room tone. Setup uses a large multicolor spatial illustration plus spaced label/help cards when enough rows are available and collapses its art/copy at short terminal heights. Full-height panels allocate flexible rows between primary controls and secondary instructions/status, anchoring the latter at the bottom instead of leaving accidental empty space underneath. Apply this vertical-section rule to new home, form, live rail, control, and Solo views. Ember/Ocean/Forest/Sunset/Aurora/Mono themes use intentional multicolor palettes and preview immediately as onboarding focus moves. Preferences includes Server, which accepts `public` or a self-hosted HTTP(S) URL and derives the WebSocket URL. Advanced includes a confirmed device-only Factory Reset that stops local Cliks, disables local autostart, clears local config/state, and restores first-run animations without deleting the server team.
- `cliks create` / `cliks delete [CODE]`: create or delete teams from the CLI. The bare TUI also has in-app create/delete forms. Team > Join should let users paste a code, save the team name/code, and auto-open the live room.
- `cliks join CODE`: validates and saves the team, then starts one detached background session by default. `cliks join --no-start CODE` only saves/selects the team for users who explicitly want the old manual flow.
- `cliks nickname [NAME]` / `cliks set nickname NAME`: configures the optional display name shared in live presence and peer activity. Names are capped at 10 characters. Empty names should be treated as anonymous; never infer names from the OS user, Git config, hostname, app/window title, or typed text.
- `cliks start`: defaults to privacy-isolated capture. Linux connects to a hardened root-owned `cliks-capture` helper whose socket emits only `k/l/r` tokens; the desktop user is never automatically added to `input`. macOS launches the dedicated open-source Cliks Capture.app and Input Monitoring belongs to that app, not Terminal. Windows uses first-party Win32 low-level hooks and needs no extra input permission. `capture.mode` can select `isolated`, `terminal`, or an explicitly less-safe `direct` compatibility fallback.
- `cliks setup`: one-time easy setup for non-tech users - built-in macOS/Windows spatial audio or automatic Linux mpv setup, plus isolated platform capture (Linux system helper, macOS Cliks Capture.app, Windows ready-by-default). Installers invoke it after replacing the binary; if any older foreground/background/boot owner is still connected, setup refreshes it as a background session on the installed version so new TUI controls and incoming notifications cannot remain stuck on stale code. Opening Live then attaches without another reconnect.
- `cliks start --evdev`: legacy alias for explicit direct Linux evdev capture. Prefer isolated mode; this broad fallback requires user `/dev/input` permission.
- `cliks start --terminal --self`: local test mode. It captures keyboard bytes and terminal mouse-report events from the active terminal and plays self audio.
- `cliks start` before joining a room prints first-run setup steps instead of a raw error.
- `cliks sound-test`: plays sample sounds without joining a room.
- `cliks solo`: opens an entirely local/offline spatial desk with 1-12 simulated coworkers, independent keyboard/click simulation switches and levels, plus a directly visible private embedded room tone and 5-100% level. Master, keyboard, click, and room-tone levels render as slider tracks. Hover chooses a slider; natural wheel scrolling or arrow keys adjusts it, clicking the track jumps to a level, and Tab cycles sliders for keyboard-only use. The responsive layout uses map + controls at wide sizes, stacked panels when enough rows remain, and a controls-first compact spatial hint at large terminal font sizes. Simulated coworkers type in short person-specific bursts with quiet gaps and occasional clicks, not memoryless isolated ticks. It must not acquire the live session lock, start capture, or contact the backend.
- `cliks capture-test`: runs local capture for a short window and reports keyboard/mouse event counts plus fix commands when nothing is captured.
- `cliks doctor`: explains privacy, builds the shared structured Go/audio/input-device report, and prints detected fix commands. Joining remains non-blocking but surfaces a short passive note when that report finds audio or capture blockers.
- `cliks config`: prints saved configuration plus current launch-at-login state. `cliks set` accepts one or more key/value pairs, including `solo.keyboardVolume` and `solo.mouseVolume`. `cliks set autostart on|off` proxies the existing platform autostart manager, and `cliks set audio.device DEVICE|default` configures advanced output routing where the selected player supports it. `CLIKS_API_URL` and `CLIKS_WS_URL` override saved URLs at load time.
- `cliks settings` / `cliks ui`: opens the Bubble Tea control screen; the old settings concept is now named Preferences inside the TUI. It supports keyboard and mouse interaction for volume, density, mute, spatial audio, dynamic circle placement, fatigue fade, self-monitoring, sharing toggles, Keep Running preference, and selected team. Preference rows should include short user-facing explanations.
- `cliks background start|stop|status [team-code]`: runs `cliks start` detached from the terminal for the selected team, reports the pid/log path/session state, or stops it. Use this for "close the terminal but keep Cliks connected" behavior; `cliks autostart` is for login-time launch.
- `cliks preset deep|balanced|social|quiet`: applies listening presets for volume, density, spatial, and fatigue fade.
- `cliks autostart enable|disable|status`: manages login-time background autoconnect for the selected team through systemd user services, macOS LaunchAgents, or the Windows Startup folder. Linux services should use `Restart=on-failure`, macOS LaunchAgents should not set `KeepAlive`, and shared stop paths must gracefully stop the current process without disabling launch-at-login. A stopped boot session should stay stopped until the next login/boot; disabling autostart is a separate explicit action. Deleted/unavailable teams may still disable launch-at-login because the stored code is invalid.
- `cliks fix-terminal`: restores sane terminal input and disables terminal mouse reporting after interrupted terminal-mode tests.
- `cli/install.sh`: installs a matching native release plus Cliks Capture.app on macOS or the hardened system helper on Linux, then runs `cliks setup`. It must never automatically grant the desktop user raw input ACL/group access. `cli/install.ps1` remains the native Windows installer. Termux never receives desktop-wide capture claims or sudo setup.
- `docs/setup.md`: non-technical setup guide for macOS, Windows, and Linux.

Important platform reality:

- Windows can use low-level hooks.
- macOS uses a dedicated listen-only Cliks Capture.app with Input Monitoring permission. Direct terminal permission is compatibility-only.
- Linux Xorg can use XRecord/XInput/native hooks.
- Linux Wayland intentionally blocks normal desktop global input APIs. A hardened `cliks-capture` system helper reads evdev and exposes only fixed activity-kind tokens over a target-user-owned `0600` Unix socket. It verifies both peer UID and installed Cliks executable and emits only while that user owns an active local logind seat. Direct evdev is an explicit compatibility fallback.
- Mouse activity means left/right click only. Do not count middle clicks, side buttons, scroll/wheel events, touchpad movement, app/window hover, pointer coordinates, or generic gestures. Linux evdev touchpads use a conservative tap heuristic: short stationary one-finger tap is left click, short stationary two-finger tap is right click, physical `BTN_LEFT`/`BTN_RIGHT` clicks are emitted directly, movement/long-press/three-or-more-finger gestures are ignored, and physical button clicks suppress duplicate tap emission.
- Evdev mode should only be reported after readable event devices are confirmed. Do not count streams that later fail with async `EACCES`, because that creates a false "connected but not sending" state. Non-EOF read failures use cancel-aware exponential retries from 1s to 30s instead of a busy loop.
- Local capture and session channels use bounded 1024-event buffers with cancellation-aware backpressure. Do not reintroduce silent default-case drops for human keyboard/click bursts. Offline WebSocket activity remains best-effort after it reaches the session send path.
- Terminal mode must capture and restore the original `stty` state and disable mouse reporting on close/error/signals. It should never modify Caps Lock, Shift state, layout, or inject keyboard events. If a terminal tab is already corrupted, use `cliks fix-terminal`.
- The `cliks start` status screen shows local captured and sent event counters. For one-way reports, use them to split capture/config failures from connection/send failures.
- Terminal capture owns its saved `term.State`, restores it through deferred/idempotent cleanup, and disables mouse reporting on close or error. A raw-mode `Ctrl+C` cancels capture without closing stdin first, so restoration still has a valid terminal file descriptor.
- `cliks start` no longer exits on ordinary WebSocket close/error. It keeps capture running, shows connection status, sends client pings, extends a 75-second read deadline on traffic/pongs, closes the socket immediately when the session context ends, and retries with exponential backoff. Offline activity pulses are best-effort and may be dropped until the socket is open again.
- `cliks start` must stop retrying when the server sends `team_deleted` or `team_unavailable`. In that case it should remove the team from local config, disable launch-at-login, and show a clear stopped/unavailable notice.
- `cliks start` uses a full-terminal Bubble Tea spatial desk when stdin/stdout are TTYs. The listener sits in the center; up to 12 named peers occupy adaptive depth rings, current typers light up, new peers animate in, reactions briefly animate inside the circle, and larger rooms collapse overflow into semantic dots. The first live visit shows a short synthetic welcome arrangement without sending activity. A readable direct action rail makes code copy, private room-tone selection/volume, notifications, notification sound, mute, spatial audio, every room-wide reaction, Preferences, Back, and Stop clickable. Reactions visibly carry shortcuts 1-5. Hit testing must be derived only from the rendered rail; do not revive the detached bottom-control coordinates that caused middle-screen hover activation. Escape/Back opens an in-session control menu and must not stop or reconnect the session. The menu can resume live, open Preferences, or switch among saved named team codes. When another local background/boot session owns the connection, the TUI attaches to its persisted live view and sends local control commands without creating a second WebSocket connection. Keyboard equivalents remain in the footer and `?` help. Live and Solo wheel volume controls follow natural touchpad scrolling; menu-list scrolling remains conventional.
- Cliks enforces one active local connection per user state directory with `session.lock` and `session.json` under `stateDir()`. Foreground `cliks start`, manual `cliks background start`, and boot autostart all share this lock. If one is active, any second local start must fail instead of creating another peer and feeding the user's own activity back as remote audio. Lock acquisition must not blindly delete a young empty `session.lock` (another process may still be writing PID metadata). Config and session state files must be written atomically (temp + fsync + rename). Every successful config save refreshes a last-known-good backup; invalid primary JSON restores that backup and warns on stderr once instead of silently erasing team history. WebSocket dials must honor session cancellation, and heartbeat writes must share `wsMu` with other writers. A `room_full` protocol error is terminal for that attempt and must not enter the reconnect loop. The session state tracks pid, mode (`foreground`, `background`, `boot`), team, connection status, active count, and local counters so the control screen can show the current connection. The Go CLI also scans for older same-executable `cliks start` processes that predate the lock file and treats them as active local sessions; when a managed session is active, the TUI should clean up those duplicate local copies.
- Session state records the running binary version. Opening Live through a newer CLI refreshes an older foreground/background owner before attaching, preventing a newly installed TUI from silently sending controls to stale code. Quick signals show `Sending` until the sender's room-wide `peer_reaction` echo acknowledges relay delivery; a missing echo becomes a visible failure instead of a false `Shared` message.
- TUI hotkeys only come from the focused terminal because Bubble Tea reads stdin. Detached `cliks background start` and login autostart run non-interactively and must not react to unrelated keyboard input.
- Home/control TUI mouse movement should update the highlighted row on hover. Use all-motion mouse tracking and keep row hit-testing aligned with the title, panel border, and padding. Binary settings should be single toggle rows, not separate on/off menu choices.
- TUI mouse clicks should activate only the row under the pointer. A keyboard-focused row may look focus-highlighted for Enter, but it must not trigger from a mouse click elsewhere.
- Keep Running must never affect navigation inside the TUI. With no active session it only toggles the persisted preference and must not start a background session. When the user explicitly quits a foreground live app, Keep Running on hands the session to one detached background process and off stops it. An already detached background or boot owner continues until Stop. Stop remains the immediate disconnect and must not disable launch-at-login.
- Fatigue protection fades dense audio bursts after sustained activity with a room-scaled threshold and smoothed nonlinear gain. Density and queue-pressure shedding thin non-essential playback locally; they never change what is sent to the relay.

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
cliks join --no-start CLIK-LOCAL
cliks start CLIK-LOCAL
cliks start --terminal --self
cliks settings
cliks set --list
cliks set hear.self off
cliks set autostart on
cliks set audio.device default
cliks preset deep
cliks background status
cliks autostart status
```

CI lives in `.github/workflows/ci.yml` and runs install/check/build/server smoke across Ubuntu, macOS, and Windows, plus Docker image build on Ubuntu. Tagged `v*` pushes run `.github/workflows/release.yml`, which has separate native Unix and Windows packaging jobs so the GitHub UI does not present expected platform branches as skipped archive steps. It tests and packages native Linux x64/arm64, macOS Intel/Apple Silicon, and Windows x64 CLI archives for the installers. Docker backend packaging is in `Dockerfile` and `docker-compose.yml`. `scripts/smoke-server.mjs` verifies health redaction, code shape, WebSocket relay, compact activity frames, nickname ANSI/control sanitization plus truncation, live room closure on delete, deleted-room lookup behavior, room limits, single-room migration, failed-join throttling, and 50ms timing quantization. Go unit tests also cover reaction allowlists and room broadcast, HTTP lookup throttling, WebSocket oversized/flood protection, non-blocking slow-peer queues, missed-heartbeat eviction, concurrent join/delete serialization, population-scaled fatigue gain, audio playback deadlines, worker cancellation, structured diagnostics, responsive onboarding/Solo layouts, and TUI diagnostic/footer behavior.

## Deploy

Vercel deploys the site. Set:

```text
NEXT_PUBLIC_CLIKS_API_URL=https://your-backend-url
```

Current production site alias is `https://site-kappa-six-64.vercel.app`. An attempt on 2026-06-20 to assign `https://cliks.vercel.app` failed because Vercel reported that alias was already in use.

The current DigitalOcean backend is a Droplet running the Go `cliks-api` service under systemd with Caddy in front for HTTPS. The bootstrap file is `deploy/droplet-cloud-init.yaml`. The live Droplet should run local Postgres and set `CLIKS_LOCAL_POSTGRES=true` so team codes survive service restarts.

The public `/health` route must stay unauthenticated for uptime checks, but it must not expose team codes, team names, peer ids, nicknames, or per-room snapshots. It returns only `ok`, `totalRooms`, and `totalPeers`.

Room creation and deletion routes have lightweight in-process per-IP rate limits before expensive bcrypt work. Their maps are rebuilt during periodic expiry cleanup so one burst of unique IPs does not permanently retain Go map buckets. Delete attempts must run a dummy bcrypt comparison when a code is missing so timing does not reveal whether a room exists. Database uniqueness should apply only to active team codes (`deleted_at is null`) so soft-deleted rows do not permanently burn code namespace. Successful deletes should also close the live room if it is currently occupied. Postgres startup retries migrations for a bounded readiness window before failing, covering normal database/container boot races without rapid crash loops.

Security posture for the live Droplet:

- the DigitalOcean API token is not in the repo, website bundle, CLI, installer, or Droplet app env
- the CLI contains only the public backend URL, which is not secret
- UFW allows only OpenSSH, HTTP, and HTTPS
- the Go server listens on port 8787 behind Caddy, and direct public access to 8787 should be blocked by firewall
- SSH password and keyboard-interactive auth are disabled
- CORS is set to the production Vercel origin

If using App Platform, Render, or another managed host, set:

```text
CORS_ORIGIN=https://your-vercel-site
SUPABASE_URL=...
SUPABASE_SERVICE_ROLE_KEY=...
```

Supabase should run `supabase/schema.sql`.

## Known Issues

- `npm audit --omit=dev` currently reports moderate advisories through Next/PostCSS dependency metadata. Do not force downgrade to old Next; wait for a patched compatible release or reassess if Next dependency graph changes.
- Global capture uses isolated helpers on Linux/macOS and native hooks on Windows. Edge cases include sandboxes/Flatpak, remote SSH, unsigned macOS Gatekeeper approval, and Windows UIPI over elevated windows.
- Full stereo pan is built into macOS/Windows release binaries. Linux requires `mpv` or `ffplay` on PATH for stereo pan (mpv uses lavfi pan); basic Linux players may only support distance-based gain or unspatialized playback. Installer and `cliks setup` try to install mpv automatically on Linux.
- The command is `cliks`; product name is Cliks.

## README Policy

Keep `README.md` from the user point of view. It should explain what Cliks does, how to install/run it, privacy guarantees, and basic deploy steps. Do not overload it with internal details like ring math, protocol internals, or backend implementation notes. Put those details here or in `docs/`.

## Public Backend URL

`cli/config.go` currently points new installs at `https://139.59.29.207.sslip.io`. This is a public backend URL, not a secret. Never put the DigitalOcean API token, SSH private key, or service credentials into the CLI, website bundle, README, install script, or committed env files.
