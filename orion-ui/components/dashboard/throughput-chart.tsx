import { cn } from "@/lib/utils";

/** Minimal responsive bar chart for run throughput over time. */
export function ThroughputChart({
  data,
  className,
}: {
  data: number[];
  className?: string;
}) {
  const max = Math.max(...data, 1);
  return (
    <div className={cn("flex h-full items-end gap-[3px]", className)}>
      {data.map((value, i) => (
        <div
          key={i}
          className="group/bar flex-1 rounded-t-sm bg-primary/60 transition-colors hover:bg-primary"
          style={{ height: `${Math.max((value / max) * 100, 3)}%` }}
          title={`${value} runs`}
        />
      ))}
    </div>
  );
}
