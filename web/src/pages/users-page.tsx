import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  ArrowUpDown,
  ChevronDown,
  ChevronUp,
  Download,
  MoreVertical,
  Pencil,
  Plus,
  Power,
  PowerOff,
  QrCode,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { ClientArtifactsDialog } from "@/components/dialogs/client-artifacts-dialog";
import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
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
  Toggle,
  cn,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { formatBytes, formatDateTime, formatRate } from "@/utils/format";

type ClientFilter = "all" | "online" | "enabled" | "disabled";
type SortField = "username" | "traffic" | "last_seen";
type SortDir = "asc" | "desc";
type SortState = { field: SortField; dir: SortDir };

const rowsPerPageOptions = [10, 25, 50, 100];
const SKELETON_ROWS = 8;

function SkeletonRow() {
  return (
    <tr className="border-t border-border/30">
      <td className="w-10 px-5 py-3.5"><div className="h-4 w-4 animate-pulse rounded bg-surface-3/60" /></td>
      <td className="hidden w-14 px-5 py-3.5 md:table-cell"><div className="h-4 w-6 animate-pulse rounded bg-surface-3/60" /></td>
      <td className="px-5 py-3.5">
        <div className="flex items-center gap-2">
          <div className="h-9 w-9 animate-pulse rounded-xl bg-surface-3/60" />
          <div className="space-y-1.5">
            <div className="h-3.5 w-24 animate-pulse rounded bg-surface-3/60" />
            <div className="h-3 w-16 animate-pulse rounded bg-surface-3/50" />
          </div>
        </div>
      </td>
      <td className="hidden px-5 py-3.5 lg:table-cell"><div className="h-5 w-10 animate-pulse rounded-badge bg-surface-3/60" /></td>
      <td className="px-5 py-3.5"><div className="h-5 w-20 animate-pulse rounded bg-surface-3/60" /></td>
      <td className="hidden px-5 py-3.5 lg:table-cell">
        <div className="space-y-1.5">
          <div className="h-1.5 w-full animate-pulse rounded-full bg-surface-3/60" />
          <div className="h-3 w-12 animate-pulse rounded bg-surface-3/50" />
        </div>
      </td>
      <td className="hidden px-5 py-3.5 text-right lg:table-cell"><div className="ml-auto h-8 w-24 animate-pulse rounded bg-surface-3/60" /></td>
      <td className="hidden px-5 py-3.5 text-right md:table-cell"><div className="ml-auto h-4 w-24 animate-pulse rounded bg-surface-3/60" /></td>
      <td className="w-[76px] px-5 py-3.5 text-center"><div className="mx-auto h-10 w-10 animate-pulse rounded-btn bg-surface-3/60" /></td>
      <td className="w-[76px] px-5 py-3.5 text-center"><div className="mx-auto h-10 w-10 animate-pulse rounded-btn bg-surface-3/60" /></td>
      <td className="w-[84px] px-5 py-3.5 text-center"><div className="mx-auto h-10 w-10 animate-pulse rounded-btn bg-surface-3/60" /></td>
    </tr>
  );
}

function SortIcon({ field, sort }: { field: SortField; sort: SortState | null }) {
  if (!sort || sort.field !== field) return <ArrowUpDown size={12} strokeWidth={1.4} className="text-txt-muted/50" />;
  return sort.dir === "asc"
    ? <ChevronUp size={12} strokeWidth={2} className="text-accent" />
    : <ChevronDown size={12} strokeWidth={2} className="text-accent" />;
}

function GradientProgress({ ratio, isHigh }: { ratio: number; isHigh: boolean }) {
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full border border-border/70 bg-border/70">
      <div
        className={cn(
          "h-full rounded-full",
          isHigh
            ? "bg-gradient-to-r from-status-warning to-status-danger"
            : "bg-gradient-to-r from-accent to-accent-light",
        )}
        style={{ width: `${ratio}%` }}
      />
    </div>
  );
}

function initials(value: string): string {
  const clean = value.trim();
  return clean ? clean.slice(0, 1).toUpperCase() : "?";
}

