import type { ReactNode } from "react";

import { cn } from "./cn";

export function Kbd({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <kbd
      className={cn(
        "inline-flex h-5 min-w-[20px] items-center justify-center rounded-[5px] border border-border/50 bg-surface-3/40 px-1.5 font-mono text-[11px] font-medium leading-none text-txt-secondary",
        className,
      )}
    >
      {children}
    </kbd>
  );
}
