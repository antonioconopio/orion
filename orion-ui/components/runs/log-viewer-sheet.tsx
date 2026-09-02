"use client";

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { LogViewer } from "./log-viewer";
import type { LogLine } from "@/lib/types";

/**
 * Opens the log viewer in a slide-over sheet. Used to inspect a single task's
 * logs from the timeline, or the full run log from the run header.
 */
export function LogViewerSheet({
  logs,
  taskId,
  title,
  description,
  trigger,
}: {
  logs: LogLine[];
  taskId?: string;
  title: string;
  description?: string;
  trigger: React.ReactNode;
}) {
  return (
    <Sheet>
      <SheetTrigger asChild>{trigger}</SheetTrigger>
      <SheetContent
        side="right"
        className="flex w-full flex-col gap-0 p-0 sm:max-w-2xl"
      >
        <SheetHeader className="border-b">
          <SheetTitle className="font-mono text-sm">{title}</SheetTitle>
          {description && (
            <SheetDescription className="font-mono text-xs">
              {description}
            </SheetDescription>
          )}
        </SheetHeader>
        <LogViewer logs={logs} taskId={taskId} className="flex-1" />
      </SheetContent>
    </Sheet>
  );
}
