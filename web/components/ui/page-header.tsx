import { motion } from "framer-motion";
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
    <div className="space-y-4">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <motion.h1
            layoutId="page-title"
            className="text-[22px] font-bold tracking-tight text-txt-primary sm:text-[24px]"
            transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
          >
            {title}
          </motion.h1>
          {subtitle ? <p className="mt-1 text-[14px] text-txt-secondary">{subtitle}</p> : null}
        </div>
        <div className={cn("flex w-full flex-wrap items-center gap-3 sm:w-auto sm:flex-nowrap sm:justify-end", !actions && "hidden")}>{actions}</div>
      </div>
      <motion.div layoutId="page-divider" className="h-px bg-gradient-to-r from-border/60 via-border/30 to-transparent" transition={{ duration: 0.25 }} />
    </div>
  );
}
