import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import WebSocket from "ws";

const port = Number(process.env.CLIKS_SMOKE_PORT ?? 18878);
const apiUrl = `http://127.0.0.1:${port}`;
const wsUrl = `ws://127.0.0.1:${port}/ws`;
const rootDir = dirname(dirname(fileURLToPath(import.meta.url)));
const serverBin = process.env.CLIKS_SERVER_BIN ?? firstExisting([
  join(rootDir, "server", "dist", process.platform === "win32" ? "cliks-server.exe" : "cliks-server"),
  join(rootDir, "server", "dist", "cliks-server")
]);
if (!existsSync(serverBin)) {
  throw new Error(`Server binary not found at ${serverBin}. Run: npm --workspace @cliks/server run build`);
}

const server = spawn(serverBin, [], {
  cwd: rootDir,
  env: { ...process.env, PORT: String(port) },
  stdio: ["ignore", "pipe", "pipe"]
});

let serverOutput = "";
server.stdout.on("data", (chunk) => {
  serverOutput += chunk.toString();
});
server.stderr.on("data", (chunk) => {
  serverOutput += chunk.toString();
});

try {
  await waitForHealth(apiUrl);

  const team = await createTeam(apiUrl);
  if (!/^CLIK-[A-Z2-9]{6}$/.test(team.code)) {
    throw new Error(`Unexpected team code shape: ${team.code}`);
  }

  const relay = await websocketRelaySmoke(wsUrl, team.code);
  if (relay.offsets !== "50,200") {
    throw new Error(`Expected quantized offsets 50,200, got ${relay.offsets}`);
  }
  if (relay.compactOffsets !== "50,200") {
    throw new Error(`Expected compact offsets 50,200, got ${relay.compactOffsets}`);
  }
  if (relay.nickname !== "Alice Long") {
    throw new Error(`Expected sanitized nickname Alice Long, got ${JSON.stringify(relay.nickname)}`);
  }
  const migrationTeam = await createTeam(apiUrl);
  await websocketRoomMigrationSmoke(wsUrl, team.code, migrationTeam.code);
  await websocketRoomLimitSmoke(wsUrl, team.code);

  const health = await fetchJson(`${apiUrl}/health`);
  if (!health.ok || "rooms" in health) {
    throw new Error(`Unsafe health response: ${JSON.stringify(health)}`);
  }

  await deleteTeam(apiUrl, team.code, "delete-me");
  const deletedLookup = await fetch(`${apiUrl}/api/teams/${team.code}`);
  if (deletedLookup.status !== 404) {
    throw new Error(`Deleted team lookup should return 404, got ${deletedLookup.status}`);
  }

  const liveDeleteTeam = await createTeam(apiUrl);
  await websocketDeleteSmoke(apiUrl, wsUrl, liveDeleteTeam.code);
  await deleteTeam(apiUrl, migrationTeam.code, "delete-me");
  await websocketJoinRateLimitSmoke(wsUrl);

  console.log(JSON.stringify({ ok: true, code: team.code, offsets: relay.offsets }));
} finally {
  server.kill("SIGTERM");
}

async function waitForHealth(baseUrl) {
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    try {
      const health = await fetchJson(`${baseUrl}/health`);
      if (health.ok === true) return;
    } catch {
      await sleep(150);
    }
  }
  throw new Error(`Server did not become healthy.\n${serverOutput}`);
}

async function createTeam(baseUrl) {
  const result = await fetchJson(`${baseUrl}/api/teams`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ name: "Smoke Team", deletePassword: "delete-me" })
  });
  return result.team;
}

async function deleteTeam(baseUrl, code, deletePassword) {
  await fetchJson(`${baseUrl}/api/teams/${code}`, {
    method: "DELETE",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ deletePassword })
  });
}