function resolveStatus(client: HysteriaClient): "online" | "offline" | "disabled" {
  if (!client.enabled) {
    return "disabled";
  }
  if (client.online_count > 0) {
    return "online";
  }
  return "offline";
}

function MobileActions({
  client,
  onArtifacts,
  onEdit,
  onDelete,
}: {
  client: HysteriaClient;
  onArtifacts: (c: HysteriaClient) => void;
  onEdit: (c: HysteriaClient) => void;
  onDelete: (c: HysteriaClient) => void;
}) {
  return (
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
          className="z-50 min-w-[160px] rounded-[10px] bg-surface-2/95 p-1 shadow-[0_18px_42px_-24px_var(--dialog-shadow)] backdrop-blur-xl"
        >
          <DropdownMenu.Item
            onSelect={() => void onArtifacts(client)}
            className="flex cursor-pointer items-center gap-2 rounded-[7px] px-2.5 py-2 text-[12px] text-txt outline-none transition-colors hover:bg-surface-3/60"
          >
            <QrCode size={15} strokeWidth={1.4} />
            Show QR
          </DropdownMenu.Item>
          <DropdownMenu.Item
            onSelect={() => onEdit(client)}
            className="flex cursor-pointer items-center gap-2 rounded-[7px] px-2.5 py-2 text-[12px] text-txt outline-none transition-colors hover:bg-surface-3/60"
          >
            <Pencil size={15} strokeWidth={1.4} />
            Edit
          </DropdownMenu.Item>
          <DropdownMenu.Item
            onSelect={() => onDelete(client)}
            className="flex cursor-pointer items-center gap-2 rounded-[7px] px-2.5 py-2 text-[12px] text-status-danger outline-none transition-colors hover:bg-status-danger/8"
          >
            <Trash2 size={15} strokeWidth={1.4} />
            Delete
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
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
  const [sort, setSort] = useState<SortState | null>(null);

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState("");
  const [editingClient, setEditingClient] = useState<HysteriaClient | null>(null);

  const [artifactOpen, setArtifactOpen] = useState(false);
  const [artifactLoading, setArtifactLoading] = useState(false);
  const [artifactClient, setArtifactClient] = useState<HysteriaClient | null>(null);
  const [artifactPayload, setArtifactPayload] = useState<HysteriaUserPayload | null>(null);

  const toast = useToast();

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
    setPage(0);
  }, [searchQuery, filter]);

  useEffect(() => {
    const existing = new Set(clients.map((client) => client.id));
    setSelectedClientIDs((current) => current.filter((id) => existing.has(id)));
  }, [clients]);

  const filteredClients = useMemo(() => {
    const needle = searchQuery.trim().toLowerCase();
    const filtered = clients.filter((client) => {
      if (filter === "online" && client.online_count <= 0) return false;
      if (filter === "enabled" && !client.enabled) return false;
      if (filter === "disabled" && client.enabled) return false;
      if (!needle) return true;
      const haystack = [client.username, client.username_normalized, client.note || "", client.id].join(" ").toLowerCase();
      return haystack.includes(needle);
    });

    const sorted = [...filtered];
    if (sort) {
      const dir = sort.dir === "asc" ? 1 : -1;
      sorted.sort((a, b) => {
        switch (sort.field) {
          case "username":
            return dir * a.username.localeCompare(b.username, undefined, { sensitivity: "base" });
          case "traffic":
            return dir * ((a.last_tx_bytes + a.last_rx_bytes) - (b.last_tx_bytes + b.last_rx_bytes));
          case "last_seen": {
            const ta = new Date(a.last_seen_at || a.updated_at).getTime();
            const tb = new Date(b.last_seen_at || b.updated_at).getTime();
            return dir * (ta - tb);
          }
          default:
            return 0;
        }
      });
    } else {
      sorted.sort((a, b) => a.username.localeCompare(b.username, undefined, { sensitivity: "base" }));
    }
    return sorted;
  }, [clients, filter, searchQuery, sort]);

  function toggleSort(field: SortField) {
    setSort((prev) => {
      if (prev?.field === field) {
        return prev.dir === "asc" ? { field, dir: "desc" } : null;
      }
      return { field, dir: "asc" };
    });
  }

  const selectedSet = useMemo(() => new Set(selectedClientIDs), [selectedClientIDs]);
  const filteredIDs = useMemo(() => filteredClients.map((client) => client.id), [filteredClients]);
  const selectedFilteredCount = useMemo(
    () => filteredIDs.reduce((sum, id) => sum + (selectedSet.has(id) ? 1 : 0), 0),
    [filteredIDs, selectedSet],
  );

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

  function escapeCSV(value: string): string {
    const escaped = value.replace(/"/g, '""');
    if (/^[=+\-@\t\r]/.test(escaped)) {
      return `"'${escaped}"`;
    }
    return `"${escaped}"`;
  }

  function exportCSV() {
    const header = "username,enabled,status,traffic_bytes,download_bps,upload_bps,last_seen,note";
    const rows = filteredClients.map((c) => {
      const status = resolveStatus(c);
      const traffic = c.last_tx_bytes + c.last_rx_bytes;
      return `${escapeCSV(c.username)},${c.enabled},${status},${traffic},${c.download_bps || 0},${c.upload_bps || 0},${escapeCSV(c.last_seen_at || c.updated_at)},${escapeCSV(c.note || "")}`;
    });
    const blob = new Blob([header + "\n" + rows.join("\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `users-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
    toast.notify("CSV exported");
  }

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
        toast.notify("User created");
      } else if (editingClient) {
        await updateClient(editingClient.id, toUpdateRequest(values));
        toast.notify("User updated");
      }
      setFormOpen(false);
      await load();
    } catch (err) {
      setFormError(err instanceof APIError ? err.message : "Failed to save user");
    } finally {
      setFormBusy(false);
    }
  }

  async function removeClient(clientID: string) {
    try {
      await deleteClient(clientID);
      toast.notify("User deleted");
      await load();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to delete user");
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
        toast.notify(deletedCount === 1 ? "1 user deleted" : `${deletedCount} users deleted`);
      }

      if (failedIDs.length > 0) {
        setSelectedClientIDs(failedIDs);
        setError(firstError || `Deleted ${deletedCount} of ${targetIDs.length} users`);
      } else {
        setSelectedClientIDs([]);
      }
    } finally {
      await load();
    }
  }

  async function toggleEnabled(client: HysteriaClient) {
    const prev = clients;
    setClients((list) => list.map((c) => c.id === client.id ? { ...c, enabled: !c.enabled } : c));
    try {
      await setClientEnabled(client.id, !client.enabled);
    } catch (err) {
      setClients(prev);
      setError(err instanceof APIError ? err.message : "Failed to change state");
    }
  }

  async function bulkSetEnabled(enabled: boolean) {
    if (!selectedClientIDs.length) return;
    const targetIDs = [...selectedClientIDs];
    const prev = clients;
    setClients((list) => list.map((c) => targetIDs.includes(c.id) ? { ...c, enabled } : c));
    let failCount = 0;
    for (const id of targetIDs) {
      try {
        await setClientEnabled(id, enabled);
      } catch {
        failCount++;
      }
    }
    if (failCount > 0) {
      setClients(prev);
      toast.notify(`Failed to update ${failCount} users`, "error");
      await load();
    } else {
      toast.notify(`${targetIDs.length} users ${enabled ? "enabled" : "disabled"}`);
    }
  }

  async function openArtifacts(client: HysteriaClient) {
    setArtifactClient(client);
    setArtifactOpen(true);
    setArtifactLoading(true);
    try {
      setArtifactPayload(await getClientArtifacts(client.id));
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
      toast.notify("Copied");
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

  function handleRowsPerPageChange(value: string) {
    const parsed = Number.parseInt(value, 10);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return;
    }
    setRowsPerPage(parsed);
    setPage(0);
  }

  const pageCount = Math.max(1, Math.ceil(filteredClients.length / rowsPerPage));
  const hasSelectedClients = selectedClientIDs.length > 0;

  useEffect(() => {
    const timer = setInterval(() => {
      if (hasSelectedClients) {
        return;
      }
      void load();
    }, 5000);
    return () => clearInterval(timer);
  }, [hasSelectedClients, load]);

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
            <Button onClick={exportCSV} disabled={!filteredClients.length} className="h-12 w-full rounded-2xl px-5 sm:w-auto">
              <Download size={18} strokeWidth={1.6} />
              Export CSV
            </Button>
            <div className="relative w-full sm:w-[300px] lg:w-[340px]">
              <Search size={16} strokeWidth={1.6} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-tertiary" />
              <Input
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="Search users..."
                className="h-12 rounded-2xl border-border/80 bg-surface-2/70 pl-11 shadow-[inset_0_1px_0_var(--shell-highlight)]"
              />
            </div>
            <div className="flex w-full items-center gap-1 rounded-2xl bg-surface-2/70 p-1 shadow-[inset_0_1px_0_var(--shell-highlight)] sm:w-auto">
              {(["all", "online", "enabled", "disabled"] as ClientFilter[]).map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setFilter(item)}
                  className={cn(
                    "flex-1 rounded-xl px-3 py-2 text-center text-[13px] font-semibold capitalize transition-colors sm:flex-none sm:px-4",
                    filter === item ? "bg-surface-4 text-txt-primary" : "text-txt-secondary hover:text-txt-primary",
                  )}
                >
                  {item}
                </button>
              ))}
            </div>
          </>
        }
      />

      <div className="flex items-center justify-between gap-3 text-[13px] text-txt-secondary">
        <span>{filteredClients.length} users</span>
        <div className="hidden items-center gap-2 sm:flex">
          <span>Rows:</span>
          <select
            value={rowsPerPage}
            onChange={(event) => handleRowsPerPageChange(event.target.value)}
            className="rounded-lg bg-surface-1 px-3 py-1.5 text-[13px] text-txt outline-none shadow-[inset_0_0_0_1px_var(--control-border)]"
          >
            {rowsPerPageOptions.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </div>
      </div>

      {hasSelectedClients ? (
        <div className="fixed bottom-6 left-1/2 z-40 -translate-x-1/2">
          <div className="flex items-center gap-2 rounded-2xl bg-surface-2/95 px-4 py-2.5 shadow-[0_20px_46px_-12px_var(--dialog-shadow)] backdrop-blur-xl">
            <span className="mr-1 inline-flex h-7 min-w-[28px] items-center justify-center rounded-lg bg-accent/15 px-2 text-[13px] font-bold tabular-nums text-accent">
              {selectedClientIDs.length}
            </span>
            <span className="mr-2 text-[13px] font-medium text-txt-secondary">selected</span>

            <div className="h-5 w-px bg-border/50" />

            <button
              type="button"
              onClick={() => void bulkSetEnabled(true)}
              className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-[12px] font-semibold text-status-success transition-colors hover:bg-status-success/10"
            >
              <Power size={14} strokeWidth={1.8} />
              Enable
            </button>
            <button
              type="button"
              onClick={() => void bulkSetEnabled(false)}
              className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-[12px] font-semibold text-status-warning transition-colors hover:bg-status-warning/10"
            >
              <PowerOff size={14} strokeWidth={1.8} />
              Disable
            </button>
            <ConfirmPopover
              title="Delete selected users"
              description={`Delete ${selectedClientIDs.length} users?`}
              confirmText="Delete"
              onConfirm={() => void deleteSelectedClients()}
            >
              <button
                type="button"
                className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-[12px] font-semibold text-status-danger transition-colors hover:bg-status-danger/10"
              >
                <Trash2 size={14} strokeWidth={1.8} />
                Delete
              </button>
            </ConfirmPopover>

            <div className="h-5 w-px bg-border/50" />

            <button
              type="button"
              onClick={() => setSelectedClientIDs([])}
              className="inline-flex items-center justify-center rounded-lg p-1.5 text-txt-muted transition-colors hover:bg-surface-3 hover:text-txt"
            >
              <X size={14} strokeWidth={1.8} />
            </button>
          </div>
        </div>
      ) : null}

      {error && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}

      {/* ── Desktop table ── */}
      <TableContainer className="hidden overflow-x-auto sm:block">
        {loading ? (
          <Table>
            <TableHeader>
              <TableRow className="border-t-0 hover:bg-transparent">
                <TableHead className="w-10"><div className="h-4 w-4 animate-pulse rounded bg-surface-3/60" /></TableHead>
                <TableHead className="hidden w-14 md:table-cell">#</TableHead>
                <TableHead>USERS</TableHead>
                <TableHead className="hidden lg:table-cell">PROTOCOL</TableHead>
                <TableHead>STATUS</TableHead>
                <TableHead className="hidden lg:table-cell">TRAFFIC</TableHead>
                <TableHead className="hidden text-right lg:table-cell">NETWORK</TableHead>
                <TableHead className="hidden text-right md:table-cell">LAST SEEN</TableHead>
                <TableHead className="w-[76px] text-center" />
                <TableHead className="w-[76px] text-center" />
                <TableHead className="w-[84px] text-center" />
              </TableRow>
            </TableHeader>
            <tbody>
              {Array.from({ length: SKELETON_ROWS }, (_, i) => <SkeletonRow key={i} />)}
            </tbody>
          </Table>
        ) : (
          <>
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
                  <TableHead>
                    <button type="button" onClick={() => toggleSort("username")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                      USERS <SortIcon field="username" sort={sort} />
                    </button>
                  </TableHead>
                  <TableHead className="hidden lg:table-cell">PROTOCOL</TableHead>
                  <TableHead>STATUS</TableHead>
                  <TableHead className="hidden lg:table-cell">
                    <button type="button" onClick={() => toggleSort("traffic")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                      TRAFFIC <SortIcon field="traffic" sort={sort} />
                    </button>
                  </TableHead>
                  <TableHead className="hidden text-right lg:table-cell">
                    NETWORK
                  </TableHead>
                  <TableHead className="hidden text-right md:table-cell">
                    <button type="button" onClick={() => toggleSort("last_seen")} className="ml-auto inline-flex items-center gap-1.5 hover:text-txt-primary">
                      LAST SEEN <SortIcon field="last_seen" sort={sort} />
                    </button>
                  </TableHead>
                  <TableHead className="w-[76px] text-center" />
                  <TableHead className="w-[76px] text-center" />
                  <TableHead className="w-[84px] text-center" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {pagedClients.length ? (
                  pagedClients.map((client, index) => {
                    const traffic = client.last_tx_bytes + client.last_rx_bytes;
                    const ratio = maxTraffic > 0 ? Math.min(100, (traffic / maxTraffic) * 100) : 0;
                    const ratioWidth = traffic > 0 ? Math.max(ratio, 4) : 0;
                    const status = resolveStatus(client);
                    const statusOnline = status === "online";
                    const downBps = Math.max(0, client.download_bps || 0);
                    const upBps = Math.max(0, client.upload_bps || 0);

                    return (
                      <TableRow key={client.id}>
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
                              className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-accent/15 to-accent-secondary/10 text-[13px] font-bold text-txt-primary"
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
                          <div className="flex min-w-[154px] items-center gap-3">
                            <span className="inline-flex w-[76px] items-center gap-2">
                              <span
                                className={cn(
                                  "h-[6px] w-[6px] rounded-full",
                                  statusOnline && "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]",
                                  !statusOnline && status !== "disabled" && "bg-status-warning",
                                  status === "disabled" && "bg-txt-muted",
                                )}
                              />
                              <span className="w-[62px] text-[11px] text-txt-secondary">{status}</span>
                            </span>
                            <Toggle className="shrink-0" checked={client.enabled} onCheckedChange={() => void toggleEnabled(client)} />
                          </div>
                        </TableCell>
                        <TableCell className="hidden lg:table-cell">
                          <div className="space-y-1.5">
                            <GradientProgress ratio={ratioWidth} isHigh={ratio > 90} />
                            <p className="text-[11px] font-medium text-txt-tertiary">{formatBytes(traffic)}</p>
                          </div>
                        </TableCell>
                        <TableCell className="hidden text-right lg:table-cell">
                          <div className="flex flex-col items-end gap-1 text-[11px] font-semibold tabular-nums text-txt-secondary">
                            <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
                              <ArrowDownToLine size={12} strokeWidth={1.8} className="text-status-success" />
                              {formatRate(downBps)}
                            </span>
                            <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
                              <ArrowUpFromLine size={12} strokeWidth={1.8} className="text-status-warning" />
                              {formatRate(upBps)}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="hidden whitespace-nowrap text-right md:table-cell">
                          {formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}
                        </TableCell>
                        <TableCell className="text-center">
                          <button
                            type="button"
                            onClick={() => void openArtifacts(client)}
                            title="QR"
                            aria-label={`show qr for ${client.username}`}
                            className="inline-flex h-10 w-10 items-center justify-center rounded-xl text-txt-tertiary transition-colors hover:bg-surface-3 hover:text-txt"
                          >
                            <QrCode size={18} strokeWidth={1.7} />
                          </button>
                        </TableCell>
                        <TableCell className="text-center">
                          <button
                            type="button"
                            onClick={() => openEdit(client)}
                            title="Edit"
                            aria-label={`edit ${client.username}`}
                            className="inline-flex h-10 w-10 items-center justify-center rounded-xl text-txt-tertiary transition-colors hover:bg-surface-3 hover:text-txt"
                          >
                            <Pencil size={18} strokeWidth={1.7} />
                          </button>
                        </TableCell>
                        <TableCell className="text-center">
                          <ConfirmPopover
                            title="Delete user"
                            description={`Remove ${client.username} and revoke access?`}
                            confirmText="Delete"
                            onConfirm={() => void removeClient(client.id)}
                          >
                            <button
                              type="button"
                              title="Delete"
                              aria-label={`delete ${client.username}`}
                              className="inline-flex h-10 w-10 items-center justify-center rounded-xl text-txt-tertiary transition-colors hover:bg-status-danger/10 hover:text-status-danger"
                            >
                              <Trash2 size={18} strokeWidth={1.7} />
                            </button>
                          </ConfirmPopover>
                        </TableCell>
                      </TableRow>
                    );
                  })
                ) : (
                  <TableRow>
                    <TableCell colSpan={11}>{clients.length ? "No users match the current filters." : "No users yet."}</TableCell>
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

      {/* ── Mobile card layout ── */}
      <div className="space-y-3 sm:hidden">
        {loading ? (
          Array.from({ length: 4 }, (_, i) => (
            <div key={i} className="animate-pulse rounded-2xl bg-surface-2 p-4 space-y-3">
              <div className="flex items-center gap-3">
                <div className="h-10 w-10 rounded-xl bg-surface-3/60" />
                <div className="flex-1 space-y-1.5">
                  <div className="h-4 w-28 rounded bg-surface-3/60" />
                  <div className="h-3 w-16 rounded bg-surface-3/50" />
                </div>
                <div className="h-6 w-14 rounded-badge bg-surface-3/60" />
              </div>
              <div className="h-1.5 rounded-full bg-surface-3/60" />
              <div className="flex justify-between">
                <div className="h-3 w-20 rounded bg-surface-3/50" />
                <div className="h-3 w-20 rounded bg-surface-3/50" />
              </div>
            </div>
          ))
        ) : pagedClients.length ? (
          pagedClients.map((client) => {
            const traffic = client.last_tx_bytes + client.last_rx_bytes;
            const ratio = maxTraffic > 0 ? Math.min(100, (traffic / maxTraffic) * 100) : 0;
            const ratioWidth = traffic > 0 ? Math.max(ratio, 4) : 0;
            const status = resolveStatus(client);
            const statusOnline = status === "online";
            const downBps = Math.max(0, client.download_bps || 0);
            const upBps = Math.max(0, client.upload_bps || 0);

            return (
              <div key={client.id} className="card-hover rounded-2xl bg-surface-2 p-4">
                {/* Header */}
                <div className="flex items-center gap-3">
                  <Checkbox
                    checked={selectedSet.has(client.id)}
                    onCheckedChange={(value) => toggleClientSelection(client.id, value === true)}
                    aria-label={`select ${client.username}`}
                  />
                  <button
                    type="button"
                    onClick={() => void openArtifacts(client)}
                    className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-accent/15 to-accent-secondary/10 text-[14px] font-bold text-txt-primary"
                  >
                    {initials(client.username)}
                  </button>
                  <div className="min-w-0 flex-1">
                    <button
                      type="button"
                      onClick={() => void openArtifacts(client)}
                      className="block max-w-full truncate text-[14px] font-medium text-txt hover:text-txt-primary"
                    >
                      {client.username}
                    </button>
                    <p className="truncate text-[12px] text-txt-muted">{client.note || "-"}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="inline-flex items-center gap-1.5 rounded-lg bg-surface-3/40 px-2 py-1 text-[11px] font-medium text-txt-secondary">
                      <span
                        className={cn(
                          "h-[6px] w-[6px] rounded-full",
                          statusOnline && "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]",
                          !statusOnline && status !== "disabled" && "bg-status-warning",
                          status === "disabled" && "bg-txt-muted",
                        )}
                      />
                      {status}
                    </span>
                    <MobileActions client={client} onArtifacts={openArtifacts} onEdit={openEdit} onDelete={(c) => void removeClient(c.id)} />
                  </div>
                </div>

                {/* Traffic bar */}
                <div className="mt-3 space-y-1.5">
                  <GradientProgress ratio={ratioWidth} isHigh={ratio > 90} />
                  <div className="flex items-center justify-between text-[11px] font-medium text-txt-tertiary">
                    <span>{formatBytes(traffic)}</span>
                    <div className="flex items-center gap-3">
                      <span className="inline-flex items-center gap-1">
                        <ArrowDownToLine size={10} strokeWidth={1.8} className="text-status-success" />
                        {formatRate(downBps)}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <ArrowUpFromLine size={10} strokeWidth={1.8} className="text-status-warning" />
                        {formatRate(upBps)}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Footer */}
                <div className="mt-2.5 flex items-center justify-between border-t border-border/20 pt-2.5">
                  <Toggle className="shrink-0" checked={client.enabled} onCheckedChange={() => void toggleEnabled(client)} />
                  <span className="text-[11px] text-txt-muted">
                    {formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}
                  </span>
                </div>
              </div>
            );
          })
        ) : (
          <div className="rounded-2xl bg-surface-2 p-6 text-center text-[14px] text-txt-secondary">
            {clients.length ? "No users match the current filters." : "No users yet."}
          </div>
        )}

        {/* Mobile pagination */}
        {!loading && pagedClients.length > 0 && (
          <div className="flex items-center justify-between pt-2">
            <p className="text-[13px] text-txt-secondary">
              {Math.min(page + 1, pageCount)}/{pageCount}
            </p>
            <div className="flex items-center gap-2">
              <Button size="sm" disabled={page <= 0} onClick={() => setPage((v) => Math.max(0, v - 1))}>Prev</Button>
              <Button size="sm" disabled={page + 1 >= pageCount} onClick={() => setPage((v) => Math.min(pageCount - 1, v + 1))}>Next</Button>
            </div>
          </div>
        )}
      </div>

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

      <ClientArtifactsDialog
        open={artifactOpen}
        client={artifactClient}
        payload={artifactPayload}
        loading={artifactLoading}
        onClose={() => setArtifactOpen(false)}
        onCopy={(value) => void copy(value)}
      />

    </div>
  );
}
