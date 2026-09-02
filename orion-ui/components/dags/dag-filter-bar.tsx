import { ListFilter, Search, Users } from "lucide-react";

import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

/**
 * Presentational search + filter bar for the DAGs list. Visual only — no
 * filtering logic is wired in this pass.
 */
export function DagFilterBar() {
  return (
    <div className="flex w-full items-center gap-2">
      <div className="relative flex-1">
        <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search DAGs by name, team, or owner…"
          className="h-8 pl-8 font-mono text-xs"
        />
      </div>
      <Button variant="outline" size="sm">
        <ListFilter />
        Status
      </Button>
      <Button variant="outline" size="sm">
        <Users />
        Team
      </Button>
    </div>
  );
}
