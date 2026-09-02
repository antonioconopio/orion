import { cn } from "@/lib/utils";
import { TASK_STATUS_STYLE } from "@/lib/status-style";
import type { TaskStatus } from "@/lib/types";

const ORDER: TaskStatus[] = [
  "success",
  "running",
  "retrying",
  "failed",
  "pending",
];

export function GraphLegend({ className }: { className?: string }) {
  return (
    <div className={cn("flex flex-wrap items-center gap-x-4 gap-y-1.5", className)}>
      {ORDER.map((status) => {
        const style = TASK_STATUS_STYLE[status];
        return (
          <div key={status} className="flex items-center gap-1.5">
            <span className={cn("size-2 rounded-full", style.dot)} />
            <span className="font-mono text-[0.65rem] tracking-wide text-muted-foreground">
              {style.label}
            </span>
          </div>
        );
      })}
    </div>
  );
}
