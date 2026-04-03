import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Loader2,
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
import {
  expireToISO,
  formDefaults,
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
    <div className="mx-auto w-full max-w-[1400px] space-y-6 p-6">
      <PageHeader
        title="Users"
        subtitle={`${clients.length} total`}
        actions={
          <Button variant="primary" onClick={openCreate}>
            <Plus size={16} strokeWidth={2} />
            Create User
          </Button>
        }
      />

      {/* Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative max-w-[320px] flex-1">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-txt-muted" />
          <Input
            ref={searchRef}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search users..."
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

        <div className="flex items-center gap-2">
          {(["all", "enabled", "disabled"] as const).map((f) => (
            <button
              key={f}
              type="button"
              onClick={() => setStatusFilter(f)}
              className={cn(
                "rounded-lg px-3 py-1.5 text-[12px] font-semibold capitalize transition-colors",
                statusFilter === f
                  ? "bg-accent/12 text-accent-light"
                  : "text-txt-secondary hover:bg-surface-3/60 hover:text-txt",
              )}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      {/* Bulk actions */}
      {selected.size > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border/40 bg-surface-2/50 px-4 py-2.5 text-[13px]">
          <span className="font-medium text-txt-secondary">{selected.size} selected</span>
          <div className="mx-1 h-4 w-px bg-border/40" />
          <Button size="sm" onClick={() => bulkEnable(true)}>
            <Power size={14} /> Enable
          </Button>
          <Button size="sm" onClick={() => bulkEnable(false)}>
            <PowerOff size={14} /> Disable
          </Button>
          <ConfirmPopover
            title="Delete users"
            description={`Delete ${selected.size} selected user(s)? This cannot be undone.`}
            confirmText="Delete"
            onConfirm={bulkDelete}
          >
            <Button size="sm" variant="danger">
              <Trash2 size={14} /> Delete
            </Button>
          </ConfirmPopover>
          <Button size="sm" onClick={() => setSelected(new Set())}>
            <X size={14} /> Clear
          </Button>
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
          <table className="w-full min-w-[900px] text-[13px]">
            <thead>
              <tr className="border-b border-border/40 bg-surface-2/60">
                <th className="w-[40px] px-3 py-3">
                  <Checkbox checked={allOnPageSelected && paged.length > 0} onCheckedChange={toggleSelectAll} />
                </th>
                <th className="px-3 py-3 text-left font-semibold text-txt-secondary">User</th>
                <th className="px-3 py-3 text-left font-semibold text-txt-secondary">Status</th>
                <th className="px-3 py-3 text-left font-semibold text-txt-secondary">Protocols</th>
                <th className="px-3 py-3 text-left font-semibold text-txt-secondary">Traffic</th>
                <th className="px-3 py-3 text-left font-semibold text-txt-secondary">Expires</th>
                <th className="w-[160px] px-3 py-3 text-right font-semibold text-txt-secondary">Actions</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} className="border-b border-border/20">
                    {Array.from({ length: 7 }).map((_, j) => (
                      <td key={j} className="px-3 py-3.5">
                        <div className="h-4 w-full animate-pulse rounded bg-surface-3/50" />
                      </td>
                    ))}
                  </tr>
                ))
              ) : paged.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-3 py-16 text-center text-[14px] text-txt-secondary">
                    {searchQuery || statusFilter !== "all" ? "No users match the current filters." : "No users yet. Click \"Create User\" to get started."}
                  </td>
                </tr>
              ) : (
                paged.map((client) => (
                  <tr
                    key={client.id}
                    className={cn(
                      "border-b border-border/20 transition-colors hover:bg-surface-2/40",
                      selected.has(client.id) && "bg-accent/4",
                    )}
                  >
                    {/* Select */}
                    <td className="px-3 py-3">
                      <Checkbox checked={selected.has(client.id)} onCheckedChange={() => toggleSelect(client.id)} />
                    </td>

                    {/* Username */}
                    <td className="px-3 py-3">
                      <button
                        type="button"
                        className="font-medium text-txt-primary hover:text-accent-light hover:underline"
                        onClick={() => openArtifacts(client)}
                      >
                        {client.username}
                      </button>
                    </td>

                    {/* Status */}
                    <td className="px-3 py-3">
                      <div className="flex items-center gap-2">
                        <Badge variant={client.enabled ? "success" : "danger"}>
                          {client.enabled ? "Active" : "Disabled"}
                        </Badge>
                        <Toggle
                          checked={client.enabled}
                          onCheckedChange={() => handleToggle(client)}
                          className="scale-75"
                        />
                      </div>
                    </td>

                    {/* Protocols */}
                    <td className="px-3 py-3">
                      <div className="flex gap-1.5">
                        {client.protocols.includes("vless") && <Badge variant="protocol-vless">VLESS</Badge>}
                        {client.protocols.includes("hy2") && <Badge variant="protocol-hy2">HY2</Badge>}
                        {client.protocols.length === 0 && <span className="text-txt-muted">-</span>}
                      </div>
                    </td>

                    {/* Traffic */}
                    <td className="px-3 py-3">
                      <div className="space-y-1">
                        <span className="text-txt-primary">
                          {formatBytes(totalTraffic(client))}
                          {client.traffic_limit_bytes > 0 && (
                            <span className="text-txt-muted"> / {formatBytes(client.traffic_limit_bytes)}</span>
                          )}
                        </span>
                        {client.traffic_limit_bytes > 0 && (
                          <div className="h-1.5 w-24 overflow-hidden rounded-full bg-surface-3">
                            <div
                              className={cn(
                                "h-full rounded-full transition-all",
                                trafficPercent(client) >= 90 ? "bg-status-danger" : "bg-accent",
                              )}
                              style={{ width: `${trafficPercent(client)}%` }}
                            />
                          </div>
                        )}
                      </div>
                    </td>

                    {/* Expires */}
                    <td className="px-3 py-3 text-txt-secondary">
                      {client.expire_at ? formatDateTime(client.expire_at, { includeSeconds: false }) : "-"}
                    </td>

                    {/* Actions */}
                    <td className="px-3 py-3">
                      <div className="flex items-center justify-end gap-1">
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
          <div className="flex items-center justify-between border-t border-border/40 px-4 py-2.5 text-[12px] text-txt-secondary">
            <span>
              {page * ROWS_PER_PAGE + 1}–{Math.min((page + 1) * ROWS_PER_PAGE, filtered.length)} of {filtered.length}
            </span>
            <div className="flex gap-1">
              <Button size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>Prev</Button>
              <Button size="sm" disabled={page >= totalPages - 1} onClick={() => setPage(page + 1)}>Next</Button>
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
