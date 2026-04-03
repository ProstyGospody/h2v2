import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Pencil,
  Plus,
  Power,
  PowerOff,
  QrCode,
  Search,
  Trash2,
  Users2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ClientArtifactsDialog } from "@/components/dialogs/client-artifacts-dialog";
import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { ClientFormDialog } from "@/components/forms/client-form-dialog";
import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import {
  expireToISO,
  trafficLimitToBytes,
} from "@/domain/clients/adapters";
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
  Input,
  Toggle,
  Tooltip,
  cn,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { queryRefetchInterval } from "@/src/queries/polling";
import { formatBytes, formatDateTime } from "@/utils/format";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const SEARCH_DEBOUNCE_MS = 250;
const ROWS_PER_PAGE = 25;
const EXPIRE_SOON_DAYS = 7;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function totalTraffic(c: Client) {
  return c.traffic_used_up_bytes + c.traffic_used_down_bytes;
}

function trafficPercent(c: Client) {
  if (c.traffic_limit_bytes <= 0) return 0;
  return Math.min(100, (totalTraffic(c) / c.traffic_limit_bytes) * 100);
}

function matchSearch(c: Client, q: string) {
  if (!q) return true;
  return c.username.toLowerCase().includes(q);
}

function matchStatus(c: Client, f: string) {
  if (f === "all") return true;
  if (f === "enabled") return c.enabled;
  if (f === "disabled") return !c.enabled;
  return true;
}

type ExpireState = "expired" | "soon" | "ok" | "none";

