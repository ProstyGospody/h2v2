import { ChevronDown, Loader2 } from "lucide-react";
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
      <form className="space-y-4" onSubmit={submit}>
        <p className="rounded-[8px] bg-accent/6 px-3 py-2 text-[11px] text-accent-light/80">Inherited: {defaultsSummary(defaults)}</p>

        {error ? (
          <div className="rounded-[10px] border border-status-danger/20 bg-status-danger/8 px-4 py-3 text-[12px] text-status-danger">{error}</div>
        ) : null}

        <Input
          label="Client ID"
          value={values.username}
          onChange={(event) => setValues((prev) => ({ ...prev, username: event.target.value }))}
          required
        />

        <div>
          <label className="mb-1.5 block text-[11px] font-medium text-txt-secondary">Note</label>
          <textarea
            value={values.note}
            onChange={(event) => setValues((prev) => ({ ...prev, note: event.target.value }))}
            rows={2}
            className="w-full rounded-[8px] border border-border bg-surface-1 px-3 py-2 text-[12px] text-txt outline-none transition-all placeholder:text-txt-muted focus:border-accent/40 focus:shadow-[0_0_0_3px_rgba(99,102,241,0.06)]"
          />
        </div>

        <Input
          label="Auth Secret (optional)"
          value={values.authSecret}
          onChange={(event) => setValues((prev) => ({ ...prev, authSecret: event.target.value }))}
          placeholder={mode === "create" ? "Leave empty to auto-generate" : "Leave empty to keep current secret"}
        />

        <div className="overflow-hidden rounded-[10px] border border-border/60 bg-surface-1/50">
          <button
            type="button"
            className="flex w-full items-center justify-between px-3.5 py-2.5 text-left text-[12px] font-medium text-txt transition-colors hover:bg-surface-3/30"
            onClick={() => setPreviewOpen((prev) => !prev)}
          >
            <span>Advanced YAML</span>
            <ChevronDown
              size={15}
              strokeWidth={1.4}
              className={cn("text-txt-tertiary transition-transform duration-200", previewOpen && "rotate-180")}
            />
          </button>
          {previewOpen ? (
            <div className="border-t border-border/60 p-3">
              <textarea
                readOnly
                value={previewConfig}
                rows={12}
                className="w-full rounded-[8px] border border-border/60 bg-surface-0/80 px-3 py-2 font-mono text-[11px] leading-5 text-accent-light/80 outline-none"
              />
              <p className="mt-1.5 text-[10px] text-txt-muted">Read-only preview</p>
            </div>
          ) : null}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-border/60 pt-4">
          <Button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? (
              <>
                <Loader2 size={14} strokeWidth={1.6} className="animate-spin" />
                Saving...
              </>
            ) : (
              "Save"
            )}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
