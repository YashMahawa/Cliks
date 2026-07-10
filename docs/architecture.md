# Cliks Architecture

## Core promise

Cliks should feel live and human without becoming surveillance.

The CLI records only activity shape:

- `keyboard`
- `mouse`
- mouse button when available
- coarse interval offsets inside a 500ms batch

It must not send:

- actual key values
- key codes
- words
- mouse coordinates
- active app or window title
- screen or microphone data

## Batching

The CLI batches for 500ms by default. This keeps Render/WebSocket load lower while preserving the timing feel.

Clients may send local millisecond offsets to the relay, but the server rounds offsets into 50ms buckets before forwarding activity to teammates. This limits keystroke-rhythm fingerprinting while preserving enough timing to sound natural.

Example:

```json
{
  "type": "activity_batch",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "keyboard", "offsetMs": 100 },
    { "kind": "mouse", "button": "left", "offsetMs": 300 },
    { "kind": "keyboard", "offsetMs": 500 }
  ]
}
```

The receiving CLI schedules local sound playback using those offsets. That means the server sees only tiny pulses, while the receiver still hears a natural rhythm. New CLIs negotiate compact server-to-client activity frames with `compact-v1`; older clients still receive the verbose JSON shape.

## Team codes and public status

New team codes use the `CLIK-XXXXXX` shape. Older or local test codes such as `CLIK-LOCAL` can still be joined because API validation accepts longer code strings.

The public `/health` endpoint is intentionally safe for unauthenticated uptime checks. It returns only:

- `ok`
- `totalRooms`
- `totalPeers`

It must not return room codes, team names, peer ids, nicknames, or per-room snapshots. Detailed live room state should stay internal unless a future authenticated admin route is added.

Team creation and deletion are protected by lightweight in-memory per-IP throttles before bcrypt work runs. Plain HTTP team-code lookups are also throttled per IP before database work so scanners cannot cheaply enumerate codes through `GET /api/teams/:code`. Failed WebSocket joins have a separate 20-attempt, 5-minute per-IP budget; valid joins do not consume that budget. Once exhausted, the relay returns `join_rate_limited` and closes the socket so clients back off before retrying. WebSocket frames are capped at 8 KiB and each connection has a local message-rate guard; 8 KiB is intentional because a full 128-event verbose JSON batch can exceed 4 KiB. Deletion also uses a dummy bcrypt comparison for missing rooms so response timing does not reveal whether a code exists. Join and delete share a reference-counted per-team lifecycle gate while they read/update storage and mutate the live room. That closes the stale-read race where a join could otherwise recreate a room after deletion, while operations for unrelated teams remain concurrent. When deletion succeeds, the relay sends `team_deleted`, closes any live room for that team, and disconnects connected peers. When a client tries to join a missing or deleted code, the relay sends `team_unavailable` and closes the socket; the CLI must remove that team locally and stop retrying it. Store failures return a generic error, preserving local team configuration for the reconnect loop.

Soft-deleted team rows are retained for history, but code uniqueness is scoped to active rows only. Postgres and Supabase both use a partial unique index on `(code) where deleted_at is null`, so deleting a room does not permanently consume its code.

Users can create and delete teams from the website, from `cliks create` / `cliks delete`, or from the bare `cliks` TUI. CLI/TUI delete-password entry should remain masked when stdin is an interactive terminal.

## Scaling notes

Load is dominated by live fanout, not storage.

```text
messages ~= active senders * listener count * batches per second
```

Current defaults:

- 500ms batch window
- max 128 events per batch
- no stored event history
- rooms exist only while at least one client is connected
- rooms are capped at 20 live peers
- live peer nicknames exist only in memory and are relayed in presence/activity
- Supabase stores only team records
- relay health metrics are aggregate-only
- the relay pings every 10 seconds and evicts a peer after a missed response cycle, bounding stale presence to roughly 20 seconds
- one WebSocket connection can belong to only one room; joining another room atomically migrates it
- peer profile, leave, and activity routing use the connection's current room directly instead of scanning every room
- inbound WebSocket frames and per-connection message rates are bounded before JSON processing
- outgoing frames use one bounded 32-frame queue and writer per peer, isolating slow listeners from room fan-out
- WebSocket reads use a rolling 75-second deadline and writes use a 5-second deadline
- long-running heartbeat ticks and per-connection handlers recover and log panics with stack traces

Good next optimizations:

- adaptive batch window for large rooms
- binary WebSocket frames if compact JSON is not enough at larger scale
- Redis presence if the backend scales beyond one Render instance
- additional themed sound packs once the launch-critical setup/capture path is stable

## CLI reliability and audio

`cliks start` keeps the process alive through ordinary WebSocket close/error events. It reports connection state, sends pings, terminates heartbeat timeouts, retries with exponential backoff plus bounded +/-10% jitter, and resumes joining the selected team when the backend is reachable again. The 30-second cap remains unchanged. Linux evdev read retries use the same jittered schedule so many clients/devices do not recover in lockstep. Activity captured while disconnected is best-effort and currently dropped rather than buffered.

