import { Plus } from "lucide-react";

import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";

const CONNECTIONS = [
  { name: "warehouse_pg", type: "postgres", host: "pg.orion.internal:5432", online: true },
  { name: "task_queue", type: "redis", host: "redis.orion.internal:6379", online: true },
  { name: "raw_events", type: "s3", host: "s3://orion-raw", online: true },
  { name: "metrics_sink", type: "http", host: "https://metrics.internal", online: false },
];

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs text-muted-foreground">{label}</label>
      <Input defaultValue={value} className="font-mono text-xs" />
    </div>
  );
}

export default function SettingsPage() {
  return (
    <div className="flex h-screen w-full flex-col">
      <PageHeader label="SETTINGS" />

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-3xl flex-col gap-6">
          <div>
            <h1 className="text-lg font-semibold">Settings</h1>
            <p className="text-sm text-muted-foreground">
              Workspace configuration and external connections.
            </p>
          </div>

          {/* General */}
          <section className="flex flex-col gap-3">
            <h2 className="text-sm font-medium">General</h2>
            <Card className="gap-4 p-4">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field label="Workspace name" value="orion-prod" />
                <Field label="Scheduler timezone" value="UTC" />
                <Field label="API base URL" value="https://api.orion.internal" />
                <Field label="Default retries" value="3" />
              </div>
            </Card>
          </section>

          {/* Connections */}
          <section className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-medium">Connections</h2>
              <Button variant="outline" size="sm">
                <Plus />
                Add connection
              </Button>
            </div>
            <Card className="gap-0 p-0">
              <div className="divide-y divide-border">
                {CONNECTIONS.map((conn) => (
                  <div
                    key={conn.name}
                    className="flex items-center justify-between px-4 py-3"
                  >
                    <div className="flex min-w-0 flex-col">
                      <span className="font-mono text-sm text-foreground">
                        {conn.name}
                      </span>
                      <span className="font-mono text-xs text-muted-foreground">
                        {conn.host}
                      </span>
                    </div>
                    <div className="flex items-center gap-3">
                      <Badge variant="outline" className="font-mono uppercase">
                        {conn.type}
                      </Badge>
                      <span className="flex items-center gap-1.5">
                        <span
                          className={cn(
                            "size-1.5 rounded-full",
                            conn.online ? "bg-emerald-500" : "bg-zinc-600",
                          )}
                        />
                        <span className="font-mono text-xs text-muted-foreground">
                          {conn.online ? "connected" : "offline"}
                        </span>
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </Card>
          </section>
        </div>
      </div>
    </div>
  );
}
