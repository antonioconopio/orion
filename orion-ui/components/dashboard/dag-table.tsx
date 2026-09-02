"use client";

import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { DAG_ROW_GRID } from "./dag-table-grid";
import { DagRow } from "./dag-row";
import type { Dag } from "./types";

export function DagTable({ dags }: { dags: Dag[] }) {
  return (
    <Card className="w-full gap-0 py-0">
      <div
        className={cn(
          DAG_ROW_GRID,
          "items-center border-b border-border px-4 py-2.5 text-[0.65rem] font-medium tracking-wider text-muted-foreground",
        )}
      >
        <span>DAG</span>
        <span>LAST STATUS</span>
        <span>LAST RUN</span>
        <span>RUN HISTORY</span>
        <span className="text-right">AVG DURATION</span>
        <span />
      </div>
      <div className="divide-y divide-border">
        {dags.map((dag) => (
          <DagRow key={dag.id} dag={dag} />
        ))}
      </div>
    </Card>
  );
}
