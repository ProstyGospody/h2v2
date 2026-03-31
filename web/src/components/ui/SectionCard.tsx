import type { ReactNode } from "react";

import { cn } from "./cn";

export function SectionTitle({
  icon,
  title,
  className,
}: {
  icon: ReactNode;
  title: string;
  className?: string;
}) {
  return (
    <div className={cn("flex items-center gap-3", className)}>
      <div className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-surface-3/55 text-txt-secondary">
        {icon}
      </div>
      <h3 className="text-[15px] font-semibold text-txt-primary">{title}</h3>
    </div>
  );
}

export function SectionCard({
  title,
  icon,
  children,
  className,
}: {
  title: string;
  icon: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("panel-card min-w-0 space-y-4", className)}>
      <SectionTitle icon={icon} title={title} />
      {children}
    </section>
  );
}
