// Central domain types for Orion. These shape the static example data and
// component props so real state can be wired in later without restructuring.

/** A DAG's status is its last run's outcome; "idle" means it has never run. */
export type DagStatus = "success" | "failed" | "running" | "idle";

export type TaskStatus =
  | "pending"
  | "queued"
  | "running"
  | "success"
  | "failed"
  | "retrying"
  | "skipped"
  | "cancelled";

/** A run's overall status shares the task status vocabulary. */
export type RunStatus = TaskStatus;

/** Live workers are busy or idle; a worker that stops heartbeating just disappears. */
export type WorkerStatus = "online" | "busy" | "idle" | "offline";

export type LogLevel = "debug" | "info" | "warn" | "error";

export interface Dag {
  id: string;
  name: string;
  /** Cron expression, or "manual" when the DAG has no schedule. */
  schedule: string;
  status: DagStatus;
  lastRun: string;
  runNumber: string;
  avgDuration: string;
  /** Recent run durations in seconds, oldest → newest, for the sparkline. */
  history: number[];
  owner?: string;
  successRate?: number;
}

/** A node in a DAG's task graph, positioned for the graph view. */
export interface TaskGraphNode {
  id: string;
  label: string;
  status: TaskStatus;
  position: { x: number; y: number };
}

export interface TaskEdge {
  source: string;
  target: string;
}

/** A single task within a run, carrying gantt/timeline data. */
export interface Task {
  id: string;
  label: string;
  status: TaskStatus;
  /** Offset from the run's start, in seconds. */
  startOffset: number;
  /** Duration in seconds. */
  duration: number;
  worker: string;
  attempts: number;
}

export interface Run {
  id: string;
  dagId: string;
  dagName: string;
  number: string;
  status: RunStatus;
  trigger: "scheduled" | "manual" | "backfill";
  startedAt: string;
  duration: string;
  tasksDone: number;
  tasksTotal: number;
}

export interface Worker {
  id: string;
  name: string;
  host: string;
  pid: number;
  status: WorkerStatus;
  /** Current load as a percentage, 0–100. */
  load: number;
  activeTasks: number;
  capacity: number;
  uptime: string;
  currentTask?: string;
}

export interface LogLine {
  timestamp: string;
  level: LogLevel;
  taskId: string;
  /** Display name of the task, shown in the log viewer. */
  taskName: string;
  message: string;
}
