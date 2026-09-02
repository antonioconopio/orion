import type {
  Dag,
  LogLine,
  Run,
  Task,
  TaskEdge,
  TaskGraphNode,
  TaskStatus,
  Worker,
} from "./types";

// ---------------------------------------------------------------------------
// DAGs
// ---------------------------------------------------------------------------

export const DAGS: Dag[] = [
  {
    id: "ml_feature_pipeline",
    name: "ml_feature_pipeline",
    team: "team-ml",
    owner: "a.rivera",
    schedule: "0 6 * * *",
    status: "running",
    lastRun: "4 minutes ago",
    runNumber: "#1284",
    avgDuration: "6m 12s",
    successRate: 96,
    history: [4, 6, 5, 8, 6, 9, 7, 10, 8, 6],
  },
  {
    id: "daily_revenue_etl",
    name: "daily_revenue_etl",
    team: "team-data",
    owner: "j.chen",
    schedule: "0 2 * * *",
    status: "success",
    lastRun: "2 hours ago",
    runNumber: "#8821",
    avgDuration: "11m 4s",
    successRate: 99,
    history: [6, 8, 7, 9, 8, 10, 9, 11, 9, 10],
  },
  {
    id: "user_events_ingest",
    name: "user_events_ingest",
    team: "team-data",
    owner: "j.chen",
    schedule: "*/15 * * * *",
    status: "running",
    lastRun: "in progress",
    runNumber: "#9930",
    avgDuration: "~3m",
    successRate: 94,
    history: [3, 5, 4, 7, 5, 8, 6, 9, 7, 8],
  },
  {
    id: "fraud_scoring_batch",
    name: "fraud_scoring_batch",
    team: "team-risk",
    owner: "m.osei",
    schedule: "0 */4 * * *",
    status: "success",
    lastRun: "38 minutes ago",
    runNumber: "#4410",
    avgDuration: "2m 51s",
    successRate: 91,
    history: [5, 4, 6, 5, 7, 6, 8, 5, 7, 6],
  },
  {
    id: "warehouse_compaction",
    name: "warehouse_compaction",
    team: "platform",
    owner: "s.patel",
    schedule: "0 5 * * *",
    status: "failed",
    lastRun: "9 hours ago",
    runNumber: "#0772",
    avgDuration: "24m 9s",
    successRate: 82,
    history: [9, 6, 8, 5, 7, 9, 6, 8, 7, 14],
  },
  {
    id: "clickstream_sessionize",
    name: "clickstream_sessionize",
    team: "team-data",
    owner: "l.nguyen",
    schedule: "0 * * * *",
    status: "success",
    lastRun: "52 minutes ago",
    runNumber: "#6651",
    avgDuration: "4m 33s",
    successRate: 98,
    history: [4, 7, 5, 8, 6, 9, 6, 8, 7, 9],
  },
  {
    id: "inventory_sync",
    name: "inventory_sync",
    team: "team-ops",
    owner: "d.abara",
    schedule: "*/30 * * * *",
    status: "running",
    lastRun: "in progress",
    runNumber: "#5523",
    avgDuration: "~1m",
    successRate: 97,
    history: [3, 4, 6, 4, 7, 5, 8, 6, 7, 8],
  },
  {
    id: "churn_model_retrain",
    name: "churn_model_retrain",
    team: "team-ml",
    owner: "a.rivera",
    schedule: "0 0 1 * *",
    status: "success",
    lastRun: "3 days ago",
    runNumber: "#0091",
    avgDuration: "41m 2s",
    successRate: 88,
    history: [7, 9, 6, 8, 5, 9, 6, 10, 7, 9],
  },
];

export const DAG_STATS = {
  total: DAGS.length,
  running: DAGS.filter((d) => d.status === "running").length,
  failed: DAGS.filter((d) => d.status === "failed").length,
  success: DAGS.filter((d) => d.status === "success").length,
};

export function getDag(id: string): Dag | undefined {
  return DAGS.find((d) => d.id === id);
}

// ---------------------------------------------------------------------------
// Task graph — shared topology reused by the DAG-detail and run-detail views.
// ---------------------------------------------------------------------------

