// Central domain types for Orion. These shape the static example data and
// component props so real state can be wired in later without restructuring.

export type DagStatus = "success" | "failed" | "running";

export type TaskStatus =
  | "pending"
  | "running"
  | "success"
  | "failed"
  | "retrying";

/** A run's overall status shares the task status vocabulary. */
export type RunStatus = TaskStatus;

export type WorkerStatus = "online" | "busy" | "idle" | "offline";

export type LogLevel = "debug" | "info" | "warn" | "error";

export interface Dag {
  id: string;
  name: string;
  team: string;
  schedule: string;
  status: DagStatus;
  lastRun: string;
  runNumber: string;
  avgDuration: string;
  /** Recent run durations, oldest → newest, for the sparkline. */
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
  region: string;
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
  message: string;
}
