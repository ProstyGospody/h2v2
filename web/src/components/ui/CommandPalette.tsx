import * as DialogPrimitive from "@radix-ui/react-dialog";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { cn } from "./cn";

export type Command = {
  id: string;
  label: string;
  keywords?: string;
  icon?: ReactNode;
  shortcut?: string;
  group?: string;
  onSelect: () => void;
};

type CommandPaletteProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  commands: Command[];
  placeholder?: string;
};

export function CommandPalette({
  open,
  onOpenChange,
  commands,
  placeholder = "Search commands…",
}: CommandPaletteProps) {
  const reduce = useReducedMotion();
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((c) => {
      const hay = `${c.label} ${c.keywords ?? ""} ${c.group ?? ""}`.toLowerCase();
      return hay.includes(q);
    });
  }, [commands, query]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setActiveIndex(0);
    }
  }, [open]);

  useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  useEffect(() => {
    const item = listRef.current?.querySelector<HTMLDivElement>(`[data-idx="${activeIndex}"]`);
    item?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => Math.min(filtered.length - 1, i + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => Math.max(0, i - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const cmd = filtered[activeIndex];
      if (cmd) {
        onOpenChange(false);
        // defer so the dialog unmounts cleanly before the command runs
        queueMicrotask(() => cmd.onSelect());
      }
    }
  }

  // Group by group name (if any), preserving order
  const groups = useMemo(() => {
    const map = new Map<string, Command[]>();
    for (const cmd of filtered) {
      const g = cmd.group ?? "";
      if (!map.has(g)) map.set(g, []);
      map.get(g)!.push(cmd);
    }
    return Array.from(map.entries());
  }, [filtered]);

  // build index -> flat ordering
  let cursor = -1;

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <AnimatePresence>
        {open ? (
          <DialogPrimitive.Portal forceMount>
            <DialogPrimitive.Overlay asChild forceMount>
              <motion.div
                className="fixed inset-0 z-50 bg-[var(--dialog-overlay)] backdrop-blur-[3px]"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={reduce ? { duration: 0 } : { duration: 0.18 }}
              />
            </DialogPrimitive.Overlay>

            <div className="fixed inset-0 z-50 grid place-items-start justify-items-center p-4 pt-[12vh]">
              <DialogPrimitive.Content asChild forceMount>
                <motion.div
                  className="w-full max-w-[640px] overflow-hidden rounded-2xl border border-border/50 bg-[var(--dialog-surface)] shadow-[0_24px_56px_-16px_var(--dialog-shadow)] outline-none backdrop-blur-xl"
                  initial={reduce ? { opacity: 1 } : { opacity: 0, y: -12, scale: 0.98 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={reduce ? { opacity: 1 } : { opacity: 0, y: -8, scale: 0.98 }}
                  transition={reduce ? { duration: 0 } : { duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
                  onKeyDown={onKeyDown}
                >
                  <DialogPrimitive.Title className="sr-only">Command palette</DialogPrimitive.Title>
                  <div className="flex items-center gap-3 border-b border-border/40 px-4 py-3">
                    <Search size={16} className="shrink-0 text-txt-muted" />
                    <input
                      autoFocus
                      type="text"
                      value={query}
                      onChange={(e) => setQuery(e.target.value)}
                      placeholder={placeholder}
                      className="flex-1 bg-transparent text-[14px] font-medium text-txt-primary placeholder:text-txt-tertiary focus:outline-none"
                    />
                    <kbd className="rounded border border-border/50 bg-surface-2/60 px-1.5 py-0.5 text-[10px] font-semibold text-txt-muted">
                      ESC
                    </kbd>
                  </div>

                  <div ref={listRef} className="max-h-[60vh] overflow-y-auto p-2">
                    {filtered.length === 0 ? (
                      <div className="px-3 py-8 text-center text-[13px] text-txt-muted">
                        No commands match "{query}".
                      </div>
                    ) : (
                      groups.map(([groupName, items]) => (
                        <div key={groupName} className="mb-2 last:mb-0">
                          {groupName ? (
                            <div className="px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-txt-muted">
                              {groupName}
                            </div>
                          ) : null}
                          {items.map((cmd) => {
                            cursor += 1;
                            const idx = cursor;
                            const active = idx === activeIndex;
                            return (
                              <div
                                key={cmd.id}
                                data-idx={idx}
                                onMouseMove={() => setActiveIndex(idx)}
                                onClick={() => {
                                  onOpenChange(false);
                                  queueMicrotask(() => cmd.onSelect());
                                }}
                                className={cn(
                                  "flex cursor-pointer select-none items-center gap-3 rounded-lg px-3 py-2.5 text-[13px] font-medium",
                                  active ? "bg-surface-3/70 text-txt-primary" : "text-txt-secondary",
                                )}
                              >
                                {cmd.icon ? (
                                  <span className="grid h-4 w-4 place-items-center text-txt-muted">
                                    {cmd.icon}
                                  </span>
                                ) : null}
                                <span className="flex-1 truncate">{cmd.label}</span>
                                {cmd.shortcut ? (
                                  <kbd className="rounded border border-border/50 bg-surface-2/60 px-1.5 py-0.5 text-[10px] font-semibold text-txt-muted">
                                    {cmd.shortcut}
                                  </kbd>
                                ) : null}
                              </div>
                            );
                          })}
                        </div>
                      ))
                    )}
                  </div>
                </motion.div>
              </DialogPrimitive.Content>
            </div>
          </DialogPrimitive.Portal>
        ) : null}
      </AnimatePresence>
    </DialogPrimitive.Root>
  );
}
