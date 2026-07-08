"use client";

import React, { createContext, useCallback, useContext, useEffect, useRef } from "react";

interface AcousticContextProps {
  triggerSound: () => void;
  triggerMouseSound: () => void;
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

const mouseUrls = ["/sounds/mouse/mouse-01.wav"];

/** Flash a CSS hook without React re-renders (avoids lag on rapid keystrokes). */
function flashPresence() {
  const root = document.documentElement;
  root.dataset.cliksPulse = "1";
  window.clearTimeout((flashPresence as { t?: number }).t);
  (flashPresence as { t?: number }).t = window.setTimeout(() => {
    delete root.dataset.cliksPulse;
  }, 160);
}

export function AcousticProvider({ children }: { children: React.ReactNode }) {
  const audioCtxRef = useRef<AudioContext | null>(null);
  const keyboardBuffersRef = useRef<AudioBuffer[]>([]);
  const mouseBuffersRef = useRef<AudioBuffer[]>([]);
  const readyRef = useRef(false);

  useEffect(() => {
    let cancelled = false;

    const loadAudio = async () => {
      try {
        const AudioContextClass = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
        if (!AudioContextClass) return;
        const ctx = new AudioContextClass();
        if (cancelled) {
          void ctx.close();
          return;
        }
        audioCtxRef.current = ctx;

        const decode = async (url: string) => {
          const response = await fetch(url);
          if (!response.ok) throw new Error("audio fetch failed");
          return ctx.decodeAudioData(await response.arrayBuffer());
        };

        const [keyboard, mouse] = await Promise.all([
          Promise.all(keyboardUrls.map(decode)).catch(() => [] as AudioBuffer[]),
          Promise.all(mouseUrls.map(decode)).catch(() => [] as AudioBuffer[]),
        ]);

        if (cancelled) return;
        keyboardBuffersRef.current = keyboard;
        mouseBuffersRef.current = mouse;
        readyRef.current = true;
      } catch {
        // AUDIO INTEGRITY: fail silently, never fall back to synthesized beeps.
      }
    };

    void loadAudio();
    return () => {
      cancelled = true;
      const ctx = audioCtxRef.current;
      audioCtxRef.current = null;
      if (ctx) void ctx.close();
    };
  }, []);

  const play = useCallback((buffers: AudioBuffer[]) => {
    const ctx = audioCtxRef.current;
    if (!ctx || buffers.length === 0) return;

    if (ctx.state === "suspended") {
      void ctx.resume();
    }

    const now = ctx.currentTime;
    const buffer = buffers[Math.floor(Math.random() * buffers.length)];
    const source = ctx.createBufferSource();
    source.buffer = buffer;

    const gainNode = ctx.createGain();
    const volumeMod = 0.85 + Math.random() * 0.3;
    gainNode.gain.setValueAtTime(volumeMod * 0.6, now);

    const pitchMod = 0.92 + Math.random() * 0.16;
    source.playbackRate.setValueAtTime(pitchMod, now);

    source.connect(gainNode);
    gainNode.connect(ctx.destination);
    source.start(now);
  }, []);

  const triggerSound = useCallback(() => {
    play(keyboardBuffersRef.current);
    flashPresence();
  }, [play]);

  const triggerMouseSound = useCallback(() => {
    play(mouseBuffersRef.current);
    flashPresence();
  }, [play]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.repeat) return;
      if (e.key === "Shift" || e.key === "Control" || e.key === "Alt" || e.key === "Meta") {
        return;
      }
      // Don't play over form fields typing... actually product wants page-wide demo
      // including forms - keep global. Skip if composing IME.
      if (e.isComposing) return;
      triggerSound();
    };

    const handleMouseDown = (e: MouseEvent) => {
      // Only primary button for mouse sample (matches CLI left/right click intent)
      if (e.button !== 0 && e.button !== 2) return;
      triggerMouseSound();
    };

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("mousedown", handleMouseDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("mousedown", handleMouseDown);
    };
  }, [triggerSound, triggerMouseSound]);

  return (
    <AcousticContext.Provider value={{ triggerSound, triggerMouseSound }}>
      {children}
    </AcousticContext.Provider>
  );
}
