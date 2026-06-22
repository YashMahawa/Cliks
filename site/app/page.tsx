"use client";

import { FormEvent, useMemo, useState } from "react";
import Image from "next/image";
import { motion, AnimatePresence } from "motion/react";
import { useAcoustic } from "../components/AcousticProvider";
import { Copy, Check } from "lucide-react";

type CreatedTeam = {
  code: string;
  name: string;
};

const apiBase = process.env.NEXT_PUBLIC_CLIKS_API_URL ?? "http://localhost:8787";
const installCommand =
  "curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash";
const repoUrl = "https://github.com/YashMahawa/Cliks";
const sponsorUrl = "https://github.com/sponsors/YashMahawa";
const xUrl = "https://x.com/MahawarYas27492";

function GitHubIcon({ className = "w-4 h-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={`${className} fill-current`} aria-hidden>
      <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
    </svg>
  );
}

function XIcon({ className = "w-4 h-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={`${className} fill-current`} aria-hidden>
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  );
}

function Reveal({
  children,
  delay = 0,
  className = "",
}: {
  children: React.ReactNode;
  delay?: number;
  className?: string;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.4 }}
      transition={{ duration: 0.7, delay, ease: [0.16, 1, 0.3, 1] }}
      className={className}
    >
      {children}
    </motion.div>
  );
}

