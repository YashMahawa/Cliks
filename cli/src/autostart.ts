import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { CliksConfig } from "./config.js";

const serviceName = "cliks-typ";
const launchAgentId = "io.cliks.typ";

type AutostartAction = "enable" | "disable" | "status";

export async function runAutostart(action: AutostartAction, config: CliksConfig, code?: string) {
  const teamCode = (code ?? config.currentTeamCode)?.toUpperCase();
  if (action === "enable" && !teamCode) {
    throw new Error("No team selected. Run: typ join CLIK-XXXXXX");
  }

  if (process.platform === "linux") {
    return runLinuxAutostart(action, teamCode);
  }
  if (process.platform === "darwin") {
    return runMacAutostart(action, teamCode);
  }
  if (process.platform === "win32") {
    return runWindowsAutostart(action, teamCode);
  }

  throw new Error("Autostart is currently supported on Linux, macOS, and Windows.");
}

function cliEntryPath() {
  return resolve(dirname(fileURLToPath(import.meta.url)), "..", "bin", "typ.js");
}

function quoted(value: string) {
  return JSON.stringify(value);
}

async function runLinuxAutostart(action: AutostartAction, teamCode?: string) {
  const dir = join(process.env.XDG_CONFIG_HOME ?? join(homedir(), ".config"), "systemd", "user");
  const servicePath = join(dir, `${serviceName}.service`);

  if (action === "status") {
    printStatus(servicePath, "systemd user service");
    return;
  }

  if (action === "disable") {
    spawnSync("systemctl", ["--user", "disable", "--now", `${serviceName}.service`], { stdio: "ignore" });
    await rm(servicePath, { force: true });
    spawnSync("systemctl", ["--user", "daemon-reload"], { stdio: "ignore" });
    console.log("Cliks autostart disabled.");
    return;
  }

  await mkdir(dir, { recursive: true });
  await writeFile(
    servicePath,
    `[Unit]
Description=Cliks ambient coworking
After=network-online.target

[Service]
Type=simple
ExecStart=${process.execPath} ${cliEntryPath()} start
Restart=always
RestartSec=10
Environment=CLIKS_AUTOSTART_TEAM=${teamCode}

[Install]
WantedBy=default.target
`,
    "utf8"
  );

  const reload = spawnSync("systemctl", ["--user", "daemon-reload"], { stdio: "ignore" });
  const enable = spawnSync("systemctl", ["--user", "enable", "--now", `${serviceName}.service`], { stdio: "ignore" });
  console.log(`Cliks autostart enabled for ${teamCode}.`);
  if (reload.status !== 0 || enable.status !== 0) {
    console.log("The service file was written, but systemctl could not start it automatically.");
    console.log(`Run: systemctl --user enable --now ${serviceName}.service`);
  }
}

async function runMacAutostart(action: AutostartAction, teamCode?: string) {
  const dir = join(homedir(), "Library", "LaunchAgents");
  const plistPath = join(dir, `${launchAgentId}.plist`);

  if (action === "status") {
    printStatus(plistPath, "LaunchAgent");
    return;
  }

  if (action === "disable") {
    spawnSync("launchctl", ["bootout", `gui/${process.getuid?.() ?? ""}`, plistPath], { stdio: "ignore" });
    await rm(plistPath, { force: true });
    console.log("Cliks autostart disabled.");
    return;
  }

  await mkdir(dir, { recursive: true });
  await writeFile(
    plistPath,
    `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${launchAgentId}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${process.execPath}</string>
    <string>${cliEntryPath()}</string>
    <string>start</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>CLIKS_AUTOSTART_TEAM</key>
    <string>${teamCode}</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>${join(homedir(), "Library", "Logs", "cliks-typ.log")}</string>
  <key>StandardErrorPath</key>
  <string>${join(homedir(), "Library", "Logs", "cliks-typ.err.log")}</string>
</dict>
</plist>
`,
    "utf8"
  );

  const bootstrap = spawnSync("launchctl", ["bootstrap", `gui/${process.getuid?.() ?? ""}`, plistPath], {
    stdio: "ignore"
  });
  console.log(`Cliks autostart enabled for ${teamCode}.`);
  if (bootstrap.status !== 0) {
    console.log("The LaunchAgent was written, but launchctl could not start it automatically.");
    console.log(`Run: launchctl bootstrap gui/$(id -u) ${quoted(plistPath)}`);
  }
}

async function runWindowsAutostart(action: AutostartAction, teamCode?: string) {
  const startupDir =
    process.env.APPDATA && join(process.env.APPDATA, "Microsoft", "Windows", "Start Menu", "Programs", "Startup");
  if (!startupDir) throw new Error("Could not locate the Windows Startup folder.");

  const scriptPath = join(startupDir, "Cliks typ.cmd");

  if (action === "status") {
    printStatus(scriptPath, "Startup script");
    return;
  }

  if (action === "disable") {
    await rm(scriptPath, { force: true });
    console.log("Cliks autostart disabled.");
    return;
  }

  await mkdir(startupDir, { recursive: true });
  await writeFile(
    scriptPath,
    `@echo off
set CLIKS_AUTOSTART_TEAM=${teamCode}
start "Cliks typ" /min "${process.execPath}" "${cliEntryPath()}" start
`,
    "utf8"
  );
  console.log(`Cliks autostart enabled for ${teamCode}.`);
  console.log("It will start after your next sign-in.");
}

function printStatus(path: string, label: string) {
  console.log(`Cliks autostart: ${existsSync(path) ? "enabled" : "disabled"}`);
  console.log(`${label}: ${path}`);
}
