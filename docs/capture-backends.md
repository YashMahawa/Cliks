# Capture Backends

Cliks should eventually use OS-native global input backends. One library is unlikely to cover every desktop perfectly.

## Current prototype

- `typ start --terminal --self`: reliable local test mode. It captures keyboard bytes and terminal mouse-report events from the active terminal only.
- `typ start`: tries `uiohook-napi` first. This can work on some Xorg/macOS/Windows setups, but it is not reliable enough as the final backend.

## Production direction

- Windows: low-level keyboard and mouse hooks through `SetWindowsHookEx`.
- macOS: Event Tap APIs with Accessibility permission prompts.
- Linux Xorg: XRecord/XInput or a maintained native hook.
- Linux Wayland: no normal app can universally observe global keyboard/mouse events by design. Practical options are:
  - a small privileged `evdev`/`libinput` helper with clear permission setup,
  - compositor-specific integrations,
  - future portal support if a standard global-input portal becomes available,
  - or terminal/app-specific capture as a limited mode.

The privacy promise remains the same regardless of backend: never send key values, coordinates, window names, or typed content. Only send event kind plus timing offsets.
