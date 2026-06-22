"use client";

import React, { createContext, useContext, useEffect, useState, useRef } from "react";

interface AcousticContextProps {
  triggerSound: () => void;
  triggerMouseSound: () => void;
  pulseActive: boolean;
  triggerPulse: () => void;
}

const AcousticContext = createContext<AcousticContextProps | undefined>(undefined);

export function useAcoustic() {
  const context = useContext(AcousticContext);
  if (!context) {
    throw new Error("useAcoustic must be used within an AcousticProvider");
  }
  return context;
}

const keyboardUrls = [
  "/sounds/keyboard/key-01.wav",
  "/sounds/keyboard/key-02.wav",
  "/sounds/keyboard/key-03.wav",
  "/sounds/keyboard/key-04.wav",
  "/sounds/keyboard/key-05.wav",
];

const mouseUrls = ["/sounds/mouse/mouse-01.wav", "/sounds/mouse/mouse-02.wav"];

export function AcousticProvider({ children }: { children: React.ReactNode }) {
  const [pulseActive, setPulseActive] = useState<boolean>(false);
  const audioCtxRef = useRef<AudioContext | null>(null);
  const keyboardBuffersRef = useRef<AudioBuffer[]>([]);
  const mouseBuffersRef = useRef<AudioBuffer[]>([]);

  // Preload sound files on mount
  useEffect(() => {
    const loadAudio = async () => {
      try {
        const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext;
        if (!AudioContextClass) return;
        const ctx = new AudioContextClass();
        audioCtxRef.current = ctx;

        const decode = async (url: string) => {
          const response = await fetch(url);
          if (!response.ok) throw new Error();
          return ctx.decodeAudioData(await response.arrayBuffer());
        };

        // Load both packs independently so a failure in one fails silently.
        const [keyboard, mouse] = await Promise.all([
          Promise.all(keyboardUrls.map(decode)).catch(() => [] as AudioBuffer[]),
          Promise.all(mouseUrls.map(decode)).catch(() => [] as AudioBuffer[]),
        ]);

        keyboardBuffersRef.current = keyboard;
        mouseBuffersRef.current = mouse;
      } catch (error) {
        // AUDIO INTEGRITY: fail silently, never fall back to synthesized beeps.
      }
    };

    loadAudio();
  }, []);

  const triggerPulse = () => {
    setPulseActive(true);
    setTimeout(() => setPulseActive(false), 200);
  };

  // Play a random sample from a pack with organic gain/pitch jitter, like cli/src/audio.ts.
  const play = (buffers: AudioBuffer[]) => {
    const ctx = audioCtxRef.current;
    if (!ctx || buffers.length === 0) return;

    if (ctx.state === "suspended") {
      ctx.resume();
    }

    const now = ctx.currentTime;
    const buffer = buffers[Math.floor(Math.random() * buffers.length)];
    const source = ctx.createBufferSource();
    source.buffer = buffer;

    const gainNode = ctx.createGain();
    const volumeMod = 0.85 + Math.random() * 0.3; // 0.85–1.15
    gainNode.gain.setValueAtTime(volumeMod * 0.6, now);

    const pitchMod = 0.92 + Math.random() * 0.16; // 0.92–1.08
    source.playbackRate.setValueAtTime(pitchMod, now);

    source.connect(gainNode);
    gainNode.connect(ctx.destination);
    source.start(now);
  };

  const triggerSound = () => play(keyboardBuffersRef.current);
  const triggerMouseSound = () => play(mouseBuffersRef.current);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore standard modifier keys
      if (e.key === "Shift" || e.key === "Control" || e.key === "Alt" || e.key === "Meta") {
        return;
      }
      triggerSound();
      triggerPulse();
    };

    const handleMouseDown = () => {
      triggerMouseSound();
      triggerPulse();
    };

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("mousedown", handleMouseDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("mousedown", handleMouseDown);
    };
  }, []);

  return (
    <AcousticContext.Provider
      value={{
        triggerSound,
        triggerMouseSound,
        pulseActive,
        triggerPulse,
      }}
    >
      {children}
    </AcousticContext.Provider>
  );
}
