"use client";

import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export function DagStatCard({
  label,
  value,
  dotClassName,
  valueClassName,
}: {
  label: string;
  value: number;
  dotClassName?: string;
  valueClassName?: string;
}) {
  return (
    <Card size="sm" className="items-start gap-1 px-3.5 py-2.5">
      <div className="flex items-center gap-1.5">
        {dotClassName && (
          <span className={cn("size-1.5 rounded-full", dotClassName)} />
        )}
        <span
          className={cn(
            "font-mono text-lg leading-none font-semibold tabular-nums",
            valueClassName,
          )}
        >
          {value}
        </span>
      </div>
      <span className="text-[0.6rem] font-medium tracking-wider text-muted-foreground">
        {label}
      </span>
    </Card>
  );
}
