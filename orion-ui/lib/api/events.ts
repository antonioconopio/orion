// What the WebSocket tells us and what we do about it. The server only sends hints
// ("run X changed"); each hint becomes a list of RTK Query cache tags to mark stale, and RTK Query
// refetches whichever of those queries a mounted component is using.

import type { ApiEvent } from "./types";

export type CacheTag =
  | "DagList"
  | "RunList"
  | "Stats"
  | "Workers"
  | { type: "Dag" | "Run" | "Instances"; id: string };

/** Parses one WebSocket message. Returns null for anything that is not a known event. */
export function parseEvent(data: unknown): ApiEvent | null {
  if (typeof data !== "string") return null;
  let value: unknown;
  try {
    value = JSON.parse(data);
  } catch {
    return null;
  }
  if (typeof value !== "object" || value === null) return null;
  const v = value as Record<string, unknown>;
  if (v.type === "worker") return { type: "worker" };
  if (v.type === "run" && typeof v.run_id === "string" && typeof v.dag_id === "string") {
    return { type: "run", run_id: v.run_id, dag_id: v.dag_id };
  }
  return null;
}

/** The cached data an event makes stale. */
export function tagsForEvent(ev: ApiEvent): CacheTag[] {
  if (ev.type === "worker") return ["Workers"];
  return [
    { type: "Run", id: ev.run_id },
    { type: "Instances", id: ev.run_id },
    { type: "Dag", id: ev.dag_id },
    "RunList",
    "DagList",
    "Stats",
  ];
}

/** Removes duplicate tags, so a burst of events asks for each refetch once. */
export function uniqueTags(tags: CacheTag[]): CacheTag[] {
  const seen = new Map<string, CacheTag>();
  for (const t of tags) seen.set(typeof t === "string" ? t : `${t.type}:${t.id}`, t);
  return [...seen.values()];
}

/** Wait before reconnect attempt n (0-based): 1s, 2s, 4s ... capped at 15s. */
export function backoffMs(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, 15_000);
}
