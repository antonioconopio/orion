"use client";

import { useMemo } from "react";
import { Activity, CircleCheck, TriangleAlert, Server } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { ErrorState } from "@/components/data-states";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { MetricCard } from "@/components/dashboard/metric-card";
import { NewRunMenu } from "@/components/dashboard/new-run-menu";
import { ThroughputChart } from "@/components/dashboard/throughput-chart";
import { WorkerHealth } from "@/components/dashboard/worker-health";
import { CompactRunList } from "@/components/dashboard/compact-run-list";
import { toRun, toWorker } from "@/lib/api/adapters";
import { useNow } from "@/lib/use-now";
import { useGetRunsQuery, useGetStatsQuery, useGetWorkersQuery } from "@/store/api";

function Empty({ children }: { children: React.ReactNode }) {
  return <p className="py-3 text-xs text-muted-foreground">{children}</p>;
}

export default function DashboardPage() {
  const stats = useGetStatsQuery();
  const runsQuery = useGetRunsQuery({ limit: 100 });
  const workersQuery = useGetWorkersQuery(undefined, { pollingInterval: 10_000 }); // see the workers page
  const now = useNow(1000);

  const runs = useMemo(() => (runsQuery.data ?? []).map((r) => toRun(r, now)), [runsQuery.data, now]);
  const workers = useMemo(() => (workersQuery.data ?? []).map((w) => toWorker(w, now)), [workersQuery.data, now]);
  const activeRuns = runs.filter((r) => r.status === "running" || r.status === "queued");
  const recentFailures = runs.filter((r) => r.status === "failed").slice(0, 5);

  const s = stats.data;
  const activeTasks = workers.reduce((sum, w) => sum + w.activeTasks, 0);
  const capacity = workers.reduce((sum, w) => sum + w.capacity, 0);
  const rate = s?.success_rate_24h;

  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="OVERVIEW">
        <NewRunMenu />
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          <div>
            <h1 className="text-lg font-semibold">System overview</h1>
            <p className="text-sm text-muted-foreground">
              Live health of the Orion cluster.
            </p>
          </div>

          {stats.isError && <ErrorState onRetry={stats.refetch} />}

          {/* KPIs */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <MetricCard
              label="ACTIVE RUNS"
              value={s ? s.active_runs : "—"}
              sub="queued or running"
              icon={Activity}
            />
            <MetricCard
              label="SUCCESS RATE · 24H"
              value={rate == null ? "—" : `${(rate * 100).toFixed(1)}%`}
              sub={rate == null ? "no runs finished in 24h" : "of runs finished in 24h"}
              tone={rate == null ? "neutral" : rate >= 0.9 ? "up" : "down"}
              icon={CircleCheck}
            />
            <MetricCard
              label="FAILED · 24H"
              value={s ? s.failed_24h : "—"}
              sub="runs that failed"
              tone={s && s.failed_24h > 0 ? "down" : "neutral"}
              icon={TriangleAlert}
            />
            <MetricCard
              label="WORKERS ONLINE"
              value={workersQuery.data ? workers.length : "—"}
              sub={`${activeTasks}/${capacity} slots in use`}
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
                  {s ? `peak ${Math.max(...s.throughput_24h)}/h` : ""}
                </span>
              </div>
              <div className="h-40">
                {s ? <ThroughputChart data={s.throughput_24h} /> : <Skeleton className="size-full" />}
              </div>
            </Card>

            <Card className="gap-3 p-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">Worker pool</span>
                <span className="font-mono text-xs text-muted-foreground">
                  {workers.length} online
                </span>
              </div>
              {workersQuery.isLoading ? (
                <Skeleton className="h-24 w-full" />
              ) : workers.length === 0 ? (
                <Empty>No workers online.</Empty>
              ) : (
                <WorkerHealth workers={workers} />
              )}
            </Card>
          </div>

          {/* Runs + failures */}
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <Card className="gap-1 p-4">
              <span className="mb-1 text-sm font-medium">Active runs</span>
              {runsQuery.isLoading ? (
                <Skeleton className="h-16 w-full" />
              ) : activeRuns.length === 0 ? (
                <Empty>Nothing running right now.</Empty>
              ) : (
                <CompactRunList runs={activeRuns} />
              )}
            </Card>
            <Card className="gap-1 p-4">
              <span className="mb-1 text-sm font-medium">Recent failures</span>
              {runsQuery.isLoading ? (
                <Skeleton className="h-16 w-full" />
              ) : recentFailures.length === 0 ? (
                <Empty>No recent failures.</Empty>
              ) : (
                <CompactRunList runs={recentFailures} />
              )}
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
