import { Cpu, MapPin, Timer } from "lucide-react";

import { cn } from "@/lib/utils";
import { WORKER_STATUS_STYLE } from "@/lib/status-style";
import { Card } from "@/components/ui/card";
import type { Worker } from "@/lib/types";

function loadColor(load: number): string {
  if (load >= 80) return "bg-red-500";
  if (load >= 60) return "bg-amber-500";
  if (load > 0) return "bg-emerald-500";
  return "bg-zinc-600";
}

export function WorkerCard({ worker }: { worker: Worker }) {
  const status = WORKER_STATUS_STYLE[worker.status];
  const offline = worker.status === "offline";
  return (
    <Card className={cn("gap-3 p-4", offline && "opacity-60")}>
      <div className="flex items-start justify-between">
        <div className="flex flex-col">
          <span className="font-mono text-sm font-medium text-foreground">
            {worker.name}
          </span>
          <span className="font-mono text-xs text-muted-foreground">
            {worker.host}
          </span>
        </div>
        <span
          className={cn(
            "inline-flex items-center gap-1.5 rounded-sm px-2 py-0.5 font-mono text-[0.65rem] font-medium tracking-wide ring-1 ring-inset",
            status.badge,
          )}
        >
          <span className={cn("size-1.5 rounded-full", status.dot)} />
          {status.label}
        </span>
      </div>

      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">Load</span>
          <span className="font-mono tabular-nums text-foreground">
            {worker.load}%
          </span>
        </div>
        <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
          <div
            className={cn("h-full rounded-full", loadColor(worker.load))}
            style={{ width: `${worker.load}%` }}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2 text-xs">
        <div className="flex items-center gap-1.5 text-muted-foreground">
          <Cpu className="size-3.5" />
          <span className="font-mono tabular-nums">
            {worker.activeTasks}/{worker.capacity} slots
          </span>
        </div>
        <div className="flex items-center gap-1.5 text-muted-foreground">
          <Timer className="size-3.5" />
          <span className="font-mono tabular-nums">{worker.uptime}</span>
        </div>
        <div className="flex items-center gap-1.5 text-muted-foreground">
          <MapPin className="size-3.5" />
          <span className="font-mono">{worker.region}</span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-muted-foreground">›</span>
          <span className="truncate font-mono text-foreground/80">
            {worker.currentTask ?? "idle"}
          </span>
        </div>
      </div>
    </Card>
  );
}