async function websocketDeleteSmoke(baseUrl, url, teamCode) {
  const socket = new WebSocket(url);
  let deleteMessageSeen = false;

  socket.on("message", (raw) => {
    const message = JSON.parse(raw.toString());
    if (message.type === "team_deleted" && message.message === "This team was deleted.") {
      deleteMessageSeen = true;
    }
  });

  await once(socket, "open");
  socket.send(JSON.stringify({ type: "join", teamCode, nickname: "deleteme" }));
  await sleep(250);

  const closed = onceWithTimeout(socket, "close", 1_500);
  await deleteTeam(baseUrl, teamCode, "delete-me");
  await closed;

  if (!deleteMessageSeen) {
    throw new Error("Deleted live room did not notify connected peer before closing");
  }
}

async function websocketRelaySmoke(url, teamCode) {
  const a = new WebSocket(url);
  const b = new WebSocket(url);
  const c = new WebSocket(url);
  const batches = [];
  const compactBatches = [];
  const presences = [];
  const reactions = [];

  await Promise.all([once(a, "open"), once(b, "open"), once(c, "open")]);
  b.on("message", (raw) => {
    const message = JSON.parse(raw.toString());
    if (message.type === "presence") presences.push(message);
    if (message.type === "peer_activity_batch") batches.push(message);
    if (message.type === "peer_reaction") reactions.push(message);
  });
  c.on("message", (raw) => {
    const message = JSON.parse(raw.toString());
    if (message.type === "a") compactBatches.push(message);
  });
  a.send(JSON.stringify({ type: "join", teamCode, nickname: "a" }));
  b.send(JSON.stringify({ type: "join", teamCode, nickname: "b" }));
  c.send(JSON.stringify({ type: "join", teamCode, nickname: "c", client: { name: "cliks", version: "test", features: ["compact-v1"] } }));

  await sleep(250);
  a.send(JSON.stringify({ type: "profile", nickname: "\u001b[31mAlice\u001b[0m\u001b]0;owned\u0007 Long Name" }));
  await sleep(250);
  a.send(
    JSON.stringify({
      type: "activity_batch",
      teamCode,
      batchStartedAt: Date.now(),
      events: [
        { kind: "keyboard", offsetMs: 73 },
        { kind: "mouse", button: "left", offsetMs: 188 }
      ]
    })
  );

  a.send(JSON.stringify({ type: "reaction", reaction: "wave" }));

  await sleep(300);
  a.close();
  b.close();
  c.close();
  await sleep(150);

  if (batches.length !== 1) {
    throw new Error(`Expected one relayed batch, got ${batches.length}`);
  }
  if (compactBatches.length !== 1) {
    throw new Error(`Expected one compact relayed batch, got ${compactBatches.length}`);
  }
  if (batches[0].nickname !== "Alice Long") {
    throw new Error(`Expected relayed activity nickname "Alice Long", got ${JSON.stringify(batches[0].nickname)}`);
  }
  if (reactions.length !== 1 || reactions[0].reaction !== "wave" || reactions[0].nickname !== "Alice Long") {
    throw new Error(`Expected one named room-wide wave, got ${JSON.stringify(reactions)}`);
  }
  const sawNamedPresence = presences.some((message) => {
    const names = new Set((message.peers ?? []).map((peer) => peer.nickname));
    return names.has("Alice Long") && names.has("b");
  });
  if (!sawNamedPresence) {
    throw new Error(`Expected named presence for both peers, got ${JSON.stringify(presences)}`);
  }

  return {
    offsets: batches[0].events.map((event) => event.offsetMs).join(","),
    compactOffsets: compactBatches[0].e.map((event) => event[1]).join(","),
    nickname: batches[0].nickname
  };
}

