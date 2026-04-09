import { useQueries } from "@tanstack/react-query";
import { Copy, Loader2, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { getPolicyUsage } from "@/domain/policy/services";
import { getAPIErrorMessage } from "@/services/api";
import type { PolicyUsage } from "@/types/common";
import {
  Badge,
  Button,
  Dialog,
  Input,
  SelectField,
  Toggle,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";

export type PolicyEntity = Record<string, unknown> & { id?: string; server_id?: string };

export type PolicyField = {
  key: string;
  kind: "csv" | "json" | "number" | "select" | "text" | "textarea" | "toggle";
  label: string;
  options?: { label: string; value: string }[];
  placeholder?: string;
  rows?: number;
  format?: (value: unknown) => string;
  parse?: (value: string) => unknown;
};

export type PolicyDescriptor = {
  clone: (item: PolicyEntity) => PolicyEntity;
  createEmpty: (serverID: string) => PolicyEntity;
  describe?: (item: PolicyEntity) => string;
  fields: PolicyField[];
  items: PolicyEntity[];
  kind: string;
  label: string;
  loading: boolean;
  noun: string;
  onChanged: () => Promise<void> | void;
  remove: (id: string) => Promise<void>;
  save: (body: PolicyEntity, id?: string) => Promise<unknown>;
  serverID: string;
  title: (item: PolicyEntity) => string;
};

function formatFieldValue(field: PolicyField, value: unknown) {
  if (field.format) return field.format(value);
  if (field.kind === "csv") {
    return Array.isArray(value) ? value.join(", ") : "";
  }
  if (field.kind === "number") {
    return value == null ? "" : String(value);
  }
  if (field.kind === "toggle") {
    return Boolean(value);
  }
  return typeof value === "string" ? value : "";
}

function parseFieldValue(field: PolicyField, value: string) {
  if (field.parse) return field.parse(value);
  if (field.kind === "csv") {
    return value
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }
  if (field.kind === "number") {
    if (!value.trim()) return null;
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return value;
}

function usageBadges(usage?: PolicyUsage) {
  if (!usage) return null;
  return (
    <div className="mt-3 flex flex-wrap gap-2">
      <Badge variant="default">Users {usage.used_by_users}</Badge>
      <Badge variant="default">Access {usage.used_by_access}</Badge>
      <Badge variant="default">Inbounds {usage.used_by_inbounds}</Badge>
      {usage.used_by_route_rules > 0 ? (
        <Badge variant="default">Route rules {usage.used_by_route_rules}</Badge>
      ) : null}
      {usage.used_by_outbounds > 0 ? (
        <Badge variant="default">Outbounds {usage.used_by_outbounds}</Badge>
      ) : null}
      {usage.affected_subscriptions > 0 ? (
        <Badge variant="warning">Subscriptions {usage.affected_subscriptions}</Badge>
      ) : null}
      {usage.affected_artifacts > 0 ? <Badge variant="warning">Artifacts {usage.affected_artifacts}</Badge> : null}
      {usage.requires_runtime_apply ? <Badge variant="warning">Runtime</Badge> : null}
    </div>
  );
}

export function PolicyManager({ descriptor }: { descriptor: PolicyDescriptor }) {
  const toast = useToast();
  const [editor, setEditor] = useState<{ item: PolicyEntity; mode: "clone" | "create" | "edit" } | null>(null);
  const [form, setForm] = useState<PolicyEntity | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [deleteItem, setDeleteItem] = useState<PolicyEntity | null>(null);

  const usageQueries = useQueries({
    queries: descriptor.items.map((item) => ({
      enabled: typeof item.id === "string" && item.id.length > 0,
      queryFn: () => getPolicyUsage(descriptor.kind, item.id as string),
      queryKey: ["policy-usage", descriptor.kind, item.id],
      staleTime: 10_000,
    })),
  });

  const usageByID = useMemo(() => {
    const result = new Map<string, PolicyUsage>();
    descriptor.items.forEach((item, index) => {
      if (typeof item.id !== "string" || !item.id) return;
      const usage = usageQueries[index]?.data;
      if (usage) result.set(item.id, usage);
    });
    return result;
  }, [descriptor.items, usageQueries]);

  useEffect(() => {
    if (!editor) return;
    setForm(editor.item);
    setError("");
    setBusy(false);
  }, [editor]);

  const currentUsage = form && typeof form.id === "string" ? usageByID.get(form.id) : undefined;
  const deleteUsage =
    deleteItem && typeof deleteItem.id === "string" ? usageByID.get(deleteItem.id) : undefined;
  const deleteUsageLoading =
    !!deleteItem && typeof deleteItem.id === "string" && !usageByID.has(deleteItem.id);

  async function save() {
    if (!form) return;
    setBusy(true);
    setError("");
    try {
      const payload: PolicyEntity = { ...form, server_id: descriptor.serverID };
      if (!payload.id) delete payload.id;
      delete payload.created_at;
      delete payload.updated_at;
      await descriptor.save(payload, typeof form.id === "string" ? form.id : undefined);
      await descriptor.onChanged();
      setEditor(null);
      toast.notify("Saved");
    } catch (err) {
      setError(getAPIErrorMessage(err, "Save failed"));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!deleteItem || typeof deleteItem.id !== "string") return;
    if (deleteUsage?.unsafe_delete) return;
    setBusy(true);
    setError("");
    try {
      await descriptor.remove(deleteItem.id);
      await descriptor.onChanged();
      setDeleteItem(null);
      toast.notify("Deleted");
    } catch (err) {
      setError(getAPIErrorMessage(err, "Delete failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 className="text-[17px] font-semibold text-txt-primary">{descriptor.label}</h3>
          <p className="mt-1 text-[15px] text-txt-secondary">{descriptor.items.length}</p>
        </div>
        <Button
          variant="primary"
          onClick={() => setEditor({ item: descriptor.createEmpty(descriptor.serverID), mode: "create" })}
        >
          <Plus size={14} /> New
        </Button>
      </div>

      {descriptor.loading ? (
        <div className="flex items-center gap-2 rounded-2xl bg-surface-1/50 px-4 py-8 text-[15px] text-txt-secondary">
          <Loader2 size={16} className="animate-spin" /> Loading
        </div>
      ) : descriptor.items.length === 0 ? (
        <div className="rounded-2xl bg-surface-1/50 px-4 py-10 text-center text-[15px] text-txt-muted shadow-[inset_0_0_0_1px_var(--control-border)]">
          No {descriptor.noun}
        </div>
      ) : (
        <div className="grid gap-3">
          {descriptor.items.map((item) => {
            const itemID = typeof item.id === "string" ? item.id : "";
            const usage = itemID ? usageByID.get(itemID) : undefined;
            return (
              <div
                key={itemID || descriptor.title(item)}
                className="rounded-2xl bg-surface-1/50 px-4 py-4 shadow-[inset_0_0_0_1px_var(--control-border)]"
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="text-[16px] font-semibold text-txt-primary">{descriptor.title(item)}</div>
                    {descriptor.describe ? (
                      <div className="mt-1 text-[14px] text-txt-secondary">{descriptor.describe(item)}</div>
                    ) : null}
                    {usageBadges(usage)}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button size="sm" onClick={() => setEditor({ item, mode: "edit" })}>
                      Edit
                    </Button>
                    <Button size="sm" onClick={() => setEditor({ item: descriptor.clone(item), mode: "clone" })}>
                      <Copy size={13} /> Clone
                    </Button>
                    <Button size="sm" variant="danger" onClick={() => setDeleteItem(item)}>
                      <Trash2 size={13} /> Delete
                    </Button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      <Dialog
        open={!!editor}
        onOpenChange={(value) => {
          if (!value && !busy) setEditor(null);
        }}
        title={editor?.mode === "create" ? `New ${descriptor.noun}` : editor?.mode === "clone" ? `Clone ${descriptor.noun}` : `Edit ${descriptor.noun}`}
        contentClassName="max-w-[760px]"
        footer={
          <>
            <Button onClick={() => setEditor(null)} disabled={busy}>
              Cancel
            </Button>
            <Button variant="primary" onClick={() => void save()} disabled={busy}>
              {busy ? <Loader2 size={14} className="animate-spin" /> : "Save"}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {error ? (
            <div className="rounded-2xl bg-status-danger/10 px-4 py-3 text-[15px] text-status-danger">
              {error}
            </div>
          ) : null}
          {currentUsage ? (
            <div className="rounded-2xl bg-surface-1/50 px-4 py-4 shadow-[inset_0_0_0_1px_var(--control-border)]">
              <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Impact</div>
              {usageBadges(currentUsage)}
            </div>
          ) : null}
          <div className="grid gap-4 md:grid-cols-2">
            {descriptor.fields.map((field) => {
              const raw = form?.[field.key];
              if (field.kind === "toggle") {
                return (
                  <label key={field.key} className="flex items-center justify-between gap-4 rounded-xl bg-surface-1/40 px-4 py-3 shadow-[inset_0_0_0_1px_var(--control-border)] md:col-span-2">
                    <span className="text-[15px] font-medium text-txt-primary">{field.label}</span>
                    <Toggle
                      checked={Boolean(raw)}
                      onCheckedChange={(value) =>
                        setForm((prev) => ({ ...(prev ?? {}), [field.key]: Boolean(value) }))
                      }
                    />
                  </label>
                );
              }
              if (field.kind === "select") {
                return (
                  <SelectField
                    key={field.key}
                    label={field.label}
                    value={String(raw ?? "")}
                    onValueChange={(value) => setForm((prev) => ({ ...(prev ?? {}), [field.key]: value }))}
                    options={field.options ?? []}
                  />
                );
              }
              const value = formatFieldValue(field, raw);
              if (field.kind === "textarea" || field.kind === "json") {
                return (
                  <div key={field.key} className="md:col-span-2">
                    <label className="mb-2 block text-[15px] font-medium text-txt-secondary">{field.label}</label>
                    <textarea
                      rows={field.rows ?? 6}
                      value={String(value)}
                      placeholder={field.placeholder}
                      onChange={(event) =>
                        setForm((prev) => ({
                          ...(prev ?? {}),
                          [field.key]: parseFieldValue(field, event.target.value),
                        }))
                      }
                      className="w-full rounded-xl bg-[var(--control-bg)] px-4 py-3 text-[15px] text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors placeholder:text-txt-tertiary focus:bg-[var(--control-bg-hover)] focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]"
                    />
                  </div>
                );
              }
              return (
                <Input
                  key={field.key}
                  label={field.label}
                  type={field.kind === "number" ? "number" : "text"}
                  value={String(value)}
                  placeholder={field.placeholder}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...(prev ?? {}),
                      [field.key]: parseFieldValue(field, event.target.value),
                    }))
                  }
                />
              );
            })}
          </div>
        </div>
      </Dialog>

      <Dialog
        open={!!deleteItem}
        onOpenChange={(value) => {
          if (!value && !busy) setDeleteItem(null);
        }}
        title={`Delete ${descriptor.noun}`}
        footer={
          <>
            <Button onClick={() => setDeleteItem(null)} disabled={busy}>
              Close
            </Button>
            <Button
              variant="danger"
              onClick={() => void remove()}
              disabled={busy || deleteUsageLoading || !!deleteUsage?.unsafe_delete}
            >
              {busy ? <Loader2 size={14} className="animate-spin" /> : "Delete"}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {deleteItem ? (
            <div className="rounded-2xl bg-surface-1/50 px-4 py-4 shadow-[inset_0_0_0_1px_var(--control-border)]">
              <div className="text-[16px] font-semibold text-txt-primary">{descriptor.title(deleteItem)}</div>
              {descriptor.describe ? (
                <div className="mt-1 text-[14px] text-txt-secondary">{descriptor.describe(deleteItem)}</div>
              ) : null}
              {usageBadges(deleteUsage)}
            </div>
          ) : null}
          {deleteUsage?.unsafe_delete ? (
            <div className="rounded-2xl bg-status-warning/10 px-4 py-3 text-[15px] text-status-warning">
              In use
            </div>
          ) : null}
          {deleteUsageLoading ? (
            <div className="rounded-2xl bg-surface-1/50 px-4 py-3 text-[15px] text-txt-secondary shadow-[inset_0_0_0_1px_var(--control-border)]">
              Loading impact
            </div>
          ) : null}
        </div>
      </Dialog>
    </div>
  );
}
