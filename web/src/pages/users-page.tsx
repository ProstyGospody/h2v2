import {
  type ColumnDef,
  type ColumnFiltersState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  type Row,
  type RowSelectionState,
  type SortingState,
  type Table as TanStackTable,
  useReactTable,
} from "@tanstack/react-table";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowDownToLine,
  ArrowUpDown,
  ArrowUpFromLine,
  ChevronDown,
  ChevronUp,
  Download,
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
import { toCreateRequest, toUpdateRequest, type ClientFormValues } from "@/domain/clients/adapters";
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
import { type Client, type UserPayload } from "@/domain/clients/types";
import { APIError, getAPIErrorMessage } from "@/services/api";
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
  Tooltip,
  cn,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { queryRefetchInterval } from "@/src/queries/polling";
import { formatBytes, formatDateTime, formatRate } from "@/utils/format";

type StatusFilter = "all" | "online" | "enabled" | "disabled";
type UsersColumnMeta = {
  headClassName?: string;
  cellClassName?: string;
};

const SEARCH_DEBOUNCE_MS = 250;
const ROWS_PER_PAGE = 25;
const EMPTY_CLIENTS: Client[] = [];
const SKELETON_ROWS = 8;
const STICKY_HEAD_CLASS = "sticky top-0 z-10 bg-surface-2/96";
const USERS_TABLE_MIN_WIDTH = "1080px";
const FILTER_OPTIONS: StatusFilter[] = ["all", "online", "enabled", "disabled"];
const HEADER_SECONDARY_BTN = "header-btn w-full rounded-2xl px-5 sm:w-auto border-border/80 bg-surface-2/70 shadow-[inset_0_1px_0_var(--shell-highlight)] hover:bg-surface-3/60";

