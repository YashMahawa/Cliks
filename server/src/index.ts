import cors from "@fastify/cors";
import websocket from "@fastify/websocket";
import Fastify from "fastify";
import { customAlphabet } from "nanoid";
import { z } from "zod";
import { RoomHub } from "./rooms.js";
import { createTeamStoreFromEnv } from "./store.js";

const port = Number(process.env.PORT ?? 8787);
const corsOrigin = process.env.CORS_ORIGIN ?? true;
const server = Fastify({ logger: true });
const store = createTeamStoreFromEnv();
const hub = new RoomHub(store);
const makePeerId = customAlphabet("abcdefghijklmnopqrstuvwxyz0123456789", 12);

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
  nickname: z.string().trim().max(32).optional(),
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
  rooms: hub.snapshot()
}));

server.post("/api/teams", async (request, reply) => {
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
  const parsed = deleteTeamSchema.safeParse({
    ...(request.body as object),
    code: (request.params as { code: string }).code
  });
  if (!parsed.success) return reply.code(400).send({ error: "Invalid delete request." });

  const deleted = await store.deleteTeam(parsed.data);
  if (!deleted) return reply.code(403).send({ error: "Could not delete that team." });
  return { ok: true };
});

server.get("/ws", { websocket: true }, (socket) => {
  const peerId = `peer_${makePeerId()}`;
  let joinedCode: string | undefined;

  socket.on("message", async (raw) => {
    try {
      const json = JSON.parse(raw.toString());

      if (json.type === "join") {
        const parsed = joinSchema.parse(json);
        joinedCode = parsed.teamCode.toUpperCase();
        await hub.join({
          teamCode: joinedCode,
          nickname: parsed.nickname,
          socket,
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

      socket.send(JSON.stringify({ type: "error", message: "Join a team before sending activity." }));
    } catch (error) {
      socket.send(
        JSON.stringify({
          type: "error",
          message: error instanceof Error ? error.message : "Invalid message."
        })
      );
    }
  });

  socket.on("close", () => {
    hub.leave(peerId);
  });
});

server.listen({ port, host: "0.0.0.0" }).catch((error) => {
  server.log.error(error);
  process.exit(1);
});
