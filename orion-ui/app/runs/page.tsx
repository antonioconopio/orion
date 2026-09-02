import { Search } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { DagStatCard } from "@/components/dashboard/dag-stat-card";
import { RunTable } from "@/components/runs/run-table";
import { RUNS } from "@/lib/sample-data";

const running = RUNS.filter((r) => r.status === "running").length;
const failed = RUNS.filter((r) => r.status === "failed").length;
const succeeded = RUNS.filter((r) => r.status === "success").length;

export default function RunsPage() {
  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="RUNS" />

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          <div className="flex items-end justify-between">
            <div>
              <h1 className="text-lg font-semibold">Runs</h1>
              <p className="text-sm text-muted-foreground">
                Recent workflow executions across all DAGs.
              </p>
            </div>
            <div className="flex gap-2">
              <DagStatCard
                label="RUNNING"
                value={running}
                dotClassName="bg-amber-500"
                valueClassName="text-amber-500"
              />
              <DagStatCard
                label="FAILED"
                value={failed}
                dotClassName="bg-red-500"
                valueClassName="text-red-500"
              />
              <DagStatCard
                label="SUCCESS"
                value={succeeded}
                dotClassName="bg-emerald-500"
                valueClassName="text-emerald-500"
              />
            </div>
          </div>

          <div className="flex w-full items-center gap-2">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search runs by DAG, run id, or status…"
                className="h-8 pl-8 font-mono text-xs"
              />
            </div>
            <Button variant="outline" size="sm">
              Last 24h
            </Button>
          </div>

          <RunTable runs={RUNS} />
        </div>
      </div>
    </div>
  );
}
