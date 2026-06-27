# Capture Backends

Cliks should eventually use OS-native global input backends. One library is unlikely to cover every desktop perfectly.

## Current prototype

- `cliks start --terminal --self`: reliable local test mode. It captures keyboard bytes and terminal mouse-report events from the active terminal only.
- `cliks start`: on Linux tries `/dev/input` evdev capture first. macOS and Windows native global capture hooks are still future work in the Go CLI.
- `cliks start --evdev`: Linux-only global capture through readable `/dev/input/event*` devices.

Terminal mode captures and restores the original terminal state and disables mouse reporting on close, error, process exit, and top-level CLI failures. If a terminal is already in a bad state, run `cliks fix-terminal`.

Linux evdev mouse capture is deliberately narrow:

- physical `BTN_LEFT` and `BTN_RIGHT` presses emit left/right mouse activity
- short stationary one-finger touchpad tap emits left click
- short stationary two-finger touchpad tap emits right click
- long press, cursor movement, scroll/wheel, side buttons, three-or-more-finger gestures, and pointer coordinates are ignored
- if a touch gesture also produces a physical button event, the tap heuristic suppresses its duplicate event

This keeps touchpad click users working without turning ordinary cursor movement or gestures into coworking noise.

Readable evdev files are consumed with interruptible exponential retry delays from 1 second up to 30 seconds after non-EOF read errors. This prevents a disconnected or permission-changing device from driving a 100% CPU busy loop, while allowing transient failures to recover. Capture and session handoff use bounded 1024-event channels with cancellation-aware backpressure, so ordinary fast typing/click bursts are not silently dropped and shutdown cannot deadlock behind a full channel.

## Production direction

- Windows: low-level keyboard and mouse hooks through `SetWindowsHookEx`.
- macOS: Event Tap APIs with Accessibility permission prompts.
- Linux Xorg: XRecord/XInput or a maintained native hook.
- Linux Wayland: no normal app can universally observe global keyboard/mouse events by design. Practical options are:
  - a small privileged `evdev`/`libinput` helper with clear permission setup,
  - compositor-specific integrations,
  - future portal support if a standard global-input portal becomes available,
  - or terminal/app-specific capture as a limited mode.

The privacy promise remains the same regardless of backend: never send key values, coordinates, window names, or typed content. Only send event kind plus coarse timing offsets.
