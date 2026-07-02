# Cliks Protocol

Cliks uses tiny JSON messages over WebSocket.

Incoming WebSocket messages are capped at 8 KiB and each connection has a local message-rate guard. The 8 KiB cap leaves room for a full 128-event verbose activity batch while rejecting oversized frames before JSON processing. Plain HTTP team-code lookups and failed WebSocket joins are rate-limited per source IP to reduce code-scanning risk. Outgoing frames are serialized by one writer per connection through a bounded 32-frame queue; a full queue or 5-second write timeout closes only the slow connection.

## Client to server

### Join

```json
{
  "type": "join",
  "teamCode": "CLIK-842KQ9",
  "nickname": "local optional name",
  "client": {
    "name": "cliks",
    "version": "0.2.1",
    "features": ["compact-v1"]
  }
}
```

`nickname` is an explicit optional display name, capped at 10 Unicode characters by clients and the relay. ANSI escape sequences, control characters, and Unicode formatting controls are stripped before whitespace normalization and truncation. Empty or whitespace-only names are treated as anonymous. Clients must not infer a name from typed text, OS users, hostnames, app names, or window titles. `features` is optional; new CLIs send `compact-v1` to receive compact peer-activity frames.

A WebSocket connection has exactly one current room. Sending another valid `join` migrates that connection to the new room and emits updated presence to both rooms; activity is routed only to the new room. Failed joins are limited per source IP. After 20 failed attempts in five minutes, the relay sends the following error and closes the socket so the client reconnect loop backs off:

```json
{
  "type": "error",
  "code": "join_rate_limited",
  "message": "Too many team join attempts. Wait a few minutes and try again."
}
```

Valid joins do not consume the failed-join budget.

If a connected client sends too many WebSocket messages in a short window, the relay sends:

```json
{
  "type": "error",
  "code": "message_rate_limited",
  "message": "Too many WebSocket messages. Slow down and reconnect."
}
```

The socket is then closed.

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

### Profile update

Used after join when a running CLI notices the local nickname changed.

```json
{
  "type": "profile",
  "nickname": "Mira"
}
```

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

When a recipient negotiated `compact-v1`, the relay sends the same peer activity as:

```json
{
  "type": "a",
  "p": "peer_xyz987",
  "n": "Aarav",
  "t": 1780000000000,
  "e": [
    ["k", 0],
    ["m", 200, "l"]
  ]
}
```

Compact event kind `k` means keyboard and `m` means mouse. Compact mouse buttons use `l` for left, `r` for right, and `u` for unknown.

## Connection health

The relay sends WebSocket pings every 10 seconds. A connection is marked pending before a ping and is removed if it has not answered by the next heartbeat, so stale presence is bounded to roughly 20 seconds even when an underlying TCP failure is not reported immediately. Both sides also use a rolling 75-second read deadline that is extended by traffic and pong responses. The CLI sends its own pings, links socket closure to session cancellation, and reconnects when heartbeat responses time out. Reconnect delays use exponential backoff with bounded jitter and a 30-second cap so many clients do not retry in lockstep after an outage.

## Team deletion

When a team is deleted successfully, the relay closes any live room for that code. Join and delete operations for the same team are serialized through a per-team lifecycle gate, so a join cannot publish stale team data after deletion. Connected peers receive:

```json
{ "type": "team_deleted", "teamCode": "CLIK-842KQ9", "message": "This team was deleted." }
```

The socket is then closed and future lookups for that team code return 404. If a client tries to join a missing or deleted code, the relay sends:

```json
{
  "type": "team_unavailable",
  "teamCode": "CLIK-842KQ9",
  "reason": "not_found",
  "message": "Team code was not found or was deleted."
}
```

The CLI should remove that team from local config, disable launch-at-login, stop the current session, and avoid reconnecting to that code. Generic server/store errors are retryable and must not remove the saved team.

## Room limits

Rooms are capped at 20 live peers. The 21st peer receives an error with code `room_full` and the socket closes.
