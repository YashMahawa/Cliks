import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = dirname(dirname(fileURLToPath(import.meta.url)));
const cliBin = process.env.CLIKS_BIN ?? [
  join(rootDir, "cli", "dist", process.platform === "win32" ? "cliks.exe" : "cliks"),
  join(rootDir, "cli", "dist", "cliks")
].find((p) => existsSync(p));

if (!cliBin || !existsSync(cliBin)) {
  throw new Error(`CLI binary not found at ${cliBin}. Run: npm --workspace @cliks/cli run build`);
}

const result = spawnSync(cliBin, ["doctor"], {
  cwd: rootDir,
  stdio: "inherit"
});

process.exit(result.status ?? 1);
