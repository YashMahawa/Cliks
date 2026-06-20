import { ActivityCapture, type CaptureMode, type LocalActivityEvent } from "./activity.js";
import type { CliksConfig } from "./config.js";

type CaptureTestOptions = {
  captureMode?: CaptureMode;
  seconds?: number;
};

export async function runCaptureTest(config: CliksConfig, options: CaptureTestOptions = {}) {
  const seconds = Math.max(3, Math.min(60, options.seconds ?? 10));
  const capture = new ActivityCapture();
  const counts = { keyboard: 0, mouse: 0 };
  let stopped = false;

  console.log("Cliks capture test");
  console.log("");
  console.log(`Sharing keyboard: ${config.sharing.keyboard ? "on" : "off"}`);
  console.log(`Sharing mouse: ${config.sharing.mouse ? "on" : "off"}`);
  console.log(`Duration: ${seconds}s`);
  console.log("");

  if (!config.sharing.keyboard && !config.sharing.mouse) {
    console.log("Nothing can be captured because both sharing settings are off.");
    console.log("Run:");
    console.log("  typ set share.keyboard on");
    console.log("  typ set share.mouse on");
    return;
  }

  capture.on("activity", (event: LocalActivityEvent) => {
    counts[event.kind] += 1;
  });

  const state = await capture.start({ ...config.sharing, mode: options.captureMode ?? "auto" });
  console.log(`Capture mode: ${state.mode}`);
  if (state.permissionHint) console.log(`Permission: ${state.permissionHint}`);

  if (state.mode === "off") {
    capture.stop();
    console.log("");
    printNoCaptureFixes();
    return;
  }

  console.log("");
  console.log("Now press a few keys and click once or twice.");

  await new Promise<void>((resolve) => {
    const finish = () => {
      if (stopped) return;
      stopped = true;
      capture.stop();
      resolve();
    };
    const timer = setTimeout(finish, seconds * 1000);
    const stopFromSignal = () => {
      clearTimeout(timer);
      finish();
    };
    process.once("SIGINT", stopFromSignal);
    process.once("SIGTERM", stopFromSignal);
  });

  console.log("");
  console.log(`Captured keyboard events: ${counts.keyboard}`);
  console.log(`Captured mouse events: ${counts.mouse}`);

  if (config.sharing.keyboard && counts.keyboard === 0) {
    console.log("");
    console.log("Keyboard capture did not fire.");
    printNoCaptureFixes();
  } else {
    console.log("");
    console.log("Capture is working locally. If teammates still cannot hear you, run typ start and check Local sent events.");
  }
}

function printNoCaptureFixes() {
  console.log("Fixes to try:");
  if (process.platform === "linux") {
    console.log("  typ doctor");
    console.log("  sudo usermod -aG input $USER");
    console.log("  Log out and log back in, or reboot");
    console.log("  typ capture-test --evdev");
    console.log("  For terminal-only testing: typ capture-test --terminal");
    return;
  }
  if (process.platform === "darwin") {
    console.log("  Open System Settings > Privacy & Security > Accessibility");
    console.log("  Allow your terminal app");
    console.log("  typ capture-test");
    return;
  }
  console.log("  typ doctor");
  console.log("  typ capture-test --terminal");
}
