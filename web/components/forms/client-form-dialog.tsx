import { Loader2 } from "lucide-react";
import { FormEvent, useEffect, useId, useState } from "react";

import { defaultsSummary, formFromClient, type ClientFormValues } from "@/domain/clients/adapters";
import { HysteriaClient, HysteriaClientDefaults } from "@/domain/clients/types";
import { Button, Dialog, Input } from "@/src/components/ui";

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
  const formID = useId();

  useEffect(() => { if (!open) return; setValues(formFromClient(client)); }, [client, mode, open]);

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
      </form>
    </Dialog>
  );
}
