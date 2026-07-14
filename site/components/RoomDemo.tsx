"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAcoustic } from "./AcousticProvider";

/** Six peers scattered across near / mid / far rings — angles offset so they don't clump. */
const PEERS = [
  { name: "Mira", role: "design", seat: "near" as const, angle: 28, ring: 0 },
  { name: "Jules", role: "backend", seat: "mid" as const, angle: 97, ring: 1 },
  { name: "Ken", role: "mobile", seat: "far" as const, angle: 151, ring: 2 },
  { name: "Ava", role: "product", seat: "mid" as const, angle: 203, ring: 1 },
  { name: "Rio", role: "infra", seat: "far" as const, angle: 268, ring: 2 },
  { name: "Sam", role: "research", seat: "near" as const, angle: 331, ring: 0 },
];

/** Distance from center as % of stage — keeps clear air around YOU. */
const RING_R = [34, 41, 46];

type EventKind = "keyboard" | "mouse";

function buildSchedule(durationMs: number, peerCount: number): { at: number; peer: number; kind: EventKind }[] {
  const events: { at: number; peer: number; kind: EventKind }[] = [];
  let t = 280;
  while (t < durationMs - 400) {
    const peer = Math.floor(Math.random() * peerCount);
    const burst = 3 + Math.floor(Math.random() * 7);
    for (let i = 0; i < burst && t < durationMs - 200; i++) {
      events.push({ at: t, peer, kind: Math.random() < 0.1 ? "mouse" : "keyboard" });
      t += 55 + Math.floor(Math.random() * 95);
    }
    t += 180 + Math.floor(Math.random() * 520);
    if (Math.random() < 0.4) {
      const other = (peer + 1 + Math.floor(Math.random() * (peerCount - 1))) % peerCount;
      let ot = t - 200;
      for (let i = 0; i < 2 + Math.floor(Math.random() * 4); i++) {
        events.push({ at: Math.max(0, ot), peer: other, kind: "keyboard" });
        ot += 70 + Math.floor(Math.random() * 80);
      }
    }
  }
  return events.sort((a, b) => a.at - b.at);
}

function polar(angleDeg: number, rPct: number) {
  const rad = (angleDeg * Math.PI) / 180;
  return {
    left: 50 + rPct * Math.sin(rad),
    top: 50 - rPct * Math.cos(rad),
  };
}

const DEMO_MS = 14000;

