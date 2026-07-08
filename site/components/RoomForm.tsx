"use client";

import { FormEvent, useMemo, useState } from "react";
import { useAcoustic } from "./AcousticProvider";
import { CommandLine, CopyButton, InstallCopy } from "./CommandBits";

type CreatedTeam = {
  code: string;
  name: string;
};

const apiBase = process.env.NEXT_PUBLIC_CLIKS_API_URL ?? "http://localhost:8787";
const installCommand =
  "curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash";

export function RoomForm() {
  const { triggerSound } = useAcoustic();
  const [name, setName] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [createdTeam, setCreatedTeam] = useState<CreatedTeam | null>(null);
  const [error, setError] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const joinCommand = useMemo(
    () => (createdTeam ? `cliks join ${createdTeam.code}` : ""),
    [createdTeam]
  );

  const shareText = useMemo(
    () =>
      createdTeam
        ? `Join my Cliks room ${createdTeam.code} - hear each other work without video or mics. Install: ${installCommand} then run: cliks join ${createdTeam.code}`
        : "",
    [createdTeam]
  );

  async function createTeam(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setCreatedTeam(null);
    setIsCreating(true);
    try {
      const controller = new AbortController();
      const timer = window.setTimeout(() => controller.abort(), 8000);
      const response = await fetch(`${apiBase}/api/teams`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, deletePassword }),
        signal: controller.signal,
      });
      window.clearTimeout(timer);

      let payload: { error?: string; team?: CreatedTeam } = {};
      try {
        payload = await response.json();
      } catch {
        throw new Error("Server returned an unexpected response.");
      }
      if (!response.ok) throw new Error(payload.error ?? "Could not create room.");
      if (!payload.team?.code) throw new Error("Could not create room.");
      setCreatedTeam(payload.team);
      triggerSound();
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === "AbortError") {
        setError("Server took too long. Is the Cliks API running on :8787?");
      } else if (caught instanceof TypeError) {
        setError("Cannot reach the API. Start it with npm run dev:server.");
      } else {
        setError(caught instanceof Error ? caught.message : "Could not create room.");
      }
    } finally {
      setIsCreating(false);
    }
  }

  if (createdTeam) {
    return (
      <div className="panel p-7 md:p-9 xl:p-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-accent">Room is live</p>
        <h3 className="mt-2 text-2xl font-bold tracking-tight xl:text-3xl">{createdTeam.name}</h3>
        <p className="mt-2 text-soft">Send this code to your team. That is the whole onboarding.</p>

        <div className="mt-8 flex flex-wrap items-end justify-between gap-4 border border-line bg-3 p-5 xl:p-6">
          <div>
            <div className="font-mono text-[11px] uppercase tracking-widest text-mute">room code</div>
            <div className="mt-1 font-mono text-4xl font-bold tracking-[0.12em] text-accent md:text-5xl">
              {createdTeam.code}
            </div>
          </div>
          <CopyButton
            value={createdTeam.code}
            ariaLabel="Copy room code"
            className="btn-ghost h-11 px-4 text-fg"
          />
        </div>

        <div className="mt-6 space-y-4">
          <div>
            <p className="mb-2 font-mono text-xs text-mute">1. They install (once)</p>
            <InstallCopy value={installCommand} label="Copy install command" />
          </div>
          <div>
            <p className="mb-2 font-mono text-xs text-mute">2. They join your room</p>
            <CommandLine value={joinCommand} />
          </div>
          <div>
            <p className="mb-2 font-mono text-xs text-mute">3. Or paste a full message</p>
            <div className="flex items-start gap-3 border border-line bg-[var(--cmd)] p-3">
              <p className="flex-1 text-sm leading-relaxed text-soft">{shareText}</p>
              <CopyButton
                value={shareText}
                ariaLabel="Copy share message"
                className="h-9 shrink-0 bg-white/[0.04] px-3 text-fg hover:bg-white/[0.08]"
              />
            </div>
          </div>
        </div>

        <button
          type="button"
          onClick={() => {
            setCreatedTeam(null);
            setName("");
            setDeletePassword("");
          }}
          className="mt-8 font-mono text-xs text-mute underline-offset-2 hover:text-soft hover:underline"
        >
          Create another room
        </button>
      </div>
    );
  }

  return (
    <div className="panel p-7 md:p-9 xl:p-10">
      <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-mute">Free · no account</p>
      <h3 className="mt-2 text-xl font-bold tracking-tight xl:text-2xl">Create a room in 20 seconds</h3>
      <p className="mt-2 text-sm text-soft">
        You get a code. Teammates join from their terminal. Nothing else to set up.
      </p>

      <form onSubmit={createTeam} className="mt-8 flex flex-col gap-7">
        <div className="flex flex-col gap-2">
          <label htmlFor="team-name" className="font-mono text-xs text-mute">
            Room name
          </label>
          <input
            id="team-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            minLength={2}
            maxLength={80}
            placeholder="Friday deep work"
            required
            className="w-full border-b border-line bg-transparent pb-2 text-lg text-fg outline-none transition-colors placeholder:text-mute focus:border-[var(--accent)]"
          />
        </div>

        <div className="flex flex-col gap-2">
          <label htmlFor="delete-password" className="font-mono text-xs text-mute">
            Delete password
          </label>
          <input
            id="delete-password"
            type="password"
            value={deletePassword}
            onChange={(e) => setDeletePassword(e.target.value)}
            minLength={6}
            maxLength={128}
            placeholder="Only used to tear the room down"
            required
            className="w-full border-b border-line bg-transparent pb-2 text-lg text-fg outline-none transition-colors placeholder:text-mute focus:border-[var(--accent)]"
          />
          <p className="text-xs text-mute">Not a login. Just a kill switch for this room.</p>
        </div>

        <button type="submit" disabled={isCreating} className="btn-primary mt-1 flex h-12 items-center justify-center">
          {isCreating ? "Generating…" : "Generate room code"}
        </button>
      </form>

      {error ? (
        <p className="mt-5 font-mono text-xs leading-relaxed text-accent" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}
