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
const doctorCommand = "typ doctor";

export default function HomePage() {
  const {
    hasEntered,
    activeSwitch,
    setActiveSwitch,
    triggerSound,
    pulseActive,
  } = useAcoustic();

  const [name, setName] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [createdTeam, setCreatedTeam] = useState<CreatedTeam | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"install" | "doctor" | "code" | "command" | null>(null);
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

  async function copyText(value: string, kind: "install" | "doctor" | "code" | "command") {
    await navigator.clipboard.writeText(value);
    setCopied(kind);
    window.setTimeout(() => setCopied(null), 1600);
  }

  const soundscapes = [
    {
      id: "base",
      name: "The Base Switch",
      desc: "Clean mechanical linear",
      price: "Free forever",
    },
    {
      id: "model_m",
      name: "Vintage IBM Model M",
      desc: "Heavy buckling spring",
      price: "$5 one-time",
    },
    {
      id: "library",
      name: "Quiet Library",
      desc: "Muffled keystrokes with ambient room tone",
      price: "$5 one-time",
    },
    {
      id: "rain",
      name: "Rainy Window",
      desc: "Soft tactile switches mixed with low-frequency rain",
      price: "$5 one-time",
    },
  ];

  if (!hasEntered) {
    return (
      <div className="flex min-h-[100dvh] items-center justify-center bg-[#09090b]">
        <div className="text-zinc-400 font-mono text-sm tracking-widest flex items-center gap-1 select-none">
          <span>Press any key to enter the room.</span>
          <span className="w-1.5 h-4 bg-[#22d3ee] blinking-cursor inline-block" />
        </div>
      </div>
    );
  }

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.8, ease: "easeOut" }}
        className="w-full flex flex-col items-center px-6 md:px-12"
      >
        {/* Navigation */}
        <header className="w-full max-w-5xl h-18 flex items-center justify-between border-b border-zinc-900">
          <div className="font-mono text-lg font-bold tracking-tight text-zinc-100">
            Cliks
          </div>
          <div className="flex items-center gap-6">
            <span className="text-zinc-500 font-mono text-[10px] uppercase tracking-wider hidden md:inline">
              presence, not surveillance
            </span>
            <a
              href="https://github.com/YashMahawa/Cliks"
              target="_blank"
              rel="noopener noreferrer"
              className="text-zinc-400 hover:text-[#22d3ee] text-xs font-mono transition-colors"
            >
              GitHub
            </a>
          </div>
        </header>

        {/* Section 1: The Interactive Hero */}
        <section className="w-full max-w-5xl min-h-[85dvh] flex flex-col justify-center pt-16 pb-20">
          <div className="max-w-3xl">
            <h1 className="text-5xl md:text-7xl lg:text-8xl font-bold tracking-tight text-zinc-100 leading-none">
              Hear your remote team.
            </h1>
            <p className="mt-8 text-lg md:text-xl text-zinc-400 leading-relaxed max-w-[65ch]">
              Ambient coworking presence without sharing a single keystroke. Open the CLI, keep your privacy, and listen.
            </p>
            <div className="mt-10 flex flex-col sm:flex-row items-start gap-4">
              <button
                type="button"
                onClick={() => copyText(installCommand, "install")}
                className="w-full sm:w-auto h-12 px-6 rounded-lg bg-zinc-100 hover:bg-zinc-200 text-zinc-950 font-medium text-sm transition-colors flex items-center justify-center gap-2 cursor-pointer active:scale-98"
              >
                {copied === "install" ? (
                  <>
                    <Check className="w-4 h-4 text-emerald-600" />
                    <span>Copied</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-4 h-4" />
                    <span>Copy install command</span>
                  </>
                )}
              </button>
            </div>
            
            {/* Live Telemetry */}
            <div className="mt-16 border-t border-zinc-900 pt-8">
              <div className="font-mono text-sm tracking-wide text-zinc-400">
                <span className={`inline-block transition-all duration-200 font-semibold ${
                  pulseActive ? "text-[#22d3ee] scale-105" : "text-zinc-300"
                }`}>
                  8,402
                </span>{" "}
                global pulses active right now
              </div>
            </div>
          </div>
        </section>

        {/* Section 2: The Mechanics (Asymmetric Split) */}
        <section className="w-full max-w-5xl py-24 border-t border-zinc-900">
          <div className="grid grid-cols-1 lg:grid-cols-[1.2fr_1.8fr] gap-12 lg:gap-24">
            <div className="lg:sticky lg:top-24 h-fit">
              <h2 className="text-4xl md:text-5xl font-bold tracking-tight text-zinc-100 leading-tight">
                Presence, not surveillance.
              </h2>
            </div>
            <div className="flex flex-col gap-12 lg:gap-16">
              <div className="flex flex-col gap-3">
                <h3 className="text-xl font-medium text-zinc-200">Global Capture</h3>
                <p className="text-zinc-400 leading-relaxed text-sm md:text-base max-w-[65ch]">
                  Native OS hooks read input timing, never the keys themselves.
                </p>
              </div>
              <div className="flex flex-col gap-3">
                <h3 className="text-xl font-medium text-zinc-200">Batching</h3>
                <p className="text-zinc-400 leading-relaxed text-sm md:text-base max-w-[65ch]">
                  Pulses are grouped into 500ms bursts to preserve acoustic rhythm without network flood.
                </p>
              </div>
              <div className="flex flex-col gap-3">
                <h3 className="text-xl font-medium text-zinc-200">Spatial Audio</h3>
                <p className="text-zinc-400 leading-relaxed text-sm md:text-base max-w-[65ch]">
                  Web Audio API places teammates in distinct spatial coordinates around your head.
                </p>
              </div>
            </div>
          </div>
        </section>

        {/* Section 3: Acoustic Materials */}
        <section className="w-full max-w-5xl py-24 border-t border-zinc-900">
          <span className="text-[11px] uppercase tracking-[0.2em] font-mono text-[#22d3ee] mb-4 block">
            ACOUSTIC MATERIALS
          </span>
          <h2 className="text-3xl font-bold tracking-tight text-zinc-100 mb-12">
            Sound Libraries
          </h2>
          <div className="flex flex-col border-t border-zinc-800">
            {soundscapes.map((item) => (
              <div
                key={item.id}
                onMouseEnter={() => {
                  setActiveSwitch(item.id);
                  triggerSound();
                }}
                className={`grid grid-cols-1 md:grid-cols-[1.5fr_2.5fr_1fr] items-center py-6 border-b border-zinc-800 gap-4 group cursor-pointer transition-colors ${
                  activeSwitch === item.id ? "text-[#22d3ee]" : "text-zinc-400 hover:text-zinc-200"
                }`}
              >
                <div className="font-medium text-zinc-100 group-hover:text-[#22d3ee] transition-colors flex items-center">
                  <span className="font-mono text-xs text-[#22d3ee] mr-3 select-none">
                    {activeSwitch === item.id ? "[active]" : "[      ]"}
                  </span>
                  {item.name}
                </div>
                <div className="text-zinc-500 group-hover:text-zinc-400 transition-colors text-sm">
                  {item.desc}
                </div>
                <div className="font-mono text-xs md:text-right text-zinc-400 group-hover:text-zinc-200">
                  {item.price}
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Section: Create a Room */}
        <section className="w-full max-w-5xl py-24 border-t border-zinc-900">
          <div className="max-w-2xl mx-auto">
            <h2 className="text-3xl font-bold tracking-tight text-zinc-100 mb-4">
              Create a Room
            </h2>
            <p className="text-zinc-400 text-sm mb-8">
              Generate a temporary room code to cowork with your team in absolute privacy.
            </p>
            
            <form onSubmit={createTeam} className="space-y-6">
              <div className="flex flex-col gap-2">
                <label htmlFor="team-name" className="text-xs uppercase tracking-wider text-zinc-400 font-mono">
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
                  className="w-full h-11 bg-zinc-900 border border-zinc-800 focus:border-[#22d3ee] rounded-lg px-4 text-zinc-100 text-sm outline-none transition-colors"
                />
              </div>
              <div className="flex flex-col gap-2">
                <label htmlFor="delete-password" className="text-xs uppercase tracking-wider text-zinc-400 font-mono">
                  Delete password
                </label>
                <input
                  id="delete-password"
                  value={deletePassword}
                  onChange={(event) => setDeletePassword(event.target.value)}
                  minLength={6}
                  maxLength={128}
                  type="password"
                  placeholder="Used only to delete this team"
                  required
                  className="w-full h-11 bg-zinc-900 border border-zinc-800 focus:border-[#22d3ee] rounded-lg px-4 text-zinc-100 text-sm outline-none transition-colors"
                />
              </div>
              <button
                type="submit"
                disabled={isCreating}
                className="w-full h-11 rounded-lg bg-zinc-100 hover:bg-zinc-200 text-zinc-950 font-medium text-sm transition-colors cursor-pointer disabled:opacity-50 flex items-center justify-center active:scale-98"
              >
                {isCreating ? "Creating..." : "Generate team code"}
              </button>
            </form>

            {error ? <div className="mt-4 text-red-400 text-sm font-mono">{error}</div> : null}

            {createdTeam ? (
              <div className="mt-8 border border-zinc-800 bg-zinc-950 rounded-lg p-6 space-y-4">
                <div className="text-zinc-300 font-medium">{createdTeam.name}</div>
                
                <div className="flex items-center justify-between border border-zinc-900 bg-zinc-900/50 rounded-lg p-4">
                  <div className="font-mono text-2xl font-bold tracking-wider text-[#22d3ee]">
                    {createdTeam.code}
                  </div>
                  <button
                    className="h-9 px-3 rounded-lg border border-zinc-800 hover:border-zinc-700 bg-zinc-950 text-zinc-300 text-xs font-mono transition-colors flex items-center gap-1.5 cursor-pointer"
                    type="button"
                    onClick={() => copyText(createdTeam.code, "code")}
                  >
                    {copied === "code" ? <Check className="w-3.5 h-3.5 text-emerald-600" /> : <Copy className="w-3.5 h-3.5" />}
                    <span>{copied === "code" ? "Copied" : "Copy code"}</span>
                  </button>
                </div>

                <div className="flex flex-col gap-2">
                  <div className="text-zinc-500 text-xs font-mono">JOIN COMMAND</div>
                  <div className="flex items-stretch border border-zinc-900 bg-zinc-950 rounded-lg overflow-hidden">
                    <div className="flex-1 font-mono text-xs text-zinc-300 bg-zinc-900/30 p-3 overflow-x-auto select-all whitespace-nowrap">
                      {joinCommand}
                    </div>
                    <button
                      className="px-4 border-l border-zinc-900 hover:bg-zinc-900 text-zinc-400 hover:text-zinc-200 transition-colors flex items-center justify-center cursor-pointer"
                      type="button"
                      onClick={() => copyText(joinCommand, "command")}
                    >
                      {copied === "command" ? <Check className="w-4 h-4 text-emerald-600" /> : <Copy className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
                
                <p className="text-zinc-500 text-xs leading-relaxed">
                  Teammates install once, paste this command, then run <code>typ start</code> whenever they want to join the room again.
                </p>
              </div>
            ) : null}
          </div>
        </section>

        {/* Section 4: Terminal / Exit */}
        <section className="w-full max-w-2xl py-24 border-t border-zinc-900 flex flex-col items-center">
          <p className="text-zinc-400 text-sm text-center mb-8">
            Run the installer. Create a team. Run <code className="bg-zinc-900 px-1.5 py-0.5 rounded text-zinc-200 font-mono text-xs">typ start</code>. Leave it in the background.
          </p>

          <div className="w-full border border-zinc-800 bg-[#0c0c0e] rounded-lg p-5 flex items-center justify-between">
            <code className="text-zinc-300 font-mono text-xs overflow-x-auto select-all mr-4 whitespace-nowrap">
              {installCommand}
            </code>
            <button
              className="h-9 px-3 rounded-lg border border-zinc-800 hover:border-zinc-700 bg-zinc-900 text-zinc-300 text-xs font-mono transition-colors flex items-center gap-1.5 cursor-pointer active:scale-98"
              type="button"
              onClick={() => copyText(installCommand, "install")}
            >
              {copied === "install" ? <Check className="w-3.5 h-3.5 text-emerald-600" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copied === "install" ? "Copied" : "Copy"}</span>
            </button>
          </div>

          <div className="mt-12 w-full border border-zinc-800 bg-[#0c0c0e] rounded-lg p-5 flex items-center justify-between">
            <code className="text-zinc-300 font-mono text-xs overflow-x-auto select-all mr-4 whitespace-nowrap">
              {doctorCommand}
            </code>
            <button
              className="h-9 px-3 rounded-lg border border-zinc-800 hover:border-zinc-700 bg-zinc-900 text-zinc-300 text-xs font-mono transition-colors flex items-center gap-1.5 cursor-pointer active:scale-98"
              type="button"
              onClick={() => copyText(doctorCommand, "doctor")}
            >
              {copied === "doctor" ? <Check className="w-3.5 h-3.5 text-emerald-600" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copied === "doctor" ? "Copied" : "Copy"}</span>
            </button>
          </div>
          
          <p className="mt-6 text-zinc-500 text-xs leading-relaxed text-center max-w-[60ch]">
            On Linux, the doctor explains if global keyboard and mouse capture needs permission for Wayland or Xorg. On macOS and Windows, use the native system prompts.
          </p>
        </section>

        {/* Footer */}
        <footer className="w-full max-w-5xl py-12 border-t border-zinc-900 flex flex-col md:flex-row items-center justify-between gap-4">
          <p className="text-zinc-600 text-xs font-mono">
            &copy; {new Date().getFullYear()} Cliks. Open source.
          </p>
          <p className="text-zinc-600 text-xs font-mono">
            No typed data is ever stored or transmitted.
          </p>
        </footer>
      </motion.div>
    </AnimatePresence>
  );
}
