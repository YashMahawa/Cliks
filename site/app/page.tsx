import Image from "next/image";
import { RoomForm } from "../components/RoomForm";
import { RoomDemo } from "../components/RoomDemo";
import { HeroCtas } from "../components/HeroCtas";
import { InstallOptions } from "../components/InstallOptions";
const repoUrl = "https://github.com/YashMahawa/Cliks";
const xUrl = "https://x.com/MahawarYas27492";

function GitHubIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={`${className} fill-current`} aria-hidden>
      <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
    </svg>
  );
}

function XIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={`${className} fill-current`} aria-hidden>
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  );
}

const trust = [
  { k: "No keystrokes", v: "Only that someone typed — never what" },
  { k: "No microphone", v: "Local samples, not a live audio stream" },
  { k: "No account", v: "A room is a code. That is it." },
  { k: "Open source", v: "Read the CLI. Audit the relay." },
];

const steps = [
  {
    n: "01",
    title: "Hear it",
    body: "Play the room. That click is the whole pitch — ambient company without a call.",
  },
  {
    n: "02",
    title: "Make a room",
    body: "Generate a code in about twenty seconds. No signup, no dashboard, no SSO.",
  },
  {
    n: "03",
    title: "Share two lines",
    body: "Teammates install once, join with the code, and the desk fills up.",
  },
];

const faqs = [
  {
    q: "Can you read what I type?",
    a: "No. Cliks immediately reduces local input to ‘keyboard activity happened.’ Key values, codes, and content never reach the relay.",
  },
  {
    q: "Is this a microphone stream?",
    a: "No. Teammates receive tiny timing pulses. Your computer plays local keyboard and mouse samples.",
  },
  {
    q: "Can I use it alone or offline?",
    a: "Yes. Run cliks solo for a local simulated desk with 1–12 coworkers and personal rain, cafe, or deep-focus room tones. It uses no team, capture permission, or internet.",
  },
  {
    q: "What gets stored?",
    a: "Room name, code, and a password hash. Live presence is memory-only. Rooms expire after 48 hours without a live connection; reconnecting refreshes the clock.",
  },
	{
		q: "Can I use my own server?",
		a: "Yes. Paste its HTTPS URL under Cliks → More → Server. The shared public relay stays at 20 people and 500 ms batching; self-hosting unlocks configurable room capacity and 100–2000 ms batches.",
	},
];