function asText(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function initials(value: string): string {
  const clean = asText(value).trim();
  return clean ? clean.slice(0, 1).toUpperCase() : "?";
}

function resolveStatus(client: Client): "online" | "offline" | "disabled" {
  if (!client.enabled) {
    return "disabled";
  }
  if (client.online_count > 0) {
    return "online";
  }
  return "offline";
}

function statusBadgeVariant(status: ReturnType<typeof resolveStatus>): "default" | "success" | "warning" {
  if (status === "online") return "success";
  if (status === "offline") return "warning";
  return "default";
}

function protocolBadgeVariant(protocol: string): "protocol-hy2" | "protocol-vless" {
  return protocol === "vless" ? "protocol-vless" : "protocol-hy2";
}

function escapeCSV(value: string): string {
  const escaped = value.replace(/"/g, '""');
  if (/^[=+\-@\t\r]/.test(escaped)) {
    return `"'${escaped}"`;
  }
  return `"${escaped}"`;
}

function selectedDeleteDescription(selectedClients: Client[], selectedCount: number): string {
  const names = selectedClients.map((client) => client.username);
  if (names.length === 0) {
    return `Delete ${selectedCount} users?`;
  }
  if (names.length <= 3) {
    return `Delete ${names.join(", ")}?`;
  }
  return `Delete ${names.slice(0, 3).join(", ")} and ${names.length - 3} more?`;
}

function resolveBulkStateErrorMessage(error: APIError): string {
  if (!error.details || typeof error.details !== "object") {
    return error.message;
  }
  const details = error.details as {
    sync_error?: unknown;
    rollback_error?: unknown;
    rollback_sync_error?: unknown;
    sync_status?: unknown;
    rollback_sync_status?: unknown;
  };
  const parts = [details.sync_error, details.rollback_error, details.rollback_sync_error]
    .filter((value): value is string => typeof value === "string" && value.trim().length > 0)
    .map((value) => value.trim());
  const status = [details.sync_status, details.rollback_sync_status]
    .filter((value): value is string => typeof value === "string" && value.trim().length > 0)
    .map((value) => value.trim());
  if (parts.length === 0) {
    if (status.length === 0) {
      return error.message;
    }
    return `${error.message}: ${status.join(" | ")}`;
  }
  if (status.length === 0) {
    return `${error.message}: ${parts.join(" | ")}`;
  }
  return `${error.message}: ${parts.join(" | ")} | ${status.join(" | ")}`;
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tagName = target.tagName.toLowerCase();
  return tagName === "input" || tagName === "textarea" || tagName === "select";
}

function toAriaSort(sorted: false | "asc" | "desc"): "none" | "ascending" | "descending" {
  if (!sorted) {
    return "none";
  }
  return sorted === "asc" ? "ascending" : "descending";
}

function applyFilteredSelection(current: RowSelectionState, rows: Row<Client>[], checked: boolean): RowSelectionState {
  const next: RowSelectionState = { ...current };
  for (const row of rows) {
    if (checked) {
      next[row.id] = true;
    } else {
      delete next[row.id];
    }
  }
  return next;
}

function resolveFilteredSelection(table: TanStackTable<Client>): { all: boolean; some: boolean } {
  const filteredCount = table.getFilteredRowModel().rows.length;
  const selectedFilteredCount = table.getFilteredSelectedRowModel().rows.length;
  const all = filteredCount > 0 && selectedFilteredCount === filteredCount;
  const some = selectedFilteredCount > 0 && !all;
  return { all, some };
}

function SortIcon({ sorted }: { sorted: false | "asc" | "desc" }) {
  if (!sorted) return <ArrowUpDown size={12} strokeWidth={1.5} className="text-txt-muted/60" />;
  return sorted === "asc"
    ? <ChevronUp size={13} strokeWidth={2} className="text-accent" />
    : <ChevronDown size={13} strokeWidth={2} className="text-accent" />;
}

function TrafficMeter({ value, maxValue }: { value: number; maxValue: number }) {
  const rawRatio = maxValue > 0 ? (value / maxValue) * 100 : 0;
  const ratio = value > 0 ? Math.max(4, Math.min(100, rawRatio)) : 0;

  return (
    <div className="space-y-1.5">
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface-3/45">
        <div className="h-full rounded-full bg-gradient-to-r from-accent to-accent-secondary" style={{ width: `${ratio}%` }} />
      </div>
      <p className="text-[12px] font-medium text-txt-secondary">{formatBytes(value)}</p>
    </div>
  );
}

function UserProtocols({ client }: { client: Client }) {
  const raw = Array.isArray(client.protocols) && client.protocols.length ? client.protocols : [client.preferred_protocol || "hy2"];
  const protocols = Array.from(new Set(raw));
  return (
    <div className="mt-1.5 flex flex-wrap gap-1">
      {protocols.map((protocol) => (
        <Badge key={protocol} variant={protocolBadgeVariant(protocol)} className="px-1.5 py-0 text-[9px]">
          {protocol}
        </Badge>
      ))}
    </div>
  );
}

function UserActions({
  client,
  onOpenArtifacts,
  onOpenEdit,
  onRemoveClient,
  compact = false,
}: {
  client: Client;
  onOpenArtifacts: (client: Client) => void;
  onOpenEdit: (client: Client) => void;
  onRemoveClient: (clientID: string) => void;
  compact?: boolean;
}) {
  const qrButton = (
    <button
      type="button"
      onClick={() => onOpenArtifacts(client)}
      aria-label={`show qr for ${client.username}`}
      className={cn(
        "inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/55 hover:text-txt",
        compact && "h-10 w-10",
      )}
    >
      <QrCode size={16} strokeWidth={1.8} />
    </button>
  );
  const editButton = (
    <button
      type="button"
      onClick={() => onOpenEdit(client)}
      aria-label={`edit ${client.username}`}
      className={cn(
        "inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/55 hover:text-txt",
        compact && "h-10 w-10",
      )}
    >
      <Pencil size={16} strokeWidth={1.8} />
    </button>
  );
  const deleteButton = (
    <button
      type="button"
      aria-label={`delete ${client.username}`}
      className={cn(
        "inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-status-danger/10 hover:text-status-danger",
        compact && "h-10 w-10",
      )}
    >
      <Trash2 size={16} strokeWidth={1.8} />
    </button>
  );

  if (compact) {
    return (
      <div className="flex items-center justify-end gap-1">
        {qrButton}
        {editButton}
        <ConfirmPopover
          title="Delete user"
          description={`Remove ${client.username}?`}
          confirmText="Delete"
          onConfirm={() => onRemoveClient(client.id)}
        >
          {deleteButton}
        </ConfirmPopover>
      </div>
    );
  }

  return (
    <div className="flex items-center justify-end gap-1">
      <Tooltip content="QR">{qrButton}</Tooltip>
      <Tooltip content="Edit">{editButton}</Tooltip>
      <ConfirmPopover
        title="Delete user"
        description={`Remove ${client.username}?`}
        confirmText="Delete"
        onConfirm={() => onRemoveClient(client.id)}
      >
        {deleteButton}
      </ConfirmPopover>
    </div>
  );
}

function SkeletonRow() {
  return (
    <tr className="border-t border-border/30">
      <td className="w-10 px-4 py-3.5"><div className="h-4 w-4 animate-pulse rounded bg-surface-3/60" /></td>
      <td className="px-4 py-3.5"><div className="h-10 w-full animate-pulse rounded-xl bg-surface-3/55" /></td>
      <td className="px-4 py-3.5"><div className="h-7 w-[132px] animate-pulse rounded-xl bg-surface-3/55" /></td>
      <td className="px-4 py-3.5"><div className="h-7 w-[148px] animate-pulse rounded-xl bg-surface-3/55" /></td>
      <td className="px-4 py-3.5"><div className="h-7 w-[124px] animate-pulse rounded-xl bg-surface-3/55" /></td>
      <td className="px-4 py-3.5"><div className="h-7 w-[128px] animate-pulse rounded-xl bg-surface-3/55" /></td>
      <td className="px-4 py-3.5"><div className="ml-auto h-9 w-[132px] animate-pulse rounded-xl bg-surface-3/55" /></td>
    </tr>
  );
}

function MobileSkeletonCard() {
  return (
    <div className="space-y-3 rounded-xl bg-surface-0/45 p-3.5 animate-pulse">
      <div className="flex items-center gap-2.5">
        <div className="h-4 w-4 rounded bg-surface-3/60" />
        <div className="h-9 w-9 rounded-xl bg-surface-3/60" />
        <div className="flex-1 space-y-2">
          <div className="h-3.5 w-24 rounded bg-surface-3/60" />
          <div className="h-3 w-32 rounded bg-surface-3/50" />
        </div>
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        <div className="h-16 rounded-lg bg-surface-3/50" />
        <div className="h-16 rounded-lg bg-surface-3/50" />
        <div className="h-16 rounded-lg bg-surface-3/50" />
        <div className="h-16 rounded-lg bg-surface-3/50" />
      </div>
      <div className="h-10 rounded-lg bg-surface-3/50" />
    </div>
  );
}

function UserCard({
  client,
  selected,
  maxTraffic,
  onToggleSelection,
  onOpenArtifacts,
  onOpenEdit,
  onRemoveClient,
  onToggleEnabled,
}: {
  client: Client;
  selected: boolean;
  maxTraffic: number;
  onToggleSelection: (clientID: string, checked: boolean) => void;
  onOpenArtifacts: (client: Client) => void;
  onOpenEdit: (client: Client) => void;
  onRemoveClient: (clientID: string) => void;
  onToggleEnabled: (client: Client) => void;
}) {
  const status = resolveStatus(client);
  const traffic = client.last_tx_bytes + client.last_rx_bytes;
  const downBps = Math.max(0, client.download_bps || 0);
  const upBps = Math.max(0, client.upload_bps || 0);

  return (
    <article className="rounded-xl bg-surface-0/45 p-3.5">
      <div className="flex items-start gap-2.5">
        <div className="pt-1">
          <Checkbox
            checked={selected}
            onCheckedChange={(value) => onToggleSelection(client.id, value === true)}
            aria-label={`select ${client.username}`}
          />
        </div>
        <button
          type="button"
          onClick={() => onOpenArtifacts(client)}
          className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-gradient-to-br from-accent/16 to-accent-secondary/12 text-[13px] font-bold text-txt-primary"
        >
          {initials(client.username)}
        </button>
        <div className="min-w-0 flex-1">
          <button
            type="button"
            onClick={() => onOpenArtifacts(client)}
            className="max-w-full truncate text-left text-[13px] font-semibold text-txt-primary hover:text-txt"
          >
            {client.username}
          </button>
          <UserProtocols client={client} />
        </div>
      </div>

      <div className="mt-3 grid gap-2 sm:grid-cols-2">
        <div className="rounded-lg bg-surface-2/50 p-2.5">
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Status</p>
          <div className="mt-1.5 flex items-center justify-between gap-2">
            <Badge variant={statusBadgeVariant(status)} className="px-2 py-0.5 text-[10px]">{status}</Badge>
            <Toggle checked={client.enabled} onCheckedChange={() => void onToggleEnabled(client)} className="shrink-0" />
          </div>
        </div>

        <div className="rounded-lg bg-surface-2/50 p-2.5">
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Traffic</p>
          <div className="mt-1.5">
            <TrafficMeter value={traffic} maxValue={maxTraffic} />
          </div>
        </div>

        <div className="rounded-lg bg-surface-2/50 p-2.5">
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Rate</p>
          <div className="mt-1.5 space-y-1 text-[12px] font-medium tabular-nums text-txt-secondary">
            <p className="inline-flex items-center gap-1.5">
              <ArrowDownToLine size={12} strokeWidth={1.8} className="text-status-success" />
              {formatRate(downBps)}
            </p>
            <p className="inline-flex items-center gap-1.5">
              <ArrowUpFromLine size={12} strokeWidth={1.8} className="text-status-warning" />
              {formatRate(upBps)}
            </p>
          </div>
        </div>

        <div className="rounded-lg bg-surface-2/50 p-2.5">
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Seen</p>
          <p className="mt-1.5 text-[12px] text-txt-secondary">
            {formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}
          </p>
        </div>
      </div>

      <div className="mt-3 border-t border-border/35 pt-2">
        <UserActions
          compact
          client={client}
          onOpenArtifacts={onOpenArtifacts}
          onOpenEdit={onOpenEdit}
          onRemoveClient={onRemoveClient}
        />
      </div>
    </article>
  );
}

export default function UsersPage() {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [sorting, setSorting] = useState<SortingState>([{ id: "seen", desc: true }]);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: ROWS_PER_PAGE });
  const [actionError, setActionError] = useState("");
  const [dismissedQueryError, setDismissedQueryError] = useState(false);

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState("");
  const [editingClient, setEditingClient] = useState<Client | null>(null);

  const [artifactOpen, setArtifactOpen] = useState(false);
  const [artifactLoading, setArtifactLoading] = useState(false);
  const [artifactClient, setArtifactClient] = useState<Client | null>(null);
  const [artifactPayload, setArtifactPayload] = useState<UserPayload | null>(null);

  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const hasSelectedRef = useRef(false);

  const hasSelection = Object.values(rowSelection).some((value) => value === true);
  hasSelectedRef.current = hasSelection;

  const toast = useToast();
  const queryClient = useQueryClient();

  const usersQuery = useQuery({
    queryKey: ["users", "page-data"],
    queryFn: async () => {
      const clientsPayload = await listClients();
      return {
        clients: clientsPayload.items,
        limited: clientsPayload.limited,
      };
    },
    staleTime: 3_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(5_000, query, { enabled: !hasSelectedRef.current }),
  });

  const clients = usersQuery.data?.clients ?? EMPTY_CLIENTS;
  const limitWarning = usersQuery.data?.limited ?? false;
  const loading = usersQuery.isPending;
  const queryError = usersQuery.error ? getAPIErrorMessage(usersQuery.error, "Failed to load users") : "";
  const error = actionError || (dismissedQueryError ? "" : queryError);

  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    setPagination((current) => (current.pageIndex === 0 ? current : { ...current, pageIndex: 0 }));
  }, [searchQuery, statusFilter, sorting]);

  useEffect(() => {
    const existing = new Set(clients.map((client) => client.id));
    setRowSelection((current) => {
      let changed = false;
      const next: RowSelectionState = {};
      for (const [id, selected] of Object.entries(current)) {
        if (!selected) {
          changed = true;
          continue;
        }
        if (!existing.has(id)) {
          changed = true;
          continue;
        }
        next[id] = true;
      }
      if (!changed && Object.keys(current).length === Object.keys(next).length) {
        return current;
      }
      return next;
    });
  }, [clients]);

  useEffect(() => {
    if (usersQuery.isSuccess) {
      setDismissedQueryError(false);
    }
  }, [usersQuery.dataUpdatedAt, usersQuery.isSuccess]);

  const refreshUsers = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ["users", "page-data"] });
  }, [queryClient]);

  const openCreate = useCallback(() => {
    setFormMode("create");
    setEditingClient(null);
    setFormError("");
    setFormOpen(true);
  }, []);

  const openEdit = useCallback((client: Client) => {
    setFormMode("edit");
    setEditingClient(client);
    setFormError("");
    setFormOpen(true);
  }, []);

  const openArtifacts = useCallback(async (client: Client) => {
    setArtifactClient(client);
    setArtifactOpen(true);
    setArtifactLoading(true);
    try {
      setArtifactPayload(await getClientArtifacts(client.id));
    } catch (err) {
      setArtifactPayload(null);
      setActionError(err instanceof APIError ? err.message : "Failed to load artifacts");
    } finally {
      setArtifactLoading(false);
    }
  }, []);

  const copy = useCallback(async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.notify("Copied");
    } catch {
      setActionError("Clipboard write failed");
    }
  }, [toast]);

  const toggleEnabled = useCallback(async (client: Client) => {
    try {
      await setClientEnabled(client.id, !client.enabled);
      await refreshUsers();
    } catch (err) {
      const message = err instanceof APIError ? resolveBulkStateErrorMessage(err) : "Failed to change state";
      setActionError(message);
    }
  }, [refreshUsers]);

  const removeClient = useCallback(async (clientID: string) => {
    try {
      await deleteClient(clientID);
      toast.notify("User deleted");
      await refreshUsers();
    } catch (err) {
      setActionError(err instanceof APIError ? err.message : "Failed to delete user");
    }
  }, [refreshUsers, toast]);

  const userSearchFilter = useCallback((row: Row<Client>, _columnID: string, value: unknown) => {
    const needle = typeof value === "string" ? value.trim().toLowerCase() : "";
    if (!needle) {
      return true;
    }
    const client = row.original;
    const haystack = [asText(client.username), asText(client.username_normalized), asText(client.id)].join(" ").toLowerCase();
    return haystack.includes(needle);
  }, []);

  const userStatusFilter = useCallback((row: Row<Client>, _columnID: string, value: unknown) => {
    const filter = typeof value === "string" ? value : "all";
    if (filter === "all") {
      return true;
    }
    const client = row.original;
    if (filter === "online") {
      return client.online_count > 0;
    }
    if (filter === "enabled") {
      return client.enabled;
    }
    if (filter === "disabled") {
      return !client.enabled;
    }
    return true;
  }, []);

  const toggleSort = useCallback((columnID: "username" | "traffic" | "seen", initialDesc: boolean) => {
    setSorting((current) => {
      const active = current[0];
      if (!active || active.id !== columnID) {
        return [{ id: columnID, desc: initialDesc }];
      }
      return [{ id: columnID, desc: !active.desc }];
    });
  }, []);

  const maxTraffic = useMemo(() => {
    return clients.reduce((max, client) => Math.max(max, client.last_tx_bytes + client.last_rx_bytes), 0);
  }, [clients]);

  const columns = useMemo<ColumnDef<Client>[]>(() => [
    {
      id: "select",
      size: 48,
      enableSorting: false,
      meta: { cellClassName: "w-10" },
      header: ({ table }) => {
        const selection = resolveFilteredSelection(table);
        return (
          <Checkbox
            checked={selection.all ? true : selection.some ? "indeterminate" : false}
            onCheckedChange={(value) => {
              const checked = value === true;
              setRowSelection((current) => applyFilteredSelection(current, table.getFilteredRowModel().rows, checked));
            }}
            aria-label="select filtered users"
          />
        );
      },
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(value === true)}
          aria-label={`select ${row.original.username}`}
        />
      ),
    },
    {
      id: "username",
      accessorFn: (client) => client.username,
      size: 280,
      filterFn: userSearchFilter,
      sortingFn: (left, right) => asText(left.original.username).localeCompare(asText(right.original.username), undefined, { sensitivity: "base" }),
      header: ({ column }) => (
        <button type="button" onClick={() => toggleSort("username", false)} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
          User <SortIcon sorted={column.getIsSorted()} />
        </button>
      ),
      cell: ({ row }) => {
        const client = row.original;
        return (
          <div className="flex min-w-0 items-center gap-2.5">
            <button
              type="button"
              onClick={() => void openArtifacts(client)}
              className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-gradient-to-br from-accent/16 to-accent-secondary/12 text-[13px] font-bold text-txt-primary"
            >
              {initials(client.username)}
            </button>
            <div className="min-w-0 flex-1">
              <button
                type="button"
                onClick={() => void openArtifacts(client)}
                className="block w-full truncate text-left text-[13px] font-semibold text-txt-primary hover:text-txt"
              >
                {client.username}
              </button>
              <UserProtocols client={client} />
            </div>
          </div>
        );
      },
    },
    {
      id: "status",
      accessorFn: (client) => resolveStatus(client),
      size: 170,
      filterFn: userStatusFilter,
      enableSorting: false,
      header: "Status",
      cell: ({ row }) => {
        const client = row.original;
        const status = resolveStatus(client);
        return (
          <div className="flex items-center justify-between gap-2">
            <Badge variant={statusBadgeVariant(status)} className="min-w-[72px] justify-center px-2 py-0.5 text-[10px]">{status}</Badge>
            <Toggle checked={client.enabled} onCheckedChange={() => void toggleEnabled(client)} className="shrink-0" />
          </div>
        );
      },
    },
    {
      id: "traffic",
      accessorFn: (client) => client.last_tx_bytes + client.last_rx_bytes,
      size: 190,
      sortingFn: "basic",
      header: ({ column }) => (
        <button type="button" onClick={() => toggleSort("traffic", true)} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
          Traffic <SortIcon sorted={column.getIsSorted()} />
        </button>
      ),
      cell: ({ row }) => {
        const traffic = row.original.last_tx_bytes + row.original.last_rx_bytes;
        return (
          <div className="w-full min-w-0">
            <TrafficMeter value={traffic} maxValue={maxTraffic} />
          </div>
        );
      },
    },
    {
      id: "rate",
      accessorFn: (client) => Math.max(0, client.download_bps || 0) + Math.max(0, client.upload_bps || 0),
      size: 170,
      enableSorting: false,
      header: "Rate",
      cell: ({ row }) => {
        const downBps = Math.max(0, row.original.download_bps || 0);
        const upBps = Math.max(0, row.original.upload_bps || 0);
        return (
          <div className="space-y-1 text-[12px] font-medium tabular-nums text-txt-secondary">
            <p className="inline-flex items-center gap-1.5">
              <ArrowDownToLine size={12} strokeWidth={1.8} className="text-status-success" />
              {formatRate(downBps)}
            </p>
            <p className="inline-flex items-center gap-1.5">
              <ArrowUpFromLine size={12} strokeWidth={1.8} className="text-status-warning" />
              {formatRate(upBps)}
            </p>
          </div>
        );
      },
    },
    {
      id: "seen",
      accessorFn: (client) => client.last_seen_at || client.updated_at,
      size: 170,
      sortingFn: (left, right) => {
        const a = new Date(left.original.last_seen_at || left.original.updated_at).getTime();
        const b = new Date(right.original.last_seen_at || right.original.updated_at).getTime();
        return a - b;
      },
      header: ({ column }) => (
        <button type="button" onClick={() => toggleSort("seen", true)} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
          Seen <SortIcon sorted={column.getIsSorted()} />
        </button>
      ),
      cell: ({ row }) => (
        <div className="truncate whitespace-nowrap text-[12px] text-txt-secondary">
          {formatDateTime(row.original.last_seen_at || row.original.updated_at, { includeSeconds: false })}
        </div>
      ),
    },
    {
      id: "actions",
      size: 152,
      enableSorting: false,
      meta: { headClassName: "text-right", cellClassName: "text-right" },
      header: "Actions",
      cell: ({ row }) => (
        <UserActions
          client={row.original}
          onOpenArtifacts={(client) => void openArtifacts(client)}
          onOpenEdit={openEdit}
          onRemoveClient={(clientID) => void removeClient(clientID)}
        />
      ),
    },
  ], [maxTraffic, openArtifacts, openEdit, removeClient, toggleEnabled, toggleSort, userSearchFilter, userStatusFilter]);

  const columnFilters = useMemo<ColumnFiltersState>(() => {
    const filters: ColumnFiltersState = [];
    if (searchQuery.trim().length > 0) {
      filters.push({ id: "username", value: searchQuery.trim() });
    }
    if (statusFilter !== "all") {
      filters.push({ id: "status", value: statusFilter });
    }
    return filters;
  }, [searchQuery, statusFilter]);

  const table = useReactTable({
    data: clients,
    columns,
    state: {
      sorting,
      rowSelection,
      pagination,
      columnFilters,
    },
    onSortingChange: setSorting,
    onRowSelectionChange: setRowSelection,
    onPaginationChange: setPagination,
    getRowId: (row) => row.id,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    enableRowSelection: true,
    autoResetPageIndex: false,
  });

  const filteredRows = table.getFilteredRowModel().rows;
  const pageRows = table.getRowModel().rows;
  const filteredCount = filteredRows.length;
  const pageCount = Math.max(1, table.getPageCount());
  const pageIndex = table.getState().pagination.pageIndex;
  const pageStart = filteredCount === 0 ? 0 : pageIndex * ROWS_PER_PAGE + 1;
  const pageEnd = filteredCount === 0 ? 0 : Math.min(filteredCount, (pageIndex + 1) * ROWS_PER_PAGE);
  const selectedRows = table.getSelectedRowModel().rows;
  const selectedClients = selectedRows.map((row) => row.original);
  const selectedClientIDs = selectedClients.map((client) => client.id);
  const selectedCount = selectedClientIDs.length;
  const hasSelectedClients = selectedCount > 0;
  const filteredSelection = resolveFilteredSelection(table);

  useEffect(() => {
    const maxPage = Math.max(0, table.getPageCount() - 1);
    if (table.getState().pagination.pageIndex > maxPage) {
      table.setPageIndex(maxPage);
    }
  }, [filteredCount, table]);

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
      await refreshUsers();
    } catch (err) {
      setFormError(err instanceof APIError ? err.message : "Failed to save user");
    } finally {
      setFormBusy(false);
    }
  }

  async function exportCSV() {
    const header = "username,enabled,status,traffic_bytes,download_bps,upload_bps,last_seen";
    const rows = filteredRows.map((entry) => {
      const client = entry.original;
      const status = resolveStatus(client);
      const traffic = client.last_tx_bytes + client.last_rx_bytes;
      return `${escapeCSV(client.username)},${client.enabled},${status},${traffic},${client.download_bps || 0},${client.upload_bps || 0},${escapeCSV(client.last_seen_at || client.updated_at)}`;
    });
    const blob = new Blob([header + "\n" + rows.join("\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `users-${new Date().toISOString().slice(0, 10)}.csv`;
    anchor.click();
    URL.revokeObjectURL(url);
    toast.notify("CSV exported");
  }

  async function deleteSelectedClients() {
    if (!selectedClientIDs.length) {
      return;
    }

    const targetIDs = [...selectedClientIDs];
    setActionError("");
    const toastId = toast.notify(targetIDs.length === 1 ? "Deleting 1 user..." : `Deleting ${targetIDs.length} users...`, "info");

    try {
      const result = await deleteClientsBulk(targetIDs);
      const deleted = Math.max(0, result.deleted || 0);
      toast.update(toastId, deleted === 1 ? "1 user deleted" : `${deleted} users deleted`, "success");
      setRowSelection({});
    } catch (err) {
      const message = err instanceof APIError ? resolveBulkStateErrorMessage(err) : "Failed to delete users";
      setActionError(message);
      toast.update(toastId, message, "error");
    } finally {
      await refreshUsers();
    }
  }

  async function bulkSetEnabled(enabled: boolean) {
    if (!selectedClientIDs.length) {
      return;
    }

    const targetIDs = [...selectedClientIDs];
    setActionError("");
    try {
      const result = await setClientsEnabledBulk(targetIDs, enabled);
      const updated = Math.max(0, result.updated || 0);
      if (updated > 0) {
        toast.notify(`${updated} users ${enabled ? "enabled" : "disabled"}`);
      } else {
        toast.notify("No users updated", "info");
      }
    } catch (err) {
      const message = err instanceof APIError ? resolveBulkStateErrorMessage(err) : "Failed to update users";
      setActionError(message);
      toast.notify(message, "error");
    } finally {
      await refreshUsers();
    }
  }

  const retryUsers = useCallback(() => {
    setDismissedQueryError(false);
    void usersQuery.refetch();
  }, [usersQuery]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const editable = isEditableTarget(event.target);
      const key = event.key.toLowerCase();

      if (!formOpen && !artifactOpen && (event.ctrlKey || event.metaKey) && key === "k") {
        event.preventDefault();
        searchInputRef.current?.focus();
        searchInputRef.current?.select();
        return;
      }

      if (!formOpen && !artifactOpen && !editable && !event.ctrlKey && !event.metaKey && !event.altKey && key === "/") {
        event.preventDefault();
        searchInputRef.current?.focus();
        searchInputRef.current?.select();
        return;
      }

      if (!editable && !formOpen && !artifactOpen && key === "n") {
        event.preventDefault();
        openCreate();
        return;
      }

      if (key !== "escape") {
        return;
      }

      if (artifactOpen) {
        event.preventDefault();
        setArtifactOpen(false);
        return;
      }
      if (formOpen) {
        event.preventDefault();
        setFormOpen(false);
        return;
      }
      if (hasSelectedClients) {
        event.preventDefault();
        setRowSelection({});
        return;
      }
      if (searchInput) {
        event.preventDefault();
        setSearchInput("");
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [artifactOpen, formOpen, hasSelectedClients, openCreate, searchInput]);

  return (
    <div className="space-y-6 pb-20 sm:pb-12">
      <PageHeader
        title="Users"
        actions={
          <>
            <Button variant="primary" onClick={openCreate} className="header-btn w-full rounded-2xl px-5 sm:w-auto">
              <Plus size={17} strokeWidth={1.8} />
              Add user
            </Button>

            <Tooltip content={filteredCount > 0 ? "Export" : "No users"}>
              <span className="inline-flex w-full sm:w-auto">
                <Button
                  onClick={() => void exportCSV()}
                  disabled={filteredCount === 0}
                  className={cn(HEADER_SECONDARY_BTN, filteredCount === 0 && "pointer-events-none")}
                >
                  <Download size={17} strokeWidth={1.8} />
                  Export
                </Button>
              </span>
            </Tooltip>

            <Tooltip content="Search">
              <div className="relative w-full sm:w-[300px] lg:w-[340px]">
                <Search size={16} strokeWidth={1.6} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-tertiary" />
                {searchInput !== searchQuery && (
                  <Loader2 size={14} strokeWidth={2} className="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 animate-spin text-txt-muted" />
                )}
                <Input
                  ref={searchInputRef}
                  value={searchInput}
                  onChange={(event) => setSearchInput(event.target.value)}
                  placeholder="Search users"
                  className="header-btn rounded-2xl border-border/80 bg-surface-2/70 pl-11 shadow-[inset_0_1px_0_var(--shell-highlight)]"
                />
              </div>
            </Tooltip>

            <div className="flex w-full items-center gap-1 rounded-2xl bg-surface-2/70 p-1 shadow-[inset_0_1px_0_var(--shell-highlight)] sm:w-auto">
              {FILTER_OPTIONS.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setStatusFilter(item)}
                  className={cn(
                    "flex-1 rounded-xl px-3 py-2 text-center text-[13px] font-semibold capitalize transition-colors sm:flex-none sm:px-4",
                    statusFilter === item ? "bg-surface-4 text-txt-primary" : "text-txt-secondary hover:text-txt-primary",
                  )}
                >
                  {item}
                </button>
              ))}
            </div>

            <div className="flex w-full flex-wrap items-center gap-1 rounded-2xl bg-surface-2/70 p-1 shadow-[inset_0_1px_0_var(--shell-highlight)] lg:w-auto">
              <span className="inline-flex h-9 min-w-[36px] items-center justify-center rounded-xl bg-accent/15 px-2 text-[13px] font-bold tabular-nums text-accent">
                {selectedCount}
              </span>
              <span className="px-2 text-[13px] font-medium text-txt-secondary">selected</span>

              <button
                type="button"
                onClick={() => setRowSelection({})}
                disabled={!hasSelectedClients}
                className="inline-flex h-9 w-11 items-center justify-center rounded-2xl text-txt-muted transition-colors hover:bg-surface-3/60 hover:text-txt disabled:pointer-events-none disabled:opacity-50"
              >
                <X size={14} strokeWidth={1.9} />
              </button>

              <button
                type="button"
                onClick={() => void bulkSetEnabled(true)}
                disabled={!hasSelectedClients}
                className="inline-flex h-9 min-w-[116px] flex-1 items-center justify-center gap-1.5 rounded-2xl px-3 text-[13px] font-semibold text-status-success transition-colors hover:bg-status-success/10 disabled:pointer-events-none disabled:opacity-50 sm:flex-none"
              >
                <Power size={14} strokeWidth={1.8} />
                Enable
              </button>

              <button
                type="button"
                onClick={() => void bulkSetEnabled(false)}
                disabled={!hasSelectedClients}
                className="inline-flex h-9 min-w-[116px] flex-1 items-center justify-center gap-1.5 rounded-2xl px-3 text-[13px] font-semibold text-status-warning transition-colors hover:bg-status-warning/10 disabled:pointer-events-none disabled:opacity-50 sm:flex-none"
              >
                <PowerOff size={14} strokeWidth={1.8} />
                Disable
              </button>

              <ConfirmPopover
                title="Delete selected users"
                description={selectedDeleteDescription(selectedClients, selectedCount)}
                confirmText="Delete"
                onConfirm={() => void deleteSelectedClients()}
              >
                <button
                  type="button"
                  disabled={!hasSelectedClients}
                  className="inline-flex h-9 min-w-[116px] flex-1 items-center justify-center gap-1.5 rounded-2xl px-3 text-[13px] font-semibold text-status-danger transition-colors hover:bg-status-danger/10 disabled:pointer-events-none disabled:opacity-50 sm:flex-none"
                >
                  <Trash2 size={14} strokeWidth={1.8} />
                  Delete
                </button>
              </ConfirmPopover>
            </div>
          </>
        }
      />

      <div className="text-[13px] text-txt-secondary">{filteredCount} users</div>

      <ErrorBanner
        message={error}
        onDismiss={() => {
          if (actionError) {
            setActionError("");
            return;
          }
          setDismissedQueryError(true);
        }}
        actionLabel={queryError ? "Retry" : undefined}
        onAction={queryError ? retryUsers : undefined}
      />

      {limitWarning && (
        <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">
          Showing first 500 users.
        </div>
      )}

      <TableContainer>
        <div className="border-b border-border/40 p-3 xl:hidden">
          <div className="mb-2 flex items-center gap-2">
            <Checkbox
              checked={filteredSelection.all ? true : filteredSelection.some ? "indeterminate" : false}
              onCheckedChange={(value) => {
                const checked = value === true;
                setRowSelection((current) => applyFilteredSelection(current, filteredRows, checked));
              }}
              aria-label="select filtered users"
            />
            <span className="text-[12px] font-medium text-txt-secondary">Select all</span>
          </div>
          <div className="flex flex-wrap items-center gap-2 rounded-xl bg-surface-0/45 p-1.5">
            {[
              { id: "username" as const, label: "User", initialDesc: false },
              { id: "traffic" as const, label: "Traffic", initialDesc: true },
              { id: "seen" as const, label: "Seen", initialDesc: true },
            ].map((item) => {
              const column = table.getColumn(item.id);
              const sorted = column?.getIsSorted() ?? false;
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => toggleSort(item.id, item.initialDesc)}
                  className={cn(
                    "inline-flex min-w-[84px] flex-1 items-center justify-center gap-1 rounded-lg px-2.5 py-2 text-[12px] font-semibold transition-colors",
                    sorted ? "bg-surface-4 text-txt-primary" : "text-txt-secondary hover:text-txt-primary",
                  )}
                >
                  {item.label}
                  <SortIcon sorted={sorted} />
                </button>
              );
            })}
          </div>
        </div>

        <div className="hidden max-h-[calc(100dvh-23rem)] overflow-x-auto overflow-y-scroll xl:block">
          <Table className="table-fixed" style={{ minWidth: USERS_TABLE_MIN_WIDTH }} aria-rowcount={filteredCount + 1} aria-busy={loading}>
            <TableHeader className="bg-surface-2/96">
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id} className="border-t-0 hover:bg-transparent">
                  {headerGroup.headers.map((header) => {
                    const meta = header.column.columnDef.meta as UsersColumnMeta | undefined;
                    return (
                      <TableHead
                        key={header.id}
                        className={cn(STICKY_HEAD_CLASS, meta?.headClassName)}
                        style={{ width: header.getSize() }}
                        aria-sort={header.column.getCanSort() ? toAriaSort(header.column.getIsSorted()) : undefined}
                      >
                        {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                      </TableHead>
                    );
                  })}
                </TableRow>
              ))}
            </TableHeader>

            <TableBody>
              {loading ? (
                Array.from({ length: SKELETON_ROWS }, (_, index) => <SkeletonRow key={index} />)
              ) : pageRows.length ? (
                pageRows.map((row, index) => (
                  <TableRow key={row.id} aria-rowindex={pageIndex * ROWS_PER_PAGE + index + 2}>
                    {row.getVisibleCells().map((cell) => {
                      const meta = cell.column.columnDef.meta as UsersColumnMeta | undefined;
                      return (
                        <TableCell key={cell.id} className={meta?.cellClassName} style={{ width: cell.column.getSize() }}>
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                      );
                    })}
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell colSpan={table.getVisibleFlatColumns().length}>{clients.length ? "No matching users" : "No users"}</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>

        <div className="space-y-3 p-3 xl:hidden">
          {loading ? (
            Array.from({ length: SKELETON_ROWS }, (_, index) => <MobileSkeletonCard key={index} />)
          ) : pageRows.length ? (
            pageRows.map((row) => (
              <UserCard
                key={row.id}
                client={row.original}
                selected={row.getIsSelected()}
                maxTraffic={maxTraffic}
                onToggleSelection={(clientID, checked) => {
                  const targetRow = table.getRow(clientID);
                  targetRow.toggleSelected(checked);
                }}
                onOpenArtifacts={(client) => void openArtifacts(client)}
                onOpenEdit={openEdit}
                onRemoveClient={(clientID) => void removeClient(clientID)}
                onToggleEnabled={(client) => void toggleEnabled(client)}
              />
            ))
          ) : (
            <div className="rounded-xl bg-surface-0/45 px-4 py-6 text-[13px] text-txt-secondary">
              {clients.length ? "No matching users" : "No users"}
            </div>
          )}
        </div>

        {!loading && filteredCount > 0 ? (
          <div className="flex flex-col gap-3 border-t border-border/50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="space-y-0.5 text-[12px] text-txt-secondary">
              <p>Page {Math.min(pageIndex + 1, pageCount)} of {pageCount}</p>
              <p>{pageStart}-{pageEnd} of {filteredCount}</p>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" disabled={!table.getCanPreviousPage()} onClick={() => table.previousPage()}>Prev</Button>
              <Button size="sm" disabled={!table.getCanNextPage()} onClick={() => table.nextPage()}>Next</Button>
            </div>
          </div>
        ) : null}
      </TableContainer>

      <ClientFormDialog
        open={formOpen}
        mode={formMode}
        busy={formBusy}
        client={editingClient}
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
