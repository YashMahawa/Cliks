import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = dirname(dirname(fileURLToPath(import.meta.url)));
const serverDir = join(rootDir, "server");
const candidates =
  process.platform === "win32"
    ? [join(serverDir, "dist", "cliks-server.exe"), join(serverDir, "dist", "cliks-server")]
    : [join(serverDir, "dist", "cliks-server")];
const binary = candidates.find((candidate) => existsSync(candidate));

if (!binary) {
  console.error("Server binary not found. Run: npm --workspace @cliks/server run build");
  process.exit(1);
}

const child = spawn(binary, process.argv.slice(2), {
  cwd: serverDir,
  env: process.env,
  stdio: "inherit",
  windowsHide: false
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
