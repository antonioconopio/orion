import { cn } from "@/lib/utils";

/**
 * Consistent page top bar: an eyebrow label on the left and optional actions
 * on the right, matching the dashboard's header treatment.
 */
export function PageHeader({
  label,
  children,
  className,
}: {
  label: string;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <>
      <div
        className={cn(
          "flex h-12.5 w-full flex-row items-center justify-between px-4",
          className,
        )}
      >
        <p className="text-xs tracking-wider text-muted-foreground">{label}</p>
        {children && <div className="flex items-center gap-2">{children}</div>}
      </div>
      <hr className="w-full" />
    </>
  );
}
