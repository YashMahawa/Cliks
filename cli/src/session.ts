import WebSocket from "ws";
import { ActivityBatcher, ActivityCapture } from "./activity.js";
import { AudioEngine } from "./audio.js";
import type { CliksConfig } from "./config.js";

const quips = [
  "Remote office volume: cozy focus.",
  "Someone has entered decisive typing mode.",
  "The room is awake.",
  "Tiny taps, big momentum.",
  "Focus is doing its little drum solo."
];

export async function startSession(
  config: CliksConfig,
  options: { captureMode?: "native" | "terminal" | "auto"; selfMonitor?: boolean } = {}
) {
  const teamCode = config.currentTeamCode;
  if (!teamCode) {
    throw new Error("No team selected. Run: typ join CLIK-XXXX");
  }

  const ws = new WebSocket(config.wsUrl);
  const capture = new ActivityCapture();
  const batcher = new ActivityBatcher(config.batchWindowMs);
  const listening = {
    ...config.listening,
    self: options.selfMonitor ?? config.listening.self
  };
  const audio = new AudioEngine(listening);

  let activeCount = 1;
  let teamName = teamCode;
  let captureMode = "starting";

  ws.on("open", () => {
    ws.send(
      JSON.stringify({
        type: "join",
        teamCode,
        nickname: config.nickname,
        client: { name: "typ", version: "0.1.0" }
      })
    );
  });

  ws.on("message", (raw) => {
    const message = JSON.parse(raw.toString());
    if (message.type === "welcome") {
      activeCount = message.activeCount;
      teamName = message.team?.name ?? teamCode;
      renderStatus(teamName, activeCount, listening.self, captureMode);
    }
    if (message.type === "presence") {
      activeCount = message.activeCount;
      renderStatus(teamName, activeCount, listening.self, captureMode);
    }
    if (message.type === "peer_activity_batch") {
      audio.scheduleBatch(message.peerId, message.events);
    }
    if (message.type === "error") {
      console.error(`\nCliks server: ${message.message}`);
    }
  });

  ws.on("close", () => {
    console.log("\nDisconnected from Cliks.");
    process.exit(0);
  });

  ws.on("error", (error) => {
    console.error(`\nCould not connect to Cliks: ${error.message}`);
    process.exit(1);
  });

  capture.on("activity", (event) => batcher.push(event));
  batcher.on("batch", (batch) => {
    if (ws.readyState !== WebSocket.OPEN) return;
    ws.send(
      JSON.stringify({
        type: "activity_batch",
        teamCode,
        batchStartedAt: batch.batchStartedAt,
        events: batch.events
      })
    );
    if (listening.self) {
      audio.scheduleBatch("self", batch.events);
    }
  });

  const captureState = await capture.start({ ...config.sharing, mode: options.captureMode ?? "auto" });
  captureMode = captureState.mode;
  renderStatus(teamName, activeCount, listening.self, captureMode);

  const quipTimer = setInterval(() => {
    process.stdout.write(`\n${quips[Math.floor(Math.random() * quips.length)]}\n`);
    renderStatus(teamName, activeCount, listening.self, captureMode);
  }, 18_000);

  process.on("SIGINT", () => {
    clearInterval(quipTimer);
    batcher.flush();
    capture.stop();
    ws.close();
  });
}

function renderStatus(
  teamName: string,
  activeCount: number,
  hearingSelf: boolean | undefined,
  captureMode: string
) {
  process.stdout.write("\x1Bc");
  console.log("Cliks");
  console.log("");
  console.log(`Team: ${teamName}`);
  console.log(`Active now: ${activeCount}`);
  console.log("");
  console.log("Sharing exact activity pulses in 500ms batches.");
  console.log(`Self monitor: ${hearingSelf ? "on for local testing" : "off"}`);
  console.log(`Capture: ${captureMode}`);
  console.log("Press Ctrl+C to stop.");
}
