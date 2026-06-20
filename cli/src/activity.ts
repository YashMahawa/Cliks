import { EventEmitter } from "node:events";
import { createReadStream, existsSync } from "node:fs";
import { readdir } from "node:fs/promises";
import { join } from "node:path";

export type LocalActivityEvent = {
  kind: "keyboard" | "mouse";
  at: number;
  button?: "left" | "right" | "middle" | "unknown";
};

export type CaptureMode = "native" | "evdev" | "terminal" | "auto";

type ActivityOptions = {
  keyboard: boolean;
  mouse: boolean;
  mode?: CaptureMode;
};

export class ActivityCapture extends EventEmitter {
  private cleanupFns: Array<() => void> = [];
  private usingNativeHook = false;
  private mode: "native" | "evdev" | "terminal" | "off" = "off";
  private permissionHint: string | undefined;

  async start(options: ActivityOptions) {
    if (options.keyboard || options.mouse) {
      if (options.mode === "evdev") {
        await this.tryEvdevCapture(options);
      } else if (options.mode !== "terminal") {
        if (process.platform === "linux") {
          await this.tryEvdevCapture(options);
        }

        if (this.mode === "off") {
          this.usingNativeHook = await this.tryNativeHooks(options);
        }
      }
    }

    if (this.mode === "off" && options.mode === "terminal" && (options.keyboard || options.mouse)) {
      this.startTerminalCapture(options);
    }

    return {
      mode: this.mode,
      nativeHookStarted: this.usingNativeHook,
      permissionHint: this.permissionHint
    };
  }

  stop() {
    for (const cleanup of this.cleanupFns.splice(0)) cleanup();
  }

  private async tryNativeHooks(options: ActivityOptions) {
    try {
      // Optional native global input hook. If it is unavailable, the CLI falls back
      // to terminal-focused keyboard capture so development still works everywhere.
      const imported = await import("uiohook-napi");
      const hook = imported.uIOhook as {
        on(event: string, listener: (event: { button?: number }) => void): void;
        start(): void;
        stop(): void;
      };

      if (options.keyboard) {
        hook.on("keydown", () => {
          this.emit("activity", { kind: "keyboard", at: Date.now() } satisfies LocalActivityEvent);
        });
      }

      if (options.mouse) {
        hook.on("mousedown", (event: { button?: number }) => {
          this.emit("activity", {
            kind: "mouse",
            at: Date.now(),
            button: buttonName(event.button)
          } satisfies LocalActivityEvent);
        });
      }

      hook.start();
      this.cleanupFns.push(() => hook.stop());
      this.mode = "native";
      return true;
    } catch {
      return false;
    }
  }

  private async tryEvdevCapture(options: ActivityOptions) {
    if (process.platform !== "linux" || !existsSync("/dev/input")) {
      return false;
    }

    let entries: string[];
    try {
      entries = await readdir("/dev/input");
    } catch {
      return false;
    }

    const eventDevices = entries
      .filter((entry) => /^event\d+$/.test(entry))
      .map((entry) => join("/dev/input", entry));

    let opened = 0;
    for (const device of eventDevices) {
      try {
        const stream = createReadStream(device, { highWaterMark: 24 * 32 });
        const onData = (chunk: string | Buffer) => {
          if (Buffer.isBuffer(chunk)) this.handleEvdevChunk(chunk, options);
        };
        stream.on("data", onData);
        stream.on("error", () => undefined);
        this.cleanupFns.push(() => stream.destroy());
        opened += 1;
      } catch (error) {
        if (isPermissionError(error)) {
          this.permissionHint =
            "Linux global capture needs permission to read /dev/input/event*. Add your user to the input group, then log out/in: sudo usermod -aG input $USER";
        }
      }
    }

    if (opened > 0) {
      this.mode = "evdev";
      return true;
    }

    this.permissionHint ??=
      "Linux global capture could not open /dev/input/event*. Try: sudo usermod -aG input $USER, then log out and back in.";
    return false;
  }

