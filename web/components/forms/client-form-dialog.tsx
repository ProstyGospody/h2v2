import { ChevronDown, Loader2 } from "lucide-react";
import { FormEvent, useEffect, useId, useMemo, useState } from "react";

import { buildClientConfigPreview, defaultsSummary, formFromClient, type ClientFormValues } from "@/domain/clients/adapters";
import { HysteriaClient, HysteriaClientDefaults } from "@/domain/clients/types";
import { Button, Dialog, Input, cn } from "@/src/components/ui";

export function ClientFormDialog({
  open, mode, busy, client, defaults, error, onClose, onSubmit,
}: {
  open: boolean;
  mode: "create" | "edit";
  busy: boolean;
  client: HysteriaClient | null;
  defaults: HysteriaClientDefaults | null;
  error?: string;
  onClose: () => void;
  onSubmit: (values: ClientFormValues) => Promise<void>;
}) {
  const [values, setValues] = useState<ClientFormValues>(formFromClient(client));
  const [previewOpen, setPreviewOpen] = useState(true);
  const formID = useId();

  useEffect(() => { if (!open) return; setValues(formFromClient(client)); setPreviewOpen(true); }, [client, mode, open]);

  const previewConfig = useMemo(
    () => (open ? buildClientConfigPreview(values, defaults, mode, client) : ""),
    [open, values, defaults, mode, client],
  );

  async function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); await onSubmit(values); }

  return (
    <Dialog
      open={open}
      onOpenChange={(n) => { if (!n && !busy) onClose(); }}
      title={mode === "create" ? "Create User" : "Edit User"}
      contentClassName="max-w-[660px]"
      hideClose={busy}
      footer={
        <>
          <Button type="button" onClick={onClose} disabled={busy}>Cancel</Button>
          <Button form={formID} type="submit" variant="primary" disabled={busy}>
            {busy ? <><Loader2 size={16} strokeWidth={1.8} className="animate-spin" />Saving...</> : "Save"}
          </Button>
        </>
      }
    >
      <form id={formID} className="space-y-5" onSubmit={submit}>
        <p className="rounded-xl border border-border/50 bg-surface-2/65 px-4 py-3 text-[13px] text-txt-secondary">
          Inherited: {defaultsSummary(defaults)}
        </p>

        {error && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}

        <Input label="Client ID" value={values.username} onChange={(e) => setValues((p) => ({ ...p, username: e.target.value }))} required />

        <div>
          <label className="mb-2 block text-[13px] font-medium text-txt-secondary">Note</label>
          <textarea value={values.note} onChange={(e) => setValues((p) => ({ ...p, note: e.target.value }))} rows={2}
            className="w-full rounded-lg border border-[var(--control-border)] bg-[var(--control-bg)] px-4 py-2.5 text-[14px] text-txt-primary outline-none transition-colors placeholder:text-txt-tertiary focus:border-accent-secondary/50 focus:bg-[var(--control-bg-hover)] focus:shadow-[0_0_0_3px_var(--accent-soft)]" />
        </div>

        <Input label="Auth Secret" value={values.authSecret} onChange={(e) => setValues((p) => ({ ...p, authSecret: e.target.value }))} />

        <div className="overflow-hidden rounded-xl border border-border/50 bg-surface-2/65">
          <button type="button" className="flex w-full items-center justify-between px-4 py-3 text-left text-[14px] font-semibold text-txt-primary transition-colors hover:bg-[var(--control-bg-hover)]"
            onClick={() => setPreviewOpen((p) => !p)}>
            <span>Advanced YAML</span>
            <ChevronDown size={16} strokeWidth={1.6} className={cn("text-txt-tertiary transition-transform duration-200", previewOpen && "rotate-180")} />
          </button>
          {previewOpen && (
            <div className="border-t border-border/40 bg-surface-1/30 p-4">
              <textarea readOnly value={previewConfig} rows={12}
                className="w-full rounded-lg border border-[var(--control-border)] bg-[var(--control-bg)] px-4 py-2.5 font-mono text-[13px] leading-6 text-txt-secondary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none" />
            </div>
          )}
        </div>
      </form>
    </Dialog>
  );
}
