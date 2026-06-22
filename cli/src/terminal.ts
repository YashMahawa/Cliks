import { spawnSync } from "node:child_process";

const trackedTerminalStates = new Set<string | undefined>();

export function captureTerminalState() {
  if (process.platform === "win32" || !process.stdin.isTTY) return undefined;
  const result = spawnSync("stty", ["-g"], {
    stdio: ["inherit", "pipe", "ignore"],
    encoding: "utf8"
  });
  return result.status === 0 ? result.stdout.trim() : undefined;
}

export function trackTerminalState(state?: string) {
  trackedTerminalStates.add(state);
  return () => {
    trackedTerminalStates.delete(state);
  };
}

export function restoreTrackedTerminalStates() {
  if (trackedTerminalStates.size === 0) {
    disableTerminalMouseReporting();
    return;
  }

  for (const state of [...trackedTerminalStates].reverse()) {
    restoreTerminalState(state);
  }
  trackedTerminalStates.clear();
}

export function restoreTerminalState(state?: string) {
  disableTerminalMouseReporting();
  if (process.stdin.isTTY) {
    try {
      process.stdin.setRawMode(false);
    } catch {
      // Fall through to stty recovery.
    }
  }

  if (state) {
    const result = spawnSync("stty", [state], { stdio: ["inherit", "ignore", "ignore"] });
    if (result.status === 0) return;
  }

  repairTerminal();
}

export function disableTerminalMouseReporting() {
  if (!process.stdout.isTTY) return;
  process.stdout.write("\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1004l\x1b[?1005l\x1b[?1006l");
}

export function repairTerminal() {
  disableTerminalMouseReporting();
  if (process.stdin.isTTY) {
    try {
      process.stdin.setRawMode(false);
    } catch {
      // Continue with stty sane.
    }
  }
  if (process.platform !== "win32") {
    spawnSync("stty", ["sane"], { stdio: ["inherit", "ignore", "ignore"] });
  }
}
