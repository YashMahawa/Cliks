"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAcoustic } from "./AcousticProvider";

/** Six peers at listener-relative distances. */
const PEERS = [
  { name: "Mira", role: "design", seat: "near" as const, angle: 28, ring: 0, statusText: "Reviewing PR #104" },
  { name: "Jules", role: "backend", seat: "mid" as const, angle: 97, ring: 1, statusText: "Refactoring WebSocket hub" },
  { name: "Ken", role: "mobile", seat: "far" as const, angle: 151, ring: 2, statusText: "" },
  { name: "Ava", role: "product", seat: "mid" as const, angle: 203, ring: 1, statusText: "Sprint planning" },
  { name: "Rio", role: "infra", seat: "far" as const, angle: 268, ring: 2, statusText: "Scaling relay cluster" },
  { name: "Sam", role: "research", seat: "near" as const, angle: 331, ring: 0, statusText: "" },
];

/** Seat centers leave room for full name labels at narrow widths. */
const RING_R = [28, 33, 37];

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
    <div className="room-orbit mx-auto w-full max-w-[540px]">
      <div className="orbit-chrome">
        <span className={`status-chip ${running ? "is-on" : ""}`} aria-live="polite">
          {running ? `Now playing / ${secondsLeft}s` : welcoming ? "Taking seats" : finished ? "Room quiet" : "Listening room"}
        </span>
        <span className="orbit-meta" aria-live="polite">
          {tip === null ? "6 people / spatial sound" : `${PEERS[tip].name} / ${PEERS[tip].role} / ${PEERS[tip].seat}`}
        </span>
      </div>

      <div className="orbit-stage relative mx-auto w-full">
        <div ref={youRef} className="you-node" aria-label="You, listening">
          <span className="you-core">YOUR DESK</span>
        </div>

        {placements.slice(0, visiblePeers).map((peer, i) => (
          <button
            key={peer.name}
            type="button"
            ref={(el) => {
              peerRefs.current[i] = el;
            }}
            className="orbit-node"
            style={{
              left: `${peer.left}%`,
              top: `${peer.top}%`,
              animationDelay: `${i * 70}ms`,
            }}
            onMouseEnter={() => setTip(i)}
            onMouseLeave={() => setTip(null)}
            onFocus={() => setTip(i)}
            onBlur={() => setTip(null)}
            onClick={() => setTip(i)}
            aria-label={`${peer.name}, ${peer.role}, ${peer.seat}`}
          >
            <span className="orbit-avatar">{peer.name}</span>
          </button>
        ))}
        {tip !== null ? (
          <div className="orbit-tip">
            <strong>{PEERS[tip].name}</strong>
            <span>
              {PEERS[tip].role} · {PEERS[tip].seat}
              {PEERS[tip].statusText ? ` · ${PEERS[tip].statusText}` : ""}
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
        ) : null}
      </div>
    </div>
  );
}
