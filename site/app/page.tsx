"use client";

import { FormEvent, useMemo, useState } from "react";

type CreatedTeam = {
  code: string;
  name: string;
};

const apiBase = process.env.NEXT_PUBLIC_CLIKS_API_URL ?? "http://localhost:8787";

export default function HomePage() {
  const [name, setName] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [createdTeam, setCreatedTeam] = useState<CreatedTeam | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"code" | "command" | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  const joinCommand = useMemo(() => {
    if (!createdTeam) return "";
    return `typ join ${createdTeam.code} && typ start`;
  }, [createdTeam]);

  async function createTeam(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setCreatedTeam(null);
    setIsCreating(true);

    try {
      const response = await fetch(`${apiBase}/api/teams`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, deletePassword })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error ?? "Could not create team.");
      setCreatedTeam(payload.team);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not create team.");
    } finally {
      setIsCreating(false);
    }
  }

  async function copyText(value: string, kind: "code" | "command") {
    await navigator.clipboard.writeText(value);
    setCopied(kind);
    window.setTimeout(() => setCopied(null), 1600);
  }

  return (
    <div className="shell">
      <header className="nav">
        <div className="brand">Cliks</div>
        <small>ambient presence, not surveillance</small>
      </header>

      <main className="main">
        <section className="hero">
          <h1>Hear your remote team working beside you.</h1>
          <p>
            Cliks turns anonymous keyboard and mouse activity pulses into realistic local ambience.
            No login, no typed content, no screenshots, no microphone.
          </p>
          <div className="privacy-strip" aria-label="Privacy promises">
            <span>No keystrokes</span>
            <span>No mouse coordinates</span>
            <span>No accounts</span>
            <span>CLI-first</span>
          </div>
        </section>

        <section className="panel" aria-label="Create a team">
          <h2>Create a team</h2>
          <form onSubmit={createTeam}>
            <div className="field">
              <label htmlFor="team-name">Team name</label>
              <input
                id="team-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                minLength={2}
                maxLength={80}
                placeholder="Friday Project Room"
                required
              />
            </div>
            <div className="field">
              <label htmlFor="delete-password">Delete password</label>
              <input
                id="delete-password"
                value={deletePassword}
                onChange={(event) => setDeletePassword(event.target.value)}
                minLength={6}
                maxLength={128}
                type="password"
                placeholder="Used only to delete this team"
                required
              />
            </div>
            <button className="primary" type="submit" disabled={isCreating}>
              {isCreating ? "Creating..." : "Generate team code"}
            </button>
          </form>

          {error ? <div className="error">{error}</div> : null}

          {createdTeam ? (
            <div className="result">
              <div className="result-title">{createdTeam.name}</div>
              <div className="copy-row">
                <div className="code">{createdTeam.code}</div>
                <button
                  className="copy-button"
                  type="button"
                  onClick={() => copyText(createdTeam.code, "code")}
                >
                  {copied === "code" ? "Copied" : "Copy code"}
                </button>
              </div>
              <div className="copy-row command-row">
                <div className="command">{joinCommand}</div>
                <button
                  className="copy-button"
                  type="button"
                  onClick={() => copyText(joinCommand, "command")}
                >
                  {copied === "command" ? "Copied" : "Copy command"}
                </button>
              </div>
            </div>
          ) : null}

          <p className="notes">
            The website only creates room codes. The live coworking session happens in the CLI
            through the Cliks backend.
          </p>
        </section>
      </main>
    </div>
  );
}
