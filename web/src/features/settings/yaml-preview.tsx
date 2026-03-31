import { highlightYaml } from "./server-settings-utils";

export function YamlPreview({ value }: { value: string }) {
  return (
    <pre
      className="w-full max-h-[58vh] overflow-auto rounded-xl bg-[var(--control-bg)] px-4 py-3 font-mono text-[12px] leading-6 text-txt-secondary shadow-[inset_0_0_0_1px_var(--control-border)]"
      dangerouslySetInnerHTML={{ __html: highlightYaml(value) }}
    />
  );
}
