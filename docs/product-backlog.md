# Cliks Product Workboard

Last reviewed: 2026-07-18

This is the consolidated product backlog for requests made during the current
cross-platform redesign and for relevant Jules findings. It records outcomes,
not repeated wording, so the same request is not accidentally implemented two
different ways.

## Product north star

Cliks should feel like a small, delightful ambient coworking app that happens
to live in a terminal: safe input capture, one easy install, a readable spatial
room, useful lightweight social signals, private local soundscapes, and no
meeting-software clutter.

## Delivered foundation

- [x] Full-terminal home, onboarding, live spatial room, typing indicators,
  animated joins, overflow dots, saved teams, one-click code copy, direct
  toggles, keyboard help, and mouse hit regions derived from rendered output.
- [x] First-run welcome, normal launch animation, bundled UI sounds, factory
  reset, nickname and generated-name onboarding, notification setup, themes,
  backend selection, Keep Running, and launch-at-login setup.
- [x] Embedded keyboard/click samples and six embedded private room tones with
  source notices; built-in desktop playback on macOS and Windows.
- [x] Offline Solo Desk with simulated coworkers, keyboard/click controls, room
  tone, and no backend or capture dependency.
- [x] Room-wide allowlisted reactions, sender/message native notification
  content, reaction rate limiting, mute/focus/DND behavior, and no arbitrary
  message payloads.
- [x] Native/isolated capture architecture: dedicated macOS Capture.app,
  hardened Linux helper, native Windows hooks, and explicitly warned direct
  compatibility mode.
- [x] Background session ownership, attachable live view, no duplicate local
  WebSocket peers, saved team switching, and Linux/macOS/Windows autostart.
- [x] Public-backend 500 ms batching lock, easy self-hosted server override,
  configurable room cap for self-hosters, 48-hour inactive room expiry, and
  release archives for Linux, macOS, and Windows.

## Current quality pass

- [x] Make live and setup screens easier to read through stronger hierarchy,
  more whitespace, shorter copy, and responsive rails. A terminal program
  cannot change the emulator's font size, so Cliks must remain legible at the
  user's existing font setting.
- [x] Deploy the current relay to the public backend and prove, with two live
  clients, that a reaction reaches the recipient, animates over its sender in
  the spatial room, and triggers a sender/message notification when enabled.
- [x] Label quick signals with their visible 1-5 shortcuts in the live rail and
  shortcut guide.
- [x] Put private room-tone selection and room-tone volume directly in the live
  team rail. Keep them local and immediately audible without reconnecting.
- [x] Keep Solo Desk's room-tone level visible and make every +/- adjustment
  explicit. Allow the full 0-100% room-tone range.
- [x] Reverse live/Solo touchpad wheel volume adjustment to match natural
  scrolling while preserving normal list scrolling in menus.
- [x] Add intentional multicolor themes and live-preview a theme as keyboard or
  mouse focus moves during onboarding; persist only the confirmed choice.
- [ ] Verify the complete CLI/server/site test suite, then publish a native
  release so curl/PowerShell installers select the corrected artifacts.
- [x] Reflow onboarding as terminal cell dimensions change: spacious cards on
  large screens and progressively reduced art/copy at large terminal fonts.
- [x] Replace Solo's +/- volume clutter with hoverable/clickable slider tracks
  and responsive two-pane, stacked, and controls-first layouts.
- [x] Restore `CLIK-E842WU`, which the older relay failed to touch on live join
  before the 48-hour cleanup ran; the current relay refreshes successful joins.
- [x] Split Unix and Windows release packaging into clearly named jobs and
  upgrade workflow actions so expected platform branches do not resemble
  failed/skipped builds in GitHub's job UI.
- [x] Give full-height panels intentional vertical rhythm: expanded onboarding
  art at roomy sizes, primary controls above flexible space, and secondary
  guidance/status anchored to the bottom across setup, home/forms, Solo, live
  controls, diagnostics, and in-session navigation.
- [x] Make reaction delivery observable and update-safe: refresh stale
  background owners after binary installs, version session metadata, wait for
  the relay's self-echo before claiming a signal was shared, and surface a
  missing acknowledgement. Space the live listening rail into room-tone,
  alerts, and playback groups on tall terminals.
- [x] Verify Linux notification delivery rather than only checking for
  `notify-send`: require an active D-Bus notification provider, restart an
  enabled known provider during setup, and include the provider's real error
  when native delivery fails.
- [x] Remove redundant product-label text from reaction notification bodies;
  keep only the sender/emoji title and useful fixed reaction phrase.
- [x] Make team and Solo ownership exclusive: joining another team performs a
  stop-and-wait switch, and entering Solo disconnects the active team first.
- [x] Prevent private room tones from surviving Stop: gracefully handle
  background termination, use Linux parent-death signaling, and clean only
  verified Cliks ambient-cache player orphans left by older releases.

## Platform and security follow-up

- [x] Linux isolated helper: emit events only for the configured user's active
  local seat/session so another logged-in user's activity cannot contribute.
- [x] Linux isolated helper: replace the world-writable socket with a
  target-user-owned `0600` socket and validate the connecting Cliks executable
  in addition to `SO_PEERCRED` UID.
- [ ] Keep direct Linux input-group/ACL and direct macOS Terminal Input
  Monitoring as opt-in compatibility paths only. Never fall back to them
  automatically; explain how to revoke permission afterward.
- [x] Validate a team code with the selected backend before enabling autostart.
- [x] Report both launch-at-login installation and current service/session
  health instead of treating a launcher file as proof the process is healthy.
- [ ] List or validate selectable output devices where the active platform
  player exposes that information; fail visibly for an invalid configured
  device instead of silently losing audio.
- [ ] Reduce Linux click loss under sustained activity. Prefer an in-process or
  persistent mixer when it can retain CGO-free release portability; keep the
  current bounded queue and external-player fallback.

## Jules triage (last 48 hours)

- **Accepted:** Linux external-player startup can drop dense keyboard audio and
  spatial quality depends on mpv/ffplay. Address with a persistent/in-process
  path without breaking portable release builds.
- **Accepted:** autostart accepts unknown codes; validate before installation.
- **Accepted:** `audio.device` lacks discovery/validation; add capability-aware
  diagnostics and visible failures.
- **Accepted:** launcher presence is not runtime health; separate the two.
- **Accepted:** the Linux helper currently lacks active-seat filtering and its
  socket mode is broader than necessary; harden both boundaries.
- **Partially stale:** macOS/Linux direct permission risks are real, but current
  isolated mode does not silently fall back to direct capture. Preserve the
  warned compatibility option instead of removing recovery paths.
- **Rejected by privacy design:** global reaction hotkeys through the capture
  helper. The helper must continue emitting only keyboard/mouse activity-kind
  tokens; social commands remain deliberate Cliks UI/CLI actions.

## Release acceptance

- Ubuntu/Fedora/Arch packages install the isolated helper and audio dependency
  cleanly; Xorg and Wayland both work through the helper.
- macOS Intel/Apple Silicon archives include Capture.app and embedded audio;
  unsigned-open-source Gatekeeper instructions and direct compatibility mode
  are clear.
- Windows x64 includes native hooks, notifications, embedded audio, background
  operation, and login launch without a separate media player.
- Public reaction smoke test, notification tests, capture tests, sound tests,
  CI, release checksums, and installer latest-version resolution all pass.
