# Cliks Architecture

## Core promise

Cliks should feel live and human without becoming surveillance.

The CLI records only activity shape:

- `keyboard`
- `mouse`
- mouse button when available
- millisecond interval offsets inside a 500ms batch

It must not send:

- actual key values
- key codes
- words
- mouse coordinates
- active app or window title
- screen or microphone data

## Batching

The CLI batches for 500ms by default. This keeps Render/WebSocket load lower while preserving the timing feel.

Example:

```json
{
  "type": "activity_batch",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "keyboard", "offsetMs": 93 },
    { "kind": "mouse", "button": "left", "offsetMs": 288 },
    { "kind": "keyboard", "offsetMs": 491 }
  ]
}
```

The receiving CLI schedules local sound playback using those offsets. That means the server sees only tiny JSON pulses, while the receiver still hears a natural rhythm.

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

Good next optimizations:

- per-room rate limits
- adaptive batch window for large rooms
- binary WebSocket frames after the JSON prototype
- Redis presence if the backend scales beyond one Render instance
- static sound pack files instead of generated temp WAVs

## Free-tier expectation

Vercel should stay mostly idle because it serves a static team-creation page.

Supabase load is tiny because it stores team code records only.

Render is the bottleneck because it keeps WebSockets open and fans out activity batches. A $200 DigitalOcean credit runway would be useful for an always-on backend once demos move beyond a small beta.
