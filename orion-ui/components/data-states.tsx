import { AlertTriangle, Inbox } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

/** Placeholder rows while a list loads. */
export function RowsSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <Card className="w-full gap-0 py-0">
      <div className="divide-y divide-border">
        {Array.from({ length: rows }, (_, i) => (
          <div key={i} className="flex items-center gap-3 px-4 py-3.5">
            <Skeleton className="size-2 rounded-full" />
            <Skeleton className="h-4 w-48" />
            <Skeleton className="ml-auto h-4 w-24" />
          </div>
        ))}
      </div>
    </Card>
  );
}

export function EmptyState({ title, hint }: { title: string; hint?: string }) {
  return (
    <Card className="items-center gap-2 px-4 py-10 text-center">
      <Inbox className="size-6 text-muted-foreground/60" />
      <p className="text-sm font-medium">{title}</p>
      {hint && <p className="max-w-md text-xs text-muted-foreground">{hint}</p>}
    </Card>
  );
}

export function ErrorState({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  return (
    <Card className="items-center gap-2 px-4 py-10 text-center">
      <AlertTriangle className="size-6 text-red-400" />
      <p className="text-sm font-medium">{message ?? "Could not load this"}</p>
      <p className="text-xs text-muted-foreground">The Orion API may be down. This page will update on its own once it is back.</p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          Retry
        </Button>
      )}
    </Card>
  );
}
