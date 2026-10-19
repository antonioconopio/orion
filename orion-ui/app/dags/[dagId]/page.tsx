"use client";

import { use, useMemo, useState } from "react";
import Link from "next/link";
import { notFound, useRouter } from "next/navigation";
import { ArrowLeft, Play } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { EmptyState, ErrorState, RowsSkeleton } from "@/components/data-states";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/dashboard/status-badge";
import { DagGraph } from "@/components/graph/dag-graph";
import { GraphLegend } from "@/components/graph/graph-legend";
import { RunTable } from "@/components/runs/run-table";
import { statusByTask, toDag, toGraph, toRun } from "@/lib/api/adapters";
import { useNow } from "@/lib/use-now";
import {
  errorMessage,
  isNotFound,
  useGetDagDependenciesQuery,
  useGetDagQuery,
  useGetDagTasksQuery,
  useGetRunInstancesQuery,
  useGetRunsQuery,
  useTriggerRunMutation,
} from "@/store/api";

function Meta({
  label,
  value,
  mono = true,
}: {
  label: string;
  value?: string;
  mono?: boolean;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-[0.6rem] tracking-wider text-muted-foreground">
        {label}
      </span>
      <span className={mono ? "font-mono text-xs" : "text-xs"}>
        {value ?? "—"}
      </span>
    </div>
  );
}

export default function DagDetailPage({
  params,
}: {
  params: Promise<{ dagId: string }>;
}) {
  const { dagId } = use(params);
  const router = useRouter();
  const now = useNow(15_000);

  const dagQuery = useGetDagQuery(dagId);
  const tasksQuery = useGetDagTasksQuery(dagId);
  const depsQuery = useGetDagDependenciesQuery(dagId);
  const runsQuery = useGetRunsQuery({ dagId, limit: 50 });
  const lastRunId = dagQuery.data?.last_run?.id;
  const instancesQuery = useGetRunInstancesQuery(lastRunId ?? "", { skip: !lastRunId });

  const [trigger, { isLoading: triggering }] = useTriggerRunMutation();
  const [triggerError, setTriggerError] = useState<string | null>(null);

  const dag = useMemo(() => (dagQuery.data ? toDag(dagQuery.data, now) : undefined), [dagQuery.data, now]);
  const runs = useMemo(() => (runsQuery.data ?? []).map((r) => toRun(r, now)), [runsQuery.data, now]);
  // The graph is colored by the DAG's latest run; tasks with no instance yet show as pending.
  const graph = useMemo(
    () => toGraph(tasksQuery.data ?? [], depsQuery.data ?? [], statusByTask(instancesQuery.data ?? [])),
    [tasksQuery.data, depsQuery.data, instancesQuery.data],
  );

  if (isNotFound(dagQuery.error)) notFound();

  const startRun = async () => {
    setTriggerError(null);
    try {
      const { run } = await trigger(dagId).unwrap();
      router.push(`/runs/${run.id}`);
    } catch (e) {
      setTriggerError(errorMessage(e));
    }
  };

  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="DAG DETAIL">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/dags">
            <ArrowLeft />
            All DAGs
          </Link>
        </Button>
        {triggerError && <span className="text-xs text-red-400">{triggerError}</span>}
        <Button
          variant="default"
          size="sm"
          className="rounded-md"
          onClick={startRun}
          disabled={triggering || !dag}
        >
          <Play />
          Trigger run
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          {dagQuery.isError ? (
            <ErrorState onRetry={dagQuery.refetch} />
          ) : !dag ? (
            <Skeleton className="h-16 w-full" />
          ) : (
            /* Header */
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="flex flex-col gap-2">
                <div className="flex items-center gap-3">
                  <h1 className="font-mono text-lg font-semibold">{dag.name}</h1>
                  <StatusBadge status={dag.status} />
                </div>
                <div className="flex flex-wrap items-center gap-x-8 gap-y-2">
                  <Meta label="SCHEDULE" value={dag.schedule} />
                  <Meta label="OWNER" value={dag.owner} />
                  <Meta label="PYTHON" value={dagQuery.data?.python_version ?? undefined} />
                  <Meta label="AVG DURATION" value={dag.avgDuration} />
                  <Meta
                    label="SUCCESS RATE"
                    value={dag.successRate == null ? undefined : `${dag.successRate}%`}
                  />
                  <Meta label="LAST RUN" value={dag.lastRun} mono={false} />
                </div>
              </div>
            </div>
          )}

          {/* Graph */}
          <Card className="gap-0 p-0">
            <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
              <span className="text-sm font-medium">Task graph</span>
              <GraphLegend />
            </div>
            <div className="h-[520px]">
              {tasksQuery.isLoading || depsQuery.isLoading ? (
                <Skeleton className="size-full rounded-none" />
              ) : tasksQuery.isError || depsQuery.isError ? (
                <ErrorState onRetry={tasksQuery.refetch} />
              ) : graph.nodes.length === 0 ? (
                <EmptyState title="No tasks yet" hint="Add tasks with POST /dags/{id}/tasks." />
              ) : (
                <DagGraph nodes={graph.nodes} edges={graph.edges} />
              )}
            </div>
          </Card>

          {/* Run history */}
          <div className="flex flex-col gap-2">
            <h2 className="text-sm font-medium">Run history</h2>
            {runsQuery.isLoading ? (
              <RowsSkeleton rows={3} />
            ) : runsQuery.isError ? (
              <ErrorState onRetry={runsQuery.refetch} />
            ) : runs.length === 0 ? (
              <EmptyState title="No runs yet" hint="Use Trigger run to start the first one." />
            ) : (
              <RunTable runs={runs} showDag={false} />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
