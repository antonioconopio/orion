import { cn } from "@/lib/utils";
import { Card } from "@/components/ui/card";

const toneClass = {
  up: "text-emerald-400",
  down: "text-red-400",
  neutral: "text-muted-foreground",
} as const;

export function MetricCard({
  label,
  value,
  sub,
  tone = "neutral",
  icon: Icon,
}: {
  label: string;
  value: string | number;
  sub?: string;
  tone?: keyof typeof toneClass;
  icon?: React.ComponentType<{ className?: string }>;
}) {
  return (
    <Card className="gap-2 p-4">
      <div className="flex items-center justify-between">
        <span className="text-[0.65rem] font-medium tracking-wider text-muted-foreground">
          {label}
        </span>
        {Icon && <Icon className="size-4 text-muted-foreground/60" />}
      </div>
      <span className="font-mono text-2xl font-semibold tabular-nums text-foreground">
        {value}
      </span>
      {sub && <span className={cn("text-xs", toneClass[tone])}>{sub}</span>}
    </Card>
  );
}
