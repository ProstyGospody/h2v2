import { ChevronDown } from "lucide-react";
import { FormEvent, useEffect, useMemo, useState } from "react";

import { buildClientConfigPreview, defaultsSummary, formFromClient, type ClientFormValues } from "@/domain/clients/adapters";
import { HysteriaClient, HysteriaClientDefaults } from "@/domain/clients/types";
import { Button, Dialog, Input, cn } from "@/src/components/ui";

export function ClientFormDialog({
  open,
  mode,
  busy,
  client,
  defaults,
  error,
  onClose,
  onSubmit,
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

  useEffect(() => {
    if (!open) {
      return;
    }
    setValues(formFromClient(client));
    setPreviewOpen(true);
  }, [client, mode, open]);

  const previewConfig = useMemo(() => {
    return buildClientConfigPreview(values, defaults, mode, client);
  }, [values, defaults, mode, client]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await onSubmit(values);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !busy) {
          onClose();
        }
      }}
      title={mode === "create" ? "Create User" : "Edit User"}
      contentClassName="max-w-[640px]"
      hideClose={busy}
    >
      <form className="space-y-3" onSubmit={submit}>
        <p className="text-[12px] text-txt-secondary">Inherited: {defaultsSummary(defaults)}</p>

        {error ? <div className="rounded-btn border border-status-danger/20 bg-status-danger/10 px-3 py-2 text-[12px] text-status-danger">{error}</div> : null}

        <Input
          label="Client ID"
          value={values.username}
          onChange={(event) => setValues((prev) => ({ ...prev, username: event.target.value }))}
          required
        />

        <div>
          <label className="mb-1.5 block text-[11px] font-medium text-txt-muted">Note</label>
          <textarea
            value={values.note}
            onChange={(event) => setValues((prev) => ({ ...prev, note: event.target.value }))}
            rows={2}
            className="w-full rounded-btn border border-border bg-surface-1 px-3 py-2 text-[12px] text-txt outline-none transition-colors placeholder:text-txt-muted focus:border-accent/50"
          />
        </div>

        <Input
          label="Auth Secret (optional)"
          value={values.authSecret}
          onChange={(event) => setValues((prev) => ({ ...prev, authSecret: event.target.value }))}
          placeholder={mode === "create" ? "Leave empty to auto-generate" : "Leave empty to keep current secret"}
        />

        <div className="rounded-card border border-border bg-surface-1">
          <button
            type="button"
            className="flex w-full items-center justify-between px-3 py-2 text-left text-[12px] font-medium text-txt"
            onClick={() => setPreviewOpen((prev) => !prev)}
          >
            <span>Advanced YAML</span>
            <ChevronDown size={16} strokeWidth={1.4} className={cn("transition-transform", previewOpen && "rotate-180")} />
          </button>
          {previewOpen ? (
            <div className="border-t border-border p-3">
              <textarea
                readOnly
                value={previewConfig}
                rows={12}
                className="w-full rounded-btn border border-border bg-surface-0 px-3 py-2 font-mono text-[11px] text-accent-light outline-none"
              />
              <p className="mt-1 text-[11px] text-txt-muted">Generated preview</p>
            </div>
          ) : null}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-border pt-4">
          <Button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? "Saving..." : "Save"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
