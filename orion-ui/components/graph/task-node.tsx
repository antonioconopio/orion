"use client";

import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";

import { cn } from "@/lib/utils";
import { TASK_STATUS_STYLE } from "@/lib/status-style";
import { TaskStatusDot } from "@/components/task-status-dot";
import type { TaskStatus } from "@/lib/types";

export type TaskNodeData = { label: string; status: TaskStatus };
export type TaskFlowNode = Node<TaskNodeData, "task">;

const handleClass = "!size-1.5 !border-0 !bg-muted-foreground/40";

export function TaskNode({ data }: NodeProps<TaskFlowNode>) {
  const style = TASK_STATUS_STYLE[data.status];
  return (
    <div
      className={cn(
        "flex min-w-[172px] flex-col gap-1 rounded-md border px-3 py-2 shadow-sm backdrop-blur-sm",
        style.node,
      )}
    >
      <Handle type="target" position={Position.Left} className={handleClass} />
      <div className="flex items-center gap-2">
        <TaskStatusDot status={data.status} />
        <span className="font-mono text-xs font-medium">{data.label}</span>
      </div>
      <span
        className={cn(
          "font-mono text-[0.6rem] tracking-wider tabular-nums",
          style.accent,
        )}
      >
        {style.label}
      </span>
      <Handle type="source" position={Position.Right} className={handleClass} />
    </div>
  );
}
