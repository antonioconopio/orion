import { ScrollText } from "lucide-react";

import { cn } from "@/lib/utils";
import { TASK_STATUS_STYLE } from "@/lib/status-style";
import { Button } from "@/components/ui/button";
import { TaskStatusDot } from "@/components/task-status-dot";
import { LogViewerSheet } from "./log-viewer-sheet";
import type { LogLine, Task } from "@/lib/types";

function formatDuration(seconds: number): string {
  if (seconds <= 0) return "—";
  const m = Math.floor(seconds / 60);
  const s = Math.round(seconds % 60);
  return m > 0 ? `${m}m ${s.toString().padStart(2, "0")}s` : `${s}s`;
}

/**
 * Horizontal gantt of a run's task durations. Each bar is positioned by the
 * task's start offset and sized by its duration relative to the run total.
 */
export function TaskTimeline({
  tasks,
  logs,
  className,
}: {
  tasks: Task[];
  logs: LogLine[];
  className?: string;
}) {
  const total = Math.max(
    ...tasks.map((t) => t.startOffset + t.duration),
    1,
  );

  return (
    <div className={cn("flex flex-col", className)}>
      <div className="flex items-center justify-between border-b border-border px-4 py-2 text-[0.65rem] font-medium tracking-wider text-muted-foreground">
        <span>TASK</span>
        <span>{formatDuration(total)} total</span>
      </div>
      <div className="divide-y divide-border">
        {tasks.map((task) => {
          const style = TASK_STATUS_STYLE[task.status];
          const left = (task.startOffset / total) * 100;
          const width = Math.max((task.duration / total) * 100, 0.75);
          const isPending = task.duration <= 0;
          return (
            <div
              key={task.id}
              className="grid grid-cols-[220px_1fr_84px_auto] items-center gap-3 px-4 py-2.5 hover:bg-muted/30"
            >
              <div className="flex min-w-0 items-center gap-2">
                <TaskStatusDot status={task.status} />
                <span className="truncate font-mono text-xs text-foreground">
                  {task.label}
                </span>
              </div>

              <div className="relative h-5 w-full rounded-sm bg-muted/40">
                {isPending ? (
                  <div
                    className="absolute top-0 bottom-0 w-0.5 rounded bg-zinc-500/50"
                    style={{ left: `${left}%` }}
                  />
                ) : (
                  <div
                    className={cn(
                      "absolute top-0.5 bottom-0.5 rounded-sm",
                      style.dot,
                      task.status === "running" && "animate-pulse",
                      task.status === "failed" && "opacity-90",
                    )}
                    style={{ left: `${left}%`, width: `${width}%` }}
                  />
                )}
              </div>

              <span className="text-right font-mono text-xs tabular-nums text-muted-foreground">
                {formatDuration(task.duration)}
              </span>

              <LogViewerSheet
                logs={logs}
                taskId={task.id}
                title={task.label}
                description={`${style.label} · ${task.worker} · attempt ${task.attempts}`}
                trigger={
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`View logs for ${task.label}`}
                  >
                    <ScrollText />
                  </Button>
                }
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}
