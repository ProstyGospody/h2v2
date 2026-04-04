import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  type ColumnDef,
  type RowSelectionState,
  type SortingState,
  type VisibilityState,
  useReactTable,
} from "@tanstack/react-table";
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  Command as CommandIcon,
  MoreHorizontal,
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
  CommandPalette,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Input,
  Toggle,
  Tooltip,
  cn,
  type Command,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { queryRefetchInterval } from "@/src/queries/polling";
import { formatBytes, formatDateTime } from "@/utils/format";

// ---------------------------------------------------------------------------
// Constants & helpers
// ---------------------------------------------------------------------------

const SEARCH_DEBOUNCE_MS = 250;
const ROWS_PER_PAGE = 25;
const EXPIRE_SOON_DAYS = 7;

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

type StatusFilter = "all" | "enabled" | "disabled";

// ---------------------------------------------------------------------------
// Presentational cells
// ---------------------------------------------------------------------------

function StatCard({
  label,
  value,
  accent,
}: {
  label: string;
  value: string | number;
  accent?: boolean;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-0.5 rounded-xl border px-4 py-2.5",
        accent ? "border-accent/20 bg-accent/6" : "border-border/40 bg-surface-2/40",
      )}
    >
      <span className="text-[11px] font-semibold uppercase tracking-wider text-txt-muted">
        {label}
      </span>
      <span
        className={cn(
          "text-[20px] font-bold leading-tight tabular-nums",
          accent ? "text-accent-light" : "text-txt-primary",
        )}
      >
        {value}
      </span>
    </div>
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
        <span
          className={cn(
            "font-medium tabular-nums",
            danger ? "text-status-danger" : warn ? "text-status-warning" : "text-txt-primary",
          )}
        >
          {formatBytes(used)}
        </span>
        {limited && (
          <span className="text-txt-muted tabular-nums">/ {formatBytes(client.traffic_limit_bytes)}</span>
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
          <span className="w-8 text-right text-[11px] tabular-nums text-txt-muted">
            {Math.round(pct)}%
          </span>
        </div>
      )}
    </div>
  );
}

function ExpireCell({ expireAt }: { expireAt: string | null }) {
  const state = expireState(expireAt);
  if (state === "none") return <span className="text-txt-muted">—</span>;

  const label = formatDateTime(expireAt, { includeSeconds: false });
  return (
    <span
      className={cn(
        "text-[13px] tabular-nums",
        state === "expired" && "font-medium text-status-danger",
        state === "soon" && "font-medium text-status-warning",
        state === "ok" && "text-txt-secondary",
      )}
    >
      {state === "expired" && (
        <span className="mr-1.5 inline-block h-1.5 w-1.5 translate-y-[-1px] rounded-full bg-status-danger align-middle" />
      )}
      {state === "soon" && (
        <span className="mr-1.5 inline-block h-1.5 w-1.5 translate-y-[-1px] rounded-full bg-status-warning align-middle" />
      )}
      {label}
    </span>
  );
}

function SortIcon({ dir }: { dir: false | "asc" | "desc" }) {
  if (dir === "asc") return <ArrowUp size={12} strokeWidth={2.2} />;
  if (dir === "desc") return <ArrowDown size={12} strokeWidth={2.2} />;
  return <ArrowUpDown size={12} strokeWidth={1.8} className="opacity-40" />;
}

// ---------------------------------------------------------------------------
// Toolbar subcomponents
// ---------------------------------------------------------------------------

