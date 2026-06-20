"use client";

import { FormEvent, useMemo, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { useAcoustic } from "../components/AcousticProvider";
import { Copy, Check } from "lucide-react";

type CreatedTeam = {
  code: string;
  name: string;
};

const apiBase = process.env.NEXT_PUBLIC_CLIKS_API_URL ?? "http://localhost:8787";
const installCommand = "curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash";

export default function HomePage() {
  const { triggerSound, pulseActive } = useAcoustic();

  const [name, setName] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [createdTeam, setCreatedTeam] = useState<CreatedTeam | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"install" | "code" | "command" | null>(null);
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
      triggerSound();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not create team.");
    } finally {
      setIsCreating(false);
    }
  }

  async function copyText(value: string, kind: "install" | "code" | "command") {
    await navigator.clipboard.writeText(value);
    setCopied(kind);
    triggerSound();
    window.setTimeout(() => setCopied(null), 1600);
  }

  return (
    <div className="w-full flex flex-col items-center min-h-[100dvh] bg-[#11100f] text-[#eae5d9] px-6 py-8 selection:bg-[#d97746]/20 selection:text-[#d97746]">
      {/* Navigation */}
      <header className="w-full max-w-2xl flex items-center justify-between py-6">
        <div className="font-mono text-sm tracking-widest text-[#eae5d9] font-bold">
          CLIKS
        </div>
        <div className="flex items-center gap-6">
          <a
            href="https://github.com/YashMahawa/Cliks"
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs font-mono text-[#8b867c] hover:text-[#d97746] transition-colors"
          >
            source code
          </a>
        </div>
      </header>

      <main className="w-full max-w-2xl flex-1 flex flex-col items-center">
        {/* Section 1: The Sensory Hero */}
        <section className="w-full flex flex-col items-center text-center pt-20 pb-24">
          {/* Glowing ember dot */}
          <div className="flex items-center justify-center h-12">
            <div className={`w-2 h-2 rounded-full bg-[#d97746] opacity-80 ${
              pulseActive ? "ember-pulse" : ""
            }`} style={{ boxShadow: "0 0 8px rgba(217, 119, 70, 0.4)" }} />
          </div>

          <h1 className="text-4xl md:text-5xl font-semibold tracking-tight text-[#eae5d9] leading-tight max-w-lg mt-6">
            Work alone, together.
          </h1>

          <p className="mt-8 text-[#8b867c] leading-relaxed text-sm md:text-base max-w-[55ch]">
            Cliks is an open source CLI that turns your remote team's typing into ambient background sound. No keystrokes shared, no microphones, just presence.
          </p>

          {/* Copy-to-clipboard install block */}
          <div className="w-full mt-12 border border-[#2a2826] bg-[#1a1918] rounded-lg p-4 flex items-center justify-between gap-4">
            <code className="text-[#eae5d9] font-mono text-xs overflow-x-auto select-all whitespace-nowrap text-left flex-1 scrollbar-none">
              {installCommand}
            </code>
            <button
              type="button"
              onClick={() => copyText(installCommand, "install")}
              className="h-9 px-3 rounded border border-[#2a2826] hover:border-[#3a3835] bg-[#11100f] text-[#eae5d9] hover:text-[#d97746] text-xs font-mono transition-all flex items-center gap-1.5 cursor-pointer active:scale-98"
            >
              {copied === "install" ? (
                <>
                  <Check className="w-3.5 h-3.5 text-emerald-500" />
                  <span>copied</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5" />
                  <span>copy</span>
                </>
              )}
            </button>
          </div>

          <span className="mt-4 text-[#8b867c] font-mono text-xs">
            Press any key on this page to hear how it sounds.
          </span>
        </section>

        {/* Section 2: The Philosophy (Why this exists) */}
        <section className="w-full py-16 border-t border-[#2a2826] flex flex-col items-center">
          <div className="w-full max-w-[65ch] space-y-6 text-[#eae5d9] text-sm md:text-base leading-relaxed">
            <p>
              Video calls are exhausting. Complete silence is isolating. We built Cliks to bring back the feeling of sitting in a room with people getting things done.
            </p>
            
            <div className="pt-4">
              <span className="font-mono text-xs uppercase tracking-wider text-[#8b867c] block mb-4">
                Privacy Guarantee
              </span>
              <ul className="space-y-3 text-xs md:text-sm text-[#8b867c] font-mono">
                <li className="flex items-start gap-3">
                  <span className="text-[#d97746]">1.</span>
                  <span>Native OS hooks only capture timing.</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-[#d97746]">2.</span>
                  <span>Keystrokes are literally never recorded.</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-[#d97746]">3.</span>
                  <span>Pulses are batched to save bandwidth.</span>
                </li>
              </ul>
            </div>
          </div>
        </section>

        {/* Section 3: Room Creation (The Form) */}
        <section className="w-full py-16 border-t border-[#2a2826]">
          <div className="w-full bg-[#1a1918] border border-[#2a2826] rounded-lg p-6 md:p-8">
            <h2 className="text-xl font-medium text-[#eae5d9] mb-2 font-mono">
              Create a Room
            </h2>
            <p className="text-[#8b867c] text-xs mb-8">
              Generate a temporary room code to cowork with your team in absolute privacy.
            </p>

            <form onSubmit={createTeam} className="space-y-6">
              <div className="flex flex-col gap-2">
                <label htmlFor="team-name" className="text-xs uppercase tracking-wider text-[#8b867c] font-mono">
                  Team name
                </label>
                <input
                  id="team-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  minLength={2}
                  maxLength={80}
                  placeholder="Friday Project Room"
                  required
                  className="w-full h-11 bg-transparent border-b border-[#2a2826] focus:border-[#d97746] text-[#eae5d9] text-sm outline-none transition-colors rounded-none"
                />
              </div>

              <div className="flex flex-col gap-2">
                <label htmlFor="delete-password" className="text-xs uppercase tracking-wider text-[#8b867c] font-mono">
                  Delete password
                </label>
                <input
                  id="delete-password"
                  value={deletePassword}
                  onChange={(event) => setDeletePassword(event.target.value)}
                  minLength={6}
                  maxLength={128}
                  type="password"
                  placeholder="Used only to delete this room"
                  required
                  className="w-full h-11 bg-transparent border-b border-[#2a2826] focus:border-[#d97746] text-[#eae5d9] text-sm outline-none transition-colors rounded-none"
                />
              </div>

              <button
                type="submit"
                disabled={isCreating}
                className="w-full h-11 bg-[#d97746] hover:bg-[#c66436] text-[#11100f] font-medium text-sm rounded transition-colors cursor-pointer disabled:opacity-50 flex items-center justify-center active:scale-98"
              >
                {isCreating ? "Generating..." : "Generate Room Code"}
              </button>
            </form>

            {error ? <div className="mt-4 text-red-400 text-xs font-mono text-center">{error}</div> : null}

            {createdTeam ? (
              <div className="mt-8 pt-8 border-t border-[#2a2826] space-y-4">
                <div className="text-[#eae5d9] text-sm font-medium">{createdTeam.name}</div>
                
                <div className="flex items-center justify-between border border-[#2a2826] bg-[#11100f] rounded p-4">
                  <div className="font-mono text-2xl font-bold tracking-wider text-[#d97746]">
                    {createdTeam.code}
                  </div>
                  <button
                    className="h-9 px-3 rounded border border-[#2a2826] hover:border-[#3a3835] bg-[#1a1918] text-[#eae5d9] text-xs font-mono transition-colors flex items-center gap-1.5 cursor-pointer"
                    type="button"
                    onClick={() => copyText(createdTeam.code, "code")}
                  >
                    {copied === "code" ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
                    <span>{copied === "code" ? "copied" : "copy code"}</span>
                  </button>
                </div>

                <div className="flex flex-col gap-2">
                  <div className="text-[#8b867c] text-xs font-mono">JOIN COMMAND</div>
                  <div className="flex items-stretch border border-[#2a2826] bg-[#11100f] rounded overflow-hidden">
                    <div className="flex-1 font-mono text-xs text-[#eae5d9] bg-[#11100f] p-3 overflow-x-auto select-all whitespace-nowrap scrollbar-none">
                      {joinCommand}
                    </div>
                    <button
                      className="px-4 border-l border-[#2a2826] hover:bg-[#1a1918] text-[#8b867c] hover:text-[#d97746] transition-colors flex items-center justify-center cursor-pointer"
                      type="button"
                      onClick={() => copyText(joinCommand, "command")}
                    >
                      {copied === "command" ? <Check className="w-4 h-4 text-emerald-500" /> : <Copy className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
                
                <p className="text-[#8b867c] text-xs font-mono">
                  Teammates install once, paste this command, then run <code>typ start</code>.
                </p>
              </div>
            ) : null}
          </div>
        </section>

        {/* Section 4: Community & Support (The Donation Model) */}
        <section className="w-full py-16 border-t border-[#2a2826] flex flex-col items-center">
          <h2 className="text-xl font-medium text-[#eae5d9] font-mono text-center mb-6">
            Keep the servers humming.
          </h2>

          <p className="text-[#8b867c] leading-relaxed text-sm text-center max-w-[55ch] mb-10">
            Cliks is 100% free and open-source. I built this to work alongside my friends. If your team uses this every day, consider throwing a few dollars in the jar to help me pay for the DigitalOcean WebSocket server and keep it free for everyone else.
          </p>

          <div className="w-full flex flex-col sm:flex-row items-center justify-center gap-4">
            <a
              href="https://github.com/sponsors/YashMahawa"
              target="_blank"
              rel="noopener noreferrer"
              className="w-full sm:w-auto h-11 px-6 rounded bg-[#eae5d9] hover:bg-[#f4f0e6] text-[#11100f] font-medium text-sm transition-colors flex items-center justify-center cursor-pointer active:scale-98"
            >
              Sponsor on GitHub
            </a>
            <a
              href="https://github.com/YashMahawa/Cliks"
              target="_blank"
              rel="noopener noreferrer"
              className="w-full sm:w-auto h-11 px-6 rounded border border-[#2a2826] hover:border-[#3a3835] bg-[#1a1918] text-[#eae5d9] font-medium text-sm transition-colors flex items-center justify-center cursor-pointer active:scale-98"
            >
              Star the Repo
            </a>
          </div>

          <a
            href="https://x.com/yashmahawa"
            target="_blank"
            rel="noopener noreferrer"
            className="mt-6 text-xs font-mono text-[#8b867c] hover:text-[#d97746] transition-colors"
          >
            Follow the journey on X
          </a>
        </section>
      </main>

      {/* Footer */}
      <footer className="w-full max-w-2xl py-8 border-t border-[#2a2826] flex flex-col md:flex-row items-center justify-between gap-4">
        <span className="text-[#8b867c] text-xs font-mono">
          &copy; {new Date().getFullYear()} Cliks. Open source.
        </span>
        <span className="text-[#8b867c] text-xs font-mono">
          No typed data is ever stored or transmitted.
        </span>
      </footer>
    </div>
  );
}
