"use client";

import { use, useMemo, useState } from "react";
import Link from "next/link";
import { notFound, useRouter } from "next/navigation";
import { ArrowLeft, RotateCw, ScrollText } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { EmptyState, ErrorState } from "@/components/data-states";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TaskStatusBadge } from "@/components/task-status-badge";
import { DagGraph } from "@/components/graph/dag-graph";
import { GraphLegend } from "@/components/graph/graph-legend";
import { TaskTimeline } from "@/components/runs/task-timeline";
import { LogViewer } from "@/components/runs/log-viewer";
import { LogViewerSheet } from "@/components/runs/log-viewer-sheet";
import { TaskOutputs } from "@/components/runs/task-outputs";
import {
  formatDuration,
  runDurationSeconds,
  runLogs,
  statusByTask,
  toGraph,
  toOutputs,
  toRun,
  toTimelineTasks,
} from "@/lib/api/adapters";
import { useNow } from "@/lib/use-now";
import {
  errorMessage,
  isNotFound,
  useGetDagDependenciesQuery,
  useGetDagQuery,
  useGetDagTasksQuery,
  useGetRunInstancesQuery,
  useGetRunQuery,
  useTriggerRunMutation,
} from "@/store/api";

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-[0.6rem] tracking-wider text-muted-foreground">
        {label}
      </span>
      <span className="font-mono text-xs">{value}</span>
    </div>
  );
}

export default function RunDetailPage({
  params,
}: {
  params: Promise<{ runId: string }>;
}) {
  const { runId } = use(params);
  const router = useRouter();
  const now = useNow(1000); // a running run's duration ticks

  const runQuery = useGetRunQuery(runId);
  const instancesQuery = useGetRunInstancesQuery(runId);
  const apiRun = runQuery.data;
  const dagId = apiRun?.dag_id ?? "";
  // The graph's shape comes from the DAG; its colors come from this run's instances.
  const tasksQuery = useGetDagTasksQuery(dagId, { skip: !dagId });
  const depsQuery = useGetDagDependenciesQuery(dagId, { skip: !dagId });
  const dagQuery = useGetDagQuery(dagId, { skip: !dagId });

  const [trigger, { isLoading: rerunning }] = useTriggerRunMutation();
  const [rerunError, setRerunError] = useState<string | null>(null);

  const run = useMemo(() => (apiRun ? toRun(apiRun, now) : undefined), [apiRun, now]);
  const instances = instancesQuery.data;
  const graph = useMemo(
    () => toGraph(tasksQuery.data ?? [], depsQuery.data ?? [], statusByTask(instances ?? [])),
    [tasksQuery.data, depsQuery.data, instances],
  );
  const timelineTasks = useMemo(
    () => (apiRun && instances ? toTimelineTasks(instances, apiRun, now) : []),
    [apiRun, instances, now],
  );
  const logs = useMemo(() => runLogs(instances ?? []), [instances]);
  const outputs = useMemo(() => toOutputs(instances ?? []), [instances]);

  if (isNotFound(runQuery.error)) notFound();

  const rerun = async () => {
    if (!apiRun) return;
    setRerunError(null);
    try {
      const { run: next } = await trigger(apiRun.dag_id).unwrap();
      router.push(`/runs/${next.id}`);
    } catch (e) {
      setRerunError(errorMessage(e));
    }
  };

  const title = run ? `${run.dagName} ${run.number}` : "Run";
  const graphLoading = tasksQuery.isLoading || depsQuery.isLoading || instancesQuery.isLoading;

  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="RUN DETAIL">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/runs">
            <ArrowLeft />
            All runs
          </Link>
        </Button>
        <LogViewerSheet
          logs={logs}
          title={title}
          description={`full log · run ${runId}`}
          trigger={
            <Button variant="outline" size="sm">
              <ScrollText />
              Full log
            </Button>
          }
        />
        {rerunError && <span className="text-xs text-red-400">{rerunError}</span>}
        <Button
          variant="default"
          size="sm"
          className="rounded-md"
          onClick={rerun}
          disabled={rerunning || !apiRun}
        >
          <RotateCw />
          Re-run
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          {/* Header */}
          {runQuery.isError ? (
            <ErrorState onRetry={runQuery.refetch} />
          ) : !run || !apiRun ? (
            <Skeleton className="h-16 w-full" />
          ) : (
            <div className="flex flex-col gap-2">
              <div className="flex flex-wrap items-center gap-3">
                <Link
                  href={`/dags/${run.dagId}`}
                  className="font-mono text-lg font-semibold hover:underline"
                >
                  {run.dagName}
                </Link>
                <span className="font-mono text-lg text-muted-foreground">
                  {run.number}
                </span>
                <TaskStatusBadge status={run.status} />
              </div>
              <div className="flex flex-wrap items-center gap-x-8 gap-y-2">
                <Meta label="RUN ID" value={run.id} />
                <Meta label="TRIGGER" value={run.trigger} />
                <Meta label="STARTED" value={run.startedAt} />
                <Meta
                  label="DURATION"
                  value={formatDuration(runDurationSeconds(apiRun, now))}
                />
                <Meta label="TASKS" value={`${run.tasksDone}/${run.tasksTotal}`} />
                <Meta
                  label="AVG DURATION"
                  value={formatDuration(dagQuery.data?.avg_duration_seconds)}
                />
              </div>
            </div>
          )}

          {/* Graph / Timeline */}
          <Tabs defaultValue="graph">
            <TabsList>
              <TabsTrigger value="graph">Graph</TabsTrigger>
              <TabsTrigger value="timeline">Timeline</TabsTrigger>
            </TabsList>

            <TabsContent value="graph">
              <Card className="gap-0 p-0">
                <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
                  <span className="text-sm font-medium">Task graph</span>
                  <GraphLegend />
                </div>
                <div className="h-[520px]">
                  {graphLoading ? (
                    <Skeleton className="size-full rounded-none" />
                  ) : graph.nodes.length === 0 ? (
                    <EmptyState title="No tasks" />
                  ) : (
                    <DagGraph nodes={graph.nodes} edges={graph.edges} />
                  )}
                </div>
              </Card>
            </TabsContent>

            <TabsContent value="timeline">
              <Card className="gap-0 p-0">
                {timelineTasks.length === 0 ? (
                  <EmptyState title="No tasks" />
                ) : (
                  <TaskTimeline tasks={timelineTasks} logs={logs} />
                )}
              </Card>
            </TabsContent>
          </Tabs>

          {/* Outputs */}
          <div className="flex flex-col gap-2">
            <h2 className="text-sm font-medium">Outputs</h2>
            <TaskOutputs outputs={outputs} />
          </div>

          {/* Logs */}
          <Card className="gap-0 overflow-hidden p-0">
            <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
              <span className="text-sm font-medium">Logs</span>
              <LogViewerSheet
                logs={logs}
                title={title}
                description={`full log · run ${runId}`}
                trigger={
                  <Button variant="ghost" size="sm">
                    <ScrollText />
                    Expand
                  </Button>
                }
              />
            </div>
            <LogViewer logs={logs} className="h-72" />
          </Card>
        </div>
      </div>
    </div>
  );
}