export default function HomePage() {
  return (
    <div className="relative w-full">
      <div className="ambient-field" aria-hidden />

      <header className="site-header">
        <div className="shell flex h-16 items-center justify-between sm:h-[4.25rem]">
          <a href="#top" className="brand-link" aria-label="Cliks home">
            <Image
              src="/images/cliks-keycap.png"
              alt=""
              width={120}
              height={66}
              className="brand-logo"
              priority
            />
            <span className="brand-word">Cliks</span>
          </a>
          <nav className="flex items-center gap-2">
            <a
              href="#demo"
              className="hidden h-9 items-center px-3 text-sm text-soft transition-colors hover:text-fg md:flex"
            >
              Demo
            </a>
            <a href="#room" className="btn-primary hidden h-9 items-center px-3.5 text-sm sm:flex">
              Create room
            </a>
            <a
              href={repoUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="btn-ghost flex h-9 items-center gap-2 px-3 text-sm text-soft hover:text-fg"
            >
              <GitHubIcon />
              <span className="hidden sm:inline">GitHub</span>
            </a>
            <a
              href={xUrl}
              target="_blank"
              rel="noopener noreferrer"
              aria-label="Follow on X"
              className="flex h-9 w-9 items-center justify-center text-soft hover:text-fg"
            >
              <XIcon />
            </a>
          </nav>
        </div>
      </header>

      <main id="top" className="relative z-[1]">
        <section className="hero-section relative overflow-hidden border-b border-line">
          <div className="pointer-events-none absolute inset-0">
            <Image
              src="/images/warm_desk_workspace.png"
              alt=""
              fill
              sizes="100vw"
              className="macro-desk object-cover object-[50%_60%] opacity-[0.24]"
              priority
            />
            <div
              className="absolute inset-0"
              style={{
                background:
                  "radial-gradient(circle at 72% 42%, transparent 0%, var(--bg) 66%), linear-gradient(to top, var(--bg), transparent 42%)",
              }}
            />
            <div
              className="pulse-wash absolute inset-0"
              style={{
                background:
                  "radial-gradient(28rem 22rem at 78% 40%, rgba(232,184,109,0.16), transparent 64%)",
              }}
            />
          </div>

          <div className="shell relative grid grid-cols-1 items-center gap-10 py-12 sm:py-16 lg:grid-cols-12 lg:gap-8 xl:py-20">
            <div className="flex flex-col justify-center lg:col-span-6">
              <div className="reveal mb-5">
                <span className="live-kicker">
                  <span className="status-mark is-on" aria-hidden />
                  <span className="font-mono text-[11px] uppercase tracking-[0.16em] text-soft">
                    ambient coworking · free
                  </span>
                </span>
              </div>

              <h1 className="reveal reveal-d1 max-w-[20ch] text-[clamp(2.6rem,4.2vw+0.5rem,4.4rem)] font-extrabold leading-[0.98] tracking-[-0.04em]">
                Your remote team, typing in the next room.
              </h1>

              <p className="reveal reveal-d2 mt-5 max-w-[40ch] text-base leading-relaxed text-soft sm:text-lg">
                Cliks turns anonymous keyboard and mouse activity into ambient sound. You sit in the
                middle of the room — people work around you, never sharing a single keystroke.
              </p>

              <div className="reveal reveal-d3">
                <HeroCtas />
              </div>
              <p className="mt-4 font-mono text-[11px] text-mute">
                Type anywhere on this page — same samples the CLI plays.
              </p>

              <ul className="mt-10 grid grid-cols-2 gap-x-5 gap-y-5 border-t border-line pt-8 sm:grid-cols-4">
                {trust.map((item) => (
                  <li key={item.k} className="trust-tile min-w-0">
                    <p className="text-sm font-semibold text-fg">{item.k}</p>
                    <p className="mt-1 text-xs leading-snug text-mute">{item.v}</p>
                  </li>
                ))}
              </ul>
            </div>

            <div id="demo" className="lg:col-span-6">
              <div className="lg:sticky lg:top-24">
                <RoomDemo />
              </div>
            </div>
          </div>
        </section>

        <section className="border-b border-line">
          <div className="shell py-14 sm:py-16">
            <p className="section-kicker">How it works</p>
            <h2 className="mt-3 max-w-[22ch] text-[clamp(1.75rem,2.4vw,2.5rem)] font-bold tracking-tight">
              From curious to in a room in under a minute.
            </h2>
            <ol className="mt-10 grid grid-cols-1 gap-3 md:grid-cols-3 md:gap-4">
              {steps.map((step) => (
                <li key={step.n} className="step-card p-7 sm:p-8">
                  <span className="step-num font-mono text-3xl font-bold tracking-tight">{step.n}</span>
                  <h3 className="mt-4 text-xl font-bold tracking-tight">{step.title}</h3>
                  <p className="mt-2 text-sm leading-relaxed text-soft">{step.body}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="room" className="border-b border-line">
          <div className="shell grid grid-cols-1 gap-0 lg:grid-cols-12">
            <div className="flex flex-col justify-center border-b border-line py-14 lg:col-span-5 lg:border-b-0 lg:border-r lg:py-20 lg:pr-10">
              <p className="section-kicker">Start a room</p>
              <h2 className="mt-3 text-[clamp(1.75rem,2.4vw,2.5rem)] font-bold tracking-tight">
                Make a room. Text the code. Done.
              </h2>
              <p className="mt-4 max-w-[36ch] text-base leading-relaxed text-soft sm:text-lg">
                No invites, no SSO. One person creates a room; everyone else pastes a code into the
                CLI.
              </p>
			  <p className="mt-3 max-w-[42ch] font-mono text-xs leading-relaxed text-mute">
				Unused rooms clean themselves up after 48 hours. Any live connection refreshes the clock.
			  </p>
            </div>
            <div className="py-14 lg:col-span-7 lg:py-20 lg:pl-10">
              <RoomForm />
            </div>
          </div>
        </section>

        <section id="install" className="border-b border-line">
          <div className="shell py-14 sm:py-16">
            <div className="grid grid-cols-1 items-end gap-8 lg:grid-cols-12">
              <div className="lg:col-span-5">
                <p className="section-kicker">CLI</p>
                <h2 className="mt-3 text-[clamp(1.75rem,2.4vw,2.5rem)] font-bold tracking-tight">
                  One command. No toolchain.
                </h2>
                <p className="mt-3 max-w-[40ch] text-soft">
                  After install:{" "}
                  <code className="font-mono text-accent">cliks join YOUR-CODE</code>
                </p>
              </div>
              <div className="lg:col-span-7">
                <InstallOptions />
                <p className="mt-3 font-mono text-[11px] text-mute">
                  Prefer to read it first?{" "}
                  <a
                    href="https://github.com/YashMahawa/Cliks/blob/main/cli/install.sh"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-soft underline-offset-2 hover:text-accent hover:underline"
                  >
                    install.sh on GitHub
                  </a>
                </p>
              </div>
            </div>
          </div>
        </section>

        <section className="border-b border-line">
          <div className="shell py-14 sm:py-16">
            <p className="section-kicker">Privacy</p>
            <h2 className="mt-3 max-w-[18ch] text-[clamp(1.75rem,2.4vw,2.5rem)] font-bold tracking-tight">
              The creepy question, answered.
            </h2>
            <div className="mt-10 grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
              {faqs.map((item) => (
                <div key={item.q} className="faq-cell p-7 sm:p-8">
                  <h3 className="text-lg font-bold tracking-tight">{item.q}</h3>
                  <p className="mt-3 text-sm leading-relaxed text-soft">{item.a}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <footer className="shell flex flex-col items-start justify-between gap-4 py-10 sm:flex-row sm:items-center">
          <div className="flex items-center gap-3">
            <Image
              src="/images/cliks-keycap.png"
              alt="Cliks"
              width={64}
              height={36}
              className="h-8 w-auto opacity-80"
            />
            <span className="font-mono text-xs text-mute">MIT licensed</span>
          </div>
          <div className="flex items-center gap-5 font-mono text-xs text-mute">
            <a href={`${repoUrl}/stargazers`} target="_blank" rel="noopener noreferrer" className="hover:text-accent">
              Star on GitHub
            </a>
            <a href="https://github.com/sponsors/YashMahawa" target="_blank" rel="noopener noreferrer" className="hover:text-accent">
              Support
            </a>
            <a href={xUrl} target="_blank" rel="noopener noreferrer" className="hover:text-accent">
              Follow on X
            </a>
          </div>
        </footer>
      </main>
    </div>
  );
}
