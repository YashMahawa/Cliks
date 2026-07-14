"use client";

import { useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";

export function CopyButton({
  value,
  className = "",
  ariaLabel,
  withLabel = true,
}: {
  value: string;
  className?: string;
  ariaLabel?: string;
  withLabel?: boolean;
}) {
  const [done, setDone] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      /* clipboard unavailable */
    }
    setDone(true);
    window.setTimeout(() => setDone(false), 1400);
  }

  return (
    <button
      type="button"
      onClick={copy}
      aria-label={ariaLabel ?? "Copy command"}
      className={`flex shrink-0 items-center gap-1.5 font-mono text-xs transition-opacity active:scale-[0.98] ${className}`}
    >
      {done ? (
        <span className="flex items-center gap-1.5 text-accent">
          <Check className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden />
          {withLabel ? "copied" : null}
        </span>
      ) : (
        <span className="flex items-center gap-1.5">
          <Copy className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden />
          {withLabel ? "copy" : null}
        </span>
      )}
    </button>
  );
}

/** Short install CTA — never dumps the long curl into the layout. */
export function InstallCopy({
  value,
  className = "",
  label = "Copy install command",
  subtitle = "Native download · source fallback",
}: {
  value: string;
  className?: string;
  label?: string;
  subtitle?: string;
}) {
  const [done, setDone] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      /* clipboard unavailable */
    }
    setDone(true);
    window.setTimeout(() => setDone(false), 1600);
  }

  return (
    <button
      type="button"
      onClick={copy}
      className={`install-copy group flex w-full items-center justify-between gap-4 border border-line bg-[var(--cmd)] px-4 py-3.5 text-left transition-[border-color,background,transform] hover:border-[var(--line-strong)] hover:bg-3 active:scale-[0.995] ${className}`}
      aria-label={done ? "Install command copied" : label}
    >
      <span className="flex min-w-0 items-center gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center border border-line bg-2 text-accent">
          <Terminal className="h-4 w-4" strokeWidth={1.75} aria-hidden />
        </span>
        <span className="min-w-0">
          <span className="block text-sm font-semibold text-fg">
            {done ? "Copied to clipboard" : label}
          </span>
          <span className="mt-0.5 block font-mono text-[11px] text-mute">
            {done ? "Paste in your terminal" : subtitle}
          </span>
        </span>
      </span>
      <span className="flex h-9 shrink-0 items-center gap-1.5 border border-line bg-2 px-3 font-mono text-xs text-soft group-hover:text-fg">
        {done ? (
          <>
            <Check className="h-3.5 w-3.5 text-accent" strokeWidth={1.75} aria-hidden />
            done
          </>
        ) : (
          <>
            <Copy className="h-3.5 w-3.5" strokeWidth={1.75} aria-hidden />
            copy
          </>
        )}
      </span>
    </button>
  );
}

export function CommandLine({
  value,
  display,
  className = "",
}: {
  value: string;
  display?: string;
  className?: string;
}) {
  // Short values (join codes) still show the command; long install lines hide behind InstallCopy.
  const looksLong = value.length > 48 && !display;
  if (looksLong) {
    return <InstallCopy value={value} className={className} />;
  }

  return (
    <div
      className={`group flex items-center gap-3 border border-line p-1.5 pl-4 cmd-shell ${className}`}
    >
      <span className="select-none font-mono text-sm text-accent">$</span>
      <code className="flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs text-soft sm:text-sm">
        {display ?? value}
      </code>
      <CopyButton
        value={value}
        className="h-9 bg-white/[0.04] px-3 text-fg hover:bg-white/[0.08]"
      />
    </div>
  );
}