  private handleEvdevChunk(chunk: Buffer, options: ActivityOptions) {
    const eventSize = chunk.length % 24 === 0 ? 24 : chunk.length % 16 === 0 ? 16 : 0;
    if (eventSize === 0) return;

    for (let offset = 0; offset + eventSize <= chunk.length; offset += eventSize) {
      const type = chunk.readUInt16LE(offset + (eventSize - 8));
      const code = chunk.readUInt16LE(offset + (eventSize - 6));
      const value = chunk.readInt32LE(offset + (eventSize - 4));

      if (type !== 1 || value !== 1) continue;

      if (isMouseButtonCode(code)) {
        if (!options.mouse) continue;
        this.emit("activity", {
          kind: "mouse",
          at: Date.now(),
          button: mouseButtonFromEvdevCode(code)
        } satisfies LocalActivityEvent);
        continue;
      }

      if (options.keyboard) {
        this.emit("activity", { kind: "keyboard", at: Date.now() } satisfies LocalActivityEvent);
      }
    }
  }

  private startTerminalCapture(options: ActivityOptions) {
    if (!process.stdin.isTTY) return;
    process.stdin.setRawMode(true);
    this.mode = "terminal";
    if (options.mouse) process.stdout.write("\x1b[?1000h\x1b[?1006h");

    const onData = (chunk: Buffer) => {
      const text = chunk.toString("utf8");
      if (text === "\u0003") {
        process.emit("SIGINT");
        return;
      }

      const withoutMouse = text.replace(/\x1b\[<(\d+);(\d+);(\d+)([mM])/g, (_match, code, _x, _y, action) => {
        if (options.mouse && action === "M") {
          const button = Number(code) & 3;
          this.emit("activity", {
            kind: "mouse",
            at: Date.now(),
            button: buttonName(button + 1)
          } satisfies LocalActivityEvent);
        }
        return "";
      });

      if (options.keyboard && withoutMouse.length > 0) {
        this.emit("activity", { kind: "keyboard", at: Date.now() } satisfies LocalActivityEvent);
      }
    };

    process.stdin.on("data", onData);
    this.cleanupFns.push(() => {
      process.stdin.off("data", onData);
      if (options.mouse) process.stdout.write("\x1b[?1000l\x1b[?1006l");
      if (process.stdin.isTTY) process.stdin.setRawMode(false);
    });
  }
}

export class ActivityBatcher extends EventEmitter {
  private queue: LocalActivityEvent[] = [];
  private timer: NodeJS.Timeout | undefined;

  constructor(private windowMs: number) {
    super();
  }

  push(event: LocalActivityEvent) {
    this.queue.push(event);
    this.timer ??= setTimeout(() => this.flush(), this.windowMs);
  }

  flush() {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = undefined;
    }

    const events = this.queue.splice(0);
    if (events.length === 0) return;

    const batchStartedAt = events[0].at;
    this.emit("batch", {
      batchStartedAt,
      events: events.map((event) => ({
        kind: event.kind,
        offsetMs: event.at - batchStartedAt,
        ...(event.kind === "mouse" ? { button: event.button ?? "unknown" } : {})
      }))
    });
  }
}

function buttonName(button?: number): "left" | "right" | "middle" | "unknown" {
  if (button === 1) return "left";
  if (button === 2) return "right";
  if (button === 3) return "middle";
  return "unknown";
}

function isMouseButtonCode(code: number) {
  return code >= 0x110 && code <= 0x116;
}

function mouseButtonFromEvdevCode(code: number): "left" | "right" | "middle" | "unknown" {
  if (code === 0x110) return "left";
  if (code === 0x111) return "right";
  if (code === 0x112) return "middle";
  return "unknown";
}

function isPermissionError(error: unknown) {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    ((error as { code?: string }).code === "EACCES" || (error as { code?: string }).code === "EPERM")
  );
}
