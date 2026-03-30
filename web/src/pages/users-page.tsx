import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { motion } from "framer-motion";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  MoreVertical,
  Pencil,
  Plus,
  QrCode,
  Search,
  Trash2,
} from "lucide-react";
import type { MouseEvent } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { ClientArtifactsDialog } from "@/components/dialogs/client-artifacts-dialog";
import { ConfirmDialog } from "@/components/dialogs/confirm-dialog";
import { ClientFormDialog } from "@/components/forms/client-form-dialog";
import { PageHeader } from "@/components/ui/page-header";
import { toCreateRequest, toUpdateRequest, type ClientFormValues } from "@/domain/clients/adapters";
import {
  createClient,
  deleteClient,
  getClientArtifacts,
  getClientDefaults,
  listClients,
  setClientEnabled,
  updateClient,
} from "@/domain/clients/services";
import { HysteriaClient, HysteriaClientDefaults, HysteriaUserPayload } from "@/domain/clients/types";
import { useNotice } from "@/hooks/use-notice";
import { APIError } from "@/services/api";
import {
  Badge,
  Button,
  Checkbox,
  Input,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  Toast,
  Toggle,
  cn,
} from "@/src/components/ui";
import { formatBytes, formatDateTime, formatRate } from "@/utils/format";

type ClientFilter = "all" | "online" | "enabled" | "disabled";

const rowsPerPageOptions = [10, 25, 50, 100];

function initials(value: string): string {
  const clean = value.trim();
  return clean ? clean.slice(0, 1).toUpperCase() : "?";
}

