import { spawn } from "node:child_process";
import { readdir } from "node:fs/promises";
import { dirname, join } from "node:path";
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

type ListeningConfig = {
  keyboard: boolean;
  mouse: boolean;
  self?: boolean;
  volume: number;
};

export class AudioEngine {
  private placements = new Map<string, PeerPlacement>();
  private player = detectPlayer();
  private keyboardSamples: string[] | undefined;
  private mouseSamples: string[] | undefined;

  constructor(private listening: ListeningConfig) {}

  scheduleBatch(peerId: string, events: RemoteActivityEvent[]) {
    const placement = this.getPlacement(peerId);

    for (const event of events) {
      if (event.kind === "keyboard" && !this.listening.keyboard) continue;
      if (event.kind === "mouse" && !this.listening.mouse) continue;
      setTimeout(() => void this.play(event, placement), Math.max(0, event.offsetMs));
    }
  }

  async preview() {
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
    if (!this.player) return;

    const samples = event.kind === "keyboard" ? await this.getKeyboardSamples() : await this.getMouseSamples();
    const file = samples[Math.floor(Math.random() * samples.length)];
    const child = spawn(this.player.command, [...this.player.args, file], {
      stdio: "ignore",
      detached: true,
      env: {
        ...process.env,
        CLIKS_GAIN: String(this.listening.volume * (1 / placement.distance)),
        CLIKS_PAN: String(placement.pan)
      }
    });
    child.unref();
  }

  private async getKeyboardSamples() {
    this.keyboardSamples ??= await loadSamples("keyboard");
    return this.keyboardSamples;
  }

  private async getMouseSamples() {
    this.mouseSamples ??= await loadSamples("mouse");
    return this.mouseSamples;
  }

  private getPlacement(peerId: string) {
    const existing = this.placements.get(peerId);
    if (existing) return existing;

    const seed = seeded(peerId);
    const placement = {
      pan: seed.next() * 1.7 - 0.85,
      distance: 1 + seed.next() * 2.2,
      warmth: 0.7 + seed.next() * 0.6
    };
    this.placements.set(peerId, placement);
    return placement;
  }
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

function detectPlayer(): { command: string; args: string[] } | null {
  if (process.platform === "darwin") return { command: "afplay", args: [] };
  if (process.platform === "win32") {
    return {
      command: "powershell.exe",
      args: ["-NoProfile", "-Command", "(New-Object Media.SoundPlayer $args[0]).PlaySync();"]
    };
  }
  return { command: "paplay", args: [] };
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