const TASK_TOPOLOGY: Omit<TaskGraphNode, "status">[] = [
  { id: "ingest_raw", label: "ingest_raw", position: { x: 0, y: 160 } },
  { id: "validate_schema", label: "validate_schema", position: { x: 240, y: 160 } },
  { id: "dedupe_events", label: "dedupe_events", position: { x: 480, y: 160 } },
  { id: "feature_join", label: "feature_join", position: { x: 760, y: 40 } },
  { id: "compute_aggregates", label: "compute_aggregates", position: { x: 760, y: 160 } },
  { id: "dq_report", label: "dq_report", position: { x: 760, y: 280 } },
  { id: "assemble_features", label: "assemble_features", position: { x: 1060, y: 100 } },
  { id: "write_feature_store", label: "write_feature_store", position: { x: 1340, y: 100 } },
];

export const TASK_EDGES: TaskEdge[] = [
  { source: "ingest_raw", target: "validate_schema" },
  { source: "validate_schema", target: "dedupe_events" },
  { source: "dedupe_events", target: "feature_join" },
  { source: "dedupe_events", target: "compute_aggregates" },
  { source: "dedupe_events", target: "dq_report" },
  { source: "feature_join", target: "assemble_features" },
  { source: "compute_aggregates", target: "assemble_features" },
  { source: "assemble_features", target: "write_feature_store" },
];

// Statuses reflecting the DAG's last completed run (definition view).
const DAG_DEFINITION_STATUS: Record<string, TaskStatus> = {
  ingest_raw: "success",
  validate_schema: "success",
  dedupe_events: "success",
  feature_join: "success",
  compute_aggregates: "success",
  dq_report: "success",
  assemble_features: "success",
  write_feature_store: "success",
};

// Statuses for the currently-selected run (in-progress, shows all 5 states).
const RUN_TASK_STATUS: Record<string, TaskStatus> = {
  ingest_raw: "success",
  validate_schema: "success",
  dedupe_events: "success",
  feature_join: "running",
  compute_aggregates: "retrying",
  dq_report: "failed",
  assemble_features: "pending",
  write_feature_store: "pending",
};

function withStatus(status: Record<string, TaskStatus>): TaskGraphNode[] {
  return TASK_TOPOLOGY.map((node) => ({ ...node, status: status[node.id] }));
}

export const DAG_GRAPH_NODES: TaskGraphNode[] = withStatus(DAG_DEFINITION_STATUS);
export const RUN_GRAPH_NODES: TaskGraphNode[] = withStatus(RUN_TASK_STATUS);

// ---------------------------------------------------------------------------
// Run detail — per-task timeline (gantt) data for the selected run.
// ---------------------------------------------------------------------------

export const RUN_TASKS: Task[] = [
  { id: "ingest_raw", label: "ingest_raw", status: "success", startOffset: 0, duration: 45, worker: "worker-01", attempts: 1 },
  { id: "validate_schema", label: "validate_schema", status: "success", startOffset: 45, duration: 22, worker: "worker-01", attempts: 1 },
  { id: "dedupe_events", label: "dedupe_events", status: "success", startOffset: 68, duration: 96, worker: "worker-03", attempts: 1 },
  { id: "feature_join", label: "feature_join", status: "running", startOffset: 165, duration: 84, worker: "worker-02", attempts: 1 },
  { id: "compute_aggregates", label: "compute_aggregates", status: "retrying", startOffset: 165, duration: 48, worker: "worker-04", attempts: 2 },
  { id: "dq_report", label: "dq_report", status: "failed", startOffset: 165, duration: 31, worker: "worker-05", attempts: 1 },
  { id: "assemble_features", label: "assemble_features", status: "pending", startOffset: 250, duration: 0, worker: "—", attempts: 0 },
  { id: "write_feature_store", label: "write_feature_store", status: "pending", startOffset: 250, duration: 0, worker: "—", attempts: 0 },
];

// ---------------------------------------------------------------------------
// Runs
// ---------------------------------------------------------------------------

