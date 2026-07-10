# Capture backends

Cliks captures **activity kinds only** (keyboard activity, left/right click). It never sends key values, coordinates, window titles, or app names.

For non-technical setup, start with **[setup.md](./setup.md)** and `cliks setup`.

## What ships today

| Platform | Default (`cliks start`) | Fallback |
|----------|-------------------------|----------|
| **Linux** | Readable `/dev/input/event*` (evdev) | Terminal mode if devices are locked down |
| **macOS** | Global hooks (`gohook` / OS event hooks) after Accessibility | Terminal mode; soft prompt to open Settings |
| **Windows** | Global hooks for normal-privilege apps | Terminal mode; elevated windows may pause capture (UIPI) |

### Commands

```bash
cliks start                 # auto backend
cliks start --evdev         # Linux global via /dev/input
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

**Permissions (installer + `cliks setup` try to do this for you):**

1. Session ACL on `/dev/input/event*` when `setfacl` + sudo work  
2. Permanent: user in the `input` group (log out/in once)

Wayland sandboxes / Flatpak often cannot see `/dev/input`. Use a host desktop session or terminal mode.

### macOS

- Accessibility permission is required for global hooks (OS rule — apps cannot skip the dialog)
- `cliks setup` and the installer open System Settings to the right pane
- Enable your **terminal app**, then rerun `cliks setup`

### Windows

- No special permission dialog for standard-user global hooks
- **UIPI:** while an *elevated* window is focused, hooks may pause; capture resumes on normal windows  
- This is a tip, not a setup failure

## Production direction

- Windows: keep low-level hooks; optional elevated helper for Admin-window coverage  
- macOS: optional native Event Tap / helper binary without third-party hooks  
- Linux Wayland: XDG InputCapture portal when distros expose it widely  
- Privilege-separated helpers remain optional; default path stays simple for non-tech users  

Privacy promise is unchanged on every backend: event kind + coarse timing only.
