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

Team creation and deletion are protected by lightweight in-memory per-IP throttles before bcrypt work runs. Deletion also uses a dummy bcrypt comparison for missing rooms so response timing does not reveal whether a code exists. When deletion succeeds, the relay sends `team_deleted`, closes any live room for that team, and disconnects connected peers. When a client tries to join a missing or deleted code, the relay sends `team_unavailable` and closes the socket; the CLI must remove that team locally and stop retrying it.

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
- server and CLI WebSocket heartbeats clean up half-open sessions

Good next optimizations:

- adaptive batch window for large rooms
- binary WebSocket frames if compact JSON is not enough at larger scale
- Redis presence if the backend scales beyond one Render instance
- additional themed sound packs once the launch-critical setup/capture path is stable

## CLI reliability and audio

`cliks start` keeps the process alive through ordinary WebSocket close/error events. It reports connection state, sends pings, terminates heartbeat timeouts, retries with exponential backoff, and resumes joining the selected team when the backend is reachable again. Activity captured while disconnected is best-effort and currently dropped rather than buffered.

Terminal mode registers the captured `stty` state with a process-wide cleanup registry. Normal stops, top-level command failures, uncaught exceptions, unhandled rejections, and process exit all restore tracked terminal state and disable terminal mouse reporting.

The current audio engine still uses system players, but it caps concurrent playback processes and queues a bounded number of events so dense batches do not create unbounded process storms. Player priority is spatial-first: `ffplay` gets stereo pan plus gain through an FFmpeg audio filter, `mpv` gets stereo pan plus volume flags, `afplay`/`paplay`/`pw-play` get distance volume, and `aplay`/Windows `Media.SoundPlayer` remain basic fallback playback. A future native mixer could reduce process overhead further, but pan and distance now reach capable CLI players.

The Go CLI uses Bubble Tea for the live dashboard and settings UI. Interactive controls are local-only and persist to the config file:

- Up/Down: volume
- `[`/`]`: ambience density
- `m`: mute
- `s`: spatial on/off
- `f`: fatigue fade on/off
- `Tab` or `Shift+S`: open live settings without disconnecting; `Tab`/`Esc`/`q` returns to the room
- `Esc`, `q`, or Back from live: return to the main control screen
- Stop or `Ctrl+C`: disconnect the current session

Fatigue fade attenuates dense local playback after sustained bursts. Density thins local playback only; it does not change capture or relay privacy behavior.

`cliks background start` starts `cliks start` detached from the current terminal for the selected team and writes a pid/log under the user state directory. `cliks background status` reports running/stale/stopped and `cliks background stop` kills that detached process. This is separate from boot login behavior.

All local run modes share a single-session guard. `cliks start`, `cliks background start`, and boot autostart acquire `session.lock` under the user state directory and update `session.json` with pid, team, mode, connection status, active count, and local counters. A second local start must refuse to connect while that pid is alive. This prevents one device from joining the same team twice and playing the user's own activity back as a teammate. Stale locks are removed when the pid is gone. The Go CLI also scans for older same-executable `cliks start` processes that were launched before the lock existed; those are treated as active local sessions, and the TUI cleans up duplicate copies when a managed session is already active.

`cliks autostart enable` creates login-time background launchers for the current team: a systemd user service on Linux, a LaunchAgent on macOS, or a Startup-folder command on Windows. The launcher sets `CLIKS_AUTOSTART_TEAM`, sets `CLIKS_RUN_MODE=boot`, and runs `cliks start`. Linux uses `Restart=on-failure` and macOS does not set LaunchAgent `KeepAlive`; explicit Stop gracefully stops the current boot process without disabling launch-at-login, so it stays stopped until the next login/boot. Deleted or unavailable team codes may disable launch-at-login because the stored code is invalid.

Running bare `cliks` opens the Bubble Tea control screen. The home view stays intentionally small: greeting, selected team name plus code, active local connection state, `Open Live`, one-click `Keep Running`, `More`, and `Quit`. More contains Preferences, Team, Connection, and Diagnostics. Team includes Join, Create, Delete, Switch, and Nickname; Join saves the team and auto-opens the live room, while Create copies the new code when clipboard support exists. TUI actions should run in-place whenever possible, and mouse all-motion hover should move the highlighted row with hit-testing that accounts for the title, panel border, and padding. With no active connection, Keep Running only toggles the persisted preference and must not start a background session. If Keep Running is on, leaving or closing a foreground live dashboard should hand off to one detached background session. Running `cliks start` before a team is selected prints a short first-run setup checklist with `cliks join`, `cliks start`, `cliks doctor`, `cliks sound-test`, and `cliks capture-test` rather than surfacing an internal missing-team error.

`cliks nickname [NAME]` and the Team > Nickname TUI form configure an explicit display name capped at 10 characters. The server keeps that name only in live peer presence and relays it with peer activity so small-room dashboards can show names and "X, Y are typing." Larger rooms should show only total people and typing counts. When a connection is already active, turning Keep Running off should schedule it to stop when the control screen closes; use the separate Stop action for immediate disconnect. Stop must not disable launch-at-login.

Spatial placement remains client-side. Ring capacity is 4 seats in the first ring and then grows by 2 seats per ring. Optional dynamic placement counts recently received activity per peer and, on the configured interval, locally moves more active peers closer for that listener.

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

CI mirrors these on Ubuntu, macOS, and Windows through `.github/workflows/ci.yml`. The Docker job builds `Dockerfile` on Ubuntu. `scripts/smoke-server.mjs` covers health redaction, timing quantization, compact activity frames, nickname truncation, deleted-code lookup behavior, room caps, and live-room closure on delete. `scripts/load-test.mjs` provides controlled relay load tests; keep default settings safe for the live Droplet and use explicit `CLIKS_LOAD_*` env vars for larger ramps.

## Free-tier expectation

Vercel should stay mostly idle because it serves a static team-creation page.

Supabase load is tiny because it stores team code records only.

The Go relay keeps the baseline memory footprint low, but live fanout remains the bottleneck because every active sender fans out to room listeners. A $200 DigitalOcean credit runway would be useful for an always-on backend once demos move beyond a small beta.
