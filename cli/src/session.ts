import WebSocket from "ws";
import { ActivityBatcher, ActivityCapture, type CaptureMode } from "./activity.js";
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
  options: { captureMode?: CaptureMode; selfMonitor?: boolean } = {}
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
  let permissionHint: string | undefined;
  let ownPeerId: string | undefined;
  let quipTimer: NodeJS.Timeout | undefined;
  let cleanedUp = false;
  let localCapturedEvents = 0;
  let localSentEvents = 0;

  const cleanup = () => {
    if (cleanedUp) return;
    cleanedUp = true;
    if (quipTimer) clearInterval(quipTimer);
    batcher.flush();
    capture.stop();
  };

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
      ownPeerId = message.peerId;
      activeCount = message.activeCount;
      teamName = message.team?.name ?? teamCode;
      renderStatus(teamName, activeCount, listening.self, captureMode, localCapturedEvents, localSentEvents, permissionHint);
    }
    if (message.type === "presence") {
      activeCount = message.activeCount;
      audio.updatePeers(message.peers ?? [], ownPeerId);
      renderStatus(teamName, activeCount, listening.self, captureMode, localCapturedEvents, localSentEvents, permissionHint);
    }
    if (message.type === "peer_activity_batch") {
      audio.scheduleBatch(message.peerId, message.events);
    }
    if (message.type === "error") {
      console.error(`\nCliks server: ${message.message}`);
    }
  });

  ws.on("close", () => {
    cleanup();
    console.log("\nDisconnected from Cliks.");
    process.exit(0);
  });

  ws.on("error", (error) => {
    cleanup();
    console.error(`\nCould not connect to Cliks: ${error.message}`);
    process.exit(1);
  });

  capture.on("activity", (event) => {
    localCapturedEvents += 1;
    batcher.push(event);
  });
  batcher.on("batch", (batch) => {
    if (ws.readyState !== WebSocket.OPEN) return;
    localSentEvents += batch.events.length;
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
  permissionHint = captureState.permissionHint;
  renderStatus(teamName, activeCount, listening.self, captureMode, localCapturedEvents, localSentEvents, permissionHint);

  quipTimer = setInterval(() => {
    process.stdout.write(`\n${quips[Math.floor(Math.random() * quips.length)]}\n`);
    renderStatus(teamName, activeCount, listening.self, captureMode, localCapturedEvents, localSentEvents, permissionHint);
  }, 18_000);

  const stopFromSignal = () => {
    cleanup();
    ws.close();
    setTimeout(() => process.exit(0), 100).unref();
  };

  process.once("SIGINT", stopFromSignal);
  process.once("SIGTERM", stopFromSignal);
  process.once("SIGHUP", stopFromSignal);
  process.once("exit", cleanup);
}

function renderStatus(
  teamName: string,
  activeCount: number,
  hearingSelf: boolean | undefined,
  captureMode: string,
  localCapturedEvents: number,
  localSentEvents: number,
  permissionHint?: string
) {
  process.stdout.write("\x1Bc");
  console.log("Cliks");
  console.log("");
  console.log(`Team: ${teamName}`);
  console.log(`Active now: ${activeCount}`);
  console.log("");
  console.log("Sharing exact activity pulses in 500ms batches.");
  console.log("Privacy: only keyboard/mouse event type and timing are sent. Never key values.");
  console.log(`Self monitor: ${hearingSelf ? "on for local testing" : "off"}`);
  console.log(`Capture: ${captureMode}`);
  console.log(`Local captured events: ${localCapturedEvents}`);
  console.log(`Local sent events: ${localSentEvents}`);
  if (permissionHint) console.log(`Permission: ${permissionHint}`);
  if (captureMode !== "off" && localCapturedEvents === 0) {
    console.log("If teammates cannot hear you, run: typ capture-test");
  }
  console.log("Press Ctrl+C to stop.");
}
