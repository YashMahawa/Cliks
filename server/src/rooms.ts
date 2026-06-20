import type { WebSocket } from "ws";
import type { Team, TeamStore } from "./store.js";

export type ActivityEvent = {
  kind: "keyboard" | "mouse";
  offsetMs: number;
  button?: "left" | "right" | "middle" | "unknown";
};

type Peer = {
  id: string;
  nickname?: string;
  socket: WebSocket;
  team: Team;
  joinedAt: number;
  lastSeenAt: number;
};

type Room = {
  team: Team;
  peers: Map<string, Peer>;
};

export class RoomHub {
  private rooms = new Map<string, Room>();

  constructor(private store: TeamStore) {}

  async join(input: {
    teamCode: string;
    nickname?: string;
    socket: WebSocket;
    peerId: string;
  }) {
    const team = await this.store.getTeamByCode(input.teamCode);
    if (!team) {
      input.socket.send(JSON.stringify({ type: "error", message: "Team code was not found." }));
      input.socket.close();
      return;
    }

    const room = this.getOrCreateRoom(team);
    const peer: Peer = {
      id: input.peerId,
      nickname: input.nickname?.slice(0, 32),
      socket: input.socket,
      team,
      joinedAt: Date.now(),
      lastSeenAt: Date.now()
    };

    room.peers.set(peer.id, peer);
    input.socket.send(
      JSON.stringify({
        type: "welcome",
        peerId: peer.id,
        team,
        activeCount: room.peers.size
      })
    );
    this.broadcastPresence(room);
  }

  leave(peerId: string) {
    for (const [teamCode, room] of this.rooms) {
      const peer = room.peers.get(peerId);
      if (!peer) continue;

      room.peers.delete(peerId);
      if (room.peers.size === 0) {
        this.rooms.delete(teamCode);
      } else {
        this.broadcastPresence(room);
      }
      return;
    }
  }

  forwardActivity(input: {
    peerId: string;
    teamCode: string;
    batchStartedAt: number;
    events: ActivityEvent[];
  }) {
    const room = this.rooms.get(input.teamCode.toUpperCase());
    if (!room) return;
    const sender = room.peers.get(input.peerId);
    if (!sender) return;
    sender.lastSeenAt = Date.now();

    const sanitizedEvents = input.events
      .slice(0, 128)
      .map((event) => ({
        kind: event.kind,
        offsetMs: clamp(Math.round(event.offsetMs), 0, 2_000),
        ...(event.kind === "mouse" ? { button: event.button ?? "unknown" } : {})
      }))
      .filter((event) => event.kind === "keyboard" || event.kind === "mouse");

    if (sanitizedEvents.length === 0) return;

    const payload = JSON.stringify({
      type: "peer_activity_batch",
      teamCode: room.team.code,
      peerId: sender.id,
      nickname: sender.nickname,
      batchStartedAt: input.batchStartedAt,
      events: sanitizedEvents
    });

    for (const peer of room.peers.values()) {
      if (peer.id === sender.id || peer.socket.readyState !== 1) continue;
      peer.socket.send(payload);
    }
  }

  snapshot() {
    return [...this.rooms.values()].map((room) => ({
      code: room.team.code,
      name: room.team.name,
      activeCount: room.peers.size
    }));
  }

  private getOrCreateRoom(team: Team) {
    const existing = this.rooms.get(team.code);
    if (existing) return existing;
    const room = { team, peers: new Map<string, Peer>() };
    this.rooms.set(team.code, room);
    return room;
  }

  private broadcastPresence(room: Room) {
    const payload = JSON.stringify({
      type: "presence",
      teamCode: room.team.code,
      activeCount: room.peers.size,
      peers: [...room.peers.values()].map((peer) => ({
        peerId: peer.id,
        nickname: peer.nickname,
        joinedAt: peer.joinedAt
      }))
    });

    for (const peer of room.peers.values()) {
      if (peer.socket.readyState === 1) peer.socket.send(payload);
    }
  }
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}
