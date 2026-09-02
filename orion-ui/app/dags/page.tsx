import { Plus } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { DagStatCard } from "@/components/dashboard/dag-stat-card";
import { DagTable } from "@/components/dashboard/dag-table";
import { DagFilterBar } from "@/components/dags/dag-filter-bar";
import { DAGS, DAG_STATS } from "@/lib/sample-data";

export default function DagsPage() {
  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="DAGS">
        <Button variant="default" size="sm" className="rounded-md">
          <Plus />
          Register DAG
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          <div className="flex items-end justify-between">
            <div>
              <h1 className="text-lg font-semibold">DAGs</h1>
              <p className="text-sm text-muted-foreground">
                All workflows in this workspace, sorted by last activity.
              </p>
            </div>
            <div className="flex gap-2">
              <DagStatCard
                label="TOTAL"
                value={DAG_STATS.total}
                dotClassName="bg-violet-400"
              />
              <DagStatCard
                label="RUNNING"
                value={DAG_STATS.running}
                dotClassName="bg-amber-500"
                valueClassName="text-amber-500"
              />
              <DagStatCard
                label="FAILED"
                value={DAG_STATS.failed}
                dotClassName="bg-red-500"
                valueClassName="text-red-500"
              />
              <DagStatCard
                label="SUCCESS"
                value={DAG_STATS.success}
                dotClassName="bg-emerald-500"
                valueClassName="text-emerald-500"
              />
            </div>
          </div>

          <DagFilterBar />

          <DagTable dags={DAGS} />
        </div>
      </div>
    </div>
  );
}
