# Cliks Protocol

Cliks uses tiny JSON messages over WebSocket.

## Client to server

### Join

```json
{
  "type": "join",
  "teamCode": "CLIK-842KQ9",
  "nickname": "local optional name",
  "client": {
    "name": "cliks",
    "version": "0.2.0"
  }
}
```

`nickname` is an explicit optional display name. Empty or whitespace-only names are treated as anonymous. Clients must not infer a name from typed text, OS users, hostnames, app names, or window titles.

### Activity batch

The CLI sends one batch every `batchWindowMs`, currently 500ms. Local events include offsets from the first event in that batch. Before relaying to teammates, the server rounds offsets into 50ms buckets.

```json
{
  "type": "activity_batch",
  "teamCode": "CLIK-842KQ9",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "keyboard", "offsetMs": 72 },
    { "kind": "mouse", "button": "left", "offsetMs": 310 }
  ]
}
```

No key values, coordinates, windows, text, or app names are sent.
Raw client-side offsets are not forwarded as-is.

## Server to client

### Welcome

```json
{
  "type": "welcome",
  "peerId": "peer_abc123",
  "team": {
    "code": "CLIK-842KQ9",
    "name": "Design Lab"
  },
  "activeCount": 4
}
```

### Presence

```json
{
  "type": "presence",
  "teamCode": "CLIK-842KQ9",
  "activeCount": 4,
  "peers": [
    { "peerId": "peer_abc123", "nickname": "Mira", "joinedAt": 1780000000000 }
  ]
}
```

### Peer activity

```json
{
  "type": "peer_activity_batch",
  "teamCode": "CLIK-842KQ9",
  "peerId": "peer_xyz987",
  "nickname": "Aarav",
  "batchStartedAt": 1780000000000,
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "mouse", "button": "left", "offsetMs": 200 }
  ]
}
```

## Connection health

The relay sends WebSocket pings and removes peers that miss heartbeats. The CLI also sends pings and reconnects when heartbeat responses time out.

## Team deletion

When a team is deleted successfully, the relay closes any live room for that code. Connected peers receive:

```json
{ "type": "error", "message": "This team was deleted." }
```

The socket is then closed and future lookups for that team code return 404.
