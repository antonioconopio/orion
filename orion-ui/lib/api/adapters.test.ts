import { describe, expect, it } from "vitest";

import {
  formatDuration,
  formatUptime,
  layoutGraph,
  parseLogs,
  runLogs,
  timeAgo,
  toDag,
  toGraph,
  toOutputs,
  toRun,
  toTimelineTasks,
  toWorker,
} from "./adapters";
import type {
  ApiDag,
  ApiRun,
  ApiTask,
  ApiTaskInstance,
  ApiWorker,
} from "./types";

const NOW = Date.parse("2026-09-02T06:10:00Z");
const at = (hhmmss: string) => `2026-09-02T${hhmmss}Z`;

const run = (over: Partial<ApiRun> = {}): ApiRun => ({
  id: "r1",
  dag_id: "d1",
  dag_name: "etl",
  number: 12,
  status: "success",
  triggered_by: "manual",
  started_at: at("06:00:00"),
  finished_at: at("06:04:12"),
  created_at: at("05:59:59"),
  tasks_total: 4,
  tasks_done: 4,
  ...over,
});

const dag = (over: Partial<ApiDag> = {}): ApiDag => ({
  id: "d1",
  name: "etl",
  description: null,
  schedule: "0 2 * * *",
  owner: "ana",
  is_active: true,
  python_version: null,
  created_at: at("01:00:00"),
  updated_at: at("01:00:00"),
  last_run: null,
  success_rate: null,
  avg_duration_seconds: null,
  recent_durations: [],
  ...over,
});

const task = (id: string, name = id): ApiTask => ({
  id,
  dag_id: "d1",
  name,
  task_type: "python",
  config: {},
  created_at: at("01:00:00"),
});

const instance = (over: Partial<ApiTaskInstance> = {}): ApiTaskInstance => ({
  id: "i1",
  run_id: "r1",
  task_id: "t1",
  task_name: "extract",
  status: "success",
  attempt: 1,
  worker_id: "w1",
  started_at: at("06:00:10"),
  finished_at: at("06:00:40"),
  logs: null,
  output: null,
  ...over,
});

const attempt = (a: { stdout?: string; stderr?: string; timed_out?: boolean }) =>
  JSON.stringify({ stdout: "", stderr: "", timed_out: false, ...a });

describe("formatDuration", () => {
  it.each([
    [null, "—"],
    [undefined, "—"],
    [-1, "—"],
    [0.4, "<1s"],
    [45, "45s"],
    [59.6, "1m 00s"],
    [252, "4m 12s"],
    [3900, "1h 05m"],
  ])("%s -> %s", (input, expected) => {
    expect(formatDuration(input)).toBe(expected);
  });
});

describe("timeAgo", () => {
  it.each([
    [at("06:09:30"), "just now"],
    [at("06:09:00"), "1 minute ago"],
    [at("06:06:00"), "4 minutes ago"],
    [at("04:10:00"), "2 hours ago"],
    ["2026-08-30T06:10:00Z", "3 days ago"],
  ])("%s -> %s", (iso, expected) => {
    expect(timeAgo(iso, NOW)).toBe(expected);
  });
});

describe("formatUptime", () => {
  it.each([
    [30_000, "30s"],
    [12 * 60_000, "12m"],
    [(3 * 60 + 20) * 60_000, "3h 20m"],
    [(52 * 60) * 60_000, "2d 4h"],
    [-5, "0s"],
  ])("%s ms -> %s", (ms, expected) => {
    expect(formatUptime(ms)).toBe(expected);
  });
});

describe("toRun", () => {
  it("maps a finished run", () => {
    expect(toRun(run(), NOW)).toEqual({
      id: "r1",
      dagId: "d1",
      dagName: "etl",
      number: "#12",
      status: "success",
      trigger: "manual",
      startedAt: "2026-09-02 06:00:00",
      duration: "4m 12s",
      tasksDone: 4,
      tasksTotal: 4,
    });
  });

  it("a running run's duration is measured up to now", () => {
    const r = toRun(run({ status: "running", finished_at: null, started_at: at("06:08:00") }), NOW);
    expect(r.duration).toBe("2m 00s");
  });

  it("a queued run has not started", () => {
    const r = toRun(run({ status: "queued", started_at: null, finished_at: null }), NOW);
    expect(r.startedAt).toBe("—");
    expect(r.duration).toBe("—");
  });

  it("maps the schedule trigger", () => {
    expect(toRun(run({ triggered_by: "schedule" }), NOW).trigger).toBe("scheduled");
  });
});

