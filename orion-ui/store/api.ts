"use client";

import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";

import { API_URL } from "@/lib/api/config";
import type {
  ApiDag,
  ApiDependency,
  ApiRun,
  ApiStats,
  ApiTask,
  ApiTaskInstance,
  ApiTriggerResponse,
  ApiWorker,
} from "@/lib/api/types";

export interface RunsQuery {
  limit?: number;
  status?: string;
  dagId?: string;
}

/** True when the API says the thing is not there: a 404, or a 400 for a malformed id. */
export function isNotFound(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "status" in error &&
    (error.status === 404 || error.status === 400)
  );
}

/** A readable reason for a failed request: the API's {"error": "..."} text when it sent one. */
export function errorMessage(error: unknown): string {
  if (typeof error === "object" && error !== null) {
    if ("data" in error && typeof error.data === "object" && error.data !== null && "error" in error.data) {
      return String(error.data.error);
    }
    if ("status" in error && error.status === "FETCH_ERROR") return "Cannot reach the Orion API";
  }
  return "Something went wrong";
}

// Each query declares the tags it "provides"; lib/api/events.ts decides which tags a WebSocket
// event makes stale, and RTK Query refetches the matching queries that are on screen.
export const api = createApi({
  reducerPath: "api",
  baseQuery: fetchBaseQuery({ baseUrl: API_URL }),
  tagTypes: ["DagList", "Dag", "DagDefinition", "RunList", "Run", "Instances", "Stats", "Workers"],
  endpoints: (build) => ({
    getDags: build.query<ApiDag[], void>({
      query: () => "/dags",
      providesTags: ["DagList"],
    }),
    getDag: build.query<ApiDag, string>({
      query: (id) => `/dags/${id}`,
      providesTags: (_result, _error, id) => [{ type: "Dag", id }],
    }),
    // A DAG's tasks and dependencies only change when someone edits the DAG, never during a run.
    getDagTasks: build.query<ApiTask[], string>({
      query: (id) => `/dags/${id}/tasks`,
      providesTags: (_result, _error, id) => [{ type: "DagDefinition", id }],
    }),
    getDagDependencies: build.query<ApiDependency[], string>({
      query: (id) => `/dags/${id}/dependencies`,
      providesTags: (_result, _error, id) => [{ type: "DagDefinition", id }],
    }),
    getRuns: build.query<ApiRun[], RunsQuery | void>({
      query: (args) => {
        const params = new URLSearchParams();
        if (args?.limit) params.set("limit", String(args.limit));
        if (args?.status) params.set("status", args.status);
        if (args?.dagId) params.set("dag_id", args.dagId);
        const qs = params.toString();
        return qs ? `/runs?${qs}` : "/runs";
      },
      providesTags: ["RunList"],
    }),
    getRun: build.query<ApiRun, string>({
      query: (id) => `/runs/${id}`,
      providesTags: (_result, _error, id) => [{ type: "Run", id }],
    }),
    getRunInstances: build.query<ApiTaskInstance[], string>({
      query: (runId) => `/runs/${runId}/task-instances`,
      providesTags: (_result, _error, runId) => [{ type: "Instances", id: runId }],
    }),
    getStats: build.query<ApiStats, void>({
      query: () => "/stats",
      providesTags: ["Stats"],
    }),
    getWorkers: build.query<ApiWorker[], void>({
      query: () => "/workers",
      providesTags: ["Workers"],
    }),
    triggerRun: build.mutation<ApiTriggerResponse, string>({
      query: (dagId) => ({ url: `/dags/${dagId}/runs`, method: "POST" }),
      // The WebSocket tells every tab too, but this keeps the UI right even if the socket is down.
      invalidatesTags: ["RunList", "DagList", "Stats"],
    }),
  }),
});

export const {
  useGetDagsQuery,
  useGetDagQuery,
  useGetDagTasksQuery,
  useGetDagDependenciesQuery,
  useGetRunsQuery,
  useGetRunQuery,
  useGetRunInstancesQuery,
  useGetStatsQuery,
  useGetWorkersQuery,
  useTriggerRunMutation,
} = api;