export default function UsersPage() {
  const [clients, setClients] = useState<HysteriaClient[]>([]);
  const [defaults, setDefaults] = useState<HysteriaClientDefaults | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [filter, setFilter] = useState<ClientFilter>("all");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(25);
  const [selectedClientIDs, setSelectedClientIDs] = useState<string[]>([]);

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState("");
  const [editingClient, setEditingClient] = useState<HysteriaClient | null>(null);

  const [deleteTarget, setDeleteTarget] = useState<HysteriaClient | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [bulkDeleteBusy, setBulkDeleteBusy] = useState(false);

  const [artifactOpen, setArtifactOpen] = useState(false);
  const [artifactLoading, setArtifactLoading] = useState(false);
  const [artifactClient, setArtifactClient] = useState<HysteriaClient | null>(null);
  const [artifactPayload, setArtifactPayload] = useState<HysteriaUserPayload | null>(null);

  const notice = useNotice();

  const load = useCallback(async () => {
    setError("");
    try {
      const [items, inherited] = await Promise.all([listClients(), getClientDefaults()]);
      setClients(items);
      setDefaults(inherited);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load users");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    const timer = setInterval(() => void load(), 5000);
    return () => clearInterval(timer);
  }, [load]);

  useEffect(() => {
    setPage(0);
  }, [searchQuery, filter]);

  useEffect(() => {
    const existing = new Set(clients.map((client) => client.id));
    setSelectedClientIDs((current) => current.filter((id) => existing.has(id)));
  }, [clients]);

  const filteredClients = useMemo(() => {
    const needle = searchQuery.trim().toLowerCase();
    return [...clients]
      .sort((a, b) => a.username.localeCompare(b.username, undefined, { sensitivity: "base" }))
      .filter((client) => {
        if (filter === "online" && client.online_count <= 0) {
          return false;
        }
        if (filter === "enabled" && !client.enabled) {
          return false;
        }
        if (filter === "disabled" && client.enabled) {
          return false;
        }

        if (!needle) {
          return true;
        }

        const haystack = [client.username, client.username_normalized, client.note || "", client.id].join(" ").toLowerCase();
        return haystack.includes(needle);
      });
  }, [clients, filter, searchQuery]);

  const selectedSet = useMemo(() => new Set(selectedClientIDs), [selectedClientIDs]);
  const filteredIDs = useMemo(() => filteredClients.map((client) => client.id), [filteredClients]);
  const selectedFilteredCount = useMemo(() => filteredIDs.reduce((sum, id) => sum + (selectedSet.has(id) ? 1 : 0), 0), [filteredIDs, selectedSet]);

  const allFilteredSelected = filteredIDs.length > 0 && selectedFilteredCount === filteredIDs.length;
  const someFilteredSelected = selectedFilteredCount > 0 && !allFilteredSelected;

  const pagedClients = useMemo(() => {
    const start = page * rowsPerPage;
    return filteredClients.slice(start, start + rowsPerPage);
  }, [filteredClients, page, rowsPerPage]);

  const maxTraffic = useMemo(() => {
    return filteredClients.reduce((max, client) => Math.max(max, client.last_tx_bytes + client.last_rx_bytes), 0);
  }, [filteredClients]);

  useEffect(() => {
    const maxPage = Math.max(0, Math.ceil(filteredClients.length / rowsPerPage) - 1);
    if (page > maxPage) {
      setPage(maxPage);
    }
  }, [filteredClients.length, page, rowsPerPage]);

  function openCreate() {
    setFormMode("create");
    setEditingClient(null);
    setFormError("");
    setFormOpen(true);
  }

  function openEdit(client: HysteriaClient) {
    setFormMode("edit");
    setEditingClient(client);
    setFormError("");
    setFormOpen(true);
  }

  async function submitForm(values: ClientFormValues) {
    setFormBusy(true);
    setFormError("");
    try {
      if (formMode === "create") {
        await createClient(toCreateRequest(values));
        notice.notify("User created");
      } else if (editingClient) {
        await updateClient(editingClient.id, toUpdateRequest(values));
        notice.notify("User updated");
      }
      setFormOpen(false);
      await load();
    } catch (err) {
      const message = err instanceof APIError ? err.message : "Failed to save user";
      setFormError(message);
    } finally {
      setFormBusy(false);
    }
  }

  async function removeClient() {
    if (!deleteTarget) {
      return;
    }
    setDeleteBusy(true);
    try {
      await deleteClient(deleteTarget.id);
      setDeleteTarget(null);
      notice.notify("User deleted");
      await load();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to delete user");
    } finally {
      setDeleteBusy(false);
    }
  }

  async function deleteSelectedClients() {
    if (!selectedClientIDs.length) {
      return;
    }

    const targetIDs = [...selectedClientIDs];
    const failedIDs: string[] = [];
    let firstError = "";
    let deletedCount = 0;

    setBulkDeleteBusy(true);
    setError("");
    try {
      for (const id of targetIDs) {
        try {
          await deleteClient(id);
          deletedCount += 1;
        } catch (err) {
          failedIDs.push(id);
          if (!firstError) {
            firstError = err instanceof APIError ? err.message : "Failed to delete selected users";
          }
        }
      }

      if (deletedCount > 0) {
        notice.notify(deletedCount === 1 ? "1 user deleted" : `${deletedCount} users deleted`);
      }

      if (failedIDs.length > 0) {
        setSelectedClientIDs(failedIDs);
        setError(firstError || `Deleted ${deletedCount} of ${targetIDs.length} users`);
      } else {
        setSelectedClientIDs([]);
      }
    } finally {
      setBulkDeleteBusy(false);
      setBulkDeleteOpen(false);
      await load();
    }
  }

  async function toggleEnabled(client: HysteriaClient) {
    try {
      await setClientEnabled(client.id, !client.enabled);
      await load();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to change state");
    }
  }

  async function openArtifacts(client: HysteriaClient) {
    setArtifactClient(client);
    setArtifactOpen(true);
    setArtifactLoading(true);
    try {
      const payload = await getClientArtifacts(client.id);
      setArtifactPayload(payload);
    } catch (err) {
      setArtifactPayload(null);
      setError(err instanceof APIError ? err.message : "Failed to load artifacts");
    } finally {
      setArtifactLoading(false);
    }
  }

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      notice.notify("Copied");
    } catch {
      setError("Clipboard write failed");
    }
  }

  function toggleClientSelection(clientID: string, checked: boolean) {
    setSelectedClientIDs((current) => {
      if (checked) {
        if (current.includes(clientID)) {
          return current;
        }
        return [...current, clientID];
      }
      return current.filter((id) => id !== clientID);
    });
  }

  function toggleSelectFiltered(checked: boolean) {
    if (checked) {
      setSelectedClientIDs((current) => {
        const next = new Set(current);
        for (const id of filteredIDs) {
          next.add(id);
        }
        return Array.from(next);
      });
      return;
    }

    const filteredSet = new Set(filteredIDs);
    setSelectedClientIDs((current) => current.filter((id) => !filteredSet.has(id)));
  }

  function handleFilterChange(_event: MouseEvent<HTMLButtonElement>, next: ClientFilter) {
    setFilter(next);
  }

  function handleRowsPerPageChange(value: string) {
    const parsed = Number.parseInt(value, 10);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return;
    }
    setRowsPerPage(parsed);
    setPage(0);
  }

  const pageCount = Math.max(1, Math.ceil(filteredClients.length / rowsPerPage));

  return (
    <div className="space-y-6">
      <PageHeader
        title="Users"
        actions={
          <>
            <Button variant="primary" onClick={openCreate} className="h-12 w-full rounded-2xl px-5 sm:w-auto">
              <Plus size={18} strokeWidth={1.6} />
              Add user
            </Button>
            <Button
              variant="danger"
              disabled={!selectedClientIDs.length}
              onClick={() => setBulkDeleteOpen(true)}
              className="h-12 w-full rounded-2xl px-5 sm:min-w-[214px] sm:w-auto"
            >
              <Trash2 size={18} strokeWidth={1.6} />
              Delete selected
              <span className="inline-flex min-w-[2ch] items-center justify-center rounded-md bg-status-danger/20 px-1.5 py-0.5 text-[12px] leading-none tabular-nums">
                {selectedClientIDs.length}
              </span>
            </Button>
            <div className="relative w-full sm:w-auto sm:min-w-[240px]">
              <Search size={16} strokeWidth={1.6} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-tertiary" />
              <Input
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="Search users..."
                className="h-12 rounded-2xl border-border/80 bg-surface-2/70 pl-11 shadow-[inset_0_1px_0_var(--shell-highlight)]"
              />
            </div>
            <div className="flex w-full flex-wrap items-center gap-1 rounded-2xl border border-border/70 bg-surface-2/70 p-1 shadow-[inset_0_1px_0_var(--shell-highlight)] sm:inline-flex sm:w-auto sm:flex-nowrap sm:gap-0">
              {(["all", "online", "enabled", "disabled"] as ClientFilter[]).map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={(event) => handleFilterChange(event, item)}
                  className={cn(
                    "flex-1 rounded-xl px-3 py-2 text-center text-[13px] font-semibold capitalize transition-all sm:flex-none sm:px-4",
                    filter === item && "bg-surface-4 text-txt-primary shadow-sm",
                    filter !== item && "text-txt-secondary hover:text-txt-primary",
                  )}
                >
                  {item}
                </button>
              ))}
            </div>
          </>
        }
      />

      {error && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}

      <TableContainer className="overflow-x-auto">
        {loading ? (
          <div className="flex min-h-[260px] items-center justify-center">
            <div className="flex flex-col items-center gap-3">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
              <p className="text-[14px] text-txt-secondary">Loading users...</p>
            </div>
          </div>
        ) : (
          <>
            <div className="flex flex-col gap-3 border-b border-border/50 px-4 py-3.5 sm:flex-row sm:items-center sm:justify-between sm:px-5">
              <p className="text-[13px] text-txt-secondary">{filteredClients.length} users</p>
              <div className="flex items-center gap-2 text-[13px] text-txt-secondary">
                <span>Rows:</span>
                <select
                  value={rowsPerPage}
                  onChange={(event) => handleRowsPerPageChange(event.target.value)}
                  className="rounded-lg border border-border bg-surface-1 px-3 py-1.5 text-[13px] text-txt outline-none"
                >
                  {rowsPerPageOptions.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <Table>
              <TableHeader>
                <TableRow className="border-t-0 hover:bg-transparent">
                  <TableHead className="w-10">
                    <Checkbox
                      checked={allFilteredSelected ? true : someFilteredSelected ? "indeterminate" : false}
                      onCheckedChange={(value) => toggleSelectFiltered(value === true)}
                      aria-label="select filtered users"
                    />
                  </TableHead>
                  <TableHead className="hidden w-14 md:table-cell">#</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead className="hidden w-[96px] lg:table-cell">Protocol</TableHead>
                  <TableHead className="w-[170px]">Status</TableHead>
                  <TableHead className="hidden lg:table-cell">Traffic</TableHead>
                  <TableHead className="hidden w-[190px] lg:table-cell">Network</TableHead>
                  <TableHead className="hidden md:table-cell">Last Seen</TableHead>
                  <TableHead className="w-[88px] text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pagedClients.length ? (
                  pagedClients.map((client, index) => {
                    const traffic = client.last_tx_bytes + client.last_rx_bytes;
                    const ratio = maxTraffic > 0 ? Math.min(100, (traffic / maxTraffic) * 100) : 0;
                    const ratioWidth = traffic > 0 ? Math.max(ratio, 4) : 0;
                    const statusOnline = client.online_count > 0;
                    const downBps = Math.max(0, client.download_bps || 0);
                    const upBps = Math.max(0, client.upload_bps || 0);
                    return (
                      <motion.tr
                        key={client.id}
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ delay: index * 0.03 }}
                        className="border-t border-border transition-colors hover:bg-surface-3"
                      >
                        <TableCell>
                          <Checkbox
                            checked={selectedSet.has(client.id)}
                            onCheckedChange={(value) => toggleClientSelection(client.id, value === true)}
                            aria-label={`select ${client.username}`}
                          />
                        </TableCell>
                        <TableCell className="hidden md:table-cell">{page * rowsPerPage + index + 1}</TableCell>
                        <TableCell>
                          <div className="flex min-w-0 items-center gap-2">
                            <button
                              type="button"
                              onClick={() => void openArtifacts(client)}
                              className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-accent/15 to-accent-secondary/10 text-[13px] font-bold text-txt-primary transition-all hover:from-accent/25 hover:to-accent-secondary/20"
                            >
                              {initials(client.username)}
                            </button>
                            <div className="min-w-0">
                              <button
                                type="button"
                                onClick={() => void openArtifacts(client)}
                                className="max-w-full truncate text-[14px] font-medium text-txt hover:text-txt-primary"
                              >
                                {client.username}
                              </button>
                              <p className="truncate text-[12px] text-txt-muted">{client.note || "-"}</p>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="hidden lg:table-cell">
                          <Badge variant="protocol-hy2">HY2</Badge>
                        </TableCell>
                        <TableCell className="whitespace-nowrap">
                          <div className="flex min-w-[150px] items-center gap-3">
                            <span className="inline-flex w-[74px] items-center gap-2">
                              <span
                                className={cn(
                                  "h-[6px] w-[6px] rounded-full",
                                  statusOnline && client.enabled && "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]",
                                  !statusOnline && client.enabled && "bg-status-warning",
                                  !client.enabled && "bg-txt-muted",
                                )}
                              />
                              <span className="w-[62px] text-[11px] text-txt-secondary">
                                {!client.enabled ? "disabled" : statusOnline ? "online" : "offline"}
                              </span>
                            </span>
                            <Toggle className="shrink-0" checked={client.enabled} onCheckedChange={() => void toggleEnabled(client)} />
                          </div>
                        </TableCell>
                        <TableCell className="hidden lg:table-cell">
                          <div className="space-y-1.5">
                            <div className="h-1.5 w-full overflow-hidden rounded-full border border-border/70 bg-border/70">
                              <div
                                className={cn(
                                  "h-full rounded-full bg-gradient-to-r from-accent to-accent-light shadow-[0_0_8px_var(--accent-soft)]",
                                  ratio > 90 && "from-status-warning to-status-danger",
                                )}
                                style={{ width: `${ratioWidth}%` }}
                              />
                            </div>
                            <p className="text-[11px] font-medium text-txt-tertiary">{formatBytes(traffic)}</p>
                          </div>
                        </TableCell>
                        <TableCell className="hidden w-[190px] lg:table-cell">
                          <div className="space-y-1.5">
                            <div className="flex items-center gap-3 text-[11px] font-semibold tabular-nums text-txt-secondary">
                              <span className="inline-flex min-w-[84px] items-center gap-1.5 whitespace-nowrap">
                                <ArrowDownToLine size={12} strokeWidth={1.8} className="text-status-success" />
                                {formatRate(downBps)}
                              </span>
                              <span className="inline-flex min-w-[84px] items-center gap-1.5 whitespace-nowrap">
                                <ArrowUpFromLine size={12} strokeWidth={1.8} className="text-status-warning" />
                                {formatRate(upBps)}
                              </span>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="hidden md:table-cell">{formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}</TableCell>
                        <TableCell className="text-right">
                          <DropdownMenu.Root>
                            <DropdownMenu.Trigger asChild>
                              <button
                                type="button"
                                className="inline-flex h-8 w-8 items-center justify-center rounded-btn text-txt-tertiary transition-colors hover:bg-surface-3 hover:text-txt"
                              >
                                <MoreVertical size={16} strokeWidth={1.4} />
                              </button>
                            </DropdownMenu.Trigger>
                            <DropdownMenu.Portal>
                              <DropdownMenu.Content
                                sideOffset={6}
                                align="end"
                                className="z-50 min-w-[160px] rounded-[10px] border border-border/80 bg-surface-2/95 p-1 shadow-[0_18px_42px_-24px_var(--dialog-shadow)] backdrop-blur-xl"
                              >
                                <DropdownMenu.Item
                                  onSelect={() => void openArtifacts(client)}
                                  className="flex cursor-pointer items-center gap-2 rounded-[7px] px-2.5 py-2 text-[12px] text-txt outline-none transition-colors hover:bg-surface-3/60"
                                >
                                  <QrCode size={15} strokeWidth={1.4} />
                                  Show QR
                                </DropdownMenu.Item>
                                <DropdownMenu.Item
                                  onSelect={() => openEdit(client)}
                                  className="flex cursor-pointer items-center gap-2 rounded-[7px] px-2.5 py-2 text-[12px] text-txt outline-none transition-colors hover:bg-surface-3/60"
                                >
                                  <Pencil size={15} strokeWidth={1.4} />
                                  Edit
                                </DropdownMenu.Item>
                                <DropdownMenu.Item
                                  onSelect={() => setDeleteTarget(client)}
                                  className="flex cursor-pointer items-center gap-2 rounded-[7px] px-2.5 py-2 text-[12px] text-status-danger outline-none transition-colors hover:bg-status-danger/8"
                                >
                                  <Trash2 size={15} strokeWidth={1.4} />
                                  Delete
                                </DropdownMenu.Item>
                              </DropdownMenu.Content>
                            </DropdownMenu.Portal>
                          </DropdownMenu.Root>
                        </TableCell>
                      </motion.tr>
                    );
                  })
                ) : (
                  <TableRow>
                    <TableCell colSpan={9}>
                      {clients.length ? "No users match the current filters." : "No users yet."}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>

            <div className="flex flex-col gap-3 border-t border-border/50 px-4 py-3.5 sm:flex-row sm:items-center sm:justify-between sm:px-5">
              <p className="text-[13px] text-txt-secondary">
                Page {Math.min(page + 1, pageCount)} of {pageCount}
              </p>
              <div className="flex items-center gap-2">
                <Button size="sm" disabled={page <= 0} onClick={() => setPage((value) => Math.max(0, value - 1))}>
                  Prev
                </Button>
                <Button
                  size="sm"
                  disabled={page + 1 >= pageCount}
                  onClick={() => setPage((value) => Math.min(pageCount - 1, value + 1))}
                >
                  Next
                </Button>
              </div>
            </div>
          </>
        )}
      </TableContainer>

      <ClientFormDialog
        open={formOpen}
        mode={formMode}
        busy={formBusy}
        client={editingClient}
        defaults={defaults}
        error={formError}
        onClose={() => setFormOpen(false)}
        onSubmit={submitForm}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="Delete user"
        description={`Delete ${deleteTarget?.username || "user"} and remove access?`}
        busy={deleteBusy}
        confirmText="Delete"
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void removeClient()}
      />

      <ConfirmDialog
        open={bulkDeleteOpen}
        title="Delete selected users"
        description={`Delete ${selectedClientIDs.length} selected users and remove access?`}
        busy={bulkDeleteBusy}
        confirmText="Delete selected"
        onClose={() => setBulkDeleteOpen(false)}
        onConfirm={() => void deleteSelectedClients()}
      />

      <ClientArtifactsDialog
        open={artifactOpen}
        client={artifactClient}
        payload={artifactPayload}
        loading={artifactLoading}
        onClose={() => setArtifactOpen(false)}
        onCopy={(value) => void copy(value)}
      />

      <Toast open={Boolean(notice.message)} onOpenChange={(open) => !open && notice.clear()} message={notice.message} variant="success" />
    </div>
  );
}
