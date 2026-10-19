// Shapes returned by the Go API (snake_case, ISO timestamps). Mirrors orion-api/internal/models.
// Components never see these: lib/api/adapters.ts converts them to the UI types in lib/types.ts.

export type ApiRunStatus = "queued" | "running" | "success" | "failed" | "cancelled";

export type ApiInstanceStatus =
  | "pending"
  | "queued"
  | "running"
  | "success"
  | "failed"
  | "retrying"
  | "skipped";

export interface ApiRun {
  id: string;
  dag_id: string;
  dag_name: string;
  number: number;
  status: ApiRunStatus;
  triggered_by: string; // "manual" | "schedule"
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
  tasks_total: number;
  tasks_done: number;
}

export interface ApiRunBrief {
  id: string;
  number: number;
  status: ApiRunStatus;
  started_at: string | null;
  finished_at: string | null;
}

export interface ApiDag {
  id: string;
  name: string;
  description: string | null;
  schedule: string | null;
  owner: string | null;
  is_active: boolean;
  python_version: string | null;
  created_at: string;
  updated_at: string;
  last_run: ApiRunBrief | null;
  success_rate: number | null; // 0..1
  avg_duration_seconds: number | null;
  recent_durations: number[]; // seconds, oldest to newest
}

export interface ApiTask {
  id: string;
  dag_id: string;
  name: string;
  task_type: string;
  config: unknown;
  created_at: string;
}

export interface ApiDependency {
  task_id: string;
  depends_on_task_id: string;
}

export interface ApiTaskInstance {
  id: string;
  run_id: string;
  task_id: string;
  task_name: string;
  status: ApiInstanceStatus;
  attempt: number;
  worker_id: string | null;
  started_at: string | null;
  finished_at: string | null;
  /** One JSON object per attempt, each on its own line: {"stdout","stderr","timed_out"}. */
  logs: string | null;
  output: unknown;
}

export interface ApiStats {
  active_runs: number;
  success_rate_24h: number | null; // 0..1
  failed_24h: number;
  throughput_24h: number[]; // runs finished per hour, oldest first
}

export interface ApiWorker {
  id: string;
  host: string;
  pid: number;
  started_at: string;
  status: "idle" | "busy";
  current_task: string | null;
  current_run_id: string | null;
}

export interface ApiTriggerResponse {
  run: ApiRun;
  task_instances: ApiTaskInstance[];
  enqueued: string[];
}

/** What the WebSocket sends: a hint about what changed, never the data itself. */
export type ApiEvent =
  | { type: "run"; run_id: string; dag_id: string }
  | { type: "worker" };
