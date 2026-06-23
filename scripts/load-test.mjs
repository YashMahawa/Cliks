import WebSocket from "ws";

const target = (process.env.CLIKS_LOAD_TARGET ?? "https://139.59.29.207.sslip.io").replace(/\/$/, "");
const rooms = numberFromEnv("CLIKS_LOAD_ROOMS", 2);
const peersPerRoom = numberFromEnv("CLIKS_LOAD_PEERS", 4);
const batchesPerPeer = numberFromEnv("CLIKS_LOAD_BATCHES", 8);
const batchIntervalMs = numberFromEnv("CLIKS_LOAD_INTERVAL_MS", 250);
const wsTarget = target.replace(/^http/, "ws") + "/ws";

const startedAt = Date.now();
const metrics = {
  rooms,
  peersPerRoom,
  sentBatches: 0,
  receivedBatches: 0,
  errors: 0
};
const latencies = [];

const teams = [];
for (let index = 0; index < rooms; index += 1) {
  teams.push(await createTeam(index));
}

const sockets = [];
try {
  for (const team of teams) {
    for (let peer = 0; peer < peersPerRoom; peer += 1) {
      sockets.push(await connectPeer(team.code, peer));
    }
  }

  for (let round = 0; round < batchesPerPeer; round += 1) {
    for (const socket of sockets) {
      if (socket.readyState !== WebSocket.OPEN) continue;
      const sentAt = Date.now();
      socket.send(
        JSON.stringify({
          type: "activity_batch",
          teamCode: socket.teamCode,
          batchStartedAt: sentAt,
          events: [
            { kind: "keyboard", offsetMs: 0 },
            { kind: "keyboard", offsetMs: 75 },
            { kind: "mouse", button: "left", offsetMs: 190 }
          ]
        })
      );
      metrics.sentBatches += 1;
    }
    await sleep(batchIntervalMs);
  }

  await sleep(1_000);
} finally {
  for (const socket of sockets) socket.close();
}

metrics.durationMs = Date.now() - startedAt;
metrics.latencyP50Ms = percentile(latencies, 50);
metrics.latencyP95Ms = percentile(latencies, 95);
metrics.target = target;

console.log(JSON.stringify(metrics, null, 2));

if (metrics.errors > 0) process.exitCode = 1;

async function createTeam(index) {
  const response = await fetch(`${target}/api/teams`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ name: `Load ${Date.now()} ${index}`, deletePassword: "delete-me" })
  });
  const text = await response.text();
  if (!response.ok) throw new Error(`create team failed: ${response.status} ${text}`);
  return JSON.parse(text).team;
}

async function connectPeer(teamCode, index) {
  const socket = new WebSocket(wsTarget);
  socket.teamCode = teamCode;
  socket.on("error", () => {
    metrics.errors += 1;
  });
  socket.on("message", (raw) => {
    const receivedAt = Date.now();
    const message = JSON.parse(raw.toString());
    if (message.type === "peer_activity_batch") {
      metrics.receivedBatches += 1;
      latencies.push(Math.max(0, receivedAt - message.batchStartedAt));
    }
    if (message.type === "error") metrics.errors += 1;
  });
  await once(socket, "open");
  socket.send(JSON.stringify({ type: "join", teamCode, nickname: `load-${index}` }));
  return socket;
}

function once(emitter, event) {
  return new Promise((resolve, reject) => {
    emitter.once(event, resolve);
    emitter.once("error", reject);
  });
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function numberFromEnv(name, fallback) {
  const value = Number(process.env[name]);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function percentile(values, p) {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length));
  return sorted[index];
}
