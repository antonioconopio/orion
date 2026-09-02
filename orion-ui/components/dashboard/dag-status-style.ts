import type { DagStatus } from "./types";

export const DAG_STATUS_STYLE: Record<
  DagStatus,
  { label: string; dot: string; fill: string; badge: string }
> = {
  success: {
    label: "SUCCESS",
    dot: "bg-emerald-500",
    fill: "fill-emerald-500",
    badge: "bg-emerald-500/10 text-emerald-400 ring-emerald-500/20",
  },
  failed: {
    label: "FAILED",
    dot: "bg-red-500",
    fill: "fill-red-500",
    badge: "bg-red-500/10 text-red-400 ring-red-500/20",
  },
  running: {
    label: "RUNNING",
    dot: "bg-amber-500",
    fill: "fill-amber-500",
    badge: "bg-amber-500/10 text-amber-400 ring-amber-500/20",
  },
};
