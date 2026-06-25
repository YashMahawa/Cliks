import cors from "@fastify/cors";
import websocket from "@fastify/websocket";
import Fastify from "fastify";
import { customAlphabet } from "nanoid";
import type { WebSocket } from "ws";
import { z } from "zod";
import { RoomHub } from "./rooms.js";
import { createTeamStoreFromEnv } from "./store.js";

const port = Number(process.env.PORT ?? process.env.DO_APP_PORT ?? 8787);
const corsOriginSetting = process.env.CORS_ORIGIN;
const corsOrigin: any = corsOriginSetting
  ? (corsOriginSetting.includes(",") ? corsOriginSetting.split(",").map(o => o.trim()) : corsOriginSetting)
  : true;
const server = Fastify({ logger: true });
const store = createTeamStoreFromEnv();
const hub = new RoomHub(store);
const makePeerId = customAlphabet("abcdefghijklmnopqrstuvwxyz0123456789", 12);
const createTeamLimiter = createRateLimiter({
  windowMs: 5 * 60_000,
  maxRequests: 20
});
const deleteTeamLimiter = createRateLimiter({
  windowMs: 5 * 60_000,
  maxRequests: 30
});
const liveSockets = new Set<HeartbeatSocket>();
const heartbeatIntervalMs = 30_000;

type HeartbeatSocket = WebSocket & { isAlive?: boolean; peerId?: string };

const createTeamSchema = z.object({
  name: z.string().trim().min(2).max(80),
  deletePassword: z.string().min(6).max(128)
});

const deleteTeamSchema = z.object({
  code: z.string().trim().min(4).max(16),
  deletePassword: z.string().min(1).max(128)
});

const joinSchema = z.object({
  type: z.literal("join"),
  teamCode: z.string().trim().min(4).max(16),
  nickname: z
    .string()
    .trim()
    .max(32)
    .transform((value) => normalizeNickname(value))
    .optional(),
  client: z
    .object({
      name: z.string().optional(),
      version: z.string().optional()
    })
    .optional()
});

const activitySchema = z.object({
  type: z.literal("activity_batch"),
  teamCode: z.string().trim().min(4).max(16),
  batchStartedAt: z.number().int().nonnegative(),
  events: z
    .array(
      z.object({
        kind: z.enum(["keyboard", "mouse"]),
        offsetMs: z.number().int().min(0).max(2_000),
        button: z.enum(["left", "right", "middle", "unknown"]).optional()
      })
    )
    .max(128)
});

await server.register(cors, { origin: corsOrigin });
await server.register(websocket);

server.get("/health", async () => ({
  ok: true,
  ...hub.aggregateSnapshot()
}));

server.post("/api/teams", async (request, reply) => {
  if (!createTeamLimiter.allow(rateLimitKey(request))) {
    return reply.code(429).send({ error: "Too many team creation requests. Please wait a moment and try again." });
  }

  const parsed = createTeamSchema.safeParse(request.body);
  if (!parsed.success) {
    return reply.code(400).send({ error: "Please provide a team name and a delete password." });
  }

  const team = await store.createTeam(parsed.data);
  return reply.code(201).send({ team });
});

server.get("/api/teams/:code", async (request, reply) => {
  const code = String((request.params as { code: string }).code ?? "").toUpperCase();
  const team = await store.getTeamByCode(code);
  if (!team) return reply.code(404).send({ error: "Team not found" });
  return { team };
});

server.delete("/api/teams/:code", async (request, reply) => {
  if (!deleteTeamLimiter.allow(rateLimitKey(request))) {
    return reply.code(429).send({ error: "Too many delete attempts. Please wait a moment and try again." });
  }

  const parsed = deleteTeamSchema.safeParse({
    ...(request.body as object),
    code: (request.params as { code: string }).code
  });
  if (!parsed.success) return reply.code(400).send({ error: "Invalid delete request." });

  const deleted = await store.deleteTeam(parsed.data);
  if (!deleted) return reply.code(403).send({ error: "Could not delete that team." });
  hub.closeRoom(parsed.data.code, "This team was deleted.");
  return { ok: true };
});

server.get("/ws", { websocket: true }, (socket) => {
  const heartbeatSocket = socket as HeartbeatSocket;
  const peerId = `peer_${makePeerId()}`;
  heartbeatSocket.isAlive = true;
  heartbeatSocket.peerId = peerId;
  liveSockets.add(heartbeatSocket);
  let joinedCode: string | undefined;

  heartbeatSocket.on("pong", () => {
    heartbeatSocket.isAlive = true;
  });

  heartbeatSocket.on("message", async (raw) => {
    try {
      const json = JSON.parse(raw.toString());

      if (json.type === "join") {
        const parsed = joinSchema.parse(json);
        joinedCode = parsed.teamCode.toUpperCase();
        await hub.join({
          teamCode: joinedCode,
          nickname: parsed.nickname,
          socket: heartbeatSocket,
          peerId
        });
        return;
      }

      if (json.type === "activity_batch" && joinedCode) {
        const parsed = activitySchema.parse(json);
        hub.forwardActivity({
          peerId,
          teamCode: joinedCode,
          batchStartedAt: parsed.batchStartedAt,
          events: parsed.events
        });
        return;
      }

      heartbeatSocket.send(JSON.stringify({ type: "error", message: "Join a team before sending activity." }));
    } catch (error) {
      heartbeatSocket.send(
        JSON.stringify({
          type: "error",
          message: error instanceof Error ? error.message : "Invalid message."
        })
      );
    }
  });

  heartbeatSocket.on("close", () => {
    liveSockets.delete(heartbeatSocket);
    hub.leave(peerId);
  });
});

const heartbeatTimer = setInterval(() => {
  for (const socket of liveSockets) {
    if (socket.isAlive === false) {
      liveSockets.delete(socket);
      if (socket.peerId) hub.leave(socket.peerId);
      socket.terminate();
      continue;
    }

    socket.isAlive = false;
    socket.ping();
  }
}, heartbeatIntervalMs);
heartbeatTimer.unref();

server.listen({ port, host: "0.0.0.0" }).catch((error) => {
  server.log.error(error);
  process.exit(1);
});

function rateLimitKey(request: { headers: Record<string, string | string[] | undefined>; ip: string }) {
  const forwardedFor = request.headers["x-forwarded-for"];
  const firstForwarded = Array.isArray(forwardedFor) ? forwardedFor[0] : forwardedFor;
  return (firstForwarded?.split(",")[0]?.trim() || request.ip || "unknown").slice(0, 128);
}

function createRateLimiter(input: { windowMs: number; maxRequests: number }) {
  const hits = new Map<string, { count: number; resetAt: number }>();

  return {
    allow(key: string) {
      const now = Date.now();
      for (const [storedKey, value] of hits) {
        if (value.resetAt <= now) hits.delete(storedKey);
      }

      const current = hits.get(key);
      if (!current || current.resetAt <= now) {
        hits.set(key, { count: 1, resetAt: now + input.windowMs });
        return true;
      }

      current.count += 1;
      return current.count <= input.maxRequests;
    }
  };
}

function normalizeNickname(value: string) {
  const normalized = value.replace(/\s+/g, " ").trim();
  return normalized === "" ? undefined : normalized;
}
