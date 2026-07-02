import WebSocket from "ws";

const target = (process.env.CLIKS_LOAD_TARGET ?? "http://127.0.0.1:8787").replace(/\/$/, "");
const rooms = numberFromEnv("CLIKS_LOAD_ROOMS", 2);
const peersPerRoom = numberFromEnv("CLIKS_LOAD_PEERS", 4);
const batchesPerPeer = numberFromEnv("CLIKS_LOAD_BATCHES", 8);
const batchIntervalMs = numberFromEnv("CLIKS_LOAD_INTERVAL_MS", 250);
const deliveryTimeoutMs = numberFromEnv("CLIKS_LOAD_DELIVERY_TIMEOUT_MS", 3_000);
const deletePassword = "delete-me";
const wsTarget = target.replace(/^http/, "ws") + "/ws";

const startedAt = Date.now();
const metrics = {
  rooms,
  peersPerRoom,
  sentBatches: 0,
  receivedBatches: 0,
  expectedReceivedBatches: 0,
  cleanupErrors: 0,
  errors: 0
};
const latencies = [];

const teams = [];
const sockets = [];
try {
  for (let index = 0; index < rooms; index += 1) {
    teams.push(await createTeam(index));
  }

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

  metrics.expectedReceivedBatches = metrics.sentBatches * Math.max(0, peersPerRoom - 1);
  await waitForDeliveries(metrics.expectedReceivedBatches, deliveryTimeoutMs);
  if (metrics.receivedBatches !== metrics.expectedReceivedBatches) metrics.errors += 1;
} finally {
  for (const socket of sockets) socket.close();
  await Promise.all(
    teams.map(async (team) => {
      try {
        await deleteTeam(team.code);
      } catch (error) {
        metrics.cleanupErrors += 1;
        metrics.errors += 1;
        console.error(error.message);
      }
    })
  );
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
    body: JSON.stringify({ name: `Load ${Date.now()} ${index}`, deletePassword })
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
  const welcomed = onceMessageType(socket, "welcome");
  socket.send(JSON.stringify({ type: "join", teamCode, nickname: `load-${index}` }));
  await welcomed;
  return socket;
}

async function deleteTeam(teamCode) {
  const response = await fetch(`${target}/api/teams/${encodeURIComponent(teamCode)}`, {
    method: "DELETE",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ deletePassword })
  });
  const text = await response.text();
  if (!response.ok) throw new Error(`delete team ${teamCode} failed: ${response.status} ${text}`);
}

function once(emitter, event) {
  return new Promise((resolve, reject) => {
    emitter.once(event, resolve);
    emitter.once("error", reject);
  });
}

function onceMessageType(socket, type) {
  return new Promise((resolve, reject) => {
    const onMessage = (raw) => {
      const message = JSON.parse(raw.toString());
      if (message.type !== type) return;
      cleanup();
      resolve(message);
    };
    const onError = (error) => {
      cleanup();
      reject(error);
    };
    const onClose = () => {
      cleanup();
      reject(new Error(`socket closed before ${type}`));
    };
    const cleanup = () => {
      socket.off("message", onMessage);
      socket.off("error", onError);
      socket.off("close", onClose);
    };
    socket.on("message", onMessage);
    socket.once("error", onError);
    socket.once("close", onClose);
  });
}

async function waitForDeliveries(expected, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (metrics.receivedBatches < expected && Date.now() < deadline) {
    await sleep(25);
  }
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
