"use client";

import { useEffect, useState } from "react";
import { InstallCopy } from "./CommandBits";

const unixCommand =
  "curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash";
const windowsCommand =
  "irm https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.ps1 | iex";

export function InstallOptions() {
  const [platform, setPlatform] = useState<"unix" | "windows">("unix");

  useEffect(() => {
    if (/Windows/i.test(navigator.userAgent)) setPlatform("windows");
  }, []);

  const windows = platform === "windows";
  return (
    <div>
      <div className="mb-2 flex gap-1 font-mono text-[11px]" aria-label="Choose operating system">
        <button
          type="button"
          onClick={() => setPlatform("unix")}
          className={`border px-3 py-1.5 ${windows ? "border-line text-mute" : "border-[var(--line-strong)] bg-3 text-fg"}`}
        >
          macOS / Linux
        </button>
        <button
          type="button"
          onClick={() => setPlatform("windows")}
          className={`border px-3 py-1.5 ${windows ? "border-[var(--line-strong)] bg-3 text-fg" : "border-line text-mute"}`}
        >
          Windows
        </button>
      </div>
      <InstallCopy
        value={windows ? windowsCommand : unixCommand}
        label={windows ? "Copy PowerShell installer" : "Copy terminal installer"}
        subtitle={windows ? "Windows 10/11 · native .exe" : "macOS · Linux · native binary"}
      />
    </div>
  );
}
