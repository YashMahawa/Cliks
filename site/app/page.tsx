"use client";

import { FormEvent, useMemo, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { useAcoustic } from "../components/AcousticProvider";
import { Copy, Check, Terminal, Users, Lock } from "lucide-react";

type CreatedTeam = {
  code: string;
  name: string;
};

const apiBase = process.env.NEXT_PUBLIC_CLIKS_API_URL ?? "http://localhost:8787";
const installCommand = "curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash";

function RevealStagger({ children, delayOffset = 0 }: { children: React.ReactNode; delayOffset?: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 24 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.3 }}
      transition={{ duration: 0.6, delay: delayOffset, ease: [0.16, 1, 0.3, 1] }}
    >
      {children}
    </motion.div>
  );
}

export default function HomePage() {
  const { triggerSound, pulseActive, typingProgress, setTypingProgress } = useAcoustic();

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
    <AnimatePresence mode="wait">
      {typingProgress < 10 ? (
        <motion.div
          key="gate"
          initial={{ opacity: 1 }}
          exit={{ opacity: 0, y: -20 }}
          transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
          className="flex flex-col min-h-[100dvh] items-center justify-center bg-[#0a0a0a] text-[#f5f5f5] px-6 select-none text-center relative overflow-hidden"
        >
          {/* Subtle background glow based on typing */}
          <motion.div 
            className="absolute inset-0 bg-[radial-gradient(circle_at_center,_rgba(217,119,70,0.03)_0%,_transparent_70%)] pointer-events-none"
            animate={{ scale: pulseActive ? 1.05 : 1, opacity: pulseActive ? 0.8 : 0.3 }}
            transition={{ duration: 0.4 }}
          />

          <div className="max-w-md space-y-6 relative z-10">
            <span className="text-[#888888] font-mono text-xs uppercase tracking-widest block">
              [ isolated ]
            </span>
            <p className="text-lg md:text-xl text-[#f5f5f5] font-medium leading-relaxed max-w-[40ch] mx-auto">
              Working from home is silent. The house is empty.
            </p>
            <div className="pt-8">
              <p className="text-[#888888] text-xs font-mono mb-4 uppercase tracking-wider">
                Type fast to break the isolation
              </p>
              <div className="font-mono text-sm tracking-widest text-[#d97746] flex items-center justify-center gap-1">
                <span>[</span>
                {Array.from({ length: 10 }).map((_, i) => (
                  <motion.span
                    key={i}
                    animate={{ opacity: i < typingProgress ? 1 : 0.3, scale: i === typingProgress - 1 && pulseActive ? 1.4 : 1 }}
                    className={`transition-colors duration-300 ${
                      i < typingProgress ? "text-[#d97746] font-bold" : "text-[#333333]"
                    }`}
                  >
                    {i < typingProgress ? "•" : "."}
                  </motion.span>
                ))}
                <span>]</span>
              </div>
            </div>
            <button
              onClick={() => setTypingProgress(10)}
              className="mt-8 text-xs font-mono text-[#888888] hover:text-[#f5f5f5] transition-colors cursor-pointer"
            >
              skip intro
            </button>
          </div>
        </motion.div>
      ) : (
        <motion.div
          key="content"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1] }}
          className="w-full flex flex-col items-center bg-[#0a0a0a] text-[#f5f5f5] px-6 md:px-12 selection:bg-[#d97746]/20 selection:text-[#d97746]"
        >
          {/* Navigation */}
          <header className="w-full max-w-6xl flex items-center justify-between py-8">
            <div className="font-mono text-sm tracking-widest text-[#f5f5f5] font-bold flex items-center gap-2">
              <div className="w-1.5 h-1.5 rounded-full bg-[#d97746] ember-pulse" />
              CLIKS
            </div>
            <div className="flex items-center gap-4">
              <a
                href="https://github.com/YashMahawa/Cliks"
                target="_blank"
                rel="noopener noreferrer"
                className="text-[#888888] hover:text-[#f5f5f5] transition-colors flex items-center justify-center"
                aria-label="GitHub Repository"
              >
                <svg viewBox="0 0 24 24" className="w-4 h-4 fill-current">
                  <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
                </svg>
              </a>
              <a
                href="https://x.com/MahawarYas27492"
                target="_blank"
                rel="noopener noreferrer"
                className="text-[#888888] hover:text-[#f5f5f5] transition-colors flex items-center justify-center"
                aria-label="X Profile"
              >
                <svg viewBox="0 0 24 24" className="w-4 h-4 fill-current">
                  <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                </svg>
              </a>
            </div>
          </header>

          <main className="w-full max-w-6xl">
            {/* Hero Section */}
            <section className="grid grid-cols-1 lg:grid-cols-[1.2fr_1fr] gap-12 lg:gap-20 py-24 md:py-32 items-center">
              <div className="flex flex-col items-start text-left relative z-10">
                <div className="flex items-center justify-start h-8 mb-4">
                  <motion.div 
                    animate={{ 
                      scale: pulseActive ? 1.8 : 1,
                      opacity: pulseActive ? 1 : 0.6,
                      boxShadow: pulseActive ? "0 0 20px rgba(217,119,70,0.6)" : "0 0 0px rgba(217,119,70,0)"
                    }}
                    transition={{ type: "spring", stiffness: 400, damping: 10 }}
                    className="w-2.5 h-2.5 rounded-full bg-[#d97746]" 
                  />
                </div>

                <h1 className="text-4xl md:text-6xl font-semibold tracking-tighter text-[#f5f5f5] leading-[1.1]">
                  Work alone, <br/>together.
                </h1>

                <p className="mt-8 text-[#888888] leading-relaxed text-base md:text-lg max-w-[45ch]">
                  Cliks turns your remote team's typing into ambient background sound. No keystrokes shared, no microphones, just presence.
                </p>

                <div className="w-full mt-10 border border-white/10 bg-[#121212]/50 backdrop-blur-md rounded-xl p-4 flex items-center justify-between gap-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.05)]">
                  <code className="text-[#f5f5f5] font-mono text-xs overflow-x-auto select-all whitespace-nowrap text-left flex-1 scrollbar-none">
                    {installCommand}
                  </code>
                  <button
                    type="button"
                    onClick={() => copyText(installCommand, "install")}
                    className="h-9 px-4 rounded-lg border border-white/10 hover:border-white/20 bg-[#1a1a1a] text-[#f5f5f5] text-xs font-mono transition-all flex items-center gap-2 cursor-pointer active:scale-95"
                  >
                    {copied === "install" ? (
                      <>
                        <Check className="w-3.5 h-3.5 text-emerald-500" />
                        <span>Copied</span>
                      </>
                    ) : (
                      <>
                        <Copy className="w-3.5 h-3.5" />
                        <span>Copy</span>
                      </>
                    )}
                  </button>
                </div>
                
                <span className="mt-6 text-[#666666] font-mono text-[11px] uppercase tracking-wider">
                  Press any key to hear presence
                </span>
              </div>

              {/* Minimalist Visual Component */}
              <div className="relative w-full aspect-square md:aspect-[4/3] lg:aspect-square rounded-2xl border border-white/10 bg-[#121212] overflow-hidden shadow-[inset_0_1px_0_rgba(255,255,255,0.05)] flex items-center justify-center group">
                <img
                  src="/images/warm_desk_workspace.png"
                  alt="Minimalist workspace"
                  className="absolute inset-0 w-full h-full object-cover opacity-30 grayscale mix-blend-luminosity group-hover:opacity-50 transition-all duration-1000 ease-out"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-[#0a0a0a] via-[#0a0a0a]/50 to-transparent opacity-80" />
                
                {/* Abstract Acoustic Rings */}
                <div className="relative z-10 flex items-center justify-center">
                  {[1, 2, 3].map((ring) => (
                    <motion.div
                      key={ring}
                      animate={{ 
                        scale: pulseActive ? 1 + (ring * 0.2) : 1 + (ring * 0.1),
                        opacity: pulseActive ? 0.4 / ring : 0.1 / ring
                      }}
                      transition={{ type: "spring", stiffness: 200, damping: 20 }}
                      className="absolute rounded-full border border-[#d97746]"
                      style={{ width: `${ring * 80}px`, height: `${ring * 80}px` }}
                    />
                  ))}
                  <div className="w-4 h-4 rounded-full bg-[#d97746] shadow-[0_0_30px_rgba(217,119,70,0.5)]" />
                </div>
              </div>
            </section>

            {/* Philosophy Bento Grid */}
            <section className="py-24 md:py-32 border-t border-white/10">
              <RevealStagger>
                <div className="mb-16">
                  <h2 className="text-3xl md:text-4xl font-semibold tracking-tighter text-[#f5f5f5]">
                    Acoustic presence,<br />complete privacy.
                  </h2>
                </div>
              </RevealStagger>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                <RevealStagger delayOffset={0.1}>
                  <div className="flex flex-col h-full bg-[#121212] border border-white/10 rounded-2xl p-8 shadow-[inset_0_1px_0_rgba(255,255,255,0.05)]">
                    <Terminal className="w-6 h-6 text-[#d97746] mb-6" />
                    <h3 className="text-lg font-medium text-[#f5f5f5] mb-3">Native OS Hooks</h3>
                    <p className="text-[#888888] text-sm leading-relaxed">
                      Captures only the timing of input events. Keystrokes, active windows, and clipboard data are physically ignored at the system level.
                    </p>
                  </div>
                </RevealStagger>
                
                <RevealStagger delayOffset={0.2}>
                  <div className="flex flex-col h-full bg-[#121212] border border-white/10 rounded-2xl p-8 shadow-[inset_0_1px_0_rgba(255,255,255,0.05)]">
                    <Lock className="w-6 h-6 text-[#d97746] mb-6" />
                    <h3 className="text-lg font-medium text-[#f5f5f5] mb-3">Zero Content Sent</h3>
                    <p className="text-[#888888] text-sm leading-relaxed">
                      Only generic "keyboard" or "mouse" event types and timing offsets are transmitted. We couldn't reconstruct your work if we tried.
                    </p>
                  </div>
                </RevealStagger>

                <RevealStagger delayOffset={0.3}>
                  <div className="flex flex-col h-full bg-[#121212] border border-white/10 rounded-2xl p-8 shadow-[inset_0_1px_0_rgba(255,255,255,0.05)]">
                    <Users className="w-6 h-6 text-[#d97746] mb-6" />
                    <h3 className="text-lg font-medium text-[#f5f5f5] mb-3">Ephemeral Rooms</h3>
                    <p className="text-[#888888] text-sm leading-relaxed">
                      No accounts, no history, no database of presence. When the last person leaves the room, it evaporates from memory.
                    </p>
                  </div>
                </RevealStagger>
              </div>
            </section>

            {/* Room Creation */}
            <section className="py-24 md:py-32 border-t border-white/10">
              <RevealStagger>
                <div className="max-w-2xl mx-auto bg-[#121212]/80 backdrop-blur-xl border border-white/10 rounded-2xl p-8 md:p-12 shadow-[inset_0_1px_0_rgba(255,255,255,0.05)] relative overflow-hidden">
                  <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-transparent via-[#d97746]/40 to-transparent" />
                  
                  <h2 className="text-2xl font-semibold tracking-tight text-[#f5f5f5] mb-3">
                    Initialize a Workspace
                  </h2>
                  <p className="text-[#888888] text-sm mb-10 max-w-[40ch]">
                    Generate a temporary room code to cowork with your team in absolute privacy.
                  </p>

                  <form onSubmit={createTeam} className="space-y-6">
                    <div className="flex flex-col gap-2">
                      <label htmlFor="team-name" className="text-[11px] uppercase tracking-widest text-[#666666] font-mono">
                        Team Name
                      </label>
                      <input
                        id="team-name"
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                        minLength={2}
                        maxLength={80}
                        placeholder="Friday Project Room"
                        required
                        className="w-full h-12 bg-[#0a0a0a] border border-white/10 focus:border-[#d97746]/50 focus:ring-1 focus:ring-[#d97746]/50 rounded-lg px-4 text-[#f5f5f5] text-sm outline-none transition-all placeholder:text-[#444444]"
                      />
                    </div>

                    <div className="flex flex-col gap-2">
                      <label htmlFor="delete-password" className="text-[11px] uppercase tracking-widest text-[#666666] font-mono">
                        Delete Password
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
                        className="w-full h-12 bg-[#0a0a0a] border border-white/10 focus:border-[#d97746]/50 focus:ring-1 focus:ring-[#d97746]/50 rounded-lg px-4 text-[#f5f5f5] text-sm outline-none transition-all placeholder:text-[#444444]"
                      />
                    </div>

                    <button
                      type="submit"
                      disabled={isCreating}
                      className="w-full h-12 mt-4 bg-[#f5f5f5] hover:bg-white text-[#0a0a0a] font-medium text-sm rounded-lg transition-all active:scale-[0.98] disabled:opacity-50 disabled:pointer-events-none flex items-center justify-center cursor-pointer"
                    >
                      {isCreating ? "Generating..." : "Generate Room Code"}
                    </button>
                  </form>

                  {error && (
                    <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="mt-6 text-red-400 text-xs font-mono text-center bg-red-400/10 py-2 rounded">
                      {error}
                    </motion.div>
                  )}

                  {createdTeam && (
                    <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} className="mt-10 pt-10 border-t border-white/10 space-y-6">
                      <div className="flex items-center justify-between">
                        <span className="text-[#f5f5f5] text-sm font-medium">{createdTeam.name}</span>
                      </div>
                      
                      <div className="flex flex-col gap-2">
                        <div className="text-[#666666] text-[11px] font-mono uppercase tracking-widest">ACCESS CODE</div>
                        <div className="flex items-center justify-between border border-white/10 bg-[#0a0a0a] rounded-lg p-4 shadow-[inset_0_2px_4px_rgba(0,0,0,0.4)]">
                          <div className="font-mono text-2xl font-bold tracking-widest text-[#d97746]">
                            {createdTeam.code}
                          </div>
                          <button
                            className="h-9 px-3 rounded-md border border-white/10 hover:border-white/20 hover:bg-[#1a1a1a] text-[#f5f5f5] text-xs font-mono transition-all flex items-center gap-2 cursor-pointer active:scale-95"
                            type="button"
                            onClick={() => copyText(createdTeam.code, "code")}
                          >
                            {copied === "code" ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
                            <span className="hidden sm:inline">{copied === "code" ? "Copied" : "Copy Code"}</span>
                          </button>
                        </div>
                      </div>

                      <div className="flex flex-col gap-2">
                        <div className="text-[#666666] text-[11px] font-mono uppercase tracking-widest">JOIN COMMAND</div>
                        <div className="flex items-stretch border border-white/10 bg-[#0a0a0a] rounded-lg overflow-hidden shadow-[inset_0_2px_4px_rgba(0,0,0,0.4)] focus-within:border-[#d97746]/50 transition-colors">
                          <div className="flex-1 font-mono text-xs text-[#f5f5f5] bg-transparent p-4 overflow-x-auto select-all whitespace-nowrap scrollbar-none flex items-center">
                            {joinCommand}
                          </div>
                          <button
                            className="px-4 border-l border-white/10 hover:bg-[#1a1a1a] text-[#888888] hover:text-[#f5f5f5] transition-colors flex items-center justify-center cursor-pointer active:bg-white/5"
                            type="button"
                            onClick={() => copyText(joinCommand, "command")}
                          >
                            {copied === "command" ? <Check className="w-4 h-4 text-emerald-500" /> : <Copy className="w-4 h-4" />}
                          </button>
                        </div>
                      </div>
                      
                      <p className="text-[#888888] text-[11px] font-mono mt-4 border-l-2 border-[#d97746] pl-3">
                        Teammates install once, paste this command, then run <code className="text-[#f5f5f5]">typ start</code>.
                      </p>
                    </motion.div>
                  )}
                </div>
              </RevealStagger>
            </section>

            {/* Support / Footer */}
            <section className="py-24 border-t border-white/10 flex flex-col items-center text-center">
              <RevealStagger>
                <h2 className="text-xl font-medium text-[#f5f5f5] mb-6">
                  Keep the servers humming.
                </h2>
                <p className="text-[#888888] leading-relaxed text-sm max-w-[50ch] mb-10 mx-auto">
                  Cliks is free and open-source. Built to work alongside friends. If your team uses this daily, consider throwing a few dollars in the jar to cover server costs and keep it accessible for everyone.
                </p>

                <div className="w-full flex flex-col sm:flex-row items-center justify-center gap-4">
                  <a
                    href="https://github.com/sponsors/YashMahawa"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="w-full sm:w-auto h-11 px-6 rounded-lg bg-[#f5f5f5] hover:bg-white text-[#0a0a0a] font-medium text-sm transition-all active:scale-[0.98] flex items-center justify-center"
                  >
                    Sponsor on GitHub
                  </a>
                  <a
                    href="https://github.com/YashMahawa/Cliks"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="w-full sm:w-auto h-11 px-6 rounded-lg border border-white/10 hover:border-white/20 bg-[#121212] hover:bg-[#1a1a1a] text-[#f5f5f5] font-medium text-sm transition-all active:scale-[0.98] flex items-center justify-center"
                  >
                    Star the Repo
                  </a>
                </div>

                <div className="mt-16 flex flex-col md:flex-row items-center justify-between gap-4 w-full max-w-2xl mx-auto pt-8 border-t border-white/5">
                  <span className="text-[#666666] text-[11px] font-mono">
                    &copy; {new Date().getFullYear()} Cliks. Open source.
                  </span>
                  <a
                    href="https://x.com/MahawarYas27492"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-[11px] font-mono text-[#666666] hover:text-[#d97746] transition-colors flex items-center gap-1.5"
                  >
                    <svg viewBox="0 0 24 24" className="w-3.5 h-3.5 fill-current">
                      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                    </svg>
                    Follow on X
                  </a>
                </div>
              </RevealStagger>
            </section>
          </main>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
