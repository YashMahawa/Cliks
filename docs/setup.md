# Cliks setup guide (macOS, Windows, Linux)

This guide is for **everyone** — you do not need to be a developer.

Cliks has two jobs on your computer:

1. **Hear activity kinds** (keyboard or mouse click happened) — never the keys you typed  
2. **Play soft spatial sound** so teammates feel nearby (left/right + distance)

---

## Super short path

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
cliks join CLIK-XXXXXX
```

That is usually enough on macOS and Linux. It downloads a native release first, so Go and Git are not required on normal machines. The installer:

- installs the native `cliks` command (with its real sound pack embedded)
- includes stereo spatial audio directly on macOS and Windows; installs **mpv** on Linux when possible
- prepares background capture access on Linux
- requests Input Monitoring on macOS and opens the exact pane (one switch)
- runs `cliks setup` for a plain-language readiness check

---

## Per operating system

### macOS

| Need | What Cliks does | What you might do once |
|------|-----------------|-------------------------|
| Spatial sound | Built into the Cliks binary (stereo pan + distance) | Nothing |
| Background capture | Uses a built-in listen-only CoreGraphics Event Tap | **One** permission: System Settings → Privacy & Security → **Input Monitoring** → enable Cliks or the terminal app that launches it (Terminal / iTerm / Warp / VS Code). |

After granting Input Monitoring, restart Cliks once:

```bash
cliks setup
cliks join CLIK-XXXXXX
```

No key values are ever read or sent — only “keyboard activity” / “mouse click”.

### Windows

| Need | What Cliks does | What you might do once |
|------|-----------------|-------------------------|
| Spatial sound | Built into the Cliks binary (stereo pan + distance) | Nothing |
| Background capture | Built-in Win32 low-level hooks work for normal apps with **no** special permission dialog | Nothing for everyday use |
| Settings location | Config lives under `%APPDATA%\cliks\` (native Windows path) | Older installs auto-migrate from `.config\cliks` |
| Launch at login | Silent VBScript startup (no console flash) | `cliks service enable` once |

**Note (not an error):** if you focus Task Manager or another *Administrator* window, Windows security may pause capture until you leave that window. Normal apps are fine.

Open PowerShell (no Administrator mode needed) and run:

```powershell
irm https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.ps1 | iex
```

Then open a new terminal and:

```bash
cliks setup
cliks join CLIK-XXXXXX
```

### Linux (Ubuntu, Fedora, Arch, …)

| Need | What Cliks does | What you might do once |
|------|-----------------|-------------------------|
| Spatial sound | Installer installs **mpv** | Nothing if install finished cleanly |
| Background capture | Tries session access + `input` group automatically | Sometimes: log out and back in **once** after group change |

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
cliks setup
cliks join CLIK-XXXXXX
```

Cliks still sends **only** activity kind + coarse timing — never key codes or text.

### Android / Termux (best-effort second device)

Termux is useful as a lightweight second-device client, but it is not a supported desktop capture target. Install the Termux:API app and then run:

```bash
pkg install termux-api mpv
cliks setup
```

Cliks uses `termux-media-player` when available, falls back to mpv, uses `termux-notification` for enabled quick signals, and uses `termux-clipboard-set` for one-click room-code copy.

Wayland and sandboxes (Flatpak) may block `/dev/input`. Prefer a normal desktop user session. Temporary local test:

```bash
cliks start --terminal --self
```

---

## Commands that keep setup easy

| Command | Purpose |
|---------|---------|
| `cliks setup` | One-time readiness: sound + capture, auto-fix what it can |
| `cliks sound-test` | Hear sample keyboard/mouse clicks |
| `cliks notification-test` | Send one OS-native test notification using your sound preference |
| `cliks doctor` | Detailed report (also under More → Diagnostics in the TUI) |
| `cliks capture-test` | Confirm activity is detected while you type/click |
| `cliks` | Friendly on-screen control panel |

## Public or self-hosted server

The default public relay is intentionally predictable for everyone: 20 people per room and 500 ms activity batches. These limits cannot be reduced or raised from a client pointed at the public backend.

To use your own relay, open `cliks` → More → Server and paste its `https://` address. The WebSocket address is filled automatically. You can also run:

```bash
cliks set api.url https://your-cliks-server
cliks set batch.ms 250
```

Type `public` in the Server field, or run `cliks set api.url public`, to restore the shared Cliks backend. Self-hosted operators can set `CLIKS_MAX_PEERS_PER_ROOM=40` (supported range 2–200). Larger rooms and shorter batches increase server load quickly.

You should **not** need to debug raw permissions by hand. Prefer `cliks setup` over copy-pasting system commands.

---

## Spatial sound quality

macOS and Windows releases include the stereo player and embedded WAV pack. No media player installation is required. Linux prefers **mpv** or **ffplay** for left/right placement and can fall back to PulseAudio, PipeWire, or ALSA players.

For Linux:

```bash
sudo apt install mpv   # or: sudo dnf install mpv / sudo pacman -S mpv
```

Then:

```bash
cliks setup
cliks sound-test
```

---

## Capture modes (advanced)

Most people only need `cliks join` / `cliks start` (auto mode).

| Mode | When |
|------|------|
| Auto (default) | Linux evdev when readable; macOS Event Tap; Windows low-level hooks |
| `--evdev` | Force Linux `/dev/input` capture |
| `--terminal --self` | Capture only this terminal; good for demos / locked-down machines |

---

## “It installed but…”

| Symptom | Try |
|---------|-----|
| `cliks: command not found` | Open a **new** terminal, or `export PATH="$HOME/.local/bin:$PATH"` |
| No sound | `cliks setup` then `cliks sound-test` |
| “Could not locate bundled sounds” | Update to Cliks 0.4.0 or newer; release binaries contain the WAV pack and macOS/Windows player |
| No signal notification | Enable Notifications in Preferences, then run `cliks notification-test`; Linux background sessions reconnect to the user D-Bus socket automatically |
| Teammates cannot hear you | `cliks capture-test` then `cliks setup` |
| macOS capture silent | Enable Cliks/your terminal under Input Monitoring, restart Cliks, then run `cliks capture-test` |
| Linux capture silent after install | Log out/in once if you were added to the `input` group |
| Terminal looks weird after a crash | `cliks fix-terminal` |
| Volume keys do nothing | Run `cliks sound-test`; on Linux, rerun `cliks setup` to install a supported player |
| Autostart broke after updating Cliks | `cliks setup` refreshes the launch path |

---

## Privacy reminder

Cliks never sends:

- actual keys or key codes  
- typed text  
- mouse coordinates  
- app or window names  
- microphone, camera, or screen content  

Only “keyboard activity” / “mouse click” plus coarse timing inside short batches.