export const RUNS: Run[] = [
  { id: "r_9f2a17", dagId: "ml_feature_pipeline", dagName: "ml_feature_pipeline", number: "#1284", status: "running", trigger: "scheduled", startedAt: "2026-09-02 06:00:04", duration: "4m 12s", tasksDone: 3, tasksTotal: 8 },
  { id: "r_8e1b03", dagId: "user_events_ingest", dagName: "user_events_ingest", number: "#9930", status: "running", trigger: "scheduled", startedAt: "2026-09-02 06:00:00", duration: "2m 48s", tasksDone: 5, tasksTotal: 9 },
  { id: "r_7c9d55", dagId: "warehouse_compaction", dagName: "warehouse_compaction", number: "#0772", status: "failed", trigger: "scheduled", startedAt: "2026-09-01 21:00:11", duration: "24m 9s", tasksDone: 11, tasksTotal: 13 },
  { id: "r_6b7a42", dagId: "fraud_scoring_batch", dagName: "fraud_scoring_batch", number: "#4410", status: "success", trigger: "scheduled", startedAt: "2026-09-02 05:22:00", duration: "2m 51s", tasksDone: 6, tasksTotal: 6 },
  { id: "r_5a3e18", dagId: "daily_revenue_etl", dagName: "daily_revenue_etl", number: "#8821", status: "success", trigger: "scheduled", startedAt: "2026-09-02 02:00:03", duration: "11m 4s", tasksDone: 14, tasksTotal: 14 },
  { id: "r_4d2c90", dagId: "ml_feature_pipeline", dagName: "ml_feature_pipeline", number: "#1283", status: "success", trigger: "manual", startedAt: "2026-09-01 06:00:02", duration: "5m 58s", tasksDone: 8, tasksTotal: 8 },
  { id: "r_3f8b71", dagId: "churn_model_retrain", dagName: "churn_model_retrain", number: "#0091", status: "success", trigger: "backfill", startedAt: "2026-08-30 00:00:09", duration: "41m 2s", tasksDone: 12, tasksTotal: 12 },
  { id: "r_2e6a34", dagId: "clickstream_sessionize", dagName: "clickstream_sessionize", number: "#6651", status: "success", trigger: "scheduled", startedAt: "2026-09-02 05:00:00", duration: "4m 33s", tasksDone: 7, tasksTotal: 7 },
  { id: "r_1c4f88", dagId: "warehouse_compaction", dagName: "warehouse_compaction", number: "#0771", status: "failed", trigger: "scheduled", startedAt: "2026-09-01 13:00:07", duration: "3m 02s", tasksDone: 4, tasksTotal: 13 },
  { id: "r_0b9d21", dagId: "fraud_scoring_batch", dagName: "fraud_scoring_batch", number: "#4409", status: "failed", trigger: "manual", startedAt: "2026-09-02 01:14:00", duration: "1m 09s", tasksDone: 2, tasksTotal: 6 },
];

export function getRun(id: string): Run | undefined {
  return RUNS.find((r) => r.id === id);
}

/** Run history rows for a given DAG's detail page. */
export function getRunsForDag(dagId: string): Run[] {
  return RUNS.filter((r) => r.dagId === dagId);
}

// ---------------------------------------------------------------------------
// Workers
// ---------------------------------------------------------------------------

export const WORKERS: Worker[] = [
  { id: "worker-01", name: "worker-01", host: "orion-w1.us-east", region: "us-east-1", status: "busy", load: 82, activeTasks: 4, capacity: 6, uptime: "12d 4h", currentTask: "ingest_raw" },
  { id: "worker-02", name: "worker-02", host: "orion-w2.us-east", region: "us-east-1", status: "busy", load: 67, activeTasks: 3, capacity: 6, uptime: "12d 4h", currentTask: "feature_join" },
  { id: "worker-03", name: "worker-03", host: "orion-w3.us-west", region: "us-west-2", status: "online", load: 41, activeTasks: 2, capacity: 6, uptime: "6d 19h", currentTask: "sessionize_batch" },
  { id: "worker-04", name: "worker-04", host: "orion-w4.us-west", region: "us-west-2", status: "busy", load: 74, activeTasks: 4, capacity: 6, uptime: "6d 19h", currentTask: "compute_aggregates" },
  { id: "worker-05", name: "worker-05", host: "orion-w5.eu-west", region: "eu-west-1", status: "idle", load: 8, activeTasks: 0, capacity: 6, uptime: "2d 7h" },
  { id: "worker-06", name: "worker-06", host: "orion-w6.eu-west", region: "eu-west-1", status: "offline", load: 0, activeTasks: 0, capacity: 6, uptime: "—" },
];

