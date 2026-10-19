import { describe, expect, it } from "vitest";

import { backoffMs, parseEvent, tagsForEvent, uniqueTags } from "./events";

describe("parseEvent", () => {
  it("accepts the two event types the server sends", () => {
    expect(parseEvent('{"type":"run","run_id":"r1","dag_id":"d1"}')).toEqual({ type: "run", run_id: "r1", dag_id: "d1" });
    expect(parseEvent('{"type": "worker"}')).toEqual({ type: "worker" });
  });

  it.each([
    ["not json"],
    ["null"],
    ["42"],
    ['{"type":"run"}'], // missing ids
    ['{"type":"run","run_id":1,"dag_id":"d"}'],
    ['{"type":"mystery"}'],
  ])("ignores %s", (input) => {
    expect(parseEvent(input)).toBeNull();
  });

  it("ignores binary frames", () => {
    expect(parseEvent(new ArrayBuffer(2))).toBeNull();
  });
});

describe("tagsForEvent", () => {
  it("a run event makes the run, its tasks, its DAG, the lists and the stats stale", () => {
    expect(tagsForEvent({ type: "run", run_id: "r1", dag_id: "d1" })).toEqual([
      { type: "Run", id: "r1" },
      { type: "Instances", id: "r1" },
      { type: "Dag", id: "d1" },
      "RunList",
      "DagList",
      "Stats",
    ]);
  });

  it("a worker event only touches workers", () => {
    expect(tagsForEvent({ type: "worker" })).toEqual(["Workers"]);
  });
});

describe("uniqueTags", () => {
  it("collapses a burst of identical events", () => {
    const burst = [
      ...tagsForEvent({ type: "run", run_id: "r1", dag_id: "d1" }),
      ...tagsForEvent({ type: "run", run_id: "r1", dag_id: "d1" }),
      ...tagsForEvent({ type: "run", run_id: "r2", dag_id: "d1" }),
      "Workers" as const,
      "Workers" as const,
    ];
    const out = uniqueTags(burst);
    expect(out).toHaveLength(6 + 2 + 1); // r1 set, r2's Run+Instances, Workers
    expect(out.filter((t) => t === "RunList")).toHaveLength(1);
  });
});

describe("backoffMs", () => {
  it("doubles and then caps", () => {
    expect([0, 1, 2, 3, 4, 5, 10].map(backoffMs)).toEqual([1000, 2000, 4000, 8000, 15000, 15000, 15000]);
  });
});
