import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  MoreHorizontal,
  Pencil,
  Plus,
  Power,
  PowerOff,
  QrCode,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ClientArtifactsDialog } from "@/components/dialogs/client-artifacts-dialog";
import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { ClientFormDialog } from "@/components/forms/client-form-dialog";
import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { expireToISO, trafficLimitToBytes } from "@/domain/clients/adapters";
import {
  createClient,
  deleteClient,
  deleteClientsBulk,
  getClientArtifacts,
  listClients,
  setClientEnabled,
  setClientsEnabledBulk,
  updateClient,
} from "@/domain/clients/services";
import type { Client, ClientArtifacts, ClientFormValues } from "@/domain/clients/types";
import { getAPIErrorMessage } from "@/services/api";
import {
  Badge,
  Button,
  Checkbox,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Toggle,
  cn,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { queryRefetchInterval } from "@/src/queries/polling";
import { formatBytes, formatDateTime } from "@/utils/format";

const PAGE_SIZE = 30;
const EXPIRE_SOON_DAYS = 7;

type StatusFilter = "all" | "active" | "disabled";

const totalTraffic = (c: Client) => c.traffic_used_up_bytes + c.traffic_used_down_bytes;
const trafficPercent = (c: Client) =>
  c.traffic_limit_bytes <= 0 ? 0 : Math.min(100, (totalTraffic(c) / c.traffic_limit_bytes) * 100);

type ExpireState = "expired" | "soon" | "ok" | "none";
function expireState(expireAt: string | null): ExpireState {
  if (!expireAt) return "none";
  const diff = new Date(expireAt).getTime() - Date.now();
  if (diff <= 0) return "expired";
  if (diff < EXPIRE_SOON_DAYS * 86_400_000) return "soon";
  return "ok";
}

// ---------------------------------------------------------------------------

function StatusDot({ active }: { active: boolean }) {
  return (
    <span
      className={cn(
        "inline-block h-1.5 w-1.5 rounded-full",
        active ? "bg-status-success" : "bg-txt-muted/60",
      )}
    />
  );
}

function TrafficBar({ client }: { client: Client }) {
  const used = totalTraffic(client);
  const pct = trafficPercent(client);
  const limited = client.traffic_limit_bytes > 0;
  const danger = pct >= 90;
  const warn = pct >= 70;

  if (!limited) {
    return <span className="text-[13px] text-txt-secondary tabular-nums">{formatBytes(used)}</span>;
  }
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <div className="flex items-baseline gap-1 text-[12px] tabular-nums">
        <span
          className={cn(
            "font-medium",
            danger ? "text-status-danger" : warn ? "text-status-warning" : "text-txt-primary",
          )}
        >
          {formatBytes(used)}
        </span>
        <span className="text-txt-muted">/ {formatBytes(client.traffic_limit_bytes)}</span>
      </div>
      <div className="h-1 w-full overflow-hidden rounded-full bg-surface-3/60">
        <div
          className={cn(
            "h-full rounded-full transition-all duration-500",
            danger ? "bg-status-danger" : warn ? "bg-status-warning" : "bg-accent",
          )}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function ExpireLabel({ expireAt }: { expireAt: string | null }) {
  const state = expireState(expireAt);
  if (state === "none") return <span className="text-[13px] text-txt-muted">—</span>;
  const text = formatDateTime(expireAt, { includeSeconds: false });
  return (
    <span
      className={cn(
        "text-[13px] tabular-nums",
        state === "expired" && "text-status-danger",
        state === "soon" && "text-status-warning",
        state === "ok" && "text-txt-secondary",
      )}
    >
      {text}
    </span>
  );
}

function StatusSegment({
  value,
  counts,
  onChange,
}: {
  value: StatusFilter;
  counts: Record<StatusFilter, number>;
  onChange: (v: StatusFilter) => void;
}) {
  const opts: { key: StatusFilter; label: string }[] = [
    { key: "all", label: "All" },
    { key: "active", label: "Active" },
    { key: "disabled", label: "Disabled" },
  ];
  return (
    <div role="radiogroup" className="flex items-center gap-0.5">
      {opts.map((o) => (
        <button
          key={o.key}
          role="radio"
          aria-checked={value === o.key}
          type="button"
          onClick={() => onChange(o.key)}
          className={cn(
            "rounded-md px-2.5 py-1.5 text-[12px] font-medium transition-colors",
            value === o.key
              ? "bg-surface-3/70 text-txt-primary"
              : "text-txt-secondary hover:text-txt-primary",
          )}
        >
          {o.label}
          <span className="ml-1.5 text-txt-muted tabular-nums">{counts[o.key]}</span>
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------

export default function UsersPage() {
  const qc = useQueryClient();
  const toast = useToast();

  const usersQuery = useQuery({
    queryKey: ["clients"],
    queryFn: listClients,
    refetchInterval: (q) => queryRefetchInterval(10_000, q),
  });
  const clients = usersQuery.data ?? [];

  const counts = useMemo<Record<StatusFilter, number>>(
    () => ({
      all: clients.length,
      active: clients.filter((c) => c.enabled).length,
      disabled: clients.filter((c) => !c.enabled).length,
    }),
    [clients],
  );

  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const searchRef = useRef<HTMLInputElement>(null);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return clients
      .filter((c) => {
        if (status === "active" && !c.enabled) return false;
        if (status === "disabled" && c.enabled) return false;
        if (q && !c.username.toLowerCase().includes(q)) return false;
        return true;
      })
      .sort((a, b) => a.username.localeCompare(b.username));
  }, [clients, search, status]);

  useEffect(() => {
    setPage(0);
  }, [search, status]);

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const visible = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  const allOnPageSelected =
    visible.length > 0 && visible.every((c) => selected.has(c.id));
  const someOnPageSelected =
    visible.some((c) => selected.has(c.id)) && !allOnPageSelected;

  function toggleAllOnPage(v: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      for (const c of visible) {
        if (v) next.add(c.id);
        else next.delete(c.id);
      }
      return next;
    });
  }
  function toggleOne(id: string, v: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (v) next.add(id);
      else next.delete(id);
      return next;
    });
  }
  function clearSelection() {
    setSelected(new Set());
  }

  const handleToggle = useCallback(
    async (client: Client) => {
      try {
        await setClientEnabled(client.id, !client.enabled);
        await qc.invalidateQueries({ queryKey: ["clients"] });
      } catch (err) {
        toast.notify(getAPIErrorMessage(err, "Toggle failed"), "error");
      }
    },
    [qc, toast],
  );

  const handleDelete = useCallback(
    async (client: Client) => {
      try {
        await deleteClient(client.id);
        toast.notify(`Deleted ${client.username}`);
        await qc.invalidateQueries({ queryKey: ["clients"] });
      } catch (err) {
        toast.notify(getAPIErrorMessage(err, "Delete failed"), "error");
      }
    },
    [qc, toast],
  );

  // Form dialog
  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState("");
  const [editingClient, setEditingClient] = useState<Client | null>(null);

  const openCreate = useCallback(() => {
    setEditingClient(null);
    setFormMode("create");
    setFormError("");
    setFormOpen(true);
  }, []);
  const openEdit = useCallback((client: Client) => {
    setEditingClient(client);
    setFormMode("edit");
    setFormError("");
    setFormOpen(true);
  }, []);
  function closeForm() {
    if (formBusy) return;
    setFormOpen(false);
    setEditingClient(null);
  }

  async function submitForm(values: ClientFormValues) {
    setFormBusy(true);
    setFormError("");
    try {
      const payload = {
        username: values.username.trim(),
        traffic_limit_bytes: trafficLimitToBytes(values.traffic_limit_gb),
        expire_at: expireToISO(values.expire_at),
      };
      if (formMode === "create") {
        await createClient(payload);
        toast.notify(`Created ${payload.username}`);
      } else if (editingClient) {
        await updateClient(editingClient.id, payload);
        toast.notify(`Updated ${payload.username}`);
      }
      await qc.invalidateQueries({ queryKey: ["clients"] });
      closeForm();
    } catch (err) {
      setFormError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setFormBusy(false);
    }
  }

  // Artifacts drawer
  const [artifactsOpen, setArtifactsOpen] = useState(false);
  const [artifactsClient, setArtifactsClient] = useState<Client | null>(null);
  const [artifactsData, setArtifactsData] = useState<ClientArtifacts | null>(null);
  const [artifactsLoading, setArtifactsLoading] = useState(false);

  const openArtifacts = useCallback(
    async (client: Client) => {
      setArtifactsClient(client);
      setArtifactsData(null);
      setArtifactsLoading(true);
      setArtifactsOpen(true);
      try {
        const data = await getClientArtifacts(client.id);
        setArtifactsData(data);
      } catch (err) {
        toast.notify(getAPIErrorMessage(err, "Failed to load"), "error");
      } finally {
        setArtifactsLoading(false);
      }
    },
    [toast],
  );

  // Bulk
  const selectedIds = useMemo(() => Array.from(selected), [selected]);
  async function bulkEnable(enabled: boolean) {
    if (selectedIds.length === 0) return;
    try {
      const n = await setClientsEnabledBulk(selectedIds, enabled);
      toast.notify(`${n} ${enabled ? "enabled" : "disabled"}`);
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Bulk failed"), "error");
    }
  }
  async function bulkDelete() {
    if (selectedIds.length === 0) return;
    try {
      const n = await deleteClientsBulk(selectedIds);
      toast.notify(`${n} deleted`);
      clearSelection();
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Bulk delete failed"), "error");
    }
  }

  // Keyboard: Esc clears selection/search
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      if (formOpen || artifactsOpen) return;
      if (selected.size > 0) {
        clearSelection();
        return;
      }
      if (search) setSearch("");
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [artifactsOpen, formOpen, search, selected.size]);

  const isLoading = usersQuery.isLoading;
  const isError = usersQuery.isError;

  return (
    <div className="space-y-5 pb-24">
      <PageHeader
        title="Users"
        actions={
          <Button variant="primary" onClick={openCreate}>
            <Plus size={16} strokeWidth={2} /> New user
          </Button>
        }
      />

      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-[240px] flex-1 sm:max-w-[360px]">
          <Search
            size={14}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-txt-muted"
          />
          <input
            ref={searchRef}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search"
            className="w-full rounded-lg bg-surface-2/50 py-2 pl-9 pr-8 text-[13px] font-medium text-txt-primary outline-none transition-colors placeholder:text-txt-tertiary focus:bg-surface-2/80"
          />
          {search ? (
            <button
              type="button"
              onClick={() => setSearch("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-txt-muted hover:text-txt-primary"
              aria-label="Clear"
            >
              <X size={13} />
            </button>
          ) : null}
        </div>
        <StatusSegment value={status} counts={counts} onChange={setStatus} />
      </div>

      {isError ? (
        <ErrorBanner
          message={getAPIErrorMessage(usersQuery.error, "Failed to load")}
          actionLabel="Retry"
          onAction={() => usersQuery.refetch()}
        />
      ) : null}

      {/* List */}
      <div className="overflow-hidden rounded-2xl bg-surface-2/20">
        <div className="overflow-x-auto">
          <div className="min-w-[760px]">
            {/* Header */}
            <div className="flex items-center gap-4 px-5 py-3 text-[11px] font-semibold uppercase tracking-wider text-txt-muted">
              <div className="w-5">
                <Checkbox
                  checked={allOnPageSelected ? true : someOnPageSelected ? "indeterminate" : false}
                  onCheckedChange={(v) => toggleAllOnPage(!!v)}
                  aria-label="Select page"
                />
              </div>
              <div className="flex-1">User</div>
              <div className="w-[90px]">Status</div>
              <div className="w-[160px]">Traffic</div>
              <div className="w-[140px]">Expires</div>
              <div className="w-8" />
            </div>

            {/* Body */}
            {isLoading ? (
              <div className="space-y-px">
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-4 px-5 py-4">
                    <div className="h-4 w-4 rounded bg-surface-3/60" />
                    <div className="h-3.5 flex-1 max-w-[160px] animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-16 animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-24 animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-20 animate-pulse rounded bg-surface-3/60" />
                    <div className="w-8" />
                  </div>
                ))}
              </div>
            ) : visible.length === 0 ? (
              <div className="flex flex-col items-center gap-3 px-5 py-16 text-center text-txt-muted">
                <p className="text-[14px]">
                  {search || status !== "all" ? "Nothing matches." : "No users yet."}
                </p>
                {!search && status === "all" ? (
                  <Button variant="primary" size="sm" onClick={openCreate}>
                    <Plus size={14} /> Create user
                  </Button>
                ) : null}
              </div>
            ) : (
              <div>
                {visible.map((c) => {
                  const isSelected = selected.has(c.id);
                  return (
                    <div
                      key={c.id}
                      className={cn(
                        "group flex items-center gap-4 px-5 py-3.5 transition-colors",
                        isSelected ? "bg-accent/6" : "hover:bg-surface-2/50",
                      )}
                    >
                      <div className="w-5">
                        <Checkbox
                          checked={isSelected}
                          onCheckedChange={(v) => toggleOne(c.id, !!v)}
                          aria-label={`Select ${c.username}`}
                        />
                      </div>
                      <div className="flex min-w-0 flex-1 items-center gap-3">
                        <button
                          type="button"
                          onClick={() => openArtifacts(c)}
                          className="truncate text-left text-[14px] font-semibold text-txt-primary hover:text-accent-light"
                        >
                          {c.username}
                        </button>
                        <div className="flex shrink-0 gap-1">
                          {c.protocols.includes("vless") && (
                            <Badge variant="protocol-vless">VLESS</Badge>
                          )}
                          {c.protocols.includes("hy2") && (
                            <Badge variant="protocol-hy2">HY2</Badge>
                          )}
                        </div>
                      </div>
                      <div className="flex w-[90px] items-center gap-2">
                        <Toggle checked={c.enabled} onCheckedChange={() => handleToggle(c)} />
                        <StatusDot active={c.enabled} />
                      </div>
                      <div className="w-[160px]">
                        <TrafficBar client={c} />
                      </div>
                      <div className="w-[140px]">
                        <ExpireLabel expireAt={c.expire_at} />
                      </div>
                      <div className="w-8">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <button
                              type="button"
                              aria-label="Actions"
                              className="inline-grid h-8 w-8 place-items-center rounded-lg text-txt-muted opacity-0 transition-opacity hover:bg-surface-3/60 hover:text-txt-primary group-hover:opacity-100 data-[state=open]:opacity-100"
                            >
                              <MoreHorizontal size={16} />
                            </button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent>
                            <DropdownMenuItem
                              icon={<QrCode size={14} />}
                              onSelect={() => openArtifacts(c)}
                            >
                              QR & links
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              icon={<Pencil size={14} />}
                              onSelect={() => openEdit(c)}
                            >
                              Edit
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              icon={c.enabled ? <PowerOff size={14} /> : <Power size={14} />}
                              onSelect={() => handleToggle(c)}
                            >
                              {c.enabled ? "Disable" : "Enable"}
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              danger
                              icon={<Trash2 size={14} />}
                              onSelect={() => {
                                if (window.confirm(`Delete ${c.username}?`)) {
                                  void handleDelete(c);
                                }
                              }}
                            >
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>

        {pageCount > 1 && !isLoading ? (
          <div className="flex items-center justify-between px-5 py-3 text-[12px] text-txt-secondary">
            <span className="tabular-nums">
              {page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, filtered.length)} of{" "}
              {filtered.length}
            </span>
            <div className="flex items-center gap-1">
              <Button
                size="sm"
                disabled={page === 0}
                onClick={() => setPage((p) => Math.max(0, p - 1))}
              >
                Prev
              </Button>
              <span className="px-2 tabular-nums">
                {page + 1} / {pageCount}
              </span>
              <Button
                size="sm"
                disabled={page + 1 >= pageCount}
                onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        ) : null}
      </div>

      {/* Floating bulk bar */}
      {selected.size > 0 ? (
        <div className="pointer-events-none fixed inset-x-0 bottom-6 z-40 flex justify-center px-4">
          <div className="pointer-events-auto flex items-center gap-1 rounded-xl bg-surface-2/95 px-3 py-2 shadow-[0_18px_42px_-16px_var(--dialog-shadow)] backdrop-blur-xl">
            <span className="px-2 text-[13px] font-semibold text-txt-primary tabular-nums">
              {selected.size}
            </span>
            <div className="mx-1 h-4 w-px bg-border/40" />
            <Button size="sm" onClick={() => bulkEnable(true)}>
              <Power size={13} /> Enable
            </Button>
            <Button size="sm" onClick={() => bulkEnable(false)}>
              <PowerOff size={13} /> Disable
            </Button>
            <ConfirmPopover
              title="Delete users"
              description={`Delete ${selected.size} selected?`}
              confirmText="Delete"
              onConfirm={bulkDelete}
            >
              <Button size="sm" variant="danger">
                <Trash2 size={13} /> Delete
              </Button>
            </ConfirmPopover>
            <div className="mx-1 h-4 w-px bg-border/40" />
            <button
              type="button"
              onClick={clearSelection}
              className="rounded-md p-1.5 text-txt-muted hover:bg-surface-3/60 hover:text-txt-primary"
              aria-label="Clear selection"
            >
              <X size={14} />
            </button>
          </div>
        </div>
      ) : null}

      <ClientFormDialog
        open={formOpen}
        mode={formMode}
        busy={formBusy}
        client={editingClient}
        error={formError}
        onClose={closeForm}
        onSubmit={submitForm}
      />
      <ClientArtifactsDialog
        open={artifactsOpen}
        client={artifactsClient}
        artifacts={artifactsData}
        loading={artifactsLoading}
        onClose={() => setArtifactsOpen(false)}
      />
    </div>
  );
}
