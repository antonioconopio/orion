import { cn } from "@/lib/utils";
import { TASK_STATUS_STYLE } from "@/lib/status-style";
import type { TaskStatus } from "@/lib/types";

export function TaskStatusDot({
  status,
  className,
}: {
  status: TaskStatus;
  className?: string;
}) {
  const style = TASK_STATUS_STYLE[status];
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
      <span className={cn("relative inline-flex size-2 rounded-full", style.dot)} />
    </span>
  );
}
