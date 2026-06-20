"use client";

import React, { createContext, useContext, useEffect, useState, useRef } from "react";

interface AcousticContextProps {
  triggerSound: () => void;
  pulseActive: boolean;
  triggerPulse: () => void;
  typingProgress: number;
  setTypingProgress: React.Dispatch<React.SetStateAction<number>>;
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
  const [pulseActive, setPulseActive] = useState<boolean>(false);
  const [typingProgress, setTypingProgress] = useState<number>(0);
  const [buffers, setBuffers] = useState<AudioBuffer[]>([]);
  const audioCtxRef = useRef<AudioContext | null>(null);
  const buffersRef = useRef<AudioBuffer[]>([]);

  // Preload sound files on mount
  useEffect(() => {
    const loadAudio = async () => {
      try {
        const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext;
        if (!AudioContextClass) return;
        const ctx = new AudioContextClass();
        audioCtxRef.current = ctx;

        const soundUrls = [
          "/sounds/keyboard/key-01.wav",
          "/sounds/keyboard/key-02.wav",
          "/sounds/keyboard/key-03.wav",
          "/sounds/keyboard/key-04.wav",
          "/sounds/keyboard/key-05.wav",
        ];

        const loadedBuffers = await Promise.all(
          soundUrls.map(async (url) => {
            const response = await fetch(url);
            if (!response.ok) throw new Error();
            const arrayBuffer = await response.arrayBuffer();
            return await ctx.decodeAudioData(arrayBuffer);
          })
        );

        setBuffers(loadedBuffers);
        buffersRef.current = loadedBuffers;
      } catch (error) {
        // AUDIO INTEGRITY: fail silently
      }
    };

    loadAudio();
  }, []);

  const triggerPulse = () => {
    setPulseActive(true);
    setTimeout(() => setPulseActive(false), 200);
  };

  const triggerSound = () => {
    const ctx = audioCtxRef.current;
    const currentBuffers = buffersRef.current;
    if (!ctx || currentBuffers.length === 0) return;

    if (ctx.state === "suspended") {
      ctx.resume();
    }

    const now = ctx.currentTime;
    // Select a random WAV sample
    const buffer = currentBuffers[Math.floor(Math.random() * currentBuffers.length)];
    const source = ctx.createBufferSource();
    source.buffer = buffer;

    // Gain (Volume) Modulation: 0.85 to 1.15
    const gainNode = ctx.createGain();
    const volumeMod = 0.85 + Math.random() * 0.3;
    gainNode.gain.setValueAtTime(volumeMod * 0.6, now);

    // Playback Rate (Pitch) Modulation: 0.92 to 1.08
    const pitchMod = 0.92 + Math.random() * 0.16;
    source.playbackRate.setValueAtTime(pitchMod, now);

    source.connect(gainNode);
    gainNode.connect(ctx.destination);
    source.start(now);
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is writing in input fields
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        triggerSound();
        return;
      }

      // Ignore standard modifier keys
      if (e.key === "Shift" || e.key === "Control" || e.key === "Alt" || e.key === "Meta") {
        return;
      }
      
      triggerSound();
      triggerPulse();

      setTypingProgress((prev) => {
        if (prev < 10) {
          return prev + 1;
        }
        return prev;
      });
    };

    const handleMouseDown = (e: MouseEvent) => {
      // Ignore interactive HTML elements
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        e.target instanceof HTMLButtonElement ||
        e.target instanceof HTMLAnchorElement
      ) {
        triggerSound();
        return;
      }

      triggerSound();
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
        pulseActive,
        triggerPulse,
        typingProgress,
        setTypingProgress,
      }}
    >
      {children}
    </AcousticContext.Provider>
  );
}