export const WORKER_POOL = {
  total: WORKERS.length,
  online: WORKERS.filter((w) => w.status !== "offline").length,
  activeTasks: WORKERS.reduce((sum, w) => sum + w.activeTasks, 0),
  capacity: WORKERS.reduce((sum, w) => sum + w.capacity, 0),
};

// ---------------------------------------------------------------------------
// Logs
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Dashboard overview
// ---------------------------------------------------------------------------

/** Completed runs per hour over the last 24h (oldest → newest). */
export const THROUGHPUT_24H = [
  8, 6, 5, 4, 3, 4, 7, 12, 18, 22, 19, 24, 21, 17, 20, 23, 26, 22, 19, 15, 13,
  11, 9, 10,
];

export const DASHBOARD_METRICS = {
  activeRuns: RUNS.filter((r) => r.status === "running").length,
  successRate24h: 96.4,
  failed24h: 3,
  workersOnline: WORKER_POOL.online,
  workersTotal: WORKER_POOL.total,
};

// ---------------------------------------------------------------------------
// Logs
// ---------------------------------------------------------------------------

export const RUN_LOGS: LogLine[] = [
  { timestamp: "06:00:04.112", level: "info", taskId: "ingest_raw", message: "task started on worker-01 (attempt 1/3)" },
  { timestamp: "06:00:04.340", level: "debug", taskId: "ingest_raw", message: "acquired distributed lock orion:lock:ingest_raw" },
  { timestamp: "06:00:21.905", level: "info", taskId: "ingest_raw", message: "read 1,284,551 rows from s3://orion-raw/events/2026-09-02/" },
  { timestamp: "06:00:49.011", level: "info", taskId: "ingest_raw", message: "task succeeded in 44.9s" },
  { timestamp: "06:00:49.230", level: "info", taskId: "validate_schema", message: "task started on worker-01 (attempt 1/3)" },
  { timestamp: "06:01:11.400", level: "warn", taskId: "validate_schema", message: "3 rows dropped: null value in required column `user_id`" },
  { timestamp: "06:01:11.902", level: "info", taskId: "validate_schema", message: "task succeeded in 22.1s" },
  { timestamp: "06:01:12.050", level: "info", taskId: "dedupe_events", message: "task started on worker-03 (attempt 1/3)" },
  { timestamp: "06:02:48.331", level: "info", taskId: "dedupe_events", message: "deduplicated 1,284,551 → 1,190,204 rows" },
  { timestamp: "06:02:48.780", level: "info", taskId: "dedupe_events", message: "task succeeded in 96.7s" },
  { timestamp: "06:02:49.004", level: "info", taskId: "feature_join", message: "task started on worker-02 (attempt 1/3)" },
  { timestamp: "06:02:49.021", level: "info", taskId: "compute_aggregates", message: "task started on worker-04 (attempt 1/3)" },
  { timestamp: "06:02:49.044", level: "info", taskId: "dq_report", message: "task started on worker-05 (attempt 1/3)" },
  { timestamp: "06:03:20.550", level: "error", taskId: "dq_report", message: "assertion failed: freshness check `max(event_ts) > now()-1h` returned false" },
  { timestamp: "06:03:20.661", level: "error", taskId: "dq_report", message: "task failed after 31.6s (attempt 1/3), not retriable" },
  { timestamp: "06:03:37.219", level: "warn", taskId: "compute_aggregates", message: "worker-04 heartbeat timeout, rescheduling task" },
  { timestamp: "06:03:37.884", level: "info", taskId: "compute_aggregates", message: "retrying task (attempt 2/3)" },
  { timestamp: "06:04:12.006", level: "debug", taskId: "feature_join", message: "joined 1.19M rows against feature registry (partition 4/8)" },
];