describe("toDag", () => {
  const last = (status: ApiRun["status"], over = {}) => ({
    id: "r1",
    number: 3,
    status,
    started_at: at("06:00:00"),
    finished_at: at("06:04:00"),
    ...over,
  });

  it("a DAG that never ran is idle with sensible placeholders", () => {
    const d = toDag(dag({ schedule: null }), NOW);
    expect(d).toMatchObject({
      status: "idle",
      lastRun: "never",
      runNumber: "—",
      avgDuration: "—",
      schedule: "manual",
      history: [],
      successRate: undefined,
    });
  });

  it.each([
    ["success", "success"],
    ["failed", "failed"],
    ["running", "running"],
    ["queued", "running"],
    ["cancelled", "idle"],
  ] as const)("last run %s -> DAG status %s", (runStatus, expected) => {
    expect(toDag(dag({ last_run: last(runStatus) }), NOW).status).toBe(expected);
  });

  it("describes a finished and an in-progress last run", () => {
    expect(toDag(dag({ last_run: last("success") }), NOW)).toMatchObject({
      lastRun: "6 minutes ago",
      runNumber: "#3",
    });
    expect(toDag(dag({ last_run: last("running", { finished_at: null }) }), NOW).lastRun).toBe("in progress");
  });

  it("turns stats into display values", () => {
    const d = toDag(dag({ success_rate: 0.666, avg_duration_seconds: 252, recent_durations: [10, 20, 30] }), NOW);
    expect(d).toMatchObject({ successRate: 67, avgDuration: "4m 12s", history: [10, 20, 30], owner: "ana" });
  });
});

describe("layoutGraph", () => {
  const edge = (source: string, target: string) => ({ source, target });

  it("puts a chain in successive columns on one row", () => {
    const p = layoutGraph(["a", "b", "c"], [edge("a", "b"), edge("b", "c")]);
    expect([p.a.x, p.b.x, p.c.x]).toEqual([0, 280, 560]);
    expect(new Set([p.a.y, p.b.y, p.c.y]).size).toBe(1);
  });

  it("puts a diamond's parallel tasks in one column, centered around the single ones", () => {
    const p = layoutGraph(
      ["a", "b", "c", "d"],
      [edge("a", "b"), edge("a", "c"), edge("b", "d"), edge("c", "d")],
    );
    expect(p.b.x).toBe(p.c.x);
    expect(p.b.y).not.toBe(p.c.y);
    expect(p.a.y).toBe((p.b.y + p.c.y) / 2);
    expect(p.d.x).toBe(560);
  });

  it("uses the longest path, so a task waits for its slowest parent", () => {
    // a -> b -> c, and a -> c directly: c still sits two columns from a
    const p = layoutGraph(["a", "b", "c"], [edge("a", "b"), edge("b", "c"), edge("a", "c")]);
    expect(p.c.x).toBe(560);
  });

  it("puts unconnected tasks in the first column", () => {
    const p = layoutGraph(["a", "b"], []);
    expect([p.a.x, p.b.x]).toEqual([0, 0]);
    expect(p.a.y).not.toBe(p.b.y);
  });

  it("ignores edges to unknown tasks and survives a cycle", () => {
    expect(layoutGraph(["a"], [edge("ghost", "a")]).a).toEqual({ x: 0, y: 0 });
    const cyc = layoutGraph(["a", "b"], [edge("a", "b"), edge("b", "a")]);
    expect(Object.keys(cyc)).toEqual(["a", "b"]); // terminates and places both
  });

  it("handles no tasks", () => {
    expect(layoutGraph([], [])).toEqual({});
  });
});

describe("toGraph", () => {
  it("points edges from parent to child and defaults missing statuses to pending", () => {
    const g = toGraph(
      [task("t2", "load"), task("t1", "extract")],
      [{ task_id: "t2", depends_on_task_id: "t1" }],
      { t1: "success" },
    );
    expect(g.nodes.map((n) => [n.label, n.status])).toEqual([
      ["extract", "success"],
      ["load", "pending"],
    ]);
    expect(g.edges).toEqual([{ source: "t1", target: "t2" }]);
    expect(g.nodes[1].position.x).toBeGreaterThan(g.nodes[0].position.x);
  });

  it("drops dependencies on tasks that are not in the list", () => {
    const g = toGraph([task("t1")], [{ task_id: "t1", depends_on_task_id: "gone" }], {});
    expect(g.edges).toEqual([]);
  });
});

describe("toTimelineTasks", () => {
  const r = run({ started_at: at("06:00:00") });

  it("offsets and durations are relative to the run start, ordered by start", () => {
    const rows = toTimelineTasks(
      [
        instance({ task_id: "t2", task_name: "load", started_at: at("06:01:00"), finished_at: at("06:01:30") }),
        instance({ task_id: "t1", task_name: "extract", started_at: at("06:00:10"), finished_at: at("06:00:40") }),
      ],
      r,
      NOW,
    );
    expect(rows.map((t) => [t.label, t.startOffset, t.duration])).toEqual([
      ["extract", 10, 30],
      ["load", 60, 30],
    ]);
  });

  it("a running task lasts until now; a not-started one has zero length and zero attempts", () => {
    const rows = toTimelineTasks(
      [
        instance({ task_name: "a", status: "running", started_at: at("06:09:00"), finished_at: null }),
        instance({ task_id: "t2", task_name: "b", status: "pending", started_at: null, finished_at: null, worker_id: null }),
      ],
      run({ started_at: at("06:00:00"), finished_at: null, status: "running" }),
      NOW,
    );
    expect(rows[0]).toMatchObject({ label: "a", duration: 60 });
    expect(rows[1]).toMatchObject({ label: "b", startOffset: 0, duration: 0, attempts: 0, worker: "—" });
  });

  it("falls back to created_at when the run has not started", () => {
    const rows = toTimelineTasks([instance()], run({ started_at: null }), NOW);
    expect(rows[0].startOffset).toBeGreaterThanOrEqual(0);
  });
});

