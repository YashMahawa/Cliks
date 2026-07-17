# Capture backends

Cliks captures **activity kinds only** (keyboard activity, left/right click). It never sends key values, coordinates, window titles, or app names.

For non-technical setup, start with **[setup.md](./setup.md)** and `cliks setup`.

## What ships today

| Platform | Default (`cliks start`) | Fallback |
|----------|-------------------------|----------|
| **Linux** | Root-owned minimal helper; local socket emits only allowlisted kinds | Direct evdev (explicit broad fallback) or terminal-only |
| **macOS** | Dedicated Cliks Capture.app with a listen-only Event Tap | Direct terminal permission (explicit less-safe fallback) or terminal-only |
| **Windows** | First-party Win32 low-level keyboard/mouse hooks | Terminal mode; elevated windows may pause capture (UIPI) |

### Commands

```bash
cliks start                 # isolated backend (default)
cliks set capture.mode terminal
cliks set capture.mode direct  # explicit compatibility fallback
cliks start --terminal --self
cliks capture-test
cliks setup                 # grant access / check readiness
```

### Terminal mode

- Captures keys and terminal mouse reports **only in the focused terminal**
- Restores cooked mode and disables mouse reporting on exit
- After a crash: `cliks fix-terminal`

### Linux evdev details

- `BTN_LEFT` / `BTN_RIGHT` → left/right click
- Short stationary one-finger touchpad tap → left click
- Short stationary two-finger tap → right click
- Movement, scroll, multi-finger gestures ignored
- Device read errors use exponential backoff + jitter (no busy loop)

The installer creates a dedicated `cliks-capture` system user in the `input` group and runs a hardened helper. The desktop user receives only `k`, `l`, or `r` tokens over `/run/cliks/capture.sock`. Cliks no longer automatically grants per-user ACLs or adds the desktop user to `input`.

Wayland sandboxes / Flatpak often cannot see `/dev/input`. Use a host desktop session or terminal mode.

### macOS

- Input Monitoring permission is required for the native listen-only Event Tap (OS rule — apps cannot bypass it)
- Grant Input Monitoring only to Cliks Capture.app. The terminal remains unprivileged.
- `cliks setup` and the installer open System Settings to the right pane
- Unsigned community builds may need Privacy & Security → Open Anyway once. Never disable Gatekeeper.
- Direct mode is retained only as a labeled trial/fallback. Remove the terminal's Input Monitoring permission after use.

### Windows

- No special permission dialog for the built-in standard-user hooks
- **UIPI:** while an *elevated* window is focused, hooks may pause; capture resumes on normal windows  
- This is a tip, not a setup failure

## Implementation and next steps

- Windows uses `SetWindowsHookExW` with `WH_KEYBOARD_LL` / `WH_MOUSE_LL` on a dedicated message-loop thread.
- macOS uses `CGEventTapCreate` with a listen-only mask containing only key-down and left/right mouse-down.
- Both native callbacks enqueue only an activity kind; key codes and event payloads are never copied into the Cliks event model.
- `cliks doctor` probes native hook initialization on both platforms.
- Linux Wayland: XDG InputCapture portal when distros expose it widely  
- Privilege-separated helpers are the default trust boundary on Linux and macOS.

Privacy promise is unchanged on every backend: event kind + coarse timing only.
