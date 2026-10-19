"use client";

import { cn } from "@/lib/utils";
import { useAppSelector } from "@/store/hooks";

const STYLE = {
  connected: { label: "LIVE", dot: "bg-emerald-500", text: "text-emerald-400" },
  connecting: { label: "CONNECTING", dot: "bg-amber-500", text: "text-amber-400" },
  disconnected: { label: "OFFLINE", dot: "bg-red-500", text: "text-red-400" },
} as const;

/** Shows whether the live-updates WebSocket is connected. */
export function LiveIndicator() {
  const state = useAppSelector((s) => s.connection);
  const style = STYLE[state];
  return (
    <span
      className={cn("inline-flex items-center gap-1.5 font-mono text-[0.65rem] tracking-wider", style.text)}
      title={state === "connected" ? "Updates arrive live" : "Not receiving live updates"}
    >
      <span className={cn("size-1.5 rounded-full", style.dot, state === "connected" && "animate-pulse")} />
      {style.label}
    </span>
  );
}