function StatusFilterGroup({
  value,
  onChange,
}: {
  value: StatusFilter;
  onChange: (v: StatusFilter) => void;
}) {
  const options: StatusFilter[] = ["all", "enabled", "disabled"];
  return (
    <div
      role="radiogroup"
      aria-label="Filter by status"
      className="flex items-center gap-1 rounded-xl border border-border/40 bg-surface-2/40 p-1"
    >
      {options.map((f) => (
        <button
          key={f}
          role="radio"
          aria-checked={value === f}
          type="button"
          onClick={() => onChange(f)}
          className={cn(
            "rounded-lg px-3.5 py-1.5 text-[12px] font-semibold capitalize transition-all",
            value === f
              ? "bg-surface-3/80 text-txt shadow-sm"
              : "text-txt-secondary hover:text-txt",
          )}
        >
          {f}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
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

  // Stats
  const stats = useMemo(
    () => ({
      total: clients.length,
      active: clients.filter((c) => c.enabled).length,
      disabled: clients.filter((c) => !c.enabled).length,
      traffic: clients.reduce((acc, c) => acc + totalTraffic(c), 0),
    }),
    [clients],
  );

  // Search & filter
  const [searchInput, setSearchInput] = useState("");
  const [globalFilter, setGlobalFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(
      () => setGlobalFilter(searchInput.toLowerCase().trim()),
      SEARCH_DEBOUNCE_MS,
    );
    return () => clearTimeout(debounceRef.current);
  }, [searchInput]);

  // Pre-filter by status (react-table global filter only handles username)
  const filteredByStatus = useMemo(() => {
    if (statusFilter === "all") return clients;
    return clients.filter((c) => (statusFilter === "enabled" ? c.enabled : !c.enabled));
  }, [clients, statusFilter]);

  // ---------- Mutations ----------
  const handleToggle = useCallback(
    async (client: Client) => {
      try {
        await setClientEnabled(client.id, !client.enabled);
        await qc.invalidateQueries({ queryKey: ["clients"] });
      } catch (err) {
        toast.notify(getAPIErrorMessage(err, "Failed to toggle user"), "error");
      }
    },
    [qc, toast],
  );

  const handleDelete = useCallback(
    async (client: Client) => {
      try {
        await deleteClient(client.id);
        toast.notify(`User "${client.username}" deleted`);
        await qc.invalidateQueries({ queryKey: ["clients"] });
      } catch (err) {
        toast.notify(getAPIErrorMessage(err, "Failed to delete user"), "error");
      }
    },
    [qc, toast],
  );

  // ---------- Dialogs ----------
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

  // Artifacts
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
        toast.notify(getAPIErrorMessage(err, "Failed to load QR"), "error");
      } finally {
        setArtifactsLoading(false);
      }
    },
    [toast],
  );

  // ---------- react-table ----------
  const columnHelper = createColumnHelper<Client>();

  const columns = useMemo<ColumnDef<Client, unknown>[]>(
    () => [
      columnHelper.display({
        id: "select",
        size: 40,
        header: ({ table }) => (
          <Checkbox
            checked={
              table.getIsAllPageRowsSelected()
                ? true
                : table.getIsSomePageRowsSelected()
                  ? "indeterminate"
                  : false
            }
            onCheckedChange={(v) => table.toggleAllPageRowsSelected(!!v)}
            aria-label="Select all"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(v) => row.toggleSelected(!!v)}
            aria-label={`Select ${row.original.username}`}
          />
        ),
        enableSorting: false,
      }),

      columnHelper.accessor("username", {
        header: "User",
        cell: ({ row }) => {
          const c = row.original;
          return (
            <div className="flex flex-col gap-1.5">
              <button
                type="button"
                className="w-fit text-left font-semibold text-txt-primary hover:text-accent-light hover:underline"
                onClick={() => openArtifacts(c)}
              >
                {c.username}
              </button>
              <div className="flex gap-1.5">
                {c.protocols.includes("vless") && (
                  <Badge variant="protocol-vless">VLESS</Badge>
                )}
                {c.protocols.includes("hy2") && (
                  <Badge variant="protocol-hy2">HY2</Badge>
                )}
                {c.protocols.length === 0 && (
                  <span className="text-[11px] text-txt-muted">No access</span>
                )}
              </div>
            </div>
          );
        },
        filterFn: (row, _id, filter) =>
          !filter || row.original.username.toLowerCase().includes(String(filter)),
      }),

      columnHelper.accessor((c) => c.enabled, {
        id: "status",
        header: "Status",
        cell: ({ row }) => {
          const c = row.original;
          return (
            <div className="flex items-center gap-2.5">
              <Toggle checked={c.enabled} onCheckedChange={() => handleToggle(c)} />
              <span
                className={cn(
                  "text-[12px] font-medium",
                  c.enabled ? "text-status-success" : "text-txt-muted",
                )}
              >
                {c.enabled ? "Active" : "Disabled"}
              </span>
            </div>
          );
        },
        sortingFn: (a, b) => Number(b.original.enabled) - Number(a.original.enabled),
      }),

      columnHelper.accessor((c) => trafficPercent(c), {
        id: "traffic",
        header: "Traffic",
        cell: ({ row }) => <TrafficCell client={row.original} />,
      }),

      columnHelper.accessor(
        (c) => (c.expire_at ? new Date(c.expire_at).getTime() : Number.POSITIVE_INFINITY),
        {
          id: "expires",
          header: "Expires",
          cell: ({ row }) => <ExpireCell expireAt={row.original.expire_at} />,
        },
      ),

      columnHelper.display({
        id: "actions",
        size: 56,
        header: () => null,
        cell: ({ row }) => {
          const c = row.original;
          return (
            <div className="flex items-center justify-end">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    aria-label={`Actions for ${c.username}`}
                    className="inline-grid h-8 w-8 place-items-center rounded-lg text-txt-muted hover:bg-surface-3/60 hover:text-txt-primary data-[state=open]:bg-surface-3/60 data-[state=open]:text-txt-primary"
                  >
                    <MoreHorizontal size={16} strokeWidth={2} />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent>
                  <DropdownMenuItem icon={<QrCode size={14} />} onSelect={() => openArtifacts(c)}>
                    QR & links
                  </DropdownMenuItem>
                  <DropdownMenuItem icon={<Pencil size={14} />} onSelect={() => openEdit(c)}>
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
                      if (window.confirm(`Delete "${c.username}"? This cannot be undone.`)) {
                        void handleDelete(c);
                      }
                    }}
                  >
                    Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          );
        },
        enableSorting: false,
      }),
    ],
    [columnHelper, handleDelete, handleToggle, openArtifacts, openEdit],
  );

  const [sorting, setSorting] = useState<SortingState>([{ id: "username", desc: false }]);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [columnVisibility] = useState<VisibilityState>({});
  const [{ pageIndex, pageSize }, setPagination] = useState({
    pageIndex: 0,
    pageSize: ROWS_PER_PAGE,
  });

  const table = useReactTable({
    data: filteredByStatus,
    columns,
    state: { sorting, rowSelection, globalFilter, columnVisibility, pagination: { pageIndex, pageSize } },
    onSortingChange: setSorting,
    onRowSelectionChange: setRowSelection,
    onGlobalFilterChange: setGlobalFilter,
    onPaginationChange: setPagination,
    getRowId: (c) => c.id,
    globalFilterFn: (row, _id, filter) =>
      !filter || row.original.username.toLowerCase().includes(String(filter).toLowerCase()),
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  useEffect(() => {
    setPagination((p) => ({ ...p, pageIndex: 0 }));
  }, [globalFilter, statusFilter]);

  const selectedIds = useMemo(() => Object.keys(rowSelection), [rowSelection]);
  const selectedCount = selectedIds.length;

  async function bulkEnable(enabled: boolean) {
    if (selectedCount === 0) return;
    try {
      const count = await setClientsEnabledBulk(selectedIds, enabled);
      toast.notify(`${count} user(s) ${enabled ? "enabled" : "disabled"}`);
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Bulk operation failed"), "error");
    }
  }

  async function bulkDelete() {
    if (selectedCount === 0) return;
    try {
      const count = await deleteClientsBulk(selectedIds);
      toast.notify(`${count} user(s) deleted`);
      setRowSelection({});
      await qc.invalidateQueries({ queryKey: ["clients"] });
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Bulk delete failed"), "error");
    }
  }

  // ---------- Command palette ----------
  const [cmdOpen, setCmdOpen] = useState(false);

  const commands = useMemo<Command[]>(() => {
    const baseCommands: Command[] = [
      {
        id: "new-user",
        label: "Create new user",
        icon: <Plus size={14} />,
        shortcut: "N",
        group: "Actions",
        onSelect: openCreate,
      },
      {
        id: "focus-search",
        label: "Focus search",
        icon: <Search size={14} />,
        shortcut: "/",
        group: "Actions",
        onSelect: () => searchRef.current?.focus(),
      },
      {
        id: "clear-selection",
        label: "Clear selection",
        icon: <X size={14} />,
        group: "Actions",
        onSelect: () => setRowSelection({}),
      },
      {
        id: "filter-all",
        label: "Filter: all",
        group: "Filters",
        onSelect: () => setStatusFilter("all"),
      },
      {
        id: "filter-enabled",
        label: "Filter: enabled",
        group: "Filters",
        onSelect: () => setStatusFilter("enabled"),
      },
      {
        id: "filter-disabled",
        label: "Filter: disabled",
        group: "Filters",
        onSelect: () => setStatusFilter("disabled"),
      },
    ];
    const userCommands: Command[] = clients.slice(0, 50).map((c) => ({
      id: `user-${c.id}`,
      label: c.username,
      keywords: c.protocols.join(" "),
      icon: <QrCode size={14} />,
      group: "Open user",
      onSelect: () => openArtifacts(c),
    }));
    return [...baseCommands, ...userCommands];
  }, [clients, openArtifacts, openCreate]);

  // ---------- Keyboard ----------
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const isTyping =
        document.activeElement?.tagName === "INPUT" ||
        document.activeElement?.tagName === "TEXTAREA";
      if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setCmdOpen((v) => !v);
        return;
      }
      if (e.key === "Escape" && !formOpen && !artifactsOpen && !cmdOpen) {
        if (selectedCount > 0) {
          setRowSelection({});
          return;
        }
        if (searchInput) {
          setSearchInput("");
          return;
        }
      }
      if (e.key === "/" && !isTyping && !formOpen && !artifactsOpen && !cmdOpen) {
        e.preventDefault();
        searchRef.current?.focus();
      }
      if ((e.key === "n" || e.key === "N") && !isTyping && !formOpen && !artifactsOpen && !cmdOpen) {
        openCreate();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [artifactsOpen, cmdOpen, formOpen, openCreate, searchInput, selectedCount]);

  // ---------- Render ----------
  const isLoading = usersQuery.isLoading;
  const isError = usersQuery.isError;
  const rows = table.getRowModel().rows;
  const filteredCount = table.getFilteredRowModel().rows.length;
  const pageCount = table.getPageCount();

  return (
    <div className="space-y-5">
      <PageHeader
        title="Users"
        subtitle={`${clients.length} total`}
        actions={
          <div className="flex items-center gap-2">
            <Tooltip content="Command palette (⌘K)">
              <button
                type="button"
                onClick={() => setCmdOpen(true)}
                className="inline-flex h-9 items-center gap-2 rounded-xl border border-border/50 bg-surface-2/40 px-3 text-[12px] font-medium text-txt-secondary hover:bg-surface-3/40 hover:text-txt-primary"
              >
                <CommandIcon size={13} /> K
              </button>
            </Tooltip>
            <Button variant="primary" onClick={openCreate}>
              <Plus size={16} strokeWidth={2} /> New User
            </Button>
          </div>
        }
      />

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
          <Search
            size={15}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-txt-muted"
          />
          <Input
            ref={searchRef}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search users…  (press / )"
            className="pl-9 pr-8"
          />
          {searchInput && (
            <button
              type="button"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded p-0.5 text-txt-muted hover:text-txt"
              onClick={() => setSearchInput("")}
              aria-label="Clear search"
            >
              <X size={14} />
            </button>
          )}
        </div>
        <StatusFilterGroup value={statusFilter} onChange={setStatusFilter} />
      </div>

      {/* Bulk actions */}
      {selectedCount > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-accent/20 bg-accent/6 px-4 py-2.5 text-[13px]">
          <span className="font-semibold text-accent-light">{selectedCount} selected</span>
          <div className="mx-1 h-4 w-px bg-border/60" />
          <Button size="sm" onClick={() => bulkEnable(true)}>
            <Power size={13} /> Enable
          </Button>
          <Button size="sm" onClick={() => bulkEnable(false)}>
            <PowerOff size={13} /> Disable
          </Button>
          <ConfirmPopover
            title="Delete users"
            description={`Delete ${selectedCount} selected user(s)? This cannot be undone.`}
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
            onClick={() => setRowSelection({})}
          >
            <X size={13} /> Clear
          </button>
        </div>
      )}

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
              {table.getHeaderGroups().map((hg) => (
                <tr key={hg.id} className="border-b border-border/40 bg-surface-2/70">
                  {hg.headers.map((h) => {
                    const canSort = h.column.getCanSort();
                    const sortDir = h.column.getIsSorted();
                    return (
                      <th
                        key={h.id}
                        style={{ width: h.getSize() || undefined }}
                        className={cn(
                          "px-4 py-3.5 text-left text-[11px] font-semibold uppercase tracking-wider text-txt-muted",
                          h.id === "select" && "w-10 px-3",
                          h.id === "actions" && "w-14 px-3 text-right",
                        )}
                      >
                        {canSort ? (
                          <button
                            type="button"
                            onClick={h.column.getToggleSortingHandler()}
                            className="inline-flex items-center gap-1.5 hover:text-txt-primary"
                          >
                            {flexRender(h.column.columnDef.header, h.getContext())}
                            <SortIcon dir={sortDir} />
                          </button>
                        ) : (
                          flexRender(h.column.columnDef.header, h.getContext())
                        )}
                      </th>
                    );
                  })}
                </tr>
              ))}
            </thead>
            <tbody className="divide-y divide-border/20">
              {isLoading ? (
                Array.from({ length: 7 }).map((_, i) => (
                  <tr key={i}>
                    {Array.from({ length: 6 }).map((_, j) => (
                      <td key={j} className="px-4 py-4">
                        <div
                          className={cn(
                            "h-3.5 animate-pulse rounded-md bg-surface-3/60",
                            j === 1 ? "w-32" : j === 3 ? "w-28" : "w-16",
                          )}
                        />
                      </td>
                    ))}
                  </tr>
                ))
              ) : rows.length === 0 ? (
                <tr>
                  <td colSpan={columns.length} className="px-4 py-20 text-center">
                    <div className="flex flex-col items-center gap-3 text-txt-muted">
                      <Users2 size={36} strokeWidth={1.2} className="opacity-40" />
                      <p className="text-[14px]">
                        {globalFilter || statusFilter !== "all"
                          ? "No users match the current filters."
                          : "No users yet."}
                      </p>
                      {!globalFilter && statusFilter === "all" && (
                        <Button variant="primary" size="sm" onClick={openCreate}>
                          <Plus size={14} /> Create first user
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ) : (
                rows.map((row) => (
                  <tr
                    key={row.id}
                    className={cn(
                      "group/row transition-colors hover:bg-surface-2/50",
                      row.getIsSelected() && "bg-accent/4 hover:bg-accent/6",
                    )}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td
                        key={cell.id}
                        className={cn(
                          "px-4 py-3.5 align-middle",
                          cell.column.id === "select" && "w-10 px-3",
                          cell.column.id === "actions" && "w-14 px-3",
                        )}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {pageCount > 1 && (
          <div className="flex items-center justify-between border-t border-border/40 px-5 py-3 text-[12px] text-txt-secondary">
            <span>
              Showing {pageIndex * pageSize + 1}–
              {Math.min((pageIndex + 1) * pageSize, filteredCount)} of {filteredCount}
            </span>
            <div className="flex items-center gap-1">
              <Button
                size="sm"
                disabled={!table.getCanPreviousPage()}
                onClick={() => table.previousPage()}
              >
                ← Prev
              </Button>
              <span className="px-2 tabular-nums">
                {pageIndex + 1} / {pageCount}
              </span>
              <Button
                size="sm"
                disabled={!table.getCanNextPage()}
                onClick={() => table.nextPage()}
              >
                Next →
              </Button>
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
      <CommandPalette
        open={cmdOpen}
        onOpenChange={setCmdOpen}
        commands={commands}
        placeholder="Type a command or user name…"
      />
    </div>
  );
}
