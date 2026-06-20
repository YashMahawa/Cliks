"use client";

import React, { createContext, useContext, useEffect, useState, useRef } from "react";
import { AnimatePresence, motion } from "motion/react";

interface Ripple {
  id: number;
  x: number;
  y: number;
}

interface AcousticContextProps {
  activeSwitch: string;
  setActiveSwitch: (type: string) => void;
  triggerSound: () => void;
  pulseActive: boolean;
  hasEntered: boolean;
  setHasEntered: (val: boolean) => void;
}

const AcousticContext = createContext<AcousticContextProps | undefined>(undefined);

export function useAcoustic() {
  const context = useContext(AcousticContext);
  if (!context) {
    throw new Error("useAcoustic must be used within an AcousticProvider");
  }
  return context;
}

export function AcousticProvider({ children }: { children: React.ReactNode }) {
  const [activeSwitch, setActiveSwitch] = useState<string>("base");
  const [pulseActive, setPulseActive] = useState<boolean>(false);
  const [hasEntered, setHasEntered] = useState<boolean>(false);
  const [ripples, setRipples] = useState<Ripple[]>([]);
  const audioCtxRef = useRef<AudioContext | null>(null);

  // Initialize Web Audio Context on user gesture
  const initAudio = () => {
    if (!audioCtxRef.current) {
      audioCtxRef.current = new (window.AudioContext || (window as any).webkitAudioContext)();
    }
    if (audioCtxRef.current.state === "suspended") {
      audioCtxRef.current.resume();
    }
  };

  const createNoiseNode = (audioCtx: AudioContext, now: number, duration: number, lowPassFreq: number, highPassFreq: number) => {
    const bufferSize = audioCtx.sampleRate * duration;
    const buffer = audioCtx.createBuffer(1, bufferSize, audioCtx.sampleRate);
    const data = buffer.getChannelData(0);
    for (let i = 0; i < bufferSize; i++) {
      data[i] = Math.random() * 2 - 1;
    }
    const noise = audioCtx.createBufferSource();
    noise.buffer = buffer;
    
    const lp = audioCtx.createBiquadFilter();
    lp.type = "lowpass";
    lp.frequency.setValueAtTime(lowPassFreq, now);
    
    const hp = audioCtx.createBiquadFilter();
    hp.type = "highpass";
    hp.frequency.setValueAtTime(highPassFreq, now);
    
    noise.connect(hp);
    hp.connect(lp);
    return { source: noise, destination: lp };
  };

  const triggerSound = () => {
    initAudio();
    const audioCtx = audioCtxRef.current;
    if (!audioCtx) return;

    const now = audioCtx.currentTime;

    if (activeSwitch === "base") {
      // Linear Switch Clack
      const osc = audioCtx.createOscillator();
      const gainNode = audioCtx.createGain();
      
      osc.type = "sine";
      osc.frequency.setValueAtTime(750, now);
      osc.frequency.exponentialRampToValueAtTime(120, now + 0.04);
      
      gainNode.gain.setValueAtTime(0.12, now);
      gainNode.gain.exponentialRampToValueAtTime(0.001, now + 0.045);
      
      const noise = createNoiseNode(audioCtx, now, 0.025, 2200, 900);
      const noiseGain = audioCtx.createGain();
      noiseGain.gain.setValueAtTime(0.06, now);
      noiseGain.gain.exponentialRampToValueAtTime(0.001, now + 0.018);
      
      noise.destination.connect(noiseGain);
      noiseGain.connect(audioCtx.destination);
      noise.source.start(now);
      
      osc.connect(gainNode);
      gainNode.connect(audioCtx.destination);
      osc.start(now);
      osc.stop(now + 0.05);

    } else if (activeSwitch === "model_m") {
      // Heavy Buckling Spring metallic ping + snap
      const osc1 = audioCtx.createOscillator();
      const osc2 = audioCtx.createOscillator();
      const gainNode = audioCtx.createGain();
      
      osc1.type = "sine";
      osc1.frequency.setValueAtTime(2900, now);
      osc1.frequency.setValueAtTime(2600, now + 0.015);
      
      osc2.type = "triangle";
      osc2.frequency.setValueAtTime(380, now);
      osc2.frequency.exponentialRampToValueAtTime(70, now + 0.035);
      
      gainNode.gain.setValueAtTime(0.1, now);
      gainNode.gain.exponentialRampToValueAtTime(0.001, now + 0.11);
      
      const noise = createNoiseNode(audioCtx, now, 0.04, 4800, 1800);
      const noiseGain = audioCtx.createGain();
      noiseGain.gain.setValueAtTime(0.12, now);
      noiseGain.gain.exponentialRampToValueAtTime(0.001, now + 0.025);
      
      noise.destination.connect(noiseGain);
      noiseGain.connect(audioCtx.destination);
      noise.source.start(now);
      
      osc1.connect(gainNode);
      osc2.connect(gainNode);
      gainNode.connect(audioCtx.destination);
      
      osc1.start(now);
      osc2.start(now);
      osc1.stop(now + 0.12);
      osc2.stop(now + 0.12);

    } else if (activeSwitch === "library") {
      // Muffled deep room thud
      const osc = audioCtx.createOscillator();
      const gainNode = audioCtx.createGain();
      
      osc.type = "triangle";
      osc.frequency.setValueAtTime(180, now);
      osc.frequency.exponentialRampToValueAtTime(55, now + 0.05);
      
      gainNode.gain.setValueAtTime(0.1, now);
      gainNode.gain.exponentialRampToValueAtTime(0.001, now + 0.055);
      
      const filter = audioCtx.createBiquadFilter();
      filter.type = "lowpass";
      filter.frequency.setValueAtTime(280, now);
      
      osc.connect(filter);
      filter.connect(gainNode);
      gainNode.connect(audioCtx.destination);
      
      osc.start(now);
      osc.stop(now + 0.06);

    } else if (activeSwitch === "rain") {
      // High-pitched rain droplet on window
      const osc = audioCtx.createOscillator();
      const gainNode = audioCtx.createGain();
      
      osc.type = "sine";
      osc.frequency.setValueAtTime(4200, now);
      osc.frequency.exponentialRampToValueAtTime(1400, now + 0.015);
      
      gainNode.gain.setValueAtTime(0.06, now);
      gainNode.gain.exponentialRampToValueAtTime(0.001, now + 0.018);
      
      const noise = createNoiseNode(audioCtx, now, 0.12, 750, 250);
      const noiseGain = audioCtx.createGain();
      noiseGain.gain.setValueAtTime(0.03, now);
      noiseGain.gain.exponentialRampToValueAtTime(0.001, now + 0.12);
      
      noise.destination.connect(noiseGain);
      noiseGain.connect(audioCtx.destination);
      noise.source.start(now);
      
      osc.connect(gainNode);
      gainNode.connect(audioCtx.destination);
      
      osc.start(now);
      osc.stop(now + 0.025);
    }
  };

  const handleInteraction = (clientX?: number, clientY?: number) => {
    // 1. Play sound
    triggerSound();

    // 2. Pulse accent metric
    setPulseActive(true);
    setTimeout(() => setPulseActive(false), 200);

    // 3. Add transient ripple
    const id = Date.now() + Math.random();
    const x = clientX !== undefined ? clientX : Math.random() * window.innerWidth;
    const y = clientY !== undefined ? clientY : Math.random() * window.innerHeight;

    setRipples((prev) => [...prev, { id, x, y }]);
    setTimeout(() => {
      setRipples((prev) => prev.filter((r) => r.id !== id));
    }, 850);

    // 4. Set entered status
    if (!hasEntered) {
      setHasEntered(true);
    }
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Avoid triggering on modifier keys or standard typing if form is active
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        // Still play sound for tactile feel but don't intercept standard form keys for navigation
        triggerSound();
        setPulseActive(true);
        setTimeout(() => setPulseActive(false), 200);
        return;
      }
      handleInteraction();
    };

    const handleMouseDown = (e: MouseEvent) => {
      // Exclude clicks on form inputs to prevent double triggering or input issues
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        e.target instanceof HTMLButtonElement ||
        e.target instanceof HTMLAnchorElement
      ) {
        triggerSound();
        setPulseActive(true);
        setTimeout(() => setPulseActive(false), 200);
        return;
      }
      handleInteraction(e.clientX, e.clientY);
    };

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("mousedown", handleMouseDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("mousedown", handleMouseDown);
    };
  }, [activeSwitch, hasEntered]);

  return (
    <AcousticContext.Provider
      value={{
        activeSwitch,
        setActiveSwitch,
        triggerSound,
        pulseActive,
        hasEntered,
        setHasEntered,
      }}
    >
      {/* Transient Glow ripples layer */}
      <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
        <AnimatePresence>
          {ripples.map((ripple) => (
            <motion.div
              key={ripple.id}
              initial={{ opacity: 0.8, scale: 0 }}
              animate={{ opacity: 0, scale: 4 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.8, ease: "easeOut" }}
              style={{
                position: "absolute",
                left: ripple.x,
                top: ripple.y,
                width: "250px",
                height: "250px",
                transform: "translate(-50%, -50%)",
                background: "radial-gradient(circle, rgba(34, 211, 238, 0.12) 0%, rgba(34, 211, 238, 0) 70%)",
                borderRadius: "50%",
              }}
            />
          ))}
        </AnimatePresence>
      </div>

      <div className="relative z-10 flex min-h-[100dvh] flex-col">
        {children}
      </div>
    </AcousticContext.Provider>
  );
}
