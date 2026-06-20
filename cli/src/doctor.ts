import { constants } from "node:fs";
import { access, readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { getAudioPlayerStatus } from "./audio.js";
import { loadConfig } from "./config.js";

type DoctorIssue = {
  title: string;
  detail: string;
  commands: string[];
};

type LinuxInputStatus = {
  hasInputDir: boolean;
  eventCount: number;
  readableCount: number;
  inputGroupExists: boolean;
  inputGroupActive: boolean;
  username: string;
};

export async function runDoctor() {
  const issues: DoctorIssue[] = [];

  console.log("Cliks doctor");
  console.log("");
  console.log("Privacy:");
  console.log("- Cliks sends only event kind: keyboard or mouse.");
  console.log("- Cliks sends timing offsets inside each 500ms batch.");
  console.log("- Cliks does not send key values, key codes, words, coordinates, windows, or app names.");
  console.log("");
  console.log("System:");

  const config = await loadConfig();
  const nodeMajor = Number(process.versions.node.split(".")[0]);
  if (nodeMajor >= 20) {
    console.log(`- Node.js: ok (${process.versions.node})`);
  } else {
    console.log(`- Node.js: needs update (${process.versions.node})`);
    issues.push({
      title: "Update Node.js",
      detail: "Cliks needs Node.js 20 or newer.",
      commands: ["node --version", "Install Node.js 20 or newer, then rerun: typ doctor"]
    });
  }

  const audio = getAudioPlayerStatus();
  if (audio.player) {
    console.log(`- Audio player: ok (${audio.player})`);
  } else {
    console.log("- Audio player: missing");
    issues.push({
      title: "Install an audio playback tool",
      detail: audio.hint ?? "Cliks could not find a supported audio player.",
      commands: [...audio.commands, "typ sound-test"]
    });
  }

  console.log(`- Platform: ${process.platform}`);
  console.log(`- Current team: ${config.currentTeamCode ?? "not joined"}`);
  console.log(`- Sharing keyboard: ${config.sharing.keyboard ? "on" : "off"}`);
  console.log(`- Sharing mouse: ${config.sharing.mouse ? "on" : "off"}`);

  if (!config.currentTeamCode) {
    issues.push({
      title: "Join a team",
      detail: "Cliks does not have a selected team code.",
      commands: ["typ join CLIK-XXXX"]
    });
  }

  if (!config.sharing.keyboard) {
    issues.push({
      title: "Turn keyboard sharing on",
      detail: "Your keystroke timing will not reach teammates while share.keyboard is off.",
      commands: ["typ set share.keyboard on"]
    });
  }

  if (!config.sharing.mouse) {
    issues.push({
      title: "Turn mouse sharing on",
      detail: "Your mouse click timing will not reach teammates while share.mouse is off.",
      commands: ["typ set share.mouse on"]
    });
  }

  if (process.platform === "linux") {
    const input = await getLinuxInputStatus();
    console.log(`- Linux input devices: ${input.hasInputDir ? "found" : "not found"}`);
    if (input.hasInputDir) {
      console.log(`- Readable event devices: ${input.readableCount}/${input.eventCount}`);
      console.log(`- Active input group: ${input.inputGroupActive ? "yes" : "no"}`);
    }

    if (!input.hasInputDir) {
      issues.push({
        title: "Global capture is unavailable in this environment",
        detail: "Cliks cannot see /dev/input. This is normal in Termux, containers, SSH sessions, and some locked-down environments.",
        commands: ["Use a normal desktop terminal for global capture", "For a local terminal-only test: typ start --terminal --self"]
      });
    } else if (input.eventCount === 0) {
      issues.push({
        title: "No input event devices found",
        detail: "Cliks found /dev/input, but there are no /dev/input/event* devices to read.",
        commands: ["ls -l /dev/input", "Try again from the real desktop session"]
      });
    } else if (input.readableCount === 0) {
      const commands = input.inputGroupExists
        ? [`sudo usermod -aG input ${input.username}`, "Log out and log back in, or reboot", "typ doctor"]
        : ["Ask your distro how to allow reading /dev/input/event* for global input capture", "typ doctor"];
      issues.push({
        title: "Allow Cliks to read input events",
        detail: "Linux global capture needs permission to read /dev/input/event*. Cliks still sends only event type and timing, never key values.",
        commands
      });
    } else if (!input.inputGroupActive && input.inputGroupExists) {
      issues.push({
        title: "Input permission may not survive every device",
        detail: "Some event devices are readable, but your current login session is not in the input group.",
        commands: [`sudo usermod -aG input ${input.username}`, "Log out and log back in, or reboot", "typ doctor"]
      });
    }

    console.log("");
    console.log("Recommended run command:");
    console.log(input.readableCount > 0 ? "  typ start --evdev" : "  typ start --terminal --self");
  } else if (process.platform === "darwin") {
    issues.push({
      title: "Allow Accessibility permission",
      detail: "macOS global input capture needs Accessibility permission for the terminal app.",
      commands: ["Open System Settings > Privacy & Security > Accessibility", "Allow your terminal app", "typ doctor"]
    });
  } else if (process.platform === "win32") {
    console.log("");
    console.log("Recommended run command:");
    console.log("  typ start");
  } else {
    issues.push({
      title: "Unsupported global capture platform",
      detail: "This platform is not fully supported for global input capture yet.",
      commands: ["typ start --terminal --self"]
    });
  }

  console.log("");
  if (issues.length === 0) {
    console.log("No blocking issues detected.");
    console.log("Test playback: typ sound-test");
    return;
  }

  console.log("Fixes:");
  for (const issue of issues) {
    console.log("");
    console.log(`${issue.title}:`);
    console.log(`  ${issue.detail}`);
    for (const command of issue.commands) {
      console.log(`  ${command}`);
    }
  }
}

async function getLinuxInputStatus(): Promise<LinuxInputStatus> {
  const username = process.env.USER || process.env.LOGNAME || "$USER";
  const groupInfo = await readInputGroup();
  const activeGroups = typeof process.getgroups === "function" ? process.getgroups() : [];
  const inputGroupActive = groupInfo?.gid !== undefined && activeGroups.includes(groupInfo.gid);

  let entries: string[];
  try {
    entries = await readdir("/dev/input");
  } catch {
    return {
      hasInputDir: false,
      eventCount: 0,
      readableCount: 0,
      inputGroupExists: Boolean(groupInfo),
      inputGroupActive,
      username
    };
  }

  const eventDevices = entries.filter((entry) => /^event\d+$/.test(entry)).map((entry) => join("/dev/input", entry));
  let readableCount = 0;
  for (const device of eventDevices) {
    try {
      await access(device, constants.R_OK);
      readableCount += 1;
    } catch {
      // Count unreadable devices without failing the whole doctor run.
    }
  }

  return {
    hasInputDir: true,
    eventCount: eventDevices.length,
    readableCount,
    inputGroupExists: Boolean(groupInfo),
    inputGroupActive,
    username
  };
}

async function readInputGroup() {
  try {
    const groups = await readFile("/etc/group", "utf8");
    const line = groups.split("\n").find((entry) => entry.startsWith("input:"));
    if (!line) return undefined;
    const gid = Number(line.split(":")[2]);
    return Number.isFinite(gid) ? { gid } : undefined;
  } catch {
    return undefined;
  }
}
