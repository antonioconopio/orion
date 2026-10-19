"use client";

import { useMemo } from "react";

import { PageHeader } from "@/components/page-header";
import { EmptyState, ErrorState } from "@/components/data-states";
import { Skeleton } from "@/components/ui/skeleton";
import { DagStatCard } from "@/components/dashboard/dag-stat-card";
import { WorkerCard } from "@/components/workers/worker-card";
import { toWorker } from "@/lib/api/adapters";
import { useNow } from "@/lib/use-now";
import { useGetWorkersQuery } from "@/store/api";

// A worker that is killed sends no event; it just stops heartbeating. Polling notices it leaving.
const WORKER_POLL_MS = 10_000;

export default function WorkersPage() {
  const { data, isLoading, isError, refetch } = useGetWorkersQuery(undefined, { pollingInterval: WORKER_POLL_MS });
  const now = useNow(1000); // uptimes tick
  const workers = useMemo(() => (data ?? []).map((w) => toWorker(w, now)), [data, now]);

  const busy = workers.filter((w) => w.status === "busy").length;
  const activeTasks = workers.reduce((sum, w) => sum + w.activeTasks, 0);
  const capacity = workers.reduce((sum, w) => sum + w.capacity, 0);

  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="WORKERS" />

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          <div className="flex items-end justify-between">
            <div>
              <h1 className="text-lg font-semibold">Worker pool</h1>
              <p className="text-sm text-muted-foreground">
                Distributed workers pulling tasks from the Redis queue.
              </p>
            </div>
            <div className="flex gap-2">
              <DagStatCard
                label="ONLINE"
                value={workers.length}
                dotClassName="bg-emerald-500"
                valueClassName="text-emerald-500"
              />
              <DagStatCard
                label="BUSY"
                value={busy}
                dotClassName="bg-amber-500"
                valueClassName="text-amber-500"
              />
              <DagStatCard
                label="ACTIVE TASKS"
                value={activeTasks}
                dotClassName="bg-violet-400"
              />
              <DagStatCard
                label="CAPACITY"
                value={capacity}
                dotClassName="bg-sky-500"
              />
            </div>
          </div>

          {isLoading ? (
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-36" />
              ))}
            </div>
          ) : isError ? (
            <ErrorState onRetry={refetch} />
          ) : workers.length === 0 ? (
            <EmptyState
              title="No workers online"
              hint="Start one with docker compose up -d worker (a worker that stops heartbeating disappears after about 15 seconds)."
            />
          ) : (
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              {workers.map((worker) => (
                <WorkerCard key={worker.id} worker={worker} />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