export function RoomDemo() {
  const { triggerSound, triggerMouseSound } = useAcoustic();
  const [running, setRunning] = useState(false);
  const [finished, setFinished] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState(0);
  const [tip, setTip] = useState<number | null>(null);
  const [welcoming, setWelcoming] = useState(false);
  const [visiblePeers, setVisiblePeers] = useState(PEERS.length);
  const peerRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const youRef = useRef<HTMLDivElement | null>(null);
  const timersRef = useRef<number[]>([]);
  const tickRef = useRef<number | null>(null);
  const welcomeTimersRef = useRef<number[]>([]);

  const placements = useMemo(
    () =>
      PEERS.map((p) => {
        const pos = polar(p.angle, RING_R[p.ring]);
        return { ...p, ...pos };
      }),
    []
  );

  const clearTimers = useCallback(() => {
    for (const id of timersRef.current) window.clearTimeout(id);
    timersRef.current = [];
    if (tickRef.current !== null) {
      window.clearInterval(tickRef.current);
      tickRef.current = null;
    }
    for (const el of peerRefs.current) el?.classList.remove("peer-live");
    youRef.current?.classList.remove("you-listening");
  }, []);

  const clearWelcome = useCallback(() => {
    for (const id of welcomeTimersRef.current) window.clearTimeout(id);
    welcomeTimersRef.current = [];
    setWelcoming(false);
    setVisiblePeers(PEERS.length);
  }, []);

  const stop = useCallback(() => {
    clearTimers();
    setRunning(false);
  }, [clearTimers]);

  const flashPeer = useCallback((peer: number) => {
    const el = peerRefs.current[peer];
    if (!el) return;
    el.classList.add("peer-live");
    youRef.current?.classList.add("you-listening");
    window.setTimeout(() => {
      el.classList.remove("peer-live");
      youRef.current?.classList.remove("you-listening");
    }, 200);
  }, []);

  const start = useCallback(() => {
    clearWelcome();
    clearTimers();
    setFinished(false);
    setRunning(true);
    setSecondsLeft(Math.ceil(DEMO_MS / 1000));
    const started = Date.now();
    tickRef.current = window.setInterval(() => {
      setSecondsLeft(Math.max(0, Math.ceil((DEMO_MS - (Date.now() - started)) / 1000)));
    }, 250);
    for (const ev of buildSchedule(DEMO_MS, PEERS.length)) {
      timersRef.current.push(
        window.setTimeout(() => {
          flashPeer(ev.peer);
          if (ev.kind === "mouse") triggerMouseSound();
          else triggerSound();
        }, ev.at)
      );
    }
    timersRef.current.push(
      window.setTimeout(() => {
        clearTimers();
        setRunning(false);
        setFinished(true);
        setSecondsLeft(0);
      }, DEMO_MS)
    );
  }, [clearTimers, clearWelcome, flashPeer, triggerMouseSound, triggerSound]);

  useEffect(() => () => {
    clearTimers();
    clearWelcome();
  }, [clearTimers, clearWelcome]);

  // First visit: quietly show the room assembling. Audio remains click-initiated
  // so the page never surprises people or fights browser autoplay rules.
  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    if (window.sessionStorage.getItem("cliks-room-welcomed") === "1") return;
    window.sessionStorage.setItem("cliks-room-welcomed", "1");
    welcomeTimersRef.current.push(
      window.setTimeout(() => {
        setWelcoming(true);
        setVisiblePeers(0);
        PEERS.forEach((_, index) => {
          welcomeTimersRef.current.push(
            window.setTimeout(() => setVisiblePeers(index + 1), 260 + index * 230)
          );
        });
        welcomeTimersRef.current.push(
          window.setTimeout(() => setWelcoming(false), 260 + PEERS.length * 230 + 900)
        );
      }, 700)
    );
  }, []);

  useEffect(() => {
    const onExternalStart = () => {
      document.getElementById("demo")?.scrollIntoView({ behavior: "smooth", block: "nearest" });
      start();
    };
    window.addEventListener("cliks-start-demo", onExternalStart);
    return () => window.removeEventListener("cliks-start-demo", onExternalStart);
  }, [start]);

  // Your own typing on the page softly lights YOU (not a logo bounce).
  useEffect(() => {
    const onKey = () => {
      youRef.current?.classList.add("you-self");
      window.setTimeout(() => youRef.current?.classList.remove("you-self"), 140);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className={`room-orbit mx-auto w-full max-w-[420px] ${running ? "is-live" : ""} ${welcoming ? "is-welcoming" : ""}`}>
      <div className="orbit-chrome">
        <div className="status-chip">
          <span className={`status-mark ${running ? "is-on" : finished ? "is-done" : ""}`} aria-hidden />
          <span className="font-mono text-[10px] uppercase tracking-[0.16em] text-soft">
            {running ? `room live · ${secondsLeft}s` : welcoming ? "people arriving" : finished ? "room quiet" : "your room"}
          </span>
        </div>
        <span className="font-mono text-[10px] text-mute">6 peers · 3 depths</span>
      </div>

      <div className="orbit-stage relative mx-auto aspect-square w-full">
        <div className="depth-ring depth-near" aria-hidden />
        <div className="depth-ring depth-mid" aria-hidden />
        <div className="depth-ring depth-far" aria-hidden />
        {running ? <div className="orbit-haze" aria-hidden /> : null}

        {/* YOU — listener, not a logo */}
        <div ref={youRef} className="you-node" aria-label="You, listening">
          <span className="you-halo" aria-hidden />
          <span className="you-core">
            <svg viewBox="0 0 24 24" className="you-icon" aria-hidden>
              <path
                fill="currentColor"
                d="M12 3a4 4 0 0 0-4 4v2.5A6.5 6.5 0 0 0 5 15.5V17h14v-1.5A6.5 6.5 0 0 0 16 9.5V7a4 4 0 0 0-4-4Zm0 2a2 2 0 0 1 2 2v2.2c0 .3.1.5.2.7A4.5 4.5 0 0 1 16.5 14H7.5a4.5 4.5 0 0 1 2.3-3.1c.1-.2.2-.4.2-.7V7a2 2 0 0 1 2-2Zm-1 13h2v2h-2v-2Z"
              />
            </svg>
          </span>
          <span className="you-label">you</span>
        </div>

        {placements.slice(0, visiblePeers).map((peer, i) => (
          <button
            key={peer.name}
            type="button"
            ref={(el) => {
              peerRefs.current[i] = el;
            }}
            className={`orbit-node ring-${peer.seat}`}
            style={{
              left: `${peer.left}%`,
              top: `${peer.top}%`,
              animationDelay: `${i * 70}ms`,
            }}
            onMouseEnter={() => setTip(i)}
            onMouseLeave={() => setTip(null)}
            aria-label={`${peer.name}, ${peer.role}, ${peer.seat}`}
          >
            <span className="orbit-link" aria-hidden />
            <span className="orbit-avatar">{peer.name.slice(0, 1)}</span>
            <span className="orbit-strike" aria-hidden />
          </button>
        ))}

        {tip !== null ? (
          <div className="orbit-tip">
            <strong>{PEERS[tip].name}</strong>
            <span>
              {PEERS[tip].role} · {PEERS[tip].seat}
            </span>
          </div>
        ) : null}
      </div>

      <div className="orbit-actions">
        {welcoming ? (
          <p className="orbit-welcome" aria-live="polite">
            Your people settle around you. Closer sounds feel closer.
          </p>
        ) : null}
        {!running ? (
          <button type="button" onClick={start} className="btn-primary flex h-11 w-full items-center justify-center text-sm">
            {finished ? "Hear the room again" : "Hear a room (14 sec)"}
          </button>
        ) : (
          <button type="button" onClick={stop} className="btn-ghost flex h-11 w-full items-center justify-center text-sm">
            Stop
          </button>
        )}
        {finished ? (
          <p className="mt-2 text-center text-xs text-soft">
            Six people around you.{" "}
            <a href="#room" className="text-accent hover:underline">
              Create your room
            </a>
          </p>
        ) : (
          <p className="mt-2 text-center font-mono text-[10px] text-mute">
            Hover a peer · type anywhere to feel presence
          </p>
        )}
      </div>
    </div>
  );
}
