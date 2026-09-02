"use client";

import { use } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, RotateCw, ScrollText } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TaskStatusBadge } from "@/components/task-status-badge";
import { DagGraph } from "@/components/graph/dag-graph";
import { GraphLegend } from "@/components/graph/graph-legend";
import { TaskTimeline } from "@/components/runs/task-timeline";
import { LogViewer } from "@/components/runs/log-viewer";
import { LogViewerSheet } from "@/components/runs/log-viewer-sheet";
import {
  RUN_GRAPH_NODES,
  RUN_LOGS,
  RUN_TASKS,
  TASK_EDGES,
  getDag,
  getRun,
} from "@/lib/sample-data";

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
  const run = getRun(runId);
  if (!run) notFound();
  const dag = getDag(run.dagId);

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
          logs={RUN_LOGS}
          title={`${run.dagName} ${run.number}`}
          description={`full log · run ${run.id}`}
          trigger={
            <Button variant="outline" size="sm">
              <ScrollText />
              Full log
            </Button>
          }
        />
        <Button variant="default" size="sm" className="rounded-md">
          <RotateCw />
          Re-run
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          {/* Header */}
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
              <Meta label="DURATION" value={run.duration} />
              <Meta
                label="TASKS"
                value={`${run.tasksDone}/${run.tasksTotal}`}
              />
              <Meta label="AVG DURATION" value={dag?.avgDuration ?? "—"} />
            </div>
          </div>

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
                  <DagGraph nodes={RUN_GRAPH_NODES} edges={TASK_EDGES} />
                </div>
              </Card>
            </TabsContent>

            <TabsContent value="timeline">
              <Card className="gap-0 p-0">
                <TaskTimeline tasks={RUN_TASKS} logs={RUN_LOGS} />
              </Card>
            </TabsContent>
          </Tabs>

          {/* Logs */}
          <Card className="gap-0 overflow-hidden p-0">
            <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
              <span className="text-sm font-medium">Logs</span>
              <LogViewerSheet
                logs={RUN_LOGS}
                title={`${run.dagName} ${run.number}`}
                description={`full log · run ${run.id}`}
                trigger={
                  <Button variant="ghost" size="sm">
                    <ScrollText />
                    Expand
                  </Button>
                }
              />
            </div>
            <LogViewer logs={RUN_LOGS} className="h-72" />
          </Card>
        </div>
      </div>
    </div>
  );
}
