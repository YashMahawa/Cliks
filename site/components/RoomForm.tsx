"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useAcoustic } from "./AcousticProvider";
import { CommandLine, CopyButton, InstallCopy } from "./CommandBits";

type CreatedTeam = {
  code: string;
  name: string;
};

const configuredApiBase = (process.env.NEXT_PUBLIC_CLIKS_API_URL ?? "https://cliks-server.onrender.com").replace(/\/+$/, "");
const apiBase = configuredApiBase === "https://139.59.29.207.sslip.io"
  ? "https://cliks-server.onrender.com"
  : configuredApiBase;
const installCommand =
  "curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash";

export function RoomForm() {
  const { triggerSound } = useAcoustic();
  const [name, setName] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [createdTeam, setCreatedTeam] = useState<CreatedTeam | null>(null);
  const [error, setError] = useState("");
  const [isCreating, setIsCreating] = useState(false);
	const [isDeleting, setIsDeleting] = useState(false);
	const [mode, setMode] = useState<"create" | "delete">("create");
	const [teamCode, setTeamCode] = useState("");
	const [confirmDelete, setConfirmDelete] = useState(false);
	const [deletedCode, setDeletedCode] = useState("");
	const [cooldown, setCooldown] = useState(0);
	const [codeCopied, setCodeCopied] = useState(false);

	useEffect(() => {
		if (cooldown <= 0) return;
		const timer = window.setTimeout(() => setCooldown((seconds) => Math.max(0, seconds - 1)), 1000);
		return () => window.clearTimeout(timer);
	}, [cooldown]);

	async function requestTeam(path: string, method: "POST" | "DELETE", body: object) {
		const controller = new AbortController();
		const timer = window.setTimeout(() => controller.abort(), 75000);
		try {
			const response = await fetch(`${apiBase}${path}`, {
				method,
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(body),
				signal: controller.signal,
			});
			const payload: { error?: string; team?: CreatedTeam; ok?: boolean } = await response.json();
			if (!response.ok) {
				if (response.status === 429) setCooldown(300);
				throw new Error(payload.error ?? "The request could not be completed.");
			}
			return payload;
		} finally {
			window.clearTimeout(timer);
		}
	}

	function requestError(caught: unknown, action: string) {
		if (caught instanceof DOMException && caught.name === "AbortError") return "The room server took too long to wake. Please try again.";
		if (caught instanceof TypeError) return "Could not reach the room server. Check your connection and try again.";
		return caught instanceof Error ? caught.message : `Could not ${action} the room.`;
	}

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
		if (isCreating || cooldown > 0) return;
    setError("");
    setCreatedTeam(null);
    setIsCreating(true);
		setCooldown(3);
    try {
			const payload = await requestTeam("/api/teams", "POST", { name, deletePassword });
      if (!payload.team?.code) throw new Error("Could not create room.");
      setCreatedTeam(payload.team);
			setDeletePassword("");
      triggerSound();
    } catch (caught) {
			setError(requestError(caught, "create"));
    } finally {
      setIsCreating(false);
    }
  }

	async function deleteTeam(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (isDeleting || cooldown > 0) return;
		const code = teamCode.trim().toUpperCase();
		if (!/^CLIK-[A-Z0-9]{6}$/.test(code)) {
			setError("Enter a valid CLIK-XXXXXX room code.");
			return;
		}
		if (!confirmDelete) {
			setConfirmDelete(true);
			setError("");
			return;
		}
		setIsDeleting(true);
		setCooldown(3);
		setError("");
		try {
			await requestTeam(`/api/teams/${code}`, "DELETE", { deletePassword });
			setDeletedCode(code);
			setCreatedTeam(null);
			setDeletePassword("");
			setTeamCode("");
			setConfirmDelete(false);
		} catch (caught) {
			setError(requestError(caught, "delete"));
		} finally {
			setIsDeleting(false);
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
			<button
				type="button"
				onClick={async () => {
					try { await navigator.clipboard.writeText(createdTeam.code); } catch { /* clipboard unavailable */ }
					setCodeCopied(true);
					window.setTimeout(() => setCodeCopied(false), 1400);
				}}
				className="mt-1 font-mono text-4xl font-bold tracking-[0.12em] text-accent transition-opacity hover:opacity-80 active:scale-[0.99] md:text-5xl"
				aria-label="Copy room code"
			>
				{createdTeam.code}
			</button>
			<div className="mt-2 font-mono text-[11px] text-mute">{codeCopied ? "copied" : "click the code to copy"}</div>
          </div>
          <CopyButton
            value={createdTeam.code}
            ariaLabel="Copy room code"
            className="btn-ghost h-11 px-4 text-fg"
          />
        </div>
		<p className="mt-3 text-xs leading-relaxed text-mute">
			Rooms automatically expire after 120 hours (5 days) without a live connection. Reconnecting refreshes the clock.
		</p>

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
		<button
			type="button"
			onClick={() => {
				setCreatedTeam(null);
				setMode("delete");
				setTeamCode(createdTeam.code);
				setConfirmDelete(false);
				setError("");
			}}
			className="ml-6 font-mono text-xs text-mute underline-offset-2 hover:text-soft hover:underline"
		>
			Delete this room
		</button>
      </div>
    );
  }

	if (mode === "delete") {
		return (
			<div className="panel p-7 md:p-9 xl:p-10">
				<p className="font-mono text-[11px] uppercase tracking-[0.16em] text-mute">Room management</p>
				<h3 className="mt-2 text-xl font-bold tracking-tight xl:text-2xl">Delete a room</h3>
				<p className="mt-2 text-sm text-soft">You need the room code and its delete password. Everyone connected will be disconnected.</p>
				{deletedCode ? <p className="mt-5 text-sm text-accent" role="status">{deletedCode} was deleted.</p> : null}
				<form onSubmit={deleteTeam} className="mt-8 flex flex-col gap-7">
					<div className="flex flex-col gap-2">
						<label htmlFor="delete-team-code" className="font-mono text-xs text-mute">Room code</label>
						<input id="delete-team-code" value={teamCode} onChange={(event) => { setTeamCode(event.target.value.toUpperCase()); setConfirmDelete(false); }} placeholder="CLIK-XXXXXX" required maxLength={11} className="w-full border-b border-line bg-transparent pb-2 font-mono text-lg text-fg outline-none focus:border-[var(--accent)]" />
					</div>
					<div className="flex flex-col gap-2">
						<label htmlFor="delete-team-password" className="font-mono text-xs text-mute">Delete password</label>
						<input id="delete-team-password" type="password" value={deletePassword} onChange={(event) => { setDeletePassword(event.target.value); setConfirmDelete(false); }} required maxLength={128} className="w-full border-b border-line bg-transparent pb-2 text-lg text-fg outline-none focus:border-[var(--accent)]" />
					</div>
					{confirmDelete ? <p className="text-sm text-accent" role="alert">This cannot be undone. Confirm deletion of {teamCode.trim().toUpperCase()}.</p> : null}
					<button type="submit" disabled={isDeleting || cooldown > 0} className="btn-primary flex h-12 items-center justify-center disabled:opacity-50">
						{isDeleting ? "Deleting…" : cooldown > 0 ? `Wait ${cooldown}s` : confirmDelete ? "Confirm deletion" : "Delete room"}
					</button>
				</form>
				{error ? <p className="mt-5 font-mono text-xs text-accent" role="alert">{error}</p> : null}
				<button type="button" onClick={() => { setMode("create"); setConfirmDelete(false); setError(""); setDeletePassword(""); }} className="mt-8 font-mono text-xs text-mute underline-offset-2 hover:text-soft hover:underline">Back to create</button>
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

        <button type="submit" disabled={isCreating || cooldown > 0} className="btn-primary mt-1 flex h-12 items-center justify-center disabled:opacity-50">
          {isCreating ? "Generating…" : cooldown > 0 ? `Wait ${cooldown}s` : "Generate room code"}
        </button>
      </form>

      {error ? (
        <p className="mt-5 font-mono text-xs leading-relaxed text-accent" role="alert">
          {error}
        </p>
      ) : null}
		<button type="button" onClick={() => { setMode("delete"); setError(""); setDeletedCode(""); setDeletePassword(""); }} className="mt-8 font-mono text-xs text-mute underline-offset-2 hover:text-soft hover:underline">Delete an existing room</button>
    </div>
  );
}
