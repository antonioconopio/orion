"use client";

import { cn } from "@/lib/utils";
import { DAG_STATUS_STYLE } from "./dag-status-style";
import type { DagStatus } from "./types";

const WIDTH = 96;
const HEIGHT = 28;
const PADDING = 3;

export function Sparkline({
  data,
  status,
  className,
}: {
  data: number[];
  status: DagStatus;
  className?: string;
}) {
  const max = Math.max(...data);
  const min = Math.min(...data);
  const range = max - min || 1;
  const stepX = (WIDTH - PADDING * 2) / (data.length - 1);

  const points = data.map((value, index) => {
    const x = PADDING + index * stepX;
    const y = PADDING + (1 - (value - min) / range) * (HEIGHT - PADDING * 2);
    return [x, y] as const;
  });

  const path = points
    .map(([x, y], index) => `${index === 0 ? "M" : "L"}${x},${y}`)
    .join(" ");
  const [lastX, lastY] = points[points.length - 1];

  return (
    <svg
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
      className={cn("h-7 w-24 shrink-0 overflow-visible", className)}
      preserveAspectRatio="none"
    >
      <path
        d={path}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.25}
        strokeLinecap="round"
        strokeLinejoin="round"
        className="text-muted-foreground/40"
      />
      <circle
        cx={lastX}
        cy={lastY}
        r={2.25}
        className={DAG_STATUS_STYLE[status].fill}
      />
    </svg>
  );
}