export default function HomePage() {
  const { triggerSound, pulseActive } = useAcoustic();

  const [name, setName] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [createdTeam, setCreatedTeam] = useState<CreatedTeam | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"install" | "code" | "command" | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  const joinCommand = useMemo(
    () => (createdTeam ? `typ join ${createdTeam.code} && typ start` : ""),
    [createdTeam]
  );

  async function createTeam(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setCreatedTeam(null);
    setIsCreating(true);
    try {
      const response = await fetch(`${apiBase}/api/teams`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, deletePassword }),
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error ?? "Could not create room.");
      setCreatedTeam(payload.team);
      triggerSound();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not create room.");
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
    <div className="relative w-full text-[#eae5d9]">
      {/* warm ambient light pooling from the top, like a desk lamp */}
      <div
        className="pointer-events-none fixed inset-0 z-0 lamp-breathe"
        style={{
          background:
            "radial-gradient(60rem 40rem at 70% -10%, rgba(217,119,70,0.10), transparent 60%), radial-gradient(50rem 50rem at 0% 100%, rgba(217,119,70,0.05), transparent 55%)",
        }}
      />

      <div className="relative z-10 mx-auto w-full max-w-6xl px-6 md:px-10">
        {/* ─── Nav ─── */}
        <header className="flex h-[72px] items-center justify-between">
          <a href="#top" className="flex items-center gap-2.5" aria-label="Cliks home">
            <Image
              src="/images/cliks-keycap.png"
              alt="Cliks"
              width={44}
              height={24}
              className="h-7 w-auto drop-shadow-[0_2px_8px_rgba(0,0,0,0.4)]"
              priority
            />
            <span className="font-mono text-sm tracking-wide text-[#eae5d9]">cliks</span>
          </a>
          <nav className="flex items-center gap-1">
            <a
              href={repoUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex h-9 items-center gap-2 rounded-full border border-[#2a2826] px-3.5 text-sm text-[#8b867c] transition-colors hover:border-[#3a3733] hover:text-[#eae5d9]"
            >
              <GitHubIcon className="w-4 h-4" />
              <span className="hidden sm:inline">GitHub</span>
            </a>
            <a
              href={xUrl}
              target="_blank"
              rel="noopener noreferrer"
              aria-label="Follow on X"
              className="flex h-9 w-9 items-center justify-center rounded-full text-[#8b867c] transition-colors hover:text-[#eae5d9]"
            >
              <XIcon className="w-4 h-4" />
            </a>
          </nav>
        </header>

        <main id="top">
          {/* ─── Hero ─── */}
          <section className="grid grid-cols-1 items-center gap-12 pt-10 pb-20 md:pt-16 md:pb-28 lg:grid-cols-[1.05fr_1fr] lg:gap-16">
            <div className="flex flex-col items-start">
              <div className="mb-7 flex items-center gap-3">
                <motion.span
                  animate={{
                    scale: pulseActive ? 1.7 : 1,
                    boxShadow: pulseActive
                      ? "0 0 28px 4px rgba(217,119,70,0.85)"
                      : "0 0 8px rgba(217,119,70,0.45)",
                  }}
                  transition={{ type: "spring", stiffness: 380, damping: 12 }}
                  className="block h-2 w-2 rounded-full bg-[#d97746]"
                />
                <span className="font-mono text-xs tracking-wide text-[#8b867c]">
                  open source · ambient coworking
                </span>
              </div>

              <h1 className="text-5xl font-semibold leading-[1.02] tracking-tight text-[#f3ede0] md:text-6xl lg:text-7xl">
                Work alone,
                <br />
                together.
              </h1>

              <p className="mt-7 max-w-[46ch] text-lg leading-relaxed text-[#a8a298]">
                Cliks turns your remote team&rsquo;s typing into warm, ambient sound. No
                keystrokes shared, no mics &mdash; just the quiet feeling of company.
              </p>

              {/* install command */}
              <div className="mt-9 w-full max-w-md">
                <div className="flex items-center gap-3 rounded-xl border border-[#2a2826] bg-[#1a1918] p-1.5 pl-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">
                  <span className="select-none font-mono text-sm text-[#d97746]">$</span>
                  <code className="scrollbar-none flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs text-[#cdc6b8] sm:text-sm">
                    {installCommand}
                  </code>
                  <button
                    type="button"
                    onClick={() => copyText(installCommand, "install")}
                    className="flex h-9 shrink-0 items-center gap-1.5 rounded-lg bg-[#211f1d] px-3 font-mono text-xs text-[#cdc6b8] transition-all hover:bg-[#2a2826] active:scale-95"
                    aria-label="Copy install command"
                  >
                    {copied === "install" ? (
                      <>
                        <Check className="h-3.5 w-3.5 text-[#d97746]" /> copied
                      </>
                    ) : (
                      <>
                        <Copy className="h-3.5 w-3.5" /> copy
                      </>
                    )}
                  </button>
                </div>
                <p className="mt-3 pl-1 font-mono text-xs text-[#5c574e]">
                  Press any key on this page to hear how it sounds.
                </p>
              </div>
            </div>

            {/* hero visual: the warm desk, with sound rippling on keystroke */}
            <Reveal className="relative" delay={0.1}>
              <div className="relative aspect-[4/5] w-full overflow-hidden rounded-2xl border border-[#2a2826] shadow-[0_30px_80px_-20px_rgba(0,0,0,0.7)] sm:aspect-[4/3] lg:aspect-[5/6]">
                <Image
                  src="/images/warm_desk_workspace.png"
                  alt="A mechanical keyboard on a warm wooden desk, lit by a brass lamp"
                  fill
                  sizes="(max-width: 1024px) 100vw, 45vw"
                  className="object-cover"
                  priority
                />
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-[#11100f] via-transparent to-transparent" />

                {/* acoustic rings ripple from over the keyboard on each keystroke */}
                <div className="pointer-events-none absolute bottom-[34%] left-1/2 -translate-x-1/2">
                  {[0, 1, 2].map((ring) => (
                    <motion.span
                      key={ring}
                      className="absolute left-1/2 top-1/2 rounded-full border border-[#d97746]"
                      style={{
                        width: 64 + ring * 64,
                        height: 64 + ring * 64,
                        marginLeft: -(32 + ring * 32),
                        marginTop: -(32 + ring * 32),
                      }}
                      animate={{
                        scale: pulseActive ? 1.18 : 1,
                        opacity: pulseActive ? 0.5 / (ring + 1) : 0.12 / (ring + 1),
                      }}
                      transition={{ type: "spring", stiffness: 220, damping: 22 }}
                    />
                  ))}
                </div>

                <div className="absolute bottom-4 left-4 flex items-center gap-2 rounded-full border border-[#2a2826] bg-[#11100f]/70 px-3 py-1.5 backdrop-blur-sm">
                  <motion.span
                    animate={{ opacity: pulseActive ? 1 : 0.6 }}
                    className="h-1.5 w-1.5 rounded-full bg-[#d97746]"
                  />
                  <span className="font-mono text-[11px] text-[#a8a298]">live presence</span>
                </div>
              </div>
            </Reveal>
          </section>

          {/* ─── Philosophy (centered editorial prose) ─── */}
          <section className="border-t border-[#2a2826] py-24 md:py-32">
            <Reveal className="mx-auto max-w-[60ch] text-center">
              <h2 className="text-2xl font-medium leading-snug text-[#f3ede0] md:text-3xl">
                Video calls are exhausting. Total silence is isolating.
              </h2>
              <p className="mt-6 text-lg leading-relaxed text-[#a8a298]">
                Cliks brings back the feeling of sitting in a room with people getting
                things done &mdash; the soft clatter of someone deep in a problem two desks
                over. Nothing is watched, nothing is recorded. You just stop feeling like
                the last person awake.
              </p>
            </Reveal>

            {/* privacy, as quiet editorial lines — not boxes */}
            <Reveal delay={0.1} className="mx-auto mt-16 max-w-2xl divide-y divide-[#2a2826] border-y border-[#2a2826]">
              {[
                {
                  k: "Timing only",
                  v: "Native OS hooks capture the rhythm of input — never which keys.",
                },
                {
                  k: "Never recorded",
                  v: "Keystrokes and content are physically never read, stored, or sent.",
                },
                {
                  k: "Batched & ephemeral",
                  v: "Pulses are batched to save bandwidth; rooms vanish when everyone leaves.",
                },
              ].map((row) => (
                <div key={row.k} className="grid grid-cols-1 gap-1 py-5 sm:grid-cols-[180px_1fr] sm:gap-6 sm:py-6">
                  <span className="font-mono text-sm text-[#d97746]">{row.k}</span>
                  <span className="text-[#a8a298]">{row.v}</span>
                </div>
              ))}
            </Reveal>
          </section>

          {/* ─── Quick start ─── */}
          <section className="border-t border-[#2a2826] py-24 md:py-32">
            <Reveal>
              <h2 className="text-3xl font-semibold tracking-tight text-[#f3ede0] md:text-4xl">
                Three commands and you&rsquo;re in.
              </h2>
              <p className="mt-3 max-w-[50ch] text-[#8b867c]">
                No account, no config, no dashboard. Install it, hop in a room, start
                listening.
              </p>
            </Reveal>

            <div className="mt-14 flex flex-col gap-12">
              {[
                {
                  n: "01",
                  title: "Install the CLI",
                  body: "One curl. Works on macOS, Linux, and Windows.",
                  cmd: "curl -fsSL …/install.sh | bash",
                },
                {
                  n: "02",
                  title: "Create or join a room",
                  body: "Generate a code below and share it, or paste a teammate's.",
                  cmd: "typ join 7K2P9",
                },
                {
                  n: "03",
                  title: "Start listening",
                  body: "The room comes alive. Hear everyone working, in their own space.",
                  cmd: "typ start",
                },
              ].map((step, i) => (
                <Reveal key={step.n} delay={i * 0.08}>
                  <div className="grid grid-cols-1 items-baseline gap-2 sm:grid-cols-[64px_1fr_auto] sm:gap-8">
                    <span className="font-mono text-sm text-[#5c574e]">{step.n}</span>
                    <div>
                      <h3 className="text-xl font-medium text-[#eae5d9]">{step.title}</h3>
                      <p className="mt-1 max-w-[42ch] text-[#8b867c]">{step.body}</p>
                    </div>
                    <code className="mt-2 inline-block w-fit rounded-lg border border-[#2a2826] bg-[#1a1918] px-3 py-1.5 font-mono text-xs text-[#cdc6b8] sm:mt-0">
                      {step.cmd}
                    </code>
                  </div>
                </Reveal>
              ))}
            </div>
          </section>

          {/* ─── Room creation ─── */}
          <section className="border-t border-[#2a2826] py-24 md:py-32">
            <div className="grid grid-cols-1 gap-12 lg:grid-cols-[1fr_1.1fr] lg:gap-20">
              <Reveal>
                <h2 className="text-3xl font-semibold tracking-tight text-[#f3ede0] md:text-4xl">
                  Spin up a room.
                </h2>
                <p className="mt-4 max-w-[42ch] text-lg leading-relaxed text-[#a8a298]">
                  A room is just a code. Make one, share it with your team, and the desk
                  fills up. The delete password is the only way to tear it down &mdash; keep
                  it somewhere safe.
                </p>
              </Reveal>

              <Reveal delay={0.1}>
                <div className="rounded-2xl border border-[#2a2826] bg-[#1a1918] p-7 shadow-[0_20px_60px_-30px_rgba(0,0,0,0.8)] md:p-9">
                  <form onSubmit={createTeam} className="flex flex-col gap-8">
                    <div className="flex flex-col gap-2">
                      <label htmlFor="team-name" className="font-mono text-xs text-[#8b867c]">
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
                        className="w-full border-b border-[#2a2826] bg-transparent pb-2 text-lg text-[#eae5d9] outline-none transition-colors placeholder:text-[#5c574e] focus:border-[#d97746]"
                      />
                    </div>

                    <div className="flex flex-col gap-2">
                      <label htmlFor="delete-password" className="font-mono text-xs text-[#8b867c]">
                        Delete password
                      </label>
                      <input
                        id="delete-password"
                        type="password"
                        value={deletePassword}
                        onChange={(e) => setDeletePassword(e.target.value)}
                        minLength={6}
                        maxLength={128}
                        placeholder="Only used to delete the room"
                        required
                        className="w-full border-b border-[#2a2826] bg-transparent pb-2 text-lg text-[#eae5d9] outline-none transition-colors placeholder:text-[#5c574e] focus:border-[#d97746]"
                      />
                    </div>

                    <button
                      type="submit"
                      disabled={isCreating}
                      className="mt-2 flex h-12 items-center justify-center rounded-lg bg-[#d97746] font-medium text-[#11100f] transition-all hover:bg-[#e0855a] active:scale-[0.98] disabled:opacity-60"
                    >
                      {isCreating ? "Generating…" : "Generate room code"}
                    </button>
                  </form>

                  <AnimatePresence>
                    {error && (
                      <motion.p
                        initial={{ opacity: 0, y: 6 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0 }}
                        className="mt-5 font-mono text-xs text-[#e0855a]"
                      >
                        {error}
                      </motion.p>
                    )}
                  </AnimatePresence>

                  <AnimatePresence>
                    {createdTeam && (
                      <motion.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: "auto" }}
                        exit={{ opacity: 0, height: 0 }}
                        className="mt-8 overflow-hidden border-t border-[#2a2826] pt-8"
                      >
                        <p className="font-mono text-xs text-[#8b867c]">
                          {createdTeam.name} is live. Share this:
                        </p>

                        <div className="mt-4 flex items-end justify-between gap-4">
                          <div>
                            <div className="font-mono text-[11px] uppercase tracking-widest text-[#5c574e]">
                              room code
                            </div>
                            <div className="font-mono text-4xl font-bold tracking-[0.15em] text-[#d97746] md:text-5xl">
                              {createdTeam.code}
                            </div>
                          </div>
                          <button
                            type="button"
                            onClick={() => copyText(createdTeam.code, "code")}
                            className="flex h-9 items-center gap-1.5 rounded-lg border border-[#2a2826] px-3 font-mono text-xs text-[#cdc6b8] transition-colors hover:border-[#3a3733] active:scale-95"
                          >
                            {copied === "code" ? (
                              <Check className="h-3.5 w-3.5 text-[#d97746]" />
                            ) : (
                              <Copy className="h-3.5 w-3.5" />
                            )}
                            {copied === "code" ? "copied" : "copy"}
                          </button>
                        </div>

                        <div className="mt-6 flex items-center gap-2 rounded-lg border border-[#2a2826] bg-[#11100f] p-1.5 pl-3">
                          <span className="select-none font-mono text-xs text-[#d97746]">$</span>
                          <code className="scrollbar-none flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs text-[#cdc6b8]">
                            {joinCommand}
                          </code>
                          <button
                            type="button"
                            onClick={() => copyText(joinCommand, "command")}
                            className="flex h-8 shrink-0 items-center justify-center rounded-md px-2.5 text-[#8b867c] transition-colors hover:text-[#eae5d9]"
                            aria-label="Copy join command"
                          >
                            {copied === "command" ? (
                              <Check className="h-4 w-4 text-[#d97746]" />
                            ) : (
                              <Copy className="h-4 w-4" />
                            )}
                          </button>
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              </Reveal>
            </div>
          </section>

          {/* ─── Support ─── */}
          <section className="border-t border-[#2a2826] py-24 text-center md:py-32">
            <Reveal className="mx-auto max-w-[52ch]">
              <h2 className="text-3xl font-semibold tracking-tight text-[#f3ede0] md:text-4xl">
                Keep the servers humming.
              </h2>
              <p className="mt-6 text-lg leading-relaxed text-[#a8a298]">
                Cliks is 100% free and open-source. I built it to work alongside my friends.
                If your team uses it every day, consider throwing a few dollars in the jar to
                cover the WebSocket server and keep it free for everyone else.
              </p>

              <div className="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row">
                <a
                  href={sponsorUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex h-11 w-full items-center justify-center gap-2 rounded-lg bg-[#d97746] px-6 font-medium text-[#11100f] transition-all hover:bg-[#e0855a] active:scale-[0.98] sm:w-auto"
                >
                  Sponsor on GitHub
                </a>
                <a
                  href={repoUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex h-11 w-full items-center justify-center gap-2 rounded-lg border border-[#2a2826] px-6 font-medium text-[#eae5d9] transition-colors hover:border-[#3a3733] sm:w-auto"
                >
                  <GitHubIcon className="w-4 h-4" />
                  Star the repo
                </a>
              </div>
            </Reveal>
          </section>

          {/* ─── Footer ─── */}
          <footer className="flex flex-col items-center justify-between gap-4 border-t border-[#2a2826] py-10 sm:flex-row">
            <div className="flex items-center gap-2.5">
              <Image
                src="/images/cliks-keycap.png"
                alt="Cliks"
                width={36}
                height={20}
                className="h-5 w-auto opacity-80"
              />
              <span className="font-mono text-xs text-[#5c574e]">
                © {new Date().getFullYear()} · MIT licensed
              </span>
            </div>
            <a
              href={xUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 font-mono text-xs text-[#5c574e] transition-colors hover:text-[#d97746]"
            >
              <XIcon className="w-3.5 h-3.5" />
              Follow the journey on X
            </a>
          </footer>
        </main>
      </div>
    </div>
  );
}
