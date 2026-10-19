"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Plus } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { errorMessage, useGetDagsQuery, useTriggerRunMutation } from "@/store/api";

/** "New Run": pick a DAG, start a run, and open it. */
export function NewRunMenu() {
  const router = useRouter();
  const { data: dags = [] } = useGetDagsQuery();
  const [trigger, { isLoading }] = useTriggerRunMutation();
  const [error, setError] = useState<string | null>(null);

  const start = async (dagId: string) => {
    setError(null);
    try {
      const { run } = await trigger(dagId).unwrap();
      router.push(`/runs/${run.id}`);
    } catch (e) {
      setError(errorMessage(e));
    }
  };

  return (
    <>
      {error && <span className="text-xs text-red-400">{error}</span>}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="default" size="sm" className="rounded-md" disabled={isLoading || dags.length === 0}>
            <Plus />
            New Run
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="max-h-80 overflow-y-auto">
          <DropdownMenuLabel>Run a DAG</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {dags.map((dag) => (
            <DropdownMenuItem key={dag.id} className="font-mono text-xs" onSelect={() => start(dag.id)}>
              {dag.name}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}
