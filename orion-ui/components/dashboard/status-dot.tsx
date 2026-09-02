"use client";

import { cn } from "@/lib/utils";
import { DAG_STATUS_STYLE } from "./dag-status-style";
import type { DagStatus } from "./types";

export function StatusDot({
  status,
  className,
}: {
  status: DagStatus;
  className?: string;
}) {
  const style = DAG_STATUS_STYLE[status];

  return (
    <span className={cn("relative inline-flex size-2 shrink-0", className)}>
      {status === "running" && (
        <span
          className={cn(
            "absolute inline-flex size-full animate-ping rounded-full opacity-60",
            style.dot,
          )}
        />
      )}
      <span
        className={cn("relative inline-flex size-2 rounded-full", style.dot)}
      />
    </span>
  );
}
