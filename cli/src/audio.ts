import { spawn } from "node:child_process";
import { accessSync, constants } from "node:fs";
import { delimiter, dirname, join, resolve } from "node:path";
import { readdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";

export type RemoteActivityEvent = {
  kind: "keyboard" | "mouse";
  offsetMs: number;
  button?: "left" | "right" | "middle" | "unknown";
};

type PeerPlacement = {
  pan: number;
  distance: number;
  warmth: number;
};

type PeerPresence = {
  peerId: string;
  nickname?: string;
  joinedAt?: number;
};

type ListeningConfig = {
  keyboard: boolean;
  mouse: boolean;
  self?: boolean;
  volume: number;
  muted?: boolean;
  spatial?: boolean;
  fatigueProtection?: boolean;
  density?: number;
};

type DetectedPlayer = {
  command: string;
  spatial: boolean;
  argsFor: (job: PlaybackJob) => string[];
};

type PlaybackJob = {
  file: string;
  gain: number;
  pan: number;
};

const maxConcurrentPlayback = 4;
const maxQueuedPlayback = 96;
const fatigueWindowMs = 5_000;
const fatigueSoftLimit = 24;

export class AudioEngine {
  private placements = new Map<string, PeerPlacement>();
  private player = detectPlayer();
  private warnedUnavailable = false;
  private keyboardSamples: string[] | undefined;
  private mouseSamples: string[] | undefined;
  private playbackQueue: PlaybackJob[] = [];
  private activePlayback = 0;
  private recentPlaybackTimes: number[] = [];

  constructor(private listening: ListeningConfig) {}

  scheduleBatch(peerId: string, events: RemoteActivityEvent[]) {
    const placement = this.placements.get(peerId) ?? this.assignFallbackPlacement(peerId);

    for (const event of events) {
      if (this.listening.muted) continue;
      if (event.kind === "keyboard" && !this.listening.keyboard) continue;
      if (event.kind === "mouse" && !this.listening.mouse) continue;
      if (event.kind === "mouse" && !isPlayableMouseButton(event.button)) continue;
      if (!shouldPlayForDensity(this.listening.density ?? 1)) continue;
      setTimeout(() => void this.play(event, placement), Math.max(0, event.offsetMs));
    }
  }

  updatePeers(peers: PeerPresence[], ownPeerId?: string) {
    const remotePeers = peers
      .filter((peer) => peer.peerId !== ownPeerId)
      .sort((a, b) => (a.joinedAt ?? 0) - (b.joinedAt ?? 0) || a.peerId.localeCompare(b.peerId));
    const nextPlacements = new Map<string, PeerPlacement>();

    remotePeers.forEach((peer, index) => {
      nextPlacements.set(peer.peerId, placementForIndex(index, peer.peerId));
    });

    this.placements = nextPlacements;
  }

  async preview() {
    if (!this.player) {
      throw new Error(audioInstallMessage());
    }

    const placement = { pan: 0, distance: 1, warmth: 1 };
    await this.play({ kind: "keyboard", offsetMs: 0 }, placement);
    await sleep(130);
    await this.play({ kind: "keyboard", offsetMs: 0 }, placement);
    await sleep(180);
    await this.play({ kind: "mouse", button: "left", offsetMs: 0 }, placement);
    await sleep(260);
    await this.play({ kind: "mouse", button: "left", offsetMs: 0 }, placement);
    await sleep(260);
  }

  private async play(event: RemoteActivityEvent, placement: PeerPlacement) {
    if (!this.player) {
      this.warnUnavailableOnce();
      return;
    }

    const samples = event.kind === "keyboard" ? await this.getKeyboardSamples() : await this.getMouseSamples();
    const file = samples[Math.floor(Math.random() * samples.length)];
    const fatigueGain = this.listening.fatigueProtection === false ? 1 : this.recordAndGetFatigueGain();
    this.enqueuePlayback({
      file,
      gain: clamp(this.listening.volume * (1 / placement.distance) * fatigueGain, 0, 1),
      pan: this.listening.spatial === false ? 0 : placement.pan
    });
  }

  private enqueuePlayback(job: PlaybackJob) {
    if (this.playbackQueue.length >= maxQueuedPlayback) {
      this.playbackQueue.shift();
    }

    this.playbackQueue.push(job);
    this.drainPlaybackQueue();
  }

  private drainPlaybackQueue() {
    if (!this.player) return;

    while (this.activePlayback < maxConcurrentPlayback && this.playbackQueue.length > 0) {
      const job = this.playbackQueue.shift();
      if (!job) return;
      this.spawnPlayback(job);
    }
  }

  private spawnPlayback(job: PlaybackJob) {
    if (!this.player) return;
    const player = this.player;
    this.activePlayback += 1;

    const child = spawn(player.command, player.argsFor(job), {
      stdio: "ignore",
      detached: false,
      env: process.env
    });
    child.on("error", () => {
      this.player = null;
      this.warnUnavailableOnce();
    });
    child.on("close", () => {
      this.activePlayback = Math.max(0, this.activePlayback - 1);
      this.drainPlaybackQueue();
    });
  }

  private warnUnavailableOnce() {
    if (this.warnedUnavailable) return;
    this.warnedUnavailable = true;
    console.error(`\nAudio disabled: ${audioInstallMessage()}`);
  }

  private async getKeyboardSamples() {
    this.keyboardSamples ??= await loadSamples("keyboard");
    return this.keyboardSamples;
  }

  private async getMouseSamples() {
    this.mouseSamples ??= await loadSamples("mouse");
    return this.mouseSamples;
  }

  private assignFallbackPlacement(peerId: string) {
    const placement = placementForIndex(this.placements.size, peerId);
    this.placements.set(peerId, placement);
    return placement;
  }

  private recordAndGetFatigueGain() {
    const now = Date.now();
    this.recentPlaybackTimes = this.recentPlaybackTimes.filter((time) => now - time <= fatigueWindowMs);
    this.recentPlaybackTimes.push(now);

    const overload = Math.max(0, this.recentPlaybackTimes.length - fatigueSoftLimit);
    return clamp(1 - overload * 0.035, 0.35, 1);
  }
}

function placementForIndex(index: number, peerId: string): PeerPlacement {
  const seed = seeded(peerId);
  const ring = ringForIndex(index);
  const ringStart = ringStartIndex(ring);
  const positionInRing = index - ringStart;
  const capacity = ringCapacity(ring);
  const baseAngle = (Math.PI * 2 * positionInRing) / capacity;
  const jitter = (seed.next() - 0.5) * (Math.PI / capacity) * 0.7;
  const angle = baseAngle + jitter;
  const distance = 2 + ring + (seed.next() - 0.5) * 0.35;
  const pan = Math.max(-0.95, Math.min(0.95, Math.sin(angle)));

  return {
    pan,
    distance,
    warmth: 0.72 + seed.next() * 0.5
  };
}

function ringForIndex(index: number) {
  let ring = 0;
  let remaining = index;

  while (remaining >= ringCapacity(ring)) {
    remaining -= ringCapacity(ring);
    ring += 1;
  }

  return ring;
}

function ringStartIndex(ring: number) {
  let start = 0;
  for (let current = 0; current < ring; current += 1) {
    start += ringCapacity(current);
  }
  return start;
}

function ringCapacity(ring: number) {
  return 4 + ring * 4;
}

async function loadSamples(kind: "keyboard" | "mouse") {
  const root = join(dirname(fileURLToPath(import.meta.url)), "..", "assets", "sounds", kind);
  const files = (await readdir(root))
    .filter((file) => file.endsWith(".wav"))
    .sort()
    .map((file) => join(root, file));
  if (files.length === 0) throw new Error(`No ${kind} sound samples found in ${root}`);
  return files;
}

function detectPlayer(): DetectedPlayer | null {
  if (findExecutable("ffplay")) {
    return {
      command: "ffplay",
      spatial: true,
      argsFor: (job) => [
        "-nodisp",
        "-autoexit",
        "-loglevel",
        "quiet",
        "-af",
        ffmpegSpatialFilter(job.gain, job.pan),
        job.file
      ]
    };
  }

  if (findExecutable("mpv")) {
    return {
      command: "mpv",
      spatial: true,
      argsFor: (job) => [
        "--no-video",
        "--really-quiet",
        "--no-terminal",
        `--volume=${Math.round(clamp(job.gain, 0, 1) * 100)}`,
        `--audio-pan=${clamp(job.pan, -1, 1).toFixed(3)}`,
        job.file
      ]
    };
  }

  if (process.platform === "darwin") {
    return {
      command: "afplay",
      spatial: false,
      argsFor: (job) => ["-v", String(clamp(job.gain, 0, 1)), job.file]
    };
  }

  if (process.platform === "win32") {
    return {
      command: "powershell.exe",
      spatial: false,
      argsFor: (job) => ["-NoProfile", "-Command", "(New-Object Media.SoundPlayer $args[0]).PlaySync();", job.file]
    };
  }

  if (findExecutable("paplay")) {
    return {
      command: "paplay",
      spatial: false,
      argsFor: (job) => ["--volume", String(Math.round(clamp(job.gain, 0, 1) * 65536)), job.file]
    };
  }

  if (findExecutable("pw-play")) {
    return {
      command: "pw-play",
      spatial: false,
      argsFor: (job) => ["--volume", String(clamp(job.gain, 0, 1)), job.file]
    };
  }

  if (findExecutable("aplay")) {
    return { command: "aplay", spatial: false, argsFor: (job) => [job.file] };
  }

  return null;
}

export function getAudioPlayerStatus() {
  const player = detectPlayer();
  return {
    player: player?.command,
    spatial: Boolean(player?.spatial),
    spatialCommands: spatialInstallCommands(),
    hint: player ? undefined : audioInstallHint(),
    commands: player ? [] : audioInstallCommands()
  };
}

function findExecutable(command: string) {
  const pathValue = process.env.PATH ?? "";
  for (const directory of pathValue.split(delimiter)) {
    if (!directory) continue;
    const candidate = resolve(directory, command);
    try {
      accessSync(candidate, constants.X_OK);
      return candidate;
    } catch {
      // Keep checking PATH.
    }
  }
  return undefined;
}

function audioInstallHint() {
  if (process.platform === "linux") {
    return "no audio player found. Install PulseAudio/PipeWire playback tools such as pulseaudio-utils, pipewire-utils, or alsa-utils.";
  }
  return "no supported audio player found on this system.";
}

function audioInstallMessage() {
  const commands = audioInstallCommands();
  if (commands.length === 0) return audioInstallHint();
  return `${audioInstallHint()}\nRun:\n  ${commands.join("\n  ")}`;
}

function audioInstallCommands() {
  if (process.platform !== "linux") return [];
  if (findExecutable("pacman")) return ["sudo pacman -S --needed libpulse"];
  if (findExecutable("apt")) return ["sudo apt update", "sudo apt install -y pulseaudio-utils"];
  if (findExecutable("dnf")) return ["sudo dnf install -y pulseaudio-utils"];
  if (findExecutable("zypper")) return ["sudo zypper install pulseaudio-utils"];
  if (findExecutable("apk")) return ["sudo apk add pulseaudio-utils"];
  if (findExecutable("pkg")) return ["pkg install pulseaudio"];
  return [
    "Install one of these commands with your package manager: paplay, pw-play, or aplay",
    "Then run: typ sound-test"
  ];
}

function spatialInstallCommands() {
  if (findExecutable("ffplay") || findExecutable("mpv")) return [];
  if (process.platform === "darwin") return ["brew install ffmpeg", "typ sound-test"];
  if (process.platform === "win32") return ["Install mpv or FFmpeg, then make mpv.exe or ffplay.exe available on PATH", "typ sound-test"];
  if (findExecutable("pacman")) return ["sudo pacman -S --needed ffmpeg", "typ sound-test"];
  if (findExecutable("apt")) return ["sudo apt update", "sudo apt install -y ffmpeg", "typ sound-test"];
  if (findExecutable("dnf")) return ["sudo dnf install -y ffmpeg", "typ sound-test"];
  if (findExecutable("zypper")) return ["sudo zypper install ffmpeg", "typ sound-test"];
  if (findExecutable("apk")) return ["sudo apk add ffmpeg", "typ sound-test"];
  return ["Install ffplay/FFmpeg or mpv with your package manager", "typ sound-test"];
}

function isPlayableMouseButton(button?: RemoteActivityEvent["button"]) {
  return button === "left" || button === "right";
}

function shouldPlayForDensity(density: number) {
  return Math.random() <= clamp(density, 0.15, 1);
}

function seeded(text: string) {
  let state = 2166136261;
  for (const char of text) {
    state ^= char.charCodeAt(0);
    state = Math.imul(state, 16777619);
  }
  return {
    next() {
      state += 0x6d2b79f5;
      let value = state;
      value = Math.imul(value ^ (value >>> 15), value | 1);
      value ^= value + Math.imul(value ^ (value >>> 7), value | 61);
      return ((value ^ (value >>> 14)) >>> 0) / 4294967296;
    }
  };
}

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

function ffmpegSpatialFilter(gain: number, pan: number) {
  const { left, right } = stereoGains(gain, pan);
  return `pan=stereo|c0=${left.toFixed(3)}*c0|c1=${right.toFixed(3)}*c0`;
}

function stereoGains(gain: number, pan: number) {
  const normalizedPan = clamp(pan, -1, 1);
  const angle = ((normalizedPan + 1) * Math.PI) / 4;
  const clampedGain = clamp(gain, 0, 1);

  return {
    left: Math.cos(angle) * clampedGain,
    right: Math.sin(angle) * clampedGain
  };
}
