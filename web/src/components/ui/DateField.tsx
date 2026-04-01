import * as Popover from "@radix-ui/react-popover";
import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useId, useMemo, useState } from "react";

import { cn } from "./cn";

const WEEK_DAYS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];
const MONTH_LABEL = new Intl.DateTimeFormat("en-US", { month: "long", year: "numeric" });

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function parseISODate(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const [year, month, day] = value.split("-").map((item) => Number.parseInt(item, 10));
  if (!Number.isFinite(year) || !Number.isFinite(month) || !Number.isFinite(day)) return null;
  const date = new Date(year, month - 1, day);
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) return null;
  return startOfDay(date);
}

function toISODate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function isSameDate(a: Date | null, b: Date | null): boolean {
  if (!a || !b) return false;
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

type DateFieldProps = {
  label?: string;
  value: string;
  onValueChange: (next: string) => void;
  placeholder?: string;
  className?: string;
  min?: string;
  max?: string;
};

export function DateField({
  label,
  value,
  onValueChange,
  placeholder = "YYYY-MM-DD",
  className,
  min,
  max,
}: DateFieldProps) {
  const labelId = useId();
  const fieldId = useId();
  const selectedDate = useMemo(() => parseISODate(value), [value]);
  const minDate = useMemo(() => parseISODate(min || ""), [min]);
  const maxDate = useMemo(() => parseISODate(max || ""), [max]);
  const [open, setOpen] = useState(false);
  const [viewDate, setViewDate] = useState<Date>(selectedDate || startOfDay(new Date()));
  const today = startOfDay(new Date());

  useEffect(() => {
    if (!open) return;
    setViewDate(selectedDate || startOfDay(new Date()));
  }, [open, selectedDate]);

  const days = useMemo(() => {
    const year = viewDate.getFullYear();
    const month = viewDate.getMonth();
    const firstDay = new Date(year, month, 1).getDay();
    return Array.from({ length: 42 }, (_, index) => new Date(year, month, index - firstDay + 1));
  }, [viewDate]);

  function isOutOfRange(date: Date): boolean {
    const ts = startOfDay(date).getTime();
    if (minDate && ts < minDate.getTime()) return true;
    if (maxDate && ts > maxDate.getTime()) return true;
    return false;
  }

  function selectDate(date: Date) {
    if (isOutOfRange(date)) return;
    onValueChange(toISODate(date));
    setOpen(false);
  }

  return (
    <div>
      {label ? (
        <label id={labelId} htmlFor={fieldId} className="mb-2 block text-[13px] font-medium text-txt-secondary">
          {label}
        </label>
      ) : null}
      <Popover.Root open={open} onOpenChange={setOpen}>
        <Popover.Trigger asChild>
          <button
            id={fieldId}
            type="button"
            aria-labelledby={label ? labelId : undefined}
            className={cn(
              "flex w-full items-center justify-between rounded-lg border-0 bg-[var(--control-bg)] px-4 py-2.5 text-left text-[14px] font-medium shadow-[inset_0_0_0_1px_var(--control-border)] transition-colors",
              "text-txt-primary hover:bg-[var(--control-bg-hover)] focus-visible:outline-none focus-visible:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]",
              className,
            )}
          >
            <span className={value ? "text-txt-primary" : "text-txt-tertiary"}>{value || placeholder}</span>
            <CalendarDays size={15} strokeWidth={1.8} className="text-txt-tertiary" />
          </button>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            sideOffset={8}
            align="start"
            className="z-50 w-[272px] rounded-xl bg-surface-2/95 p-3 shadow-[0_18px_42px_-16px_var(--dialog-shadow)] backdrop-blur-xl"
          >
            <div className="mb-2 flex items-center justify-between">
              <button
                type="button"
                onClick={() => setViewDate((prev) => new Date(prev.getFullYear(), prev.getMonth() - 1, 1))}
                className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/60 hover:text-txt-primary"
                aria-label="Previous month"
              >
                <ChevronLeft size={15} strokeWidth={1.9} />
              </button>
              <p className="text-[13px] font-semibold text-txt-primary">{MONTH_LABEL.format(viewDate)}</p>
              <button
                type="button"
                onClick={() => setViewDate((prev) => new Date(prev.getFullYear(), prev.getMonth() + 1, 1))}
                className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/60 hover:text-txt-primary"
                aria-label="Next month"
              >
                <ChevronRight size={15} strokeWidth={1.9} />
              </button>
            </div>

            <div className="grid grid-cols-7 gap-1">
              {WEEK_DAYS.map((day) => (
                <span key={day} className="grid h-7 place-items-center text-[11px] font-semibold text-txt-muted">{day}</span>
              ))}
              {days.map((day) => {
                const inCurrentMonth = day.getMonth() === viewDate.getMonth();
                const dayValue = startOfDay(day);
                const isSelected = isSameDate(dayValue, selectedDate);
                const isToday = isSameDate(dayValue, today);
                const disabled = isOutOfRange(dayValue);

                return (
                  <button
                    key={toISODate(dayValue)}
                    type="button"
                    disabled={disabled}
                    onClick={() => selectDate(dayValue)}
                    className={cn(
                      "grid h-8 w-8 place-items-center rounded-lg text-[12px] font-medium transition-colors",
                      !inCurrentMonth && "text-txt-muted/60",
                      inCurrentMonth && "text-txt-secondary",
                      isToday && !isSelected && "shadow-[inset_0_0_0_1px_var(--border-focus)]",
                      isSelected && "bg-accent text-white",
                      !isSelected && "hover:bg-surface-3/60 hover:text-txt-primary",
                      disabled && "pointer-events-none opacity-35",
                    )}
                  >
                    {day.getDate()}
                  </button>
                );
              })}
            </div>

            <div className="mt-2 flex items-center justify-between border-t border-border/40 pt-2">
              <button
                type="button"
                onClick={() => {
                  onValueChange("");
                  setOpen(false);
                }}
                disabled={!value}
                className="rounded-lg px-2 py-1 text-[12px] font-medium text-txt-secondary transition-colors hover:bg-surface-3/60 hover:text-txt-primary disabled:pointer-events-none disabled:opacity-45"
              >
                Clear
              </button>
              <button
                type="button"
                onClick={() => selectDate(today)}
                className="rounded-lg px-2 py-1 text-[12px] font-medium text-accent transition-colors hover:bg-accent/10"
              >
                Today
              </button>
            </div>
            <Popover.Arrow className="fill-surface-2/95" />
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
    </div>
  );
}

export type { DateFieldProps };
