import { Loader2 } from "lucide-react";
import { FormEvent, useEffect, useId, useState } from "react";

import { defaultsSummary, formFromClient, type ClientFormValues } from "@/domain/clients/adapters";
import { HysteriaClient, HysteriaClientDefaults } from "@/domain/clients/types";
import { Button, Dialog, Input, SelectField } from "@/src/components/ui";

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

  useEffect(() => {
    if (!open) return;
    setValues(formFromClient(client));
  }, [client, mode, open]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await onSubmit(values);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => { if (!next && !busy) onClose(); }}
      title={mode === "create" ? "Create User" : "Edit User"}
      contentClassName="max-w-[720px]"
      hideClose={busy}
      footer={
        <>
          <Button type="button" onClick={onClose} disabled={busy}>Cancel</Button>
          <Button form={formID} type="submit" variant="primary" disabled={busy}>
            {busy ? <><Loader2 size={16} strokeWidth={1.8} className="animate-spin" />Save</> : "Save"}
          </Button>
        </>
      }
    >
      <form id={formID} className="space-y-5" onSubmit={submit}>
        <p className="rounded-xl bg-surface-2/65 px-4 py-3 text-[13px] text-txt-secondary">
          {defaultsSummary(defaults)}
        </p>

        {error && <div className="rounded-xl bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}

        <Input
          label="User"
          value={values.username}
          onChange={(event) => setValues((prev) => ({ ...prev, username: event.target.value }))}
          required
        />

        <SelectField
          label="Protocol"
          value={values.protocol}
          options={[
            { value: "hy2", label: "HY2" },
            { value: "vless", label: "VLESS" },
          ]}
          onValueChange={(next) => setValues((prev) => ({ ...prev, protocol: next as ClientFormValues["protocol"] }))}
        />

        {values.protocol === "hy2" ? (
          <Input
            label="Secret"
            value={values.authSecret}
            onChange={(event) => setValues((prev) => ({ ...prev, authSecret: event.target.value }))}
          />
        ) : (
          <Input
            label="UUID"
            value={values.uuid}
            onChange={(event) => setValues((prev) => ({ ...prev, uuid: event.target.value }))}
          />
        )}

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="Limit"
            type="number"
            value={values.trafficLimitBytes}
            onChange={(event) => setValues((prev) => ({ ...prev, trafficLimitBytes: event.target.value }))}
          />
          <Input
            label="Expire"
            type="datetime-local"
            value={values.expireAt}
            onChange={(event) => setValues((prev) => ({ ...prev, expireAt: event.target.value }))}
          />
        </div>

        <div>
          <label className="mb-2 block text-[13px] font-medium text-txt-secondary">Note</label>
          <textarea
            value={values.note}
            onChange={(event) => setValues((prev) => ({ ...prev, note: event.target.value }))}
            rows={2}
            className="w-full rounded-lg bg-[var(--control-bg)] px-4 py-2.5 text-[14px] text-txt-primary outline-none transition-colors placeholder:text-txt-tertiary focus:bg-[var(--control-bg-hover)] focus:shadow-[0_0_0_3px_var(--accent-soft)]"
          />
        </div>
      </form>
    </Dialog>
  );
}