function expireState(expireAt: string | null): ExpireState {
  if (!expireAt) return "none";
  const diff = new Date(expireAt).getTime() - Date.now();
  if (diff <= 0) return "expired";
  if (diff < EXPIRE_SOON_DAYS * 86_400_000) return "soon";
  return "ok";
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StatCard({ label, value, accent }: { label: string; value: string | number; accent?: boolean }) {
  return (
    <div className={cn(
      "flex min-w-0 flex-col gap-0.5 rounded-xl border px-4 py-2.5",
      accent
        ? "border-accent/20 bg-accent/6"
        : "border-border/40 bg-surface-2/40",
    )}>
      <span className="text-[11px] font-semibold uppercase tracking-wider text-txt-muted">{label}</span>
      <span className={cn("text-[20px] font-bold leading-tight tabular-nums", accent ? "text-accent-light" : "text-txt-primary")}>{value}</span>
    </div>
  );
}

function ExpireCell({ expireAt }: { expireAt: string | null }) {
  const state = expireState(expireAt);
  if (state === "none") return <span className="text-txt-muted">—</span>;

  const label = formatDateTime(expireAt, { includeSeconds: false });
  return (
    <span className={cn(
      "text-[13px]",
      state === "expired" && "font-medium text-status-danger",
      state === "soon" && "font-medium text-status-warning",
      state === "ok" && "text-txt-secondary",
    )}>
      {state === "expired" && <span className="mr-1.5 inline-block h-1.5 w-1.5 translate-y-[-1px] rounded-full bg-status-danger align-middle" />}
      {state === "soon" && <span className="mr-1.5 inline-block h-1.5 w-1.5 translate-y-[-1px] rounded-full bg-status-warning align-middle" />}
      {label}
    </span>
  );
}

function TrafficCell({ client }: { client: Client }) {
  const used = totalTraffic(client);
  const pct = trafficPercent(client);
  const limited = client.traffic_limit_bytes > 0;
  const danger = pct >= 90;
  const warn = pct >= 70;

  return (
    <div className="min-w-[140px] space-y-1.5">
      <div className="flex items-baseline gap-1 text-[13px]">
        <span className={cn("font-medium", danger ? "text-status-danger" : warn ? "text-status-warning" : "text-txt-primary")}>
          {formatBytes(used)}
        </span>
        {limited && (
          <span className="text-txt-muted">/ {formatBytes(client.traffic_limit_bytes)}</span>
        )}
      </div>
      {limited && (
        <div className="flex items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-surface-3">
            <div
              className={cn(
                "h-full rounded-full transition-all duration-500",
                danger ? "bg-status-danger" : warn ? "bg-status-warning" : "bg-accent",
              )}
              style={{ width: `${pct}%` }}
            />
          </div>
          <span className="w-8 text-right text-[11px] tabular-nums text-txt-muted">{Math.round(pct)}%</span>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function UsersPage() {
  const qc = useQueryClient();
  const toast = useToast();

  // Data
  const usersQuery = useQuery({
    queryKey: ["clients"],
    queryFn: listClients,
    refetchInterval: (q) => queryRefetchInterval(10_000, q),
  });

  const clients = usersQuery.data ?? [];

  // Stats
  const stats = useMemo(() => ({
    total: clients.length,
    active: clients.filter((c) => c.enabled).length,
    disabled: clients.filter((c) => !c.enabled).length,
    traffic: clients.reduce((acc, c) => acc + totalTraffic(c), 0),
  }), [clients]);

  // Search + filter
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "enabled" | "disabled">("all");
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => setSearchQuery(searchInput.toLowerCase().trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(debounceRef.current);
  }, [searchInput]);

  const filtered = useMemo(
    () => clients.filter((c) => matchSearch(c, searchQuery) && matchStatus(c, statusFilter)),
    [clients, searchQuery, statusFilter],
  );

  // Pagination
  const [page, setPage] = useState(0);
  const totalPages = Math.max(1, Math.ceil(filtered.length / ROWS_PER_PAGE));
  const paged = filtered.slice(page * ROWS_PER_PAGE, (page + 1) * ROWS_PER_PAGE);

  useEffect(() => setPage(0), [searchQuery, statusFilter]);

  // Selection
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const allOnPageSelected = paged.length > 0 && paged.every((c) => selected.has(c.id));

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  }
  function toggleSelectAll() {
    if (allOnPageSelected) {
      setSelected((prev) => {
        const next = new Set(prev);
        paged.forEach((c) => next.delete(c.id));
        return next;
      });
    } else {
      setSelected((prev) => {
        const next = new Set(prev);
        paged.forEach((c) => next.add(c.id));
        return next;
      });
    }
  }

  // Create / Edit dialog
  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState("");
  const [editingClient, setEditingClient] = useState<Client | null>(null);

  function openCreate() {
    setEditingClient(null);
    setFormMode("create");
    setFormError("");
    setFormOpen(true);
  }
  function openEdit(client: Client) {
    setEditingClient(client);
    setFormMode("edit");
    setFormError("");
    setFormOpen(true);
  }
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
        toast.notify(`User "${payload.username}" created`);
      } else if (editingClient) {
        await updateClient(editingClient.id, payload);
        toast.notify(`User "${payload.username}" updated`);
      }
      await qc.invalidateQueries({ queryKey: ["clients"] });
      closeForm();
    } catch (err) {
      setFormError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setFormBusy(false);
    }
  }

  // Artifacts dialog
  const [artifactsOpen, setArtifactsOpen] = useState(false);
  const [artifactsClient, setArtifactsClient] = useState<Client | null>(null);
  const [artifactsData, setArtifactsData] = useState<ClientArtifacts | null>(null);
  const [artifactsLoading, setArtifactsLoading] = useState(false);

  async function openArtifacts(client: Client) {
    setArtifactsClient(client);
    setArtifactsData(null);
    setArtifactsLoading(true);
    setArtifactsOpen(true);
    try {
      const data = await getClientArtifacts(client.id);
      setArtifactsData(data);
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Failed to load QR"), "error");
    } finally {
      setArtifactsLoading(false);
    }
  }

  // Actions
  const handleToggle = useCallback(async (client: Client) => {
    try {
      await setClientEnabled(client.id, !client.enabled);
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Failed to toggle user"), "error");
    }
  }, [qc, toast]);

  const handleDelete = useCallback(async (client: Client) => {
    try {
      await deleteClient(client.id);
      toast.notify(`User "${client.username}" deleted`);
      setSelected((prev) => { const s = new Set(prev); s.delete(client.id); return s; });
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Failed to delete user"), "error");
    }
  }, [qc, toast]);

  async function bulkEnable(enabled: boolean) {
    const ids = [...selected];
    if (ids.length === 0) return;
    try {
      const count = await setClientsEnabledBulk(ids, enabled);
      toast.notify(`${count} user(s) ${enabled ? "enabled" : "disabled"}`);
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Bulk operation failed"), "error");
    }
  }

  async function bulkDelete() {
    const ids = [...selected];
    if (ids.length === 0) return;
    try {
      const count = await deleteClientsBulk(ids);
      toast.notify(`${count} user(s) deleted`);
      setSelected(new Set());
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Bulk delete failed"), "error");
    }
  }

  // Keyboard
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        if (formOpen || artifactsOpen) return;
        if (selected.size > 0) { setSelected(new Set()); return; }
        if (searchInput) { setSearchInput(""); return; }
      }
      if ((e.key === "/" || (e.key === "k" && (e.metaKey || e.ctrlKey))) && !formOpen && !artifactsOpen) {
        e.preventDefault();
        searchRef.current?.focus();
      }
      if (e.key === "n" && !formOpen && !artifactsOpen && document.activeElement?.tagName !== "INPUT") {
        openCreate();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [formOpen, artifactsOpen, selected, searchInput]);

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  const isLoading = usersQuery.isLoading;
  const isError = usersQuery.isError;

  return (
    <div className="space-y-5">
      {/* Header */}
      <PageHeader
        title="Users"
        subtitle={`${clients.length} total`}
        actions={
          <Button variant="primary" onClick={openCreate}>
            <Plus size={16} strokeWidth={2} />
            New User
          </Button>
        }
      />

      {/* Stats bar */}
      {!isLoading && clients.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label="Total" value={stats.total} />
          <StatCard label="Active" value={stats.active} accent />
          <StatCard label="Disabled" value={stats.disabled} />
          <StatCard label="Traffic used" value={formatBytes(stats.traffic)} />
        </div>
      )}

      {/* Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative flex-1 sm:max-w-[380px]">
          <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-txt-muted" />
          <Input
            ref={searchRef}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search users… (/ to focus)"
            className="pl-9 pr-8"
          />
          {searchInput && (
            <button
              type="button"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded p-0.5 text-txt-muted hover:text-txt"
              onClick={() => setSearchInput("")}
            >
              <X size={14} />
            </button>
          )}
        </div>

        <div className="flex items-center gap-1 rounded-xl border border-border/40 bg-surface-2/40 p-1">
          {(["all", "enabled", "disabled"] as const).map((f) => (
            <button
              key={f}
              type="button"
              onClick={() => setStatusFilter(f)}
              className={cn(
                "rounded-lg px-3.5 py-1.5 text-[12px] font-semibold capitalize transition-all",
                statusFilter === f
                  ? "bg-surface-3/80 text-txt shadow-sm"
                  : "text-txt-secondary hover:text-txt",
              )}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      {/* Bulk actions */}
      {selected.size > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-accent/20 bg-accent/6 px-4 py-2.5 text-[13px]">
          <span className="font-semibold text-accent-light">{selected.size} selected</span>
          <div className="mx-1 h-4 w-px bg-border/60" />
          <Button size="sm" onClick={() => bulkEnable(true)}>
            <Power size={13} /> Enable
          </Button>
          <Button size="sm" onClick={() => bulkEnable(false)}>
            <PowerOff size={13} /> Disable
          </Button>
          <ConfirmPopover
            title="Delete users"
            description={`Delete ${selected.size} selected user(s)? This cannot be undone.`}
            confirmText="Delete"
            onConfirm={bulkDelete}
          >
            <Button size="sm" variant="danger">
              <Trash2 size={13} /> Delete
            </Button>
          </ConfirmPopover>
          <button
            type="button"
            className="ml-auto flex items-center gap-1.5 rounded-lg px-2 py-1 text-[12px] text-txt-muted hover:text-txt"
            onClick={() => setSelected(new Set())}
          >
            <X size={13} /> Clear
          </button>
        </div>
      )}

      {/* Error */}
      {isError && (
        <ErrorBanner
          message={getAPIErrorMessage(usersQuery.error, "Failed to load users")}
          actionLabel="Retry"
          onAction={() => usersQuery.refetch()}
        />
      )}

      {/* Table */}
      <div className="overflow-hidden rounded-2xl border border-border/40 bg-surface-2/30">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] border-collapse text-[13px]">
            <thead>
              <tr className="border-b border-border/40 bg-surface-2/70">
                <th className="w-10 px-3 py-3.5">
                  <Checkbox checked={allOnPageSelected && paged.length > 0} onCheckedChange={toggleSelectAll} />
                </th>
                <th className="px-4 py-3.5 text-left text-[11px] font-semibold uppercase tracking-wider text-txt-muted">User</th>
                <th className="px-4 py-3.5 text-left text-[11px] font-semibold uppercase tracking-wider text-txt-muted">Status</th>
                <th className="px-4 py-3.5 text-left text-[11px] font-semibold uppercase tracking-wider text-txt-muted">Traffic</th>
                <th className="px-4 py-3.5 text-left text-[11px] font-semibold uppercase tracking-wider text-txt-muted">Expires</th>
                <th className="w-28 px-3 py-3.5 text-right text-[11px] font-semibold uppercase tracking-wider text-txt-muted">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/20">
              {isLoading ? (
                Array.from({ length: 7 }).map((_, i) => (
                  <tr key={i}>
                    {Array.from({ length: 6 }).map((_, j) => (
                      <td key={j} className="px-4 py-4">
                        <div className={cn("h-3.5 animate-pulse rounded-md bg-surface-3/60", j === 1 ? "w-32" : j === 3 ? "w-28" : "w-16")} />
                      </td>
                    ))}
                  </tr>
                ))
              ) : paged.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-20 text-center">
                    <div className="flex flex-col items-center gap-3 text-txt-muted">
                      <Users2 size={36} strokeWidth={1.2} className="opacity-40" />
                      <p className="text-[14px]">
                        {searchQuery || statusFilter !== "all"
                          ? "No users match the current filters."
                          : "No users yet."}
                      </p>
                      {!searchQuery && statusFilter === "all" && (
                        <Button variant="primary" size="sm" onClick={openCreate}>
                          <Plus size={14} /> Create first user
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ) : (
                paged.map((client) => (
                  <tr
                    key={client.id}
                    className={cn(
                      "group/row transition-colors hover:bg-surface-2/50",
                      selected.has(client.id) && "bg-accent/4 hover:bg-accent/6",
                    )}
                  >
                    {/* Select */}
                    <td className="px-3 py-3.5">
                      <Checkbox checked={selected.has(client.id)} onCheckedChange={() => toggleSelect(client.id)} />
                    </td>

                    {/* User */}
                    <td className="px-4 py-3.5">
                      <div className="flex flex-col gap-1.5">
                        <button
                          type="button"
                          className="w-fit font-semibold text-txt-primary hover:text-accent-light hover:underline"
                          onClick={() => openArtifacts(client)}
                        >
                          {client.username}
                        </button>
                        <div className="flex gap-1.5">
                          {client.protocols.includes("vless") && <Badge variant="protocol-vless">VLESS</Badge>}
                          {client.protocols.includes("hy2") && <Badge variant="protocol-hy2">HY2</Badge>}
                          {client.protocols.length === 0 && (
                            <span className="text-[11px] text-txt-muted">No access</span>
                          )}
                        </div>
                      </div>
                    </td>

                    {/* Status */}
                    <td className="px-4 py-3.5">
                      <div className="flex items-center gap-2.5">
                        <Toggle
                          checked={client.enabled}
                          onCheckedChange={() => handleToggle(client)}
                        />
                        <span className={cn(
                          "text-[12px] font-medium",
                          client.enabled ? "text-status-success" : "text-txt-muted",
                        )}>
                          {client.enabled ? "Active" : "Disabled"}
                        </span>
                      </div>
                    </td>

                    {/* Traffic */}
                    <td className="px-4 py-3.5">
                      <TrafficCell client={client} />
                    </td>

                    {/* Expires */}
                    <td className="px-4 py-3.5">
                      <ExpireCell expireAt={client.expire_at} />
                    </td>

                    {/* Actions */}
                    <td className="px-3 py-3.5">
                      <div className="flex items-center justify-end gap-1 opacity-0 transition-opacity group-hover/row:opacity-100">
                        <Tooltip content="QR & Links">
                          <Button size="sm" onClick={() => openArtifacts(client)}>
                            <QrCode size={14} />
                          </Button>
                        </Tooltip>
                        <Tooltip content="Edit">
                          <Button size="sm" onClick={() => openEdit(client)}>
                            <Pencil size={14} />
                          </Button>
                        </Tooltip>
                        <ConfirmPopover
                          title="Delete user"
                          description={`Delete "${client.username}"? This cannot be undone.`}
                          confirmText="Delete"
                          onConfirm={() => handleDelete(client)}
                        >
                          <Button size="sm" variant="danger">
                            <Trash2 size={14} />
                          </Button>
                        </ConfirmPopover>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between border-t border-border/40 px-5 py-3 text-[12px] text-txt-secondary">
            <span>
              Showing {page * ROWS_PER_PAGE + 1}–{Math.min((page + 1) * ROWS_PER_PAGE, filtered.length)} of {filtered.length}
            </span>
            <div className="flex items-center gap-1">
              <Button size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>← Prev</Button>
              <span className="px-2 tabular-nums">{page + 1} / {totalPages}</span>
              <Button size="sm" disabled={page >= totalPages - 1} onClick={() => setPage(page + 1)}>Next →</Button>
            </div>
          </div>
        )}
      </div>

      {/* Dialogs */}
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
