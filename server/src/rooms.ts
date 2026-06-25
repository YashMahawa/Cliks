import type { WebSocket } from "ws";
import type { Team, TeamStore } from "./store.js";

export type ActivityEvent = {
  kind: "keyboard" | "mouse";
  offsetMs: number;
  button?: "left" | "right" | "middle" | "unknown";
};

const timingBucketMs = 50;
const maxPeersPerRoom = 20;

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
    if (room.peers.size >= maxPeersPerRoom) {
      input.socket.send(JSON.stringify({ type: "error", message: "This room is full. Cliks rooms are capped at 20 people." }));
      input.socket.close();
      return;
    }
    const peer: Peer = {
      id: input.peerId,
      nickname: normalizeNickname(input.nickname),
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

  updatePeerProfile(peerId: string, nickname: string | undefined) {
    for (const room of this.rooms.values()) {
      const peer = room.peers.get(peerId);
      if (!peer) continue;
      peer.nickname = normalizeNickname(nickname);
      peer.lastSeenAt = Date.now();
      this.broadcastPresence(room);
      return;
    }
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

  closeRoom(teamCode: string, message: string) {
    const room = this.rooms.get(teamCode.toUpperCase());
    if (!room) return;

    const payload = JSON.stringify({ type: "error", message });
    for (const peer of room.peers.values()) {
      if (peer.socket.readyState !== 1) continue;
      peer.socket.send(payload);
      peer.socket.close();
    }
    this.rooms.delete(room.team.code);
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
        offsetMs: quantizeOffset(event.offsetMs),
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

  aggregateSnapshot() {
    let totalPeers = 0;
    for (const room of this.rooms.values()) {
      totalPeers += room.peers.size;
    }

    return {
      totalRooms: this.rooms.size,
      totalPeers
    };
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

function quantizeOffset(offsetMs: number) {
  return clamp(Math.round(offsetMs / timingBucketMs) * timingBucketMs, 0, 2_000);
}

function normalizeNickname(value: string | undefined) {
  const normalized = value?.replace(/\s+/g, " ").trim().slice(0, 32);
  return normalized || undefined;
}
