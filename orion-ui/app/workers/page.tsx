import { PageHeader } from "@/components/page-header";
import { DagStatCard } from "@/components/dashboard/dag-stat-card";
import { WorkerCard } from "@/components/workers/worker-card";
import { WORKERS, WORKER_POOL } from "@/lib/sample-data";

const busy = WORKERS.filter((w) => w.status === "busy").length;

export default function WorkersPage() {
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
                value={WORKER_POOL.online}
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
                value={WORKER_POOL.activeTasks}
                dotClassName="bg-violet-400"
              />
              <DagStatCard
                label="CAPACITY"
                value={WORKER_POOL.capacity}
                dotClassName="bg-sky-500"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
            {WORKERS.map((worker) => (
              <WorkerCard key={worker.id} worker={worker} />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
