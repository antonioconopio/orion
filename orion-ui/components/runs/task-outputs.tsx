import { Braces } from "lucide-react";

import { EmptyState } from "@/components/data-states";
import { Card } from "@/components/ui/card";
import type { TaskOutput } from "@/lib/api/adapters";

/** The JSON each task published (by writing it to the file at $ORION_OUTPUT). */
export function TaskOutputs({ outputs }: { outputs: TaskOutput[] }) {
  if (outputs.length === 0) {
    return (
      <EmptyState
        title="No task published an output"
        hint="A task publishes a result by writing JSON to the file named in its ORION_OUTPUT environment variable. Downstream tasks read it from ORION_INPUTS."
      />
    );
  }
  return (
    <div className="flex flex-col gap-3">
      {outputs.map((o) => (
        <Card key={o.taskId} className="gap-0 overflow-hidden p-0">
          <div className="flex items-center gap-2 border-b border-border px-4 py-2">
            <Braces className="size-3.5 text-muted-foreground" />
            <span className="font-mono text-xs">{o.taskName}</span>
          </div>
          <pre className="max-h-60 overflow-auto bg-[#0a0a0f] px-4 py-3 font-mono text-xs leading-relaxed text-zinc-200">
            {o.json}
          </pre>
        </Card>
      ))}
    </div>
  );
}
