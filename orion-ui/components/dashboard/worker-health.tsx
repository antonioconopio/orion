import { cn } from "@/lib/utils";
import { WORKER_STATUS_STYLE } from "@/lib/status-style";
import type { Worker } from "@/lib/types";

function loadColor(load: number): string {
  if (load >= 80) return "bg-red-500";
  if (load >= 60) return "bg-amber-500";
  if (load > 0) return "bg-emerald-500";
  return "bg-zinc-600";
}

/** Compact per-worker load list for the dashboard's worker-pool card. */
export function WorkerHealth({ workers }: { workers: Worker[] }) {
  return (
    <div className="flex flex-col divide-y divide-border">
      {workers.map((worker) => {
        const status = WORKER_STATUS_STYLE[worker.status];
        return (
          <div
            key={worker.id}
            className="grid grid-cols-[130px_1fr_44px] items-center gap-3 py-2"
          >
            <div className="flex items-center gap-2">
              <span className={cn("size-1.5 rounded-full", status.dot)} />
              <span className="font-mono text-xs text-foreground">
                {worker.name}
              </span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className={cn("h-full rounded-full", loadColor(worker.load))}
                style={{ width: `${worker.load}%` }}
              />
            </div>
            <span className="text-right font-mono text-xs tabular-nums text-muted-foreground">
              {worker.load}%
            </span>
          </div>
        );
      })}
    </div>
  );
}
