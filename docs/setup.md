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

That is usually enough. The installer:

- builds the `cliks` command
- installs **mpv** for stereo spatial sound when possible
- prepares background capture access on Linux
- opens Accessibility settings on macOS (one switch)
- runs `cliks setup` for a plain-language readiness check

---

## Per operating system

### macOS

| Need | What Cliks does | What you might do once |
|------|-----------------|-------------------------|
| Spatial sound | Installer adds **mpv** via Homebrew when possible; falls back to built-in `afplay` (distance only) | Nothing if install finished cleanly |
| Background capture | Uses system hooks | **One** permission: System Settings → Privacy & Security → **Accessibility** → enable your **Terminal** (or iTerm / Warp / VS Code) |

After granting Accessibility:

```bash
cliks setup
cliks join CLIK-XXXXXX
```

No key values are ever read or sent — only “keyboard activity” / “mouse click”.

### Windows

| Need | What Cliks does | What you might do once |
|------|-----------------|-------------------------|
| Spatial sound | Installer installs **mpv** via winget/scoop/choco when possible | Reopen the terminal if `mpv` is new on PATH |
| Background capture | Works for normal apps with **no** special Windows permission dialog | Nothing for everyday use |
| Settings location | Config lives under `%APPDATA%\cliks\` (native Windows path) | Older installs auto-migrate from `.config\cliks` |
| Launch at login | Silent VBScript startup (no console flash) | `cliks service enable` once |

**Note (not an error):** if you focus Task Manager or another *Administrator* window, Windows security may pause capture until you leave that window. Normal apps are fine.

Install from **Git Bash** (or another MSYS-style shell):

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
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
| `cliks doctor` | Detailed report (also under More → Diagnostics in the TUI) |
| `cliks capture-test` | Confirm activity is detected while you type/click |
| `cliks` | Friendly on-screen control panel |

You should **not** need to debug raw permissions by hand. Prefer `cliks setup` over copy-pasting system commands.

---

## Spatial sound quality

Best experience (stereo pan + distance):

- **mpv** (preferred on all platforms)
- or **ffplay** (FFmpeg)

If only a basic player is available (`afplay` on macOS, PowerShell on Windows, `paplay` on Linux), Cliks still plays sound with **distance/volume** cues. Install mpv when you want full left/right placement:

```bash
# macOS
brew install mpv

# Windows
winget install --id mpv.mpv -e

# Linux
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
| Auto (default) | Linux evdev when readable; macOS/Windows global hooks |
| `--evdev` | Force Linux `/dev/input` capture |
| `--terminal --self` | Capture only this terminal; good for demos / locked-down machines |

---

## “It installed but…”

| Symptom | Try |
|---------|-----|
| `cliks: command not found` | Open a **new** terminal, or `export PATH="$HOME/.local/bin:$PATH"` |
| No sound | `cliks setup` then `cliks sound-test` |
| Teammates cannot hear you | `cliks capture-test` then `cliks setup` |
| macOS capture silent | Accessibility for your terminal app, then restart Cliks |
| Linux capture silent after install | Log out/in once if you were added to the `input` group |
| Terminal looks weird after a crash | `cliks fix-terminal` |
| Volume keys do nothing | Install mpv (`cliks setup`) — fallbacks still scale gain where possible |
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
