import { useId } from "react";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./Select";

export function SelectField({
  label,
  value,
  onValueChange,
  options,
  triggerClassName,
}: {
  label: string;
  value: string;
  onValueChange: (v: string) => void;
  options: { value: string; label: string }[];
  triggerClassName?: string;
}) {
  const labelId = useId();

  return (
    <div>
      <label id={labelId} className="mb-2 block text-[13px] font-medium text-txt-secondary">{label}</label>
      <Select value={value} onValueChange={onValueChange}>
        <SelectTrigger aria-labelledby={labelId} className={triggerClassName}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
