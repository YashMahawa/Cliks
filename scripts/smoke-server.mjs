import { spawn } from "node:child_process";
import WebSocket from "ws";

const port = Number(process.env.CLIKS_SMOKE_PORT ?? 18878);
const apiUrl = `http://127.0.0.1:${port}`;
const wsUrl = `ws://127.0.0.1:${port}/ws`;

const server = spawn(process.execPath, ["server/dist/index.js"], {
  cwd: new URL("..", import.meta.url),
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
    if (message.type === "error" && message.message === "This team was deleted.") {
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
  const batches = [];

  await Promise.all([once(a, "open"), once(b, "open")]);
  a.send(JSON.stringify({ type: "join", teamCode, nickname: "a" }));
  b.send(JSON.stringify({ type: "join", teamCode, nickname: "b" }));

  b.on("message", (raw) => {
    const message = JSON.parse(raw.toString());
    if (message.type === "peer_activity_batch") batches.push(message);
  });

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

  await sleep(300);
  a.close();
  b.close();

  if (batches.length !== 1) {
    throw new Error(`Expected one relayed batch, got ${batches.length}`);
  }

  return {
    offsets: batches[0].events.map((event) => event.offsetMs).join(",")
  };
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

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
