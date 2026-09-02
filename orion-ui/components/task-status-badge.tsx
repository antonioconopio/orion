import { cn } from "@/lib/utils";
import { TASK_STATUS_STYLE } from "@/lib/status-style";
import type { TaskStatus } from "@/lib/types";

export function TaskStatusBadge({
  status,
  className,
}: {
  status: TaskStatus;
  className?: string;
}) {
  const style = TASK_STATUS_STYLE[status];
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
