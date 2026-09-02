import type { LogLevel, TaskStatus, WorkerStatus } from "./types";

/**
 * Visual styling per task/run status. `node` classes are used by the graph
 * view; `badge`/`dot` mirror the dashboard's DAG status styling so the whole
 * app reads consistently. Kept legible at a glance across all five states.
 */
export const TASK_STATUS_STYLE: Record<
  TaskStatus,
  {
    label: string;
    dot: string;
    fill: string;
    badge: string;
    node: string;
    accent: string;
  }
> = {
  pending: {
    label: "PENDING",
    dot: "bg-zinc-500",
    fill: "fill-zinc-500",
    badge: "bg-zinc-500/10 text-zinc-300 ring-zinc-500/20",
    node: "border-zinc-600/60 bg-zinc-800/40 text-zinc-300",
    accent: "text-zinc-400",
  },
  running: {
    label: "RUNNING",
    dot: "bg-amber-500",
    fill: "fill-amber-500",
    badge: "bg-amber-500/10 text-amber-400 ring-amber-500/20",
    node: "border-amber-500/70 bg-amber-500/10 text-amber-200",
    accent: "text-amber-400",
  },
  success: {
    label: "SUCCESS",
    dot: "bg-emerald-500",
    fill: "fill-emerald-500",
    badge: "bg-emerald-500/10 text-emerald-400 ring-emerald-500/20",
    node: "border-emerald-500/60 bg-emerald-500/10 text-emerald-200",
    accent: "text-emerald-400",
  },
  failed: {
    label: "FAILED",
    dot: "bg-red-500",
    fill: "fill-red-500",
    badge: "bg-red-500/10 text-red-400 ring-red-500/20",
    node: "border-red-500/70 bg-red-500/10 text-red-200",
    accent: "text-red-400",
  },
  retrying: {
    label: "RETRYING",
    dot: "bg-violet-500",
    fill: "fill-violet-500",
    badge: "bg-violet-500/10 text-violet-300 ring-violet-500/20",
    node: "border-violet-500/70 bg-violet-500/10 text-violet-200",
    accent: "text-violet-400",
  },
};

export const WORKER_STATUS_STYLE: Record<
  WorkerStatus,
  { label: string; dot: string; badge: string }
> = {
  online: {
    label: "ONLINE",
    dot: "bg-emerald-500",
    badge: "bg-emerald-500/10 text-emerald-400 ring-emerald-500/20",
  },
  busy: {
    label: "BUSY",
    dot: "bg-amber-500",
    badge: "bg-amber-500/10 text-amber-400 ring-amber-500/20",
  },
  idle: {
    label: "IDLE",
    dot: "bg-sky-500",
    badge: "bg-sky-500/10 text-sky-300 ring-sky-500/20",
  },
  offline: {
    label: "OFFLINE",
    dot: "bg-zinc-600",
    badge: "bg-zinc-500/10 text-zinc-400 ring-zinc-500/20",
  },
};

export const LOG_LEVEL_STYLE: Record<LogLevel, { label: string; className: string }> = {
  debug: { label: "DEBUG", className: "text-zinc-500" },
  info: { label: "INFO", className: "text-sky-400" },
  warn: { label: "WARN", className: "text-amber-400" },
  error: { label: "ERROR", className: "text-red-400" },
};
