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

The receiving CLI schedules local sound playback using those offsets. That means the server sees only tiny JSON pulses, while the receiver still hears a natural rhythm.

## Team codes and public status

New team codes use the `CLIK-XXXXXX` shape. Older or local test codes such as `CLIK-LOCAL` can still be joined because API validation accepts longer code strings.

The public `/health` endpoint is intentionally safe for unauthenticated uptime checks. It returns only:

- `ok`
- `totalRooms`
- `totalPeers`

It must not return room codes, team names, peer ids, nicknames, or per-room snapshots. Detailed live room state should stay internal unless a future authenticated admin route is added.

Team creation and deletion are protected by lightweight in-memory per-IP throttles before bcrypt work runs. Deletion also uses a dummy bcrypt comparison for missing rooms so response timing does not reveal whether a code exists. When deletion succeeds, the relay closes any live room for that team and disconnects connected peers with a local error message.

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
- Supabase stores only team records
- relay health metrics are aggregate-only
- server and CLI WebSocket heartbeats clean up half-open sessions

Good next optimizations:

- adaptive batch window for large rooms
- binary WebSocket frames after the JSON prototype
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

Fatigue fade attenuates dense local playback after sustained bursts. Density thins local playback only; it does not change capture or relay privacy behavior.

`cliks background start` starts `cliks start` detached from the current terminal for the selected team and writes a pid/log under the user state directory. `cliks background status` reports running/stale/stopped and `cliks background stop` kills that detached process. This is separate from boot login behavior.

`cliks autostart enable` creates login-time background launchers for the current team: a systemd user service on Linux, a LaunchAgent on macOS, or a Startup-folder command on Windows. The launcher sets `CLIKS_AUTOSTART_TEAM` and runs `cliks start`.

Running bare `cliks` opens the Bubble Tea home interface. Running `cliks start` before a team is selected prints a short first-run setup checklist with `cliks join`, `cliks start`, `cliks doctor`, `cliks sound-test`, and `cliks capture-test` rather than surfacing an internal missing-team error.

Linux evdev mouse capture is click-only. It emits physical `BTN_LEFT` and `BTN_RIGHT` directly and uses a conservative touchpad tap detector for devices that do not emit button codes for tap-to-click: short stationary one-finger tap maps to left click, short stationary two-finger tap maps to right click, long holds/movement/three-or-more-finger gestures are ignored, and physical button activity suppresses duplicate tap output. The CLI must never send coordinates or pointer movement.

## Test and release gates

Required local checks before pushing:

- `npm run check`
- `npm run build`
- `npm run smoke:server`
- `bash -n cli/install.sh`
- `go test ./...` from `cli`
- cross-build the Go CLI for Linux, macOS, and Windows when capture/background/startup behavior changes

CI mirrors these on Ubuntu, macOS, and Windows through `.github/workflows/ci.yml`. The Docker job builds `Dockerfile` on Ubuntu. `scripts/smoke-server.mjs` covers health redaction, timing quantization, delete lookup behavior, and live-room closure on delete. `scripts/load-test.mjs` provides controlled relay load tests; keep default settings safe for the live Droplet and use explicit `CLIKS_LOAD_*` env vars for larger ramps.

## Free-tier expectation

Vercel should stay mostly idle because it serves a static team-creation page.

Supabase load is tiny because it stores team code records only.

Render is the bottleneck because it keeps WebSockets open and fans out activity batches. A $200 DigitalOcean credit runway would be useful for an always-on backend once demos move beyond a small beta.
