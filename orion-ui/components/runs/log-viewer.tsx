import { cn } from "@/lib/utils";
import { LOG_LEVEL_STYLE } from "@/lib/status-style";
import type { LogLine } from "@/lib/types";

/**
 * Static monospace log panel. Scrolls internally; parent controls height.
 * `taskId` optionally filters to a single task's lines.
 */
export function LogViewer({
  logs,
  taskId,
  className,
}: {
  logs: LogLine[];
  taskId?: string;
  className?: string;
}) {
  const lines = taskId ? logs.filter((l) => l.taskId === taskId) : logs;

  return (
    <div
      className={cn(
        "h-full overflow-auto bg-[#0a0a0f] py-2 font-mono text-xs leading-relaxed",
        className,
      )}
    >
      {lines.map((line, i) => {
        const level = LOG_LEVEL_STYLE[line.level];
        return (
          <div
            key={i}
            className="flex gap-3 px-4 py-0.5 hover:bg-white/[0.03]"
          >
            <span className="shrink-0 tabular-nums text-muted-foreground/50">
              {line.timestamp}
            </span>
            <span className={cn("w-11 shrink-0 font-semibold", level.className)}>
              {level.label}
            </span>
            <span className="w-44 shrink-0 truncate text-violet-300/70">
              {line.taskId}
            </span>
            <span className="min-w-0 text-foreground/85">{line.message}</span>
          </div>
        );
      })}
      {lines.length === 0 && (
        <p className="px-4 py-2 text-muted-foreground">No log lines.</p>
      )}
    </div>
  );
}
