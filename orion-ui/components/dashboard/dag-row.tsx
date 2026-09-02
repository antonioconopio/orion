"use client";

import Link from "next/link";
import { ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { DAG_ROW_GRID } from "./dag-table-grid";
import { Sparkline } from "./sparkline";
import { StatusBadge } from "./status-badge";
import { StatusDot } from "./status-dot";
import type { Dag } from "./types";

export function DagRow({ dag }: { dag: Dag }) {
  return (
    <Link
      href={`/dags/${dag.id}`}
      className={cn(
        DAG_ROW_GRID,
        "group/row items-center px-4 py-3 transition-colors hover:bg-muted/40",
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <StatusDot status={dag.status} />
        <div className="flex min-w-0 flex-col">
          <span className="truncate font-mono text-sm font-medium text-foreground">
            {dag.name}
          </span>
          <span className="truncate font-mono text-xs text-muted-foreground">
            {dag.team} · {dag.schedule}
          </span>
        </div>
      </div>

      <StatusBadge status={dag.status} />

      <div className="flex flex-col">
        <span className="text-sm text-foreground">{dag.lastRun}</span>
        <span className="font-mono text-xs text-muted-foreground">
          run {dag.runNumber}
        </span>
      </div>

      <Sparkline data={dag.history} status={dag.status} />

      <span className="text-right font-mono text-sm text-muted-foreground tabular-nums">
        {dag.avgDuration}
      </span>

      <ChevronRight className="size-4 justify-self-end text-muted-foreground/50 transition-transform group-hover/row:translate-x-0.5" />
    </Link>
  );
}
