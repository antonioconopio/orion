// Converts API responses (lib/api/types.ts) into the UI types the components already use
// (lib/types.ts). Everything here is a pure function; "now" is passed in so results are
// deterministic and testable.

import type {
  Dag,
  DagStatus,
  LogLevel,
  LogLine,
  Run,
  Task,
  TaskEdge,
  TaskGraphNode,
  TaskStatus,
  Worker,
} from "@/lib/types";
import type {
  ApiDag,
  ApiDependency,
  ApiRun,
  ApiTask,
  ApiTaskInstance,
  ApiWorker,
} from "./types";

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

const pad = (n: number, width = 2) => String(n).padStart(width, "0");

/** "6m 12s", "45s", "1h 05m"; "—" when there is no duration yet. */
export function formatDuration(seconds: number | null | undefined): string {
  if (seconds == null || seconds < 0) return "—";
  if (seconds < 1) return "<1s";
  const total = Math.round(seconds);
  if (total < 60) return `${total}s`;
  if (total < 3600) return `${Math.floor(total / 60)}m ${pad(total % 60)}s`;
  return `${Math.floor(total / 3600)}h ${pad(Math.floor((total % 3600) / 60))}m`;
}

/** "just now", "4 minutes ago", "2 hours ago", "3 days ago". */
export function timeAgo(iso: string, now: number): string {
  const seconds = Math.max(0, (now - Date.parse(iso)) / 1000);
  if (seconds < 60) return "just now";
  const unit = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"} ago`;
  if (seconds < 3600) return unit(Math.floor(seconds / 60), "minute");
  if (seconds < 86400) return unit(Math.floor(seconds / 3600), "hour");
  return unit(Math.floor(seconds / 86400), "day");
}

/** "2026-09-02 06:00:04" in the viewer's timezone. */
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/** "06:00:04.112" in the viewer's timezone. */
export function formatClock(iso: string): string {
  const d = new Date(iso);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
}

/** "45s", "12m", "3h 20m", "2d 4h". */
export function formatUptime(ms: number): string {
  const minutes = Math.floor(Math.max(0, ms) / 60000);
  if (minutes < 1) return `${Math.floor(Math.max(0, ms) / 1000)}s`;
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m`;
  return `${Math.floor(hours / 24)}d ${hours % 24}h`;
}

// ---------------------------------------------------------------------------
// Runs
// ---------------------------------------------------------------------------

/** Seconds a run has taken so far (or in total, once finished); null if it has not started. */
export function runDurationSeconds(
  run: Pick<ApiRun, "started_at" | "finished_at">,
  now: number,
): number | null {
  if (!run.started_at) return null;
  const end = run.finished_at ? Date.parse(run.finished_at) : now;
  return (end - Date.parse(run.started_at)) / 1000;
}

export function toRun(r: ApiRun, now: number): Run {
  return {
    id: r.id,
    dagId: r.dag_id,
    dagName: r.dag_name,
    number: `#${r.number}`,
    status: r.status,
    trigger: r.triggered_by === "schedule" ? "scheduled" : "manual",
    startedAt: r.started_at ? formatDateTime(r.started_at) : "—",
    duration: formatDuration(runDurationSeconds(r, now)),
    tasksDone: r.tasks_done,
    tasksTotal: r.tasks_total,
  };
}

// ---------------------------------------------------------------------------
// DAGs
// ---------------------------------------------------------------------------

function dagStatus(last: ApiDag["last_run"]): DagStatus {
  switch (last?.status) {
    case "queued":
    case "running":
      return "running";
    case "success":
      return "success";
    case "failed":
      return "failed";
    default:
      return "idle"; // never run, or cancelled
  }
}

