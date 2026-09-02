"use client";

import { useMemo } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  ReactFlow,
  type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { cn } from "@/lib/utils";
import { TaskNode, type TaskFlowNode } from "./task-node";
import type { TaskEdge, TaskGraphNode, TaskStatus } from "@/lib/types";

const nodeTypes = { task: TaskNode };

/**
 * Static, pan/zoom-able task graph. Nodes are colored by status; edges leading
 * into a running/retrying task are highlighted to trace the active path.
 * Reused by both the DAG-detail and run-detail views.
 */
export function DagGraph({
  nodes,
  edges,
  className,
}: {
  nodes: TaskGraphNode[];
  edges: TaskEdge[];
  className?: string;
}) {
  const rfNodes: TaskFlowNode[] = useMemo(
    () =>
      nodes.map((n) => ({
        id: n.id,
        type: "task",
        position: n.position,
        data: { label: n.label, status: n.status },
      })),
    [nodes],
  );

  const rfEdges: Edge[] = useMemo(() => {
    const statusById = Object.fromEntries(
      nodes.map((n) => [n.id, n.status]),
    ) as Record<string, TaskStatus>;
    return edges.map((e) => {
      const active =
        statusById[e.target] === "running" ||
        statusById[e.target] === "retrying";
      return {
        id: `${e.source}->${e.target}`,
        source: e.source,
        target: e.target,
        animated: active,
        style: {
          stroke: active ? "var(--primary)" : "var(--border)",
          strokeWidth: 1.5,
        },
      };
    });
  }, [nodes, edges]);

  return (
    <div className={cn("h-full w-full", className)}>
      <ReactFlow
        colorMode="dark"
        defaultNodes={rfNodes}
        defaultEdges={rfEdges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.24 }}
        minZoom={0.35}
        maxZoom={1.5}
        nodesDraggable={false}
        nodesConnectable={false}
        edgesFocusable={false}
        proOptions={{ hideAttribution: true }}
        className="bg-transparent"
      >
        <Background variant={BackgroundVariant.Dots} gap={22} size={1} />
        <Controls showInteractive={false} position="bottom-right" />
      </ReactFlow>
    </div>
  );
}