describe("parseLogs", () => {
  it("returns nothing when there are no logs", () => {
    expect(parseLogs(instance({ logs: null }))).toEqual([]);
    expect(parseLogs(instance({ logs: "" }))).toEqual([]);
  });

  it("splits stdout into info lines stamped with the finish time and tagged with the task", () => {
    const lines = parseLogs(instance({ logs: "\n" + attempt({ stdout: "one\ntwo\n" }) }));
    expect(lines).toEqual([
      { timestamp: "06:00:40.000", level: "info", taskId: "t1", taskName: "extract", message: "one" },
      { timestamp: "06:00:40.000", level: "info", taskId: "t1", taskName: "extract", message: "two" },
    ]);
  });

  it("stderr is an error on a failed task and a warning otherwise", () => {
    const logs = attempt({ stderr: "Traceback\nValueError: boom\n" });
    expect(parseLogs(instance({ status: "failed", logs })).map((l) => l.level)).toEqual(["error", "error"]);
    expect(parseLogs(instance({ status: "success", logs })).map((l) => l.level)).toEqual(["warn", "warn"]);
  });

  it("reports a timeout", () => {
    const lines = parseLogs(instance({ status: "failed", logs: attempt({ stdout: "partial", timed_out: true }) }));
    expect(lines.map((l) => [l.level, l.message])).toEqual([
      ["info", "partial"],
      ["error", "task timed out"],
    ]);
  });

  it("reads every attempt, in order", () => {
    const logs = "\n" + attempt({ stdout: "try 1" }) + "\n" + attempt({ stdout: "try 2" });
    expect(parseLogs(instance({ logs })).map((l) => l.message)).toEqual(["try 1", "try 2"]);
  });

  it("shows a line that is not JSON as it is", () => {
    expect(parseLogs(instance({ logs: "plain text" })).map((l) => l.message)).toEqual(["plain text"]);
  });
});

describe("runLogs", () => {
  it("orders instances by when they finished, not by name", () => {
    const lines = runLogs([
      instance({ task_id: "t1", task_name: "a_late", finished_at: at("06:02:00"), logs: attempt({ stdout: "late" }) }),
      instance({ task_id: "t2", task_name: "z_early", finished_at: at("06:01:00"), logs: attempt({ stdout: "early" }) }),
    ]);
    expect(lines.map((l) => l.message)).toEqual(["early", "late"]);
  });
});

describe("toWorker", () => {
  const w = (over: Partial<ApiWorker> = {}): ApiWorker => ({
    id: "host-1",
    host: "host",
    pid: 1,
    started_at: at("06:00:00"),
    status: "idle",
    current_task: null,
    current_run_id: null,
    ...over,
  });

  it("an idle worker has no load", () => {
    expect(toWorker(w(), NOW)).toMatchObject({ load: 0, activeTasks: 0, capacity: 1, uptime: "10m", currentTask: undefined });
  });

  it("a busy worker is fully loaded and shows its task", () => {
    expect(toWorker(w({ status: "busy", current_task: "etl / extract" }), NOW)).toMatchObject({
      status: "busy",
      load: 100,
      activeTasks: 1,
      currentTask: "etl / extract",
    });
  });
});

describe("toOutputs", () => {
  it("lists only the tasks that published an output, in finish order, pretty-printed", () => {
    const outputs = toOutputs([
      instance({ task_id: "t2", task_name: "load", finished_at: at("06:02:00"), output: { total: 19 } }),
      instance({ task_id: "t1", task_name: "extract", finished_at: at("06:01:00"), output: { n: 3, tags: ["a"] } }),
      instance({ task_id: "t3", task_name: "quiet", finished_at: at("06:01:30"), output: null }),
    ]);
    expect(outputs.map((o) => o.taskName)).toEqual(["extract", "load"]);
    expect(outputs[0].json).toBe('{\n  "n": 3,\n  "tags": [\n    "a"\n  ]\n}');
  });

  it("keeps falsy but real outputs (0, false, empty string, empty list)", () => {
    const outputs = toOutputs([
      instance({ task_id: "a", task_name: "a", output: 0 }),
      instance({ task_id: "b", task_name: "b", output: false }),
      instance({ task_id: "c", task_name: "c", output: "" }),
      instance({ task_id: "d", task_name: "d", output: [] }),
    ]);
    expect(outputs.map((o) => o.json)).toEqual(["0", "false", '""', "[]"]);
  });

  it("returns nothing when no task published an output", () => {
    expect(toOutputs([instance({ output: null }), instance({ output: undefined })])).toEqual([]);
  });
});
