"use client";

import { cn } from "@/lib/utils";
import { DAG_STATUS_STYLE } from "./dag-status-style";
import type { DagStatus } from "./types";

export function StatusBadge({
  status,
  className,
}: {
  status: DagStatus;
  className?: string;
}) {
  const style = DAG_STATUS_STYLE[status];

  return (
    <span
      className={cn(
        "inline-flex w-fit items-center rounded-sm px-2 py-0.5 font-mono text-[0.7rem] font-medium tracking-wide ring-1 ring-inset",
        style.badge,
        className,
      )}
    >
      {style.label}
    </span>
  );
}