export function toDag(d: ApiDag, now: number): Dag {
  const last = d.last_run;
  const inProgress = last?.status === "queued" || last?.status === "running";
  const lastTime = last?.finished_at ?? last?.started_at ?? null;
  return {
    id: d.id,
    name: d.name,
    schedule: d.schedule || "manual",
    status: dagStatus(last),
    lastRun: !last ? "never" : inProgress ? "in progress" : lastTime ? timeAgo(lastTime, now) : "—",
    runNumber: last ? `#${last.number}` : "—",
    avgDuration: formatDuration(d.avg_duration_seconds),
    history: d.recent_durations,
    owner: d.owner ?? undefined,
    successRate: d.success_rate == null ? undefined : Math.round(d.success_rate * 100),
  };
}

// ---------------------------------------------------------------------------
// Task graph
// ---------------------------------------------------------------------------

const COLUMN_WIDTH = 280;
const ROW_HEIGHT = 120;

/**
 * Places each task in a column by its longest dependency path from a root, and spreads the
 * tasks of a column vertically (centered on the tallest column). Order within a column
 * follows the order of `ids`.
 */
export function layoutGraph(
  ids: string[],
  edges: TaskEdge[],
): Record<string, { x: number; y: number }> {
  const parents = new Map<string, string[]>(ids.map((id) => [id, []]));
  for (const e of edges) parents.get(e.target)?.push(e.source);

  const depths = new Map<string, number>();
  const visiting = new Set<string>(); // guards against a cycle, which the API forbids but a bad row could cause
  const depthOf = (id: string): number => {
    const known = depths.get(id);
    if (known !== undefined) return known;
    if (visiting.has(id)) return 0;
    visiting.add(id);
    const d = Math.max(-1, ...(parents.get(id) ?? []).filter((p) => parents.has(p)).map(depthOf)) + 1;
    visiting.delete(id);
    depths.set(id, d);
    return d;
  };

  const columns = new Map<number, string[]>();
  for (const id of ids) {
    const d = depthOf(id);
    columns.set(d, [...(columns.get(d) ?? []), id]);
  }
  const tallest = Math.max(1, ...[...columns.values()].map((c) => c.length));

  const positions: Record<string, { x: number; y: number }> = {};
  for (const [depth, column] of columns) {
    column.forEach((id, i) => {
      positions[id] = {
        x: depth * COLUMN_WIDTH,
        y: (i + (tallest - column.length) / 2) * ROW_HEIGHT,
      };
    });
  }
  return positions;
}

/** Nodes and edges for DagGraph. A task with no status yet (e.g. added after the run) is "pending". */
export function toGraph(
  tasks: ApiTask[],
  deps: ApiDependency[],
  statusByTaskId: Record<string, TaskStatus>,
): { nodes: TaskGraphNode[]; edges: TaskEdge[] } {
  const sorted = [...tasks].sort((a, b) => a.name.localeCompare(b.name));
  const known = new Set(sorted.map((t) => t.id));
  const edges: TaskEdge[] = deps
    .filter((d) => known.has(d.task_id) && known.has(d.depends_on_task_id))
    .map((d) => ({ source: d.depends_on_task_id, target: d.task_id }));
  const positions = layoutGraph(
    sorted.map((t) => t.id),
    edges,
  );
  const nodes: TaskGraphNode[] = sorted.map((t) => ({
    id: t.id,
    label: t.name,
    status: statusByTaskId[t.id] ?? "pending",
    position: positions[t.id],
  }));
  return { nodes, edges };
}

/** task id -> status, from one run's instances. */
export function statusByTask(instances: ApiTaskInstance[]): Record<string, TaskStatus> {
  return Object.fromEntries(instances.map((i) => [i.task_id, i.status]));
}

// ---------------------------------------------------------------------------
// Timeline and logs
// ---------------------------------------------------------------------------

