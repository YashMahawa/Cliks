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
- installs privacy-isolated background capture on Linux
- installs Cliks Capture.app and opens Input Monitoring for that app only
- runs `cliks setup` for a plain-language readiness check

The first `cliks` launch then uses one full-screen card at a time. It can generate a funny nickname, open the correct permission screen, test notifications, and remember whether you want background and launch-at-login behavior. Those launchers are user-level and do not need administrator permission.

---

## Per operating system

### macOS

| Need | What Cliks does | What you might do once |
|------|-----------------|-------------------------|
| Spatial sound | Built into the Cliks binary (stereo pan + distance) | Nothing |
| Background capture | Dedicated listen-only Cliks Capture.app | **One** permission: Input Monitoring → enable **Cliks Capture** only. Keep terminal apps disabled. |
| Launch at login | User LaunchAgent, refreshed automatically after upgrades | Choose it during first setup or run `cliks service enable` |

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
| Background capture | Minimal root-owned helper works across Xorg and Wayland | Installer asks for sudo once; your user is not placed in `input` |
| Launch at login | systemd user service with one-session locking | Choose it during first setup or run `cliks service enable` |

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
| `cliks solo` | Offline simulated desk; no team, capture, server, or internet |

## Solo Desk and personal room tones

Run `cliks solo` to open a local spatial room with 1-12 simulated coworkers. Each simulated coworker types in short bursts with quiet gaps and occasional clicks. Keyboard ambience, click ambience, and the embedded room tone have separate slider tracks. Hover a slider and use arrow keys or natural scrolling, click its track to jump to a level, or press Tab to cycle sliders without a mouse. Choose rain, fireside, coffee house, cloud drift, contemplation, or night drive, and set room-tone volume anywhere up to 100%. Resizing the terminal—or changing terminal font size—automatically switches between two-pane, stacked, and controls-first layouts. Nothing from Solo Desk is captured or sent. The same private room tone and its volume are directly adjustable in a live team room, from Preferences, or with `cliks set ambient rain ambient.volume 0.7`. For scripts, set both Solo levels together with `cliks set solo.keyboardVolume 0.7 solo.mouseVolume 0.8`.

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

Most people only need `cliks join` / `cliks start` (isolated mode).

| Mode | When |
|------|------|
| Isolated (default) | Linux helper; macOS Cliks Capture.app; Windows native hooks |
| `cliks set capture.mode terminal` | Focused terminal only; no global permission |
| `cliks set capture.mode direct` | Compatibility fallback with broader Linux user/macOS terminal permission |
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
| macOS capture silent | Enable Cliks Capture under Input Monitoring. For unsigned builds, approve Open Anyway once. |
| Linux capture silent after install | Run `systemctl status cliks-capture`, then rerun the installer or `cliks setup` |
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
