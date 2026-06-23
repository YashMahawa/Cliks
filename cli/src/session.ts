import WebSocket from "ws";
import { ActivityBatcher, ActivityCapture, type CaptureMode } from "./activity.js";
import { AudioEngine } from "./audio.js";
import { saveConfig, type CliksConfig } from "./config.js";
import { captureTerminalState, restoreTerminalState, trackTerminalState } from "./terminal.js";

const quips = [
  "Remote office volume: cozy focus.",
  "Someone has entered decisive typing mode.",
  "The room is awake.",
  "Tiny taps, big momentum.",
  "Focus is doing its little drum solo."
];
const heartbeatIntervalMs = 25_000;
const heartbeatGraceMs = 10_000;

export async function startSession(
  config: CliksConfig,
  options: { captureMode?: CaptureMode; selfMonitor?: boolean } = {}
) {
  const teamCode = config.currentTeamCode;
  if (!teamCode) {
    throw new Error("No team selected. Run: typ join CLIK-XXXXXX");
  }

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
  let connectionStatus = "connecting";
  let permissionHint: string | undefined;
  let ownPeerId: string | undefined;
  let quipTimer: NodeJS.Timeout | undefined;
  let reconnectTimer: NodeJS.Timeout | undefined;
  let cleanedUp = false;
  let stopped = false;
  let localCapturedEvents = 0;
  let localSentEvents = 0;
  let reconnectAttempt = 0;
  let ws: WebSocket | undefined;
  let heartbeatTimer: NodeJS.Timeout | undefined;
  let controlCleanup: (() => void) | undefined;
  let saveTimer: NodeJS.Timeout | undefined;
  let listeningSaveInFlight: Promise<void> | undefined;

  const cleanup = () => {
    if (cleanedUp) return;
    cleanedUp = true;
    stopped = true;
    if (quipTimer) clearInterval(quipTimer);
    if (reconnectTimer) clearTimeout(reconnectTimer);
    if (heartbeatTimer) clearInterval(heartbeatTimer);
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = undefined;
      void persistListening();
    }
    controlCleanup?.();
    batcher.flush();
    capture.stop();
    ws?.close();
  };

  const render = () => {
    renderStatus(
      teamName,
      activeCount,
      listening.self,
      listening,
      captureMode,
      connectionStatus,
      localCapturedEvents,
      localSentEvents,
      permissionHint
    );
  };

  const persistListening = () => {
    config.listening = {
      ...config.listening,
      keyboard: listening.keyboard,
      mouse: listening.mouse,
      self: Boolean(listening.self),
      volume: listening.volume,
      muted: Boolean(listening.muted),
      spatial: listening.spatial !== false,
      fatigueProtection: listening.fatigueProtection !== false,
      density: listening.density ?? 1
    };
    listeningSaveInFlight = saveConfig(config).catch(() => undefined);
    return listeningSaveInFlight;
  };

  const saveListeningSoon = () => {
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      saveTimer = undefined;
      void persistListening();
    }, 250);
    saveTimer.unref();
  };

  const adjustVolume = (delta: number) => {
    listening.volume = clamp(listening.volume + delta, 0, 1);
    if (listening.volume > 0) listening.muted = false;
    saveListeningSoon();
    render();
  };

  const adjustDensity = (delta: number) => {
    listening.density = clamp((listening.density ?? 1) + delta, 0.15, 1);
    saveListeningSoon();
    render();
  };

  const toggle = (key: "muted" | "spatial" | "fatigueProtection") => {
    if (key === "muted") listening.muted = !listening.muted;
    if (key === "spatial") listening.spatial = listening.spatial === false;
    if (key === "fatigueProtection") listening.fatigueProtection = listening.fatigueProtection === false;
    saveListeningSoon();
    render();
  };

  const connect = () => {
    if (stopped) return;
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = undefined;
    }
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = undefined;
    }
    connectionStatus = reconnectAttempt === 0 ? "connecting" : `reconnecting (${reconnectAttempt})`;
    render();

    const socket = new WebSocket(config.wsUrl);
    ws = socket;

    socket.on("open", () => {
      if (stopped || ws !== socket) return;
      reconnectAttempt = 0;
      connectionStatus = "connected";
      startHeartbeat(socket);
      socket.send(
        JSON.stringify({
          type: "join",
          teamCode,
          nickname: config.nickname,
          client: { name: "typ", version: "0.1.0" }
        })
      );
      render();
    });

    socket.on("message", (raw) => {
      if (stopped || ws !== socket) return;
      const message = JSON.parse(raw.toString());
      if (message.type === "welcome") {
        ownPeerId = message.peerId;
        activeCount = message.activeCount;
        teamName = message.team?.name ?? teamCode;
        render();
      }
      if (message.type === "presence") {
        activeCount = message.activeCount;
        audio.updatePeers(message.peers ?? [], ownPeerId);
        render();
      }
      if (message.type === "peer_activity_batch") {
        audio.scheduleBatch(message.peerId, message.events);
      }
      if (message.type === "error") {
        console.error(`\nCliks server: ${message.message}`);
      }
    });

    socket.on("close", () => {
      if (stopped || ws !== socket) return;
      if (heartbeatTimer) {
        clearInterval(heartbeatTimer);
        heartbeatTimer = undefined;
      }
      scheduleReconnect();
    });

    socket.on("error", (error) => {
      if (stopped || ws !== socket) return;
      connectionStatus = `connection error: ${error.message}`;
      render();
      socket.close();
    });
  };

  const scheduleReconnect = () => {
    if (stopped) return;
    if (reconnectTimer) clearTimeout(reconnectTimer);
    reconnectAttempt += 1;
    const delayMs = Math.min(30_000, 1_000 * 2 ** Math.min(reconnectAttempt - 1, 5));
    connectionStatus = `disconnected; retrying in ${Math.round(delayMs / 1000)}s`;
    render();
    reconnectTimer = setTimeout(connect, delayMs);
  };

  const startHeartbeat = (socket: WebSocket) => {
    let awaitingPong = false;

    socket.on("pong", () => {
      if (ws !== socket) return;
      awaitingPong = false;
    });

    heartbeatTimer = setInterval(() => {
      if (stopped || ws !== socket) return;
      if (socket.readyState !== WebSocket.OPEN) return;

      if (awaitingPong) {
        connectionStatus = "heartbeat missed; reconnecting";
        render();
        socket.terminate();
        return;
      }

      awaitingPong = true;
      socket.ping();
      setTimeout(() => {
        if (!stopped && ws === socket && awaitingPong && socket.readyState === WebSocket.OPEN) {
          connectionStatus = "heartbeat timed out; reconnecting";
          render();
          socket.terminate();
        }
      }, heartbeatGraceMs).unref();
    }, heartbeatIntervalMs);
    heartbeatTimer.unref();
  };

  capture.on("activity", (event) => {
    localCapturedEvents += 1;
    batcher.push(event);
  });
  batcher.on("batch", (batch) => {
    if (ws?.readyState !== WebSocket.OPEN) return;
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
  controlCleanup = setupInteractiveControls({
    adjustVolume,
    adjustDensity,
    toggle
  });
  render();
  connect();

  quipTimer = setInterval(() => {
    process.stdout.write(`\n${quips[Math.floor(Math.random() * quips.length)]}\n`);
    render();
  }, 18_000);

  const stopFromSignal = () => {
    cleanup();
    ws?.close();
    const exitSoon = () => setTimeout(() => process.exit(0), 100).unref();
    if (listeningSaveInFlight) {
      void listeningSaveInFlight.finally(exitSoon);
    } else {
      exitSoon();
    }
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
  listening: {
    volume: number;
    muted?: boolean;
    spatial?: boolean;
    fatigueProtection?: boolean;
    density?: number;
  },
  captureMode: string,
  connectionStatus: string,
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
  console.log("Sharing coarse activity pulses in 500ms batches.");
  console.log("Privacy: only keyboard/mouse event type and coarse timing are sent. Never key values.");
  console.log(`Self monitor: ${hearingSelf ? "on for local testing" : "off"}`);
  console.log(`Listen: ${listening.muted ? "muted" : `${Math.round(listening.volume * 100)}%`} | density ${Math.round((listening.density ?? 1) * 100)}% | spatial ${listening.spatial === false ? "off" : "on"} | fade ${listening.fatigueProtection === false ? "off" : "on"}`);
  console.log(`Connection: ${connectionStatus}`);
  console.log(`Capture: ${captureMode}`);
  if (captureMode === "terminal") {
    console.log("Terminal mode: affects this terminal only. If input gets weird, run: typ fix-terminal");
  }
  console.log(`Local captured events: ${localCapturedEvents}`);
  console.log(`Local sent events: ${localSentEvents}`);
  if (permissionHint) console.log(`Permission: ${permissionHint}`);
  if (captureMode !== "off" && localCapturedEvents === 0) {
    console.log("If teammates cannot hear you, run: typ capture-test");
  }
  console.log("Controls: Up/Down volume, [/ ] density, m mute, s spatial, f fade, Ctrl+C stop.");
}

function setupInteractiveControls(input: {
  adjustVolume(delta: number): void;
  adjustDensity(delta: number): void;
  toggle(key: "muted" | "spatial" | "fatigueProtection"): void;
}) {
  if (!process.stdin.isTTY) return undefined;

  const terminalState = captureTerminalState();
  const untrack = trackTerminalState(terminalState);

  try {
    process.stdin.setRawMode(true);
    process.stdin.resume();
  } catch {
    untrack();
    return undefined;
  }

  const onData = (chunk: Buffer) => {
    const text = chunk.toString("utf8");
    if (text === "\u0003") {
      process.emit("SIGINT");
      return;
    }
    if (text === "\x1b[A") input.adjustVolume(0.05);
    else if (text === "\x1b[B") input.adjustVolume(-0.05);
    else if (text === "]" || text === "}") input.adjustDensity(0.1);
    else if (text === "[" || text === "{") input.adjustDensity(-0.1);
    else if (text === "m" || text === "M") input.toggle("muted");
    else if (text === "s" || text === "S") input.toggle("spatial");
    else if (text === "f" || text === "F") input.toggle("fatigueProtection");
  };

  process.stdin.on("data", onData);

  return () => {
    process.stdin.off("data", onData);
    restoreTerminalState(terminalState);
    untrack();
  };
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}