/** Gantt rows for one run, ordered by start time (not-started tasks last). */
export function toTimelineTasks(instances: ApiTaskInstance[], run: ApiRun, now: number): Task[] {
  const base = Date.parse(run.started_at ?? run.created_at);
  const startOf = (i: ApiTaskInstance) => (i.started_at ? Date.parse(i.started_at) : Infinity);

  return [...instances]
    .sort((a, b) => startOf(a) - startOf(b) || a.task_name.localeCompare(b.task_name))
    .map((i) => {
      const start = i.started_at ? Date.parse(i.started_at) : null;
      const end = i.finished_at ? Date.parse(i.finished_at) : i.status === "running" ? now : start;
      return {
        id: i.task_id,
        label: i.task_name,
        status: i.status,
        startOffset: start == null ? 0 : Math.max(0, (start - base) / 1000),
        duration: start == null || end == null ? 0 : Math.max(0, (end - start) / 1000),
        worker: i.worker_id ?? "—",
        attempts: start == null ? 0 : i.attempt,
      };
    });
}

interface AttemptLog {
  stdout?: string;
  stderr?: string;
  timed_out?: boolean;
}

const splitLines = (text: string | undefined): string[] =>
  (text ?? "").split("\n").filter((line) => line.length > 0);

/**
 * One instance's `logs` column holds one JSON object per attempt. The backend has no per-line
 * timestamps, so every line carries the time the instance finished. stderr is an error on a
 * failed task (it is the traceback) and a warning otherwise.
 */
export function parseLogs(i: ApiTaskInstance): LogLine[] {
  if (!i.logs) return [];
  const timestamp = formatClock(i.finished_at ?? i.started_at ?? new Date(0).toISOString());
  const stderrLevel: LogLevel = i.status === "failed" ? "error" : "warn";
  const line = (level: LogLevel, message: string): LogLine => ({
    timestamp,
    level,
    taskId: i.task_id,
    taskName: i.task_name,
    message,
  });

  const out: LogLine[] = [];
  for (const raw of splitLines(i.logs)) {
    let attempt: AttemptLog;
    try {
      attempt = JSON.parse(raw) as AttemptLog;
    } catch {
      out.push(line("info", raw)); // not JSON: show it as it is
      continue;
    }
    out.push(...splitLines(attempt.stdout).map((m) => line("info", m)));
    out.push(...splitLines(attempt.stderr).map((m) => line(stderrLevel, m)));
    if (attempt.timed_out) out.push(line("error", "task timed out"));
  }
  return out;
}

/** Every instance's log lines, ordered by when each instance finished. */
export function runLogs(instances: ApiTaskInstance[]): LogLine[] {
  const finished = (i: ApiTaskInstance) => (i.finished_at ? Date.parse(i.finished_at) : Infinity);
  return [...instances]
    .sort((a, b) => finished(a) - finished(b) || a.task_name.localeCompare(b.task_name))
    .flatMap(parseLogs);
}

/** One task's published result, ready to show. */
export interface TaskOutput {
  taskId: string;
  taskName: string;
  json: string;
}

/** The pretty-printed outputs of the tasks that published one, ordered by when they finished. */
export function toOutputs(instances: ApiTaskInstance[]): TaskOutput[] {
  const finished = (i: ApiTaskInstance) => (i.finished_at ? Date.parse(i.finished_at) : Infinity);
  return instances
    .filter((i) => i.output !== null && i.output !== undefined)
    .sort((a, b) => finished(a) - finished(b) || a.task_name.localeCompare(b.task_name))
    .map((i) => ({ taskId: i.task_id, taskName: i.task_name, json: JSON.stringify(i.output, null, 2) }));
}

// ---------------------------------------------------------------------------
// Workers
// ---------------------------------------------------------------------------

/** A worker runs one task at a time, so it has one slot: busy means fully loaded. */
export function toWorker(w: ApiWorker, now: number): Worker {
  const busy = w.status === "busy";
  return {
    id: w.id,
    name: w.id,
    host: w.host,
    pid: w.pid,
    status: w.status,
    load: busy ? 100 : 0,
    activeTasks: busy ? 1 : 0,
    capacity: 1,
    uptime: formatUptime(now - Date.parse(w.started_at)),
    currentTask: w.current_task ?? undefined,
  };
}
