import { EventEmitter } from "node:events";

export type LocalActivityEvent = {
  kind: "keyboard" | "mouse";
  at: number;
  button?: "left" | "right" | "middle" | "unknown";
};

type ActivityOptions = {
  keyboard: boolean;
  mouse: boolean;
  mode?: "native" | "terminal" | "auto";
};

export class ActivityCapture extends EventEmitter {
  private cleanupFns: Array<() => void> = [];
  private usingNativeHook = false;
  private mode: "native" | "terminal" | "off" = "off";

  async start(options: ActivityOptions) {
    if ((options.keyboard || options.mouse) && options.mode !== "terminal") {
      this.usingNativeHook = await this.tryNativeHooks(options);
    }

    if ((!this.usingNativeHook || options.mode === "terminal") && (options.keyboard || options.mouse)) {
      this.startTerminalCapture(options);
    }

    return {
      mode: this.mode,
      nativeHookStarted: this.usingNativeHook
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
