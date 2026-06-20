# Cliks Protocol

Cliks uses tiny JSON messages over WebSocket.

## Client to server

### Join

```json
{
  "type": "join",
  "teamCode": "CLIK-842K",
  "nickname": "local optional name",
  "client": {
    "name": "typ",
    "version": "0.1.0"
  }
}
```

### Activity batch

The CLI sends one batch every `batchWindowMs`, currently 500ms. Each event keeps its millisecond offset from the first event in that batch.

```json
{
  "type": "activity_batch",
  "teamCode": "CLIK-842K",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "keyboard", "offsetMs": 72 },
    { "kind": "mouse", "button": "left", "offsetMs": 310 }
  ]
}
```

No key values, coordinates, windows, text, or app names are sent.

## Server to client

### Welcome

```json
{
  "type": "welcome",
  "peerId": "peer_abc123",
  "team": {
    "code": "CLIK-842K",
    "name": "Design Lab"
  },
  "activeCount": 4
}
```

### Presence

```json
{
  "type": "presence",
  "teamCode": "CLIK-842K",
  "activeCount": 4,
  "peers": [
    { "peerId": "peer_abc123", "nickname": "Mira" }
  ]
}
```

### Peer activity

```json
{
  "type": "peer_activity_batch",
  "teamCode": "CLIK-842K",
  "peerId": "peer_xyz987",
  "nickname": "Aarav",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "mouse", "button": "left", "offsetMs": 188 }
  ]
}
```