Terminal capture stores its original `term.State` on the capture instance and restores it through deferred, idempotent cleanup. Raw-mode `Ctrl+C` cancels capture without closing stdin, so the restore still has a valid terminal file descriptor; `cliks fix-terminal` remains the manual fallback. TUI SIGHUP/SIGTERM handling uses Bubble Tea context cancellation rather than `os.Exit`, so alternate-screen/mouse cleanup and foreground-to-background handoff can finish through the normal path.

The current audio engine still uses system players, but it caps concurrent playback at four processes and queues at most 96 jobs so dense batches do not create unbounded process storms. The four workers are bound to the owning session context, and every external player command has a 2-second timeout. Session teardown cancels active players and waits for the workers, preventing goroutine/process accumulation across repeated Live sessions. Player priority is spatial-first: **`mpv` first** (lavfi stereo pan + volume; never the invalid `--audio-pan` flag), then `ffplay` with an FFmpeg pan filter, then `afplay`/`paplay`/`pw-play` for distance volume, and `aplay`/Windows `Media.SoundPlayer` as basic fallbacks. `cliks setup` and the installer try to install mpv so non-technical users get spatial sound without manual package hunting. Before queueing, keyboard events within a 20ms playback window are merged and use the latest event's offset/pan/gain; mouse clicks are never merged. Above 50% queue pressure the client progressively thins keyboard playback while preserving click events; overflow replaces stale queued work with recent work. This keeps sound current instead of replaying a long backlog. Fatigue protection allows roughly 24 queued playback events per remote peer in its five-second window, scales through all 20 room seats, and uses a gradual nonlinear reduction that reaches the 35% floor only at extreme sustained aggregate load. A future native mixer could reduce process overhead further, but pan and distance now reach capable CLI players.

Advanced users can set `audio.device`. Device routing prefers `mpv`, `paplay`, `pw-play`, or `aplay`, injects the player's native device argument without invoking a shell, and falls back with a `cliks doctor` warning when the active player cannot route. Changing the setting while a session is active reselects the player locally.

The Go CLI uses Bubble Tea for the live dashboard and settings UI. Interactive controls are local-only and persist to the config file:

- Up/Down: volume
- `[`/`]`: ambience density
- `m`: mute
- `s`: spatial on/off
- `f`: fatigue fade on/off
- `Tab` or `Shift+S`: open live settings without disconnecting; `Tab`/`Esc`/`q` returns to the room
- `Esc`, `q`, or Back from live: return to the main control screen
- Stop or `Ctrl+C`: disconnect the current session
- `?`: context-specific shortcut guide in home, preferences, live, and live preferences

TUI colors use Lip Gloss adaptive light/dark semantic pairs, avoiding fixed ANSI indexes that disappear against light terminal themes.

The live dashboard includes a local-only flow badge and a health line that pulses while connected and shows the latest local or peer activity age. These indicators are display heuristics only: they do not add server traffic, stored history, or key/content inspection.

Fatigue fade attenuates dense local playback after sustained bursts. Its 5-second threshold scales from 24 events for one peer to 48 events for ten or more peers, then follows a smoothed nonlinear curve down to the 35% floor. Density and queue-pressure shedding affect local playback only; they do not change capture or relay privacy behavior.

`cliks background start` starts `cliks start` detached from the current terminal for the selected team and writes a pid/log under the user state directory. `cliks background status` reports running/stale/stopped and `cliks background stop` kills that detached process. This is separate from boot login behavior.

All local run modes share a single-session guard. `cliks start`, `cliks background start`, and boot autostart acquire `session.lock` under the user state directory and update `session.json` with pid, team, mode, connection status, active count, and local counters. A second local start must refuse to connect while that pid is alive. This prevents one device from joining the same team twice and playing the user's own activity back as a teammate. Stale locks are removed when the pid is gone. The Go CLI also scans for older same-executable `cliks start` processes that were launched before the lock existed; those are treated as active local sessions, and the TUI cleans up duplicate copies when a managed session is already active.

`cliks autostart enable` creates login-time background launchers for the current team: a systemd user service on Linux, a LaunchAgent on macOS, or a Startup-folder command on Windows. The launcher sets `CLIKS_AUTOSTART_TEAM`, sets `CLIKS_RUN_MODE=boot`, and runs `cliks start`. Linux uses `Restart=on-failure` and macOS does not set LaunchAgent `KeepAlive`; explicit Stop gracefully stops the current boot process without disabling launch-at-login, so it stays stopped until the next login/boot. Deleted or unavailable team codes may disable launch-at-login because the stored code is invalid.

