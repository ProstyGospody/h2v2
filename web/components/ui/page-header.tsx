import { type ReactNode } from "react";

import { cn } from "@/src/components/ui";

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h1 className="text-[18px] font-semibold text-white">{title}</h1>
        {subtitle ? <p className="mt-1 text-[12px] text-txt-secondary">{subtitle}</p> : null}
      </div>
      <div className={cn("flex flex-wrap items-center justify-end gap-2", !actions && "hidden")}>{actions}</div>
    </div>
  );
}
