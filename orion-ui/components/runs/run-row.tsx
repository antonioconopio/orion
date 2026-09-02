import Link from "next/link";
import { ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";
import { TaskStatusBadge } from "@/components/task-status-badge";
import { TaskStatusDot } from "@/components/task-status-dot";
import { RUN_ROW_GRID } from "./run-table-grid";
import type { Run } from "@/lib/types";

export function RunRow({ run, showDag = true }: { run: Run; showDag?: boolean }) {
  const pct = Math.round((run.tasksDone / run.tasksTotal) * 100);
  return (
    <Link
      href={`/runs/${run.id}`}
      className={cn(
        RUN_ROW_GRID,
        "group/row px-4 py-3 transition-colors hover:bg-muted/40",
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <TaskStatusDot status={run.status} />
        <div className="flex min-w-0 flex-col">
          <span className="truncate font-mono text-sm font-medium text-foreground">
            {showDag ? run.dagName : run.number}
          </span>
          <span className="truncate font-mono text-xs text-muted-foreground">
            {showDag ? `${run.number} · ${run.id}` : run.id}
          </span>
        </div>
      </div>

      <TaskStatusBadge status={run.status} />

      <span className="font-mono text-xs text-muted-foreground">
        {run.trigger}
      </span>

      <span className="font-mono text-xs tabular-nums text-muted-foreground">
        {run.startedAt}
      </span>

      <div className="flex flex-col gap-1">
        <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-primary"
            style={{ width: `${pct}%` }}
          />
        </div>
        <span className="font-mono text-[0.65rem] text-muted-foreground tabular-nums">
          {run.tasksDone}/{run.tasksTotal} tasks
        </span>
      </div>

      <span className="text-right font-mono text-sm tabular-nums text-muted-foreground">
        {run.duration}
      </span>

      <ChevronRight className="size-4 justify-self-end text-muted-foreground/50 transition-transform group-hover/row:translate-x-0.5" />
    </Link>
  );
}