Running bare `cliks` opens the Bubble Tea control screen. With no selected team, the first screen exposes Join, Create, Sound Check, and Setup Check directly; this is a skippable first-run path rather than a mandatory wizard. Once configured, the home view stays intentionally small: greeting, selected team name plus code, active local connection state, `Open Live`, one-click `Keep Running`, `More`, and `Quit`. More contains Preferences, Advanced, Team, Connection, and Diagnostics. A shared footer polls the existing config/session state and keeps team, connection, volume/mute, and room count visible across these subviews. Doctor logic returns one structured report used by both `cliks doctor` and a wrapped, scrollable Diagnostics report with mouse/keyboard scrolling plus in-place Back and Refresh controls. Join remains non-blocking and surfaces only a short passive setup note when audio or input blockers exist. Advanced edits nickname, audio output, and the batch window; low-level backend overrides remain discoverable through `cliks set --list` instead of being easy to change accidentally. Team includes Join, Create, Delete, Switch, and Nickname; Join saves the team and auto-opens the live room, while Create copies the new code when clipboard support exists. Text forms track a rune cursor and support left/right, Home/End, Backspace/Delete, insertion, and mouse field focus. Preference rendering uses a cursor-centered visible window on short terminals. TUI actions should run in-place whenever possible, and mouse all-motion hover should move the highlighted row with hit-testing derived from rendered prefix/row locations. With no active connection, Keep Running only toggles the persisted preference and must not start a background session. If Keep Running is on, leaving or closing a foreground live dashboard should hand off to one detached background session. Running `cliks join CODE` from the shell validates and saves the team, then starts a single detached background session by default; `--no-start` preserves the manual path. `cliks start CODE` validates, saves, selects, and starts a team even when config is empty. Running `cliks start` before a team is selected prints a short first-run setup checklist with `cliks setup`, `cliks join`, `cliks sound-test`, and `cliks doctor` rather than surfacing an internal missing-team error.

`cliks nickname [NAME]` and the Team > Nickname TUI form configure an explicit display name capped at 10 Unicode characters. The CLI and server strip ANSI escape sequences, C0/C1 controls, and Unicode formatting controls before whitespace normalization and truncation. Server-side normalization protects users from modified clients. The server keeps the resulting name only in live peer presence and relays it with peer activity so small-room dashboards can show names and "X, Y are typing." Larger rooms should show only total people and typing counts. When a connection is already active, turning Keep Running off should schedule it to stop when the control screen closes; use the separate Stop action for immediate disconnect. Stop must not disable launch-at-login.

Spatial placement remains client-side. Ring capacity is 4 seats in the first ring and then grows by 2 seats per ring. Each ring starts halfway between seats from the previous ring, accumulated deterministically, and peer-id jitter spans one seat width. This preserves the documented ring distances while preventing every ring from starting at the same panning angle. Dynamic placement is enabled for new configurations, remains optional, counts recently received activity per peer, and on the configured interval locally moves more active peers closer for that listener.

Capture-to-session handoff uses bounded 1024-event channels with cancellation-aware backpressure. Human typing/click bursts wait briefly for the batch consumer instead of being silently discarded. Linux evdev read errors retry with interruptible, jittered exponential delays from about 0.9 seconds up to 30 seconds, eliminating CPU busy-waits and synchronized retry spikes while still recovering from transient errors. Activity after the send path reaches a disconnected WebSocket remains best-effort rather than being persisted.

The default 500ms client batch and server-side 50ms relay quantization remain unchanged. Jules proposals for adaptive precision/batching were not adopted because changing them would alter the current privacy contract and egress model.

Linux evdev mouse capture is click-only. It emits physical `BTN_LEFT` and `BTN_RIGHT` directly and uses a conservative touchpad tap detector for devices that do not emit button codes for tap-to-click: short stationary one-finger tap maps to left click, short stationary two-finger tap maps to right click, long holds/movement/three-or-more-finger gestures are ignored, and physical button activity suppresses duplicate tap output. The CLI must never send coordinates or pointer movement.

## Test and release gates

Required local checks before pushing:

- `npm run check`
- `npm run build`
- `npm run smoke:server`
- `bash -n cli/install.sh`
- `go test ./...` from `cli`
- cross-build the Go CLI for Linux, macOS, and Windows when capture/background/startup behavior changes
- `go test ./...` from `server` when relay/store/protocol behavior changes

CI mirrors these on Ubuntu, macOS, and Windows through `.github/workflows/ci.yml`. The Docker job builds `Dockerfile` on Ubuntu. `scripts/smoke-server.mjs` covers health redaction, timing quantization, compact activity frames, nickname truncation, deleted-code lookup behavior, room caps, live-room closure on delete, single-room auto-migration, and failed-join throttling. Go unit tests cover lookup throttling, WebSocket oversized/flood protection, slow-peer queue isolation, missed-heartbeat eviction, concurrent join/delete lifecycle ordering, population-scaled fatigue behavior, audio deadlines/cancellation, shared diagnostic rendering, and TUI report/footer behavior. `scripts/load-test.mjs` defaults to a local server on port 8787, waits for each welcome before sending, requires the exact expected fan-out count, and deletes temporary teams even when a run fails. Set `CLIKS_LOAD_TARGET` explicitly for a live backend and keep live `CLIKS_LOAD_*` values conservative.

## Free-tier expectation

Vercel should stay mostly idle because it serves a static team-creation page.

Supabase load is tiny because it stores team code records only.

The Go relay keeps the baseline memory footprint low, but live fanout remains the bottleneck because every active sender fans out to room listeners. A $200 DigitalOcean credit runway would be useful for an always-on backend once demos move beyond a small beta.