async function websocketRoomMigrationSmoke(url, firstTeamCode, secondTeamCode) {
  const mover = new WebSocket(url);
  const firstObserver = new WebSocket(url);
  const secondObserver = new WebSocket(url);
  const firstBatches = [];
  const secondBatches = [];

  try {
    await Promise.all([once(mover, "open"), once(firstObserver, "open"), once(secondObserver, "open")]);
    firstObserver.on("message", (raw) => {
      const message = JSON.parse(raw.toString());
      if (message.type === "peer_activity_batch") firstBatches.push(message);
    });
    secondObserver.on("message", (raw) => {
      const message = JSON.parse(raw.toString());
      if (message.type === "peer_activity_batch") secondBatches.push(message);
    });

    mover.send(JSON.stringify({ type: "join", teamCode: firstTeamCode, nickname: "mover" }));
    firstObserver.send(JSON.stringify({ type: "join", teamCode: firstTeamCode, nickname: "first" }));
    secondObserver.send(JSON.stringify({ type: "join", teamCode: secondTeamCode, nickname: "second" }));
    await sleep(250);

    mover.send(JSON.stringify({ type: "join", teamCode: secondTeamCode, nickname: "mover" }));
    await sleep(250);
    mover.send(JSON.stringify({
      type: "activity_batch",
      teamCode: secondTeamCode,
      batchStartedAt: Date.now(),
      events: [{ kind: "keyboard", offsetMs: 50 }]
    }));
    await sleep(250);

    if (firstBatches.length !== 0) {
      throw new Error("Room switch leaked activity to the previous room");
    }
    if (secondBatches.length !== 1) {
      throw new Error(`Room switch should relay once to the new room, got ${secondBatches.length}`);
    }
  } finally {
    mover.close();
    firstObserver.close();
    secondObserver.close();
    await sleep(100);
  }
}

async function websocketJoinRateLimitSmoke(url) {
  for (let attempt = 1; attempt <= 21; attempt++) {
    const socket = new WebSocket(url);
    await once(socket, "open");
    const response = nextJsonMessage(socket);
    const closed = onceWithTimeout(socket, "close", 1_500);
    socket.send(JSON.stringify({ type: "join", teamCode: `MISSING-${attempt}` }));
    const message = await response;
    await closed;
    if (attempt <= 20 && message.type !== "team_unavailable") {
      throw new Error(`Join attempt ${attempt} should be unavailable, got ${JSON.stringify(message)}`);
    }
    if (attempt === 21 && (message.type !== "error" || message.code !== "join_rate_limited")) {
      throw new Error(`Join limiter did not block attempt 21: ${JSON.stringify(message)}`);
    }
  }
}

async function websocketRoomLimitSmoke(url, teamCode) {
  const sockets = [];
  try {
    for (let index = 0; index < 20; index++) {
      const socket = new WebSocket(url);
      sockets.push(socket);
      await once(socket, "open");
      socket.send(JSON.stringify({ type: "join", teamCode, nickname: `p${index}` }));
      await sleep(20);
    }

    const overflow = new WebSocket(url);
    sockets.push(overflow);
    let fullMessageSeen = false;
    overflow.on("message", (raw) => {
      const message = JSON.parse(raw.toString());
      if (message.type === "error" && message.message.includes("room is full")) {
        fullMessageSeen = true;
      }
    });
    await once(overflow, "open");
    overflow.send(JSON.stringify({ type: "join", teamCode, nickname: "overflow" }));
    await onceWithTimeout(overflow, "close", 1_500);
    if (!fullMessageSeen) {
      throw new Error("21st room peer did not receive room-full error before close");
    }
  } finally {
    for (const socket of sockets) {
      try {
        socket.close();
      } catch {}
    }
    await sleep(100);
  }
}

async function fetchJson(url, options) {
  const response = await fetch(url, options);
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${response.status} ${url}: ${text}`);
  }
  return JSON.parse(text);
}

function once(emitter, event) {
  return new Promise((resolve, reject) => {
    emitter.once(event, resolve);
    emitter.once("error", reject);
  });
}

function onceWithTimeout(emitter, event, timeoutMs) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`Timed out waiting for ${event}`)), timeoutMs);
    emitter.once(event, (...args) => {
      clearTimeout(timer);
      resolve(args);
    });
    emitter.once("error", (error) => {
      clearTimeout(timer);
      reject(error);
    });
  });
}

function nextJsonMessage(socket) {
  return new Promise((resolve, reject) => {
    socket.once("message", (raw) => resolve(JSON.parse(raw.toString())));
    socket.once("error", reject);
  });
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function firstExisting(paths) {
  return paths.find((path) => existsSync(path)) ?? paths[0];
}
