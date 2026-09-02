import { Activity, CircleCheck, Plus, TriangleAlert, Server } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { MetricCard } from "@/components/dashboard/metric-card";
import { ThroughputChart } from "@/components/dashboard/throughput-chart";
import { WorkerHealth } from "@/components/dashboard/worker-health";
import { CompactRunList } from "@/components/dashboard/compact-run-list";
import {
  DASHBOARD_METRICS,
  RUNS,
  THROUGHPUT_24H,
  WORKERS,
  WORKER_POOL,
} from "@/lib/sample-data";

const activeRuns = RUNS.filter((r) => r.status === "running");
const recentFailures = RUNS.filter((r) => r.status === "failed");

export default function DashboardPage() {
  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="OVERVIEW">
        <Button variant="default" size="sm" className="rounded-md">
          <Plus />
          New Run
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          <div>
            <h1 className="text-lg font-semibold">System overview</h1>
            <p className="text-sm text-muted-foreground">
              Live health of the Orion cluster across all workspaces.
            </p>
          </div>

          {/* KPIs */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <MetricCard
              label="ACTIVE RUNS"
              value={DASHBOARD_METRICS.activeRuns}
              sub="across 2 workspaces"
              icon={Activity}
            />
            <MetricCard
              label="SUCCESS RATE · 24H"
              value={`${DASHBOARD_METRICS.successRate24h}%`}
              sub="+1.2% vs. yesterday"
              tone="up"
              icon={CircleCheck}
            />
            <MetricCard
              label="FAILED · 24H"
              value={DASHBOARD_METRICS.failed24h}
              sub="2 retriable"
              tone="down"
              icon={TriangleAlert}
            />
            <MetricCard
              label="WORKERS ONLINE"
              value={`${DASHBOARD_METRICS.workersOnline}/${DASHBOARD_METRICS.workersTotal}`}
              sub={`${WORKER_POOL.activeTasks}/${WORKER_POOL.capacity} slots in use`}
              icon={Server}
            />
          </div>

          {/* Charts */}
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
            <Card className="gap-3 p-4 lg:col-span-2">
              <div className="flex items-center justify-between">
                <div className="flex flex-col">
                  <span className="text-sm font-medium">Throughput</span>
                  <span className="text-xs text-muted-foreground">
                    Completed runs per hour · last 24h
                  </span>
                </div>
                <span className="font-mono text-xs text-muted-foreground">
                  peak 26/h
                </span>
              </div>
              <div className="h-40">
                <ThroughputChart data={THROUGHPUT_24H} />
              </div>
            </Card>

            <Card className="gap-3 p-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">Worker pool</span>
                <span className="font-mono text-xs text-muted-foreground">
                  {WORKER_POOL.online}/{WORKER_POOL.total} online
                </span>
              </div>
              <WorkerHealth workers={WORKERS} />
            </Card>
          </div>

          {/* Runs + failures */}
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <Card className="gap-1 p-4">
              <span className="mb-1 text-sm font-medium">Active runs</span>
              <CompactRunList runs={activeRuns} />
            </Card>
            <Card className="gap-1 p-4">
              <span className="mb-1 text-sm font-medium">Recent failures</span>
              <CompactRunList runs={recentFailures} />
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
