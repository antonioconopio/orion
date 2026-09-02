"use client";

import { use } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, Play } from "lucide-react";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { StatusBadge } from "@/components/dashboard/status-badge";
import { DagGraph } from "@/components/graph/dag-graph";
import { GraphLegend } from "@/components/graph/graph-legend";
import { RunTable } from "@/components/runs/run-table";
import {
  DAG_GRAPH_NODES,
  TASK_EDGES,
  getDag,
  getRunsForDag,
} from "@/lib/sample-data";

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
  const dag = getDag(dagId);
  if (!dag) notFound();
  const runs = getRunsForDag(dagId);

  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="DAG DETAIL">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/dags">
            <ArrowLeft />
            All DAGs
          </Link>
        </Button>
        <Button variant="default" size="sm" className="rounded-md">
          <Play />
          Trigger run
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          {/* Header */}
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-3">
                <h1 className="font-mono text-lg font-semibold">{dag.name}</h1>
                <StatusBadge status={dag.status} />
              </div>
              <div className="flex flex-wrap items-center gap-x-8 gap-y-2">
                <Meta label="SCHEDULE" value={dag.schedule} />
                <Meta label="OWNER" value={dag.owner} />
                <Meta label="TEAM" value={dag.team} />
                <Meta label="AVG DURATION" value={dag.avgDuration} />
                <Meta label="SUCCESS RATE" value={`${dag.successRate}%`} />
                <Meta label="LAST RUN" value={dag.lastRun} mono={false} />
              </div>
            </div>
          </div>

          {/* Graph */}
          <Card className="gap-0 p-0">
            <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
              <span className="text-sm font-medium">Task graph</span>
              <GraphLegend />
            </div>
            <div className="h-[520px]">
              <DagGraph nodes={DAG_GRAPH_NODES} edges={TASK_EDGES} />
            </div>
          </Card>

          {/* Run history */}
          <div className="flex flex-col gap-2">
            <h2 className="text-sm font-medium">Run history</h2>
            <RunTable runs={runs} showDag={false} />
          </div>
        </div>
      </div>
    </div>
  );
}
