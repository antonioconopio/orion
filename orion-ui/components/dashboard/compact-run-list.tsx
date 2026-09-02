import Link from "next/link";
import { ChevronRight } from "lucide-react";

import { TaskStatusBadge } from "@/components/task-status-badge";
import { TaskStatusDot } from "@/components/task-status-dot";
import type { Run } from "@/lib/types";

/** Condensed, single-column run list for dashboard cards. */
export function CompactRunList({ runs }: { runs: Run[] }) {
  return (
    <div className="flex flex-col divide-y divide-border">
      {runs.map((run) => (
        <Link
          key={run.id}
          href={`/runs/${run.id}`}
          className="group/row flex items-center gap-3 py-2.5 transition-colors hover:bg-muted/40"
        >
          <TaskStatusDot status={run.status} />
          <div className="flex min-w-0 flex-1 flex-col">
            <span className="truncate font-mono text-sm text-foreground">
              {run.dagName}
            </span>
            <span className="truncate font-mono text-xs text-muted-foreground">
              {run.number} · {run.startedAt}
            </span>
          </div>
          <TaskStatusBadge status={run.status} />
          <span className="w-16 text-right font-mono text-xs tabular-nums text-muted-foreground">
            {run.duration}
          </span>
          <ChevronRight className="size-4 text-muted-foreground/50 transition-transform group-hover/row:translate-x-0.5" />
        </Link>
      ))}
    </div>
  );
}
