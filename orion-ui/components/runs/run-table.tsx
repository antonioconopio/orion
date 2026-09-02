import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { RUN_ROW_GRID } from "./run-table-grid";
import { RunRow } from "./run-row";
import type { Run } from "@/lib/types";

export function RunTable({
  runs,
  showDag = true,
}: {
  runs: Run[];
  showDag?: boolean;
}) {
  return (
    <Card className="w-full gap-0 py-0">
      <div
        className={cn(
          RUN_ROW_GRID,
          "border-b border-border px-4 py-2.5 text-[0.65rem] font-medium tracking-wider text-muted-foreground",
        )}
      >
        <span>{showDag ? "DAG / RUN" : "RUN"}</span>
        <span>STATUS</span>
        <span>TRIGGER</span>
        <span>STARTED</span>
        <span>PROGRESS</span>
        <span className="text-right">DURATION</span>
        <span />
      </div>
      <div className="divide-y divide-border">
        {runs.map((run) => (
          <RunRow key={run.id} run={run} showDag={showDag} />
        ))}
      </div>
    </Card>
  );
}
