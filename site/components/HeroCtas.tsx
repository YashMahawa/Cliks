"use client";

export function HeroCtas() {
  return (
    <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:items-center">
      <button
        type="button"
        onClick={() => window.dispatchEvent(new Event("cliks-start-demo"))}
        className="btn-primary flex h-12 items-center justify-center px-6 text-[15px]"
      >
        Hear a room first
      </button>
      <a
        href="#room"
        className="btn-ghost flex h-12 items-center justify-center px-6 text-[15px] font-medium"
      >
        Create a free room
      </a>
    </div>
  );
}
