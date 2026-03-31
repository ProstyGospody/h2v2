import { useId } from "react";

import { Toggle } from "./Toggle";

export function ToggleField({
  label,
  checked,
  onCheckedChange,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: (next: boolean) => void;
}) {
  const labelId = useId();
  const toggleId = useId();

  return (
    <div className="flex items-center justify-between rounded-xl bg-[var(--control-bg)] px-4 py-3 shadow-[inset_0_0_0_1px_var(--control-border)]">
      <span id={labelId} className="text-[14px] font-medium text-txt-primary">{label}</span>
      <Toggle id={toggleId} checked={checked} onCheckedChange={onCheckedChange} aria-labelledby={labelId} />
    </div>
  );
}
