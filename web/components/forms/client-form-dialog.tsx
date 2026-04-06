import { Loader2 } from "lucide-react";
import { type FormEvent, useEffect, useId, useMemo, useState } from "react";

import { expireToISO, formDefaults, formFromClient, trafficLimitToBytes } from "@/domain/clients/adapters";
import type { Client, ClientFormValues } from "@/domain/clients/types";
import { Button, Dialog, Input } from "@/src/components/ui";

export function ClientFormDialog({
  open,
  mode,
  busy,
  client,
  error,
  onClose,
  onSubmit,
}: {
  open: boolean;
  mode: "create" | "edit";
  busy: boolean;
  client: Client | null;
  error?: string;
  onClose: () => void;
  onSubmit: (values: ClientFormValues) => Promise<void>;
}) {
  const [values, setValues] = useState<ClientFormValues>(formDefaults());
  const formID = useId();

  const validationWarnings = useMemo(() => {
    const warnings: string[] = [];
    const username = values.username.trim();
    if (username.length > 64) warnings.push("Username must be 64 characters or fewer.");
    if (values.traffic_limit_gb !== "" && trafficLimitToBytes(values.traffic_limit_gb) === 0) {
      warnings.push("Traffic limit is too small and will be treated as unlimited. Enter at least 0.001 GB.");
    }
    if (values.expire_at !== "" && expireToISO(values.expire_at) === null) {
      warnings.push("Expiry date is invalid or in the past and will be ignored.");
    }
    return warnings;
  }, [values]);

  useEffect(() => {
    if (!open) return;
    setValues(client ? formFromClient(client) : formDefaults());
  }, [client, mode, open]);

  function set<K extends keyof ClientFormValues>(key: K, val: ClientFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: val }));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    await onSubmit(values);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => { if (!v && !busy) onClose(); }}
      title={mode === "create" ? "Create User" : "Edit User"}
      contentClassName="max-w-[520px]"
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
        {error && (
          <div className="rounded-xl bg-status-danger/8 px-5 py-3.5 text-[16px] text-status-danger">{error}</div>
        )}

        {validationWarnings.length > 0 && (
          <div className="space-y-1">
            {validationWarnings.map((w) => (
              <div key={w} className="rounded-xl bg-status-warning/8 px-5 py-2.5 text-[15px] text-status-warning">{w}</div>
            ))}
          </div>
        )}

        <Input
          label="Username"
          value={values.username}
          onChange={(e) => set("username", e.target.value)}
          required
          maxLength={64}
          autoFocus={mode === "create"}
        />

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="Traffic limit (GB)"
            type="number"
            min="0"
            step="0.1"
            placeholder="Unlimited"
            value={values.traffic_limit_gb}
            onChange={(e) => set("traffic_limit_gb", e.target.value)}
          />
          <Input
            label="Expires"
            type="datetime-local"
            value={values.expire_at}
            onChange={(e) => set("expire_at", e.target.value)}
          />
        </div>
      </form>
    </Dialog>
  );
}
