import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = dirname(dirname(fileURLToPath(import.meta.url)));
const serverDir = join(rootDir, "server");
const distDir = join(serverDir, "dist");
const outputName = process.platform === "win32" ? "cliks-server.exe" : "cliks-server";
const outputPath = join(distDir, outputName);

mkdirSync(distDir, { recursive: true });

const result = spawnSync(
  "go",
  ["build", "-trimpath", "-ldflags=-s -w", "-o", outputPath, "."],
  {
    cwd: serverDir,
    stdio: "inherit"
  }
);

process.exit(result.status ?? 1);
