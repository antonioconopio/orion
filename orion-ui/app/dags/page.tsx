"use client";

import { useMemo } from "react";

import { PageHeader } from "@/components/page-header";
import { EmptyState, ErrorState, RowsSkeleton } from "@/components/data-states";
import { DagStatCard } from "@/components/dashboard/dag-stat-card";
import { DagTable } from "@/components/dashboard/dag-table";
import { DagFilterBar } from "@/components/dags/dag-filter-bar";
import { toDag } from "@/lib/api/adapters";
import { useNow } from "@/lib/use-now";
import { useGetDagsQuery } from "@/store/api";

export default function DagsPage() {
  const { data, isLoading, isError, refetch } = useGetDagsQuery();
  const now = useNow(15_000);
  const dags = useMemo(() => (data ?? []).map((d) => toDag(d, now)), [data, now]);

  const count = (status: string) => dags.filter((d) => d.status === status).length;

  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="DAGS" />

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          <div className="flex items-end justify-between">
            <div>
              <h1 className="text-lg font-semibold">DAGs</h1>
              <p className="text-sm text-muted-foreground">
                All workflows in this workspace.
              </p>
            </div>
            <div className="flex gap-2">
              <DagStatCard
                label="TOTAL"
                value={dags.length}
                dotClassName="bg-violet-400"
              />
              <DagStatCard
                label="RUNNING"
                value={count("running")}
                dotClassName="bg-amber-500"
                valueClassName="text-amber-500"
              />
              <DagStatCard
                label="FAILED"
                value={count("failed")}
                dotClassName="bg-red-500"
                valueClassName="text-red-500"
              />
              <DagStatCard
                label="SUCCESS"
                value={count("success")}
                dotClassName="bg-emerald-500"
                valueClassName="text-emerald-500"
              />
            </div>
          </div>

          <DagFilterBar />

          {isLoading ? (
            <RowsSkeleton />
          ) : isError ? (
            <ErrorState onRetry={refetch} />
          ) : dags.length === 0 ? (
            <EmptyState
              title="No DAGs yet"
              hint="Create one with POST /dags (a DAG editor arrives with the YAML spec)."
            />
          ) : (
            <DagTable dags={dags} />
          )}
        </div>
      </div>
    </div>
  );
}
