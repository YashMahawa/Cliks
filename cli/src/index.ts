import { Command } from "commander";
import { loadConfig, rememberTeam, saveConfig, toWsUrl } from "./config.js";
import { AudioEngine } from "./audio.js";
import { runCaptureTest } from "./captureTest.js";
import { runDoctor } from "./doctor.js";
import { startSession } from "./session.js";

const program = new Command();

program
  .name("typ")
  .description("Cliks ambient coworking CLI")
  .version("0.1.0");

program
  .command("join")
  .argument("<code>", "team code")
  .option("-n, --nickname <name>", "local display nickname")
  .description("remember and select a Cliks team")
  .action(async (code: string, options: { nickname?: string }) => {
    const config = await rememberTeam({ code });
    if (options.nickname) {
      config.nickname = options.nickname;
      await saveConfig(config);
    }
    console.log(`Joined ${code.toUpperCase()}. Run "typ start" to begin.`);
  });

program
  .command("start", { isDefault: true })
  .option("--evdev", "Linux global capture through /dev/input; works across Wayland and Xorg when permitted")
  .option("--terminal", "capture keystrokes typed in this terminal instead of using global capture")
  .option("--self", "hear your own local activity for testing")
  .description("start the selected team ambience")
  .action(async (options: { evdev?: boolean; terminal?: boolean; self?: boolean }) => {
    const config = await loadConfig();
    await startSession(config, {
      captureMode: options.terminal ? "terminal" : options.evdev ? "evdev" : "auto",
      selfMonitor: options.self
    });
  });

program
  .command("doctor")
  .description("check capture support and privacy expectations")
  .action(runDoctor);

program
  .command("capture-test")
  .option("--evdev", "test Linux global capture through /dev/input")
  .option("--terminal", "test keystrokes typed in this terminal")
  .option("--seconds <seconds>", "test duration in seconds")
  .description("verify that local keyboard/mouse activity is being captured")
  .action(async (options: { evdev?: boolean; terminal?: boolean; seconds?: string }) => {
    const config = await loadConfig();
    await runCaptureTest(config, {
      captureMode: options.terminal ? "terminal" : options.evdev ? "evdev" : "auto",
      seconds: options.seconds ? Number(options.seconds) : undefined
    });
    process.exit(0);
  });

program
  .command("sound-test")
  .description("play a few local Cliks clicks without connecting to a team")
  .action(async () => {
    const config = await loadConfig();
    const audio = new AudioEngine({
      ...config.listening,
      keyboard: true,
      mouse: true,
      volume: Math.max(config.listening.volume, 0.9)
    });
    await audio.preview();
    console.log("Played keyboard, keyboard, mouse test clicks.");
  });

program
  .command("teams")
  .description("list remembered teams")
  .action(async () => {
    const config = await loadConfig();
    if (config.teams.length === 0) {
      console.log("No teams saved yet. Run: typ join CLIK-XXXX");
      return;
    }
    for (const team of config.teams) {
      const marker = team.code === config.currentTeamCode ? "*" : " ";
      console.log(`${marker} ${team.code}${team.name ? `  ${team.name}` : ""}`);
    }
  });

program
  .command("switch")
  .argument("<code>", "saved team code")
  .description("switch the current team")
  .action(async (code: string) => {
    const config = await loadConfig();
    const normalized = code.toUpperCase();
    if (!config.teams.some((team) => team.code === normalized)) {
      throw new Error(`Team ${normalized} is not saved. Run: typ join ${normalized}`);
    }
    config.currentTeamCode = normalized;
    await saveConfig(config);
    console.log(`Current team is now ${normalized}.`);
  });

program
  .command("config")
  .description("show current settings")
  .action(async () => {
    const config = await loadConfig();
    console.log(JSON.stringify(config, null, 2));
  });

program
  .command("set")
  .argument("<key>", "setting key")
  .argument("<value>", "setting value")
  .description("set sharing/listening options")
  .action(async (key: string, value: string) => {
    const config = await loadConfig();
    const bool = value === "on" || value === "true" || value === "yes";
    if (key === "share.keyboard") config.sharing.keyboard = bool;
    else if (key === "share.mouse") config.sharing.mouse = bool;
    else if (key === "hear.keyboard") config.listening.keyboard = bool;
    else if (key === "hear.mouse") config.listening.mouse = bool;
    else if (key === "hear.self") config.listening.self = bool;
    else if (key === "volume") config.listening.volume = Math.max(0, Math.min(1, Number(value)));
    else if (key === "batch.ms") config.batchWindowMs = Math.max(100, Math.min(2_000, Number(value)));
    else if (key === "api.url") {
      config.apiUrl = value.replace(/\/$/, "");
      config.wsUrl = toWsUrl(config.apiUrl);
    }
    else if (key === "ws.url") config.wsUrl = value;
    else throw new Error(`Unknown setting: ${key}`);
    await saveConfig(config);
    console.log("Saved.");
  });

program.parseAsync().catch((error) => {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
});
