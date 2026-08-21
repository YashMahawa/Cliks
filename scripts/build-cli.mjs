import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = dirname(dirname(fileURLToPath(import.meta.url)));
const cliDir = join(rootDir, "cli");
const distDir = join(cliDir, "dist");
const outputName = process.platform === "win32" ? "cliks.exe" : "cliks";
const outputPath = join(distDir, outputName);

mkdirSync(distDir, { recursive: true });

const result = spawnSync(
  "go",
  ["build", "-o", outputPath, "."],
  {
    cwd: cliDir,
    stdio: "inherit"
  }
);

if (result.error) {
  throw result.error;
}

process.exit(result.status ?? 1);
