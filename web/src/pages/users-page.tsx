import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Download,
  Loader2,
  Pencil,
  Plus,
  Power,
  PowerOff,
  QrCode,
  Search,
  SlidersHorizontal,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ClientArtifactsDialog } from "@/components/dialogs/client-artifacts-dialog";
import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { ErrorBanner } from "@/components/ui/error-banner";
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
import { type HysteriaClient, type HysteriaClientDefaults, type HysteriaUserPayload } from "@/domain/clients/types";
import { APIError, getAPIErrorMessage } from "@/services/api";
import {
  Badge,
  Button,
  Checkbox,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StateBlock,
  Toggle,
  Tooltip,
  cn,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { queryRefetchInterval } from "@/src/queries/polling";
import { asText, escapeCSV, initials, resolveStatus, selectedDeleteDescription } from "@/src/features/users/users-utils";
import { formatBytes, formatDateTime, formatRate } from "@/utils/format";

type ClientFilter = "all" | "online" | "enabled" | "disabled";
type SortField = "username" | "traffic" | "last_seen";
type SortDir = "asc" | "desc";
type SortState = { field: SortField; dir: SortDir };

const EMPTY_CLIENTS: HysteriaClient[] = [];
const SEARCH_DEBOUNCE_MS = 220;
const PAGE_SIZE_OPTIONS = [24, 48, 96, 192];

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tagName = target.tagName.toLowerCase();
  return tagName === "input" || tagName === "textarea" || tagName === "select";
}

function statusColor(status: ReturnType<typeof resolveStatus>): string {
  if (status === "online") return "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]";
  if (status === "offline") return "bg-status-warning";
  return "bg-txt-muted";
}

function sortLabel(sort: SortState | null): string {
  if (!sort) return "username_asc";
  return `${sort.field}_${sort.dir}`;
}

function parseSortLabel(value: string): SortState | null {
  if (value === "username_asc") return { field: "username", dir: "asc" };
  if (value === "username_desc") return { field: "username", dir: "desc" };
  if (value === "traffic_asc") return { field: "traffic", dir: "asc" };
  if (value === "traffic_desc") return { field: "traffic", dir: "desc" };
  if (value === "last_seen_asc") return { field: "last_seen", dir: "asc" };
  if (value === "last_seen_desc") return { field: "last_seen", dir: "desc" };
  return null;
}

function UsersSkeleton() {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {Array.from({ length: 9 }, (_, i) => (
        <div key={i} className="panel-card-compact animate-pulse">
          <div className="flex items-center gap-3">
            <div className="h-4 w-4 rounded bg-surface-3/60" />
            <div className="h-11 w-11 rounded-xl bg-surface-3/60" />
            <div className="min-w-0 flex-1 space-y-2">
              <div className="h-3 w-24 rounded bg-surface-3/60" />
              <div className="h-3 w-16 rounded bg-surface-3/50" />
            </div>
          </div>
          <div className="mt-3 grid grid-cols-3 gap-2">
            <div className="h-14 rounded-lg bg-surface-3/50" />
            <div className="h-14 rounded-lg bg-surface-3/50" />
            <div className="h-14 rounded-lg bg-surface-3/50" />
          </div>
          <div className="mt-3 h-8 rounded-lg bg-surface-3/50" />
        </div>
      ))}
    </div>
  );
}

function UserCard({
  client,
  selected,
  onSelect,
  onOpenArtifacts,
  onOpenEdit,
  onDelete,
  onToggleEnabled,
}: {
  client: HysteriaClient;
  selected: boolean;
  onSelect: (clientID: string, checked: boolean) => void;
  onOpenArtifacts: (client: HysteriaClient) => void;
  onOpenEdit: (client: HysteriaClient) => void;
  onDelete: (clientID: string) => void;
  onToggleEnabled: (client: HysteriaClient) => void;
}) {
  const status = resolveStatus(client);
  const traffic = client.last_tx_bytes + client.last_rx_bytes;
  const downBps = Math.max(0, client.download_bps || 0);
  const upBps = Math.max(0, client.upload_bps || 0);

  return (
    <div className="card-hover panel-card-compact relative overflow-hidden">
      <div className="flex items-start gap-3">
        <Checkbox
          checked={selected}
          onCheckedChange={(value) => onSelect(client.id, value === true)}
          aria-label={`select ${client.username}`}
        />

        <button
          type="button"
          onClick={() => onOpenArtifacts(client)}
          className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-gradient-to-br from-accent/18 to-accent-secondary/14 text-[14px] font-bold text-txt-primary"
        >
          {initials(client.username)}
        </button>

        <div className="min-w-0 flex-1">
          <button
            type="button"
            onClick={() => onOpenArtifacts(client)}
            className="block max-w-full truncate text-left text-[14px] font-semibold text-txt-primary hover:text-txt"
          >
            {client.username}
          </button>
          <p className="mt-0.5 truncate text-[12px] text-txt-muted">{client.note || "-"}</p>
        </div>

        <div className="flex shrink-0 items-center gap-1">
          <Tooltip content="QR">
            <button
              type="button"
              aria-label={`show qr for ${client.username}`}
              onClick={() => onOpenArtifacts(client)}
              className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/55 hover:text-txt"
            >
              <QrCode size={15} strokeWidth={1.8} />
            </button>
          </Tooltip>

          <Tooltip content="Edit">
            <button
              type="button"
              aria-label={`edit ${client.username}`}
              onClick={() => onOpenEdit(client)}
              className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/55 hover:text-txt"
            >
              <Pencil size={15} strokeWidth={1.8} />
            </button>
          </Tooltip>

          <ConfirmPopover
            title="Delete user"
            description={`Remove ${client.username}?`}
            confirmText="Delete"
            onConfirm={() => onDelete(client.id)}
          >
            <button
              type="button"
              aria-label={`delete ${client.username}`}
              className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-status-danger/10 hover:text-status-danger"
            >
              <Trash2 size={15} strokeWidth={1.8} />
            </button>
          </ConfirmPopover>
        </div>
      </div>

      <div className="mt-3 grid grid-cols-3 gap-2">
        <div className="rounded-lg bg-surface-3/35 px-2.5 py-2">
          <p className="text-[10px] font-semibold uppercase tracking-wide text-txt-muted">Traffic</p>
          <p className="mt-1 text-[13px] font-semibold text-txt-primary">{formatBytes(traffic)}</p>
        </div>
        <div className="rounded-lg bg-surface-3/35 px-2.5 py-2">
          <p className="inline-flex items-center gap-1 text-[10px] font-semibold uppercase tracking-wide text-txt-muted"><ArrowDownToLine size={11} />Down</p>
          <p className="mt-1 text-[13px] font-semibold text-status-success">{formatRate(downBps)}</p>
        </div>
        <div className="rounded-lg bg-surface-3/35 px-2.5 py-2">
          <p className="inline-flex items-center gap-1 text-[10px] font-semibold uppercase tracking-wide text-txt-muted"><ArrowUpFromLine size={11} />Up</p>
          <p className="mt-1 text-[13px] font-semibold text-status-warning">{formatRate(upBps)}</p>
        </div>
      </div>

      <div className="mt-3 flex items-center justify-between gap-2 rounded-lg bg-surface-3/25 px-2.5 py-2">
        <div className="inline-flex items-center gap-2">
          <span className={cn("h-2 w-2 rounded-full", statusColor(status))} />
          <span className="text-[12px] font-medium text-txt-secondary">{status}</span>
          <Badge variant="protocol-hy2" className="px-1.5 py-0.5 text-[10px]">HY2</Badge>
        </div>

        <Toggle className="shrink-0" checked={client.enabled} onCheckedChange={() => void onToggleEnabled(client)} />
      </div>

      <p className="mt-2 text-[11px] text-txt-muted">{formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}</p>
    </div>
  );
}

export default function UsersPage() {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [filter, setFilter] = useState<ClientFilter>("all");
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(24);
  const [sort, setSort] = useState<SortState>({ field: "last_seen", dir: "desc" });
  const [selectedClientIDs, setSelectedClientIDs] = useState<string[]>([]);
  const [actionError, setActionError] = useState("");
  const [dismissedQueryError, setDismissedQueryError] = useState(false);

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState("");
  const [editingClient, setEditingClient] = useState<HysteriaClient | null>(null);

  const [artifactOpen, setArtifactOpen] = useState(false);
  const [artifactLoading, setArtifactLoading] = useState(false);
  const [artifactClient, setArtifactClient] = useState<HysteriaClient | null>(null);
  const [artifactPayload, setArtifactPayload] = useState<HysteriaUserPayload | null>(null);

  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const hasSelectedRef = useRef(false);
  hasSelectedRef.current = selectedClientIDs.length > 0;

  const toast = useToast();
  const queryClient = useQueryClient();

  const usersQuery = useQuery({
    queryKey: ["users", "page-data"],
    queryFn: async () => {
      const [clientsPayload, defaultsPayload] = await Promise.all([listClients(), getClientDefaults()]);
      return {
        clients: clientsPayload.items,
        defaults: defaultsPayload,
        limited: clientsPayload.limited,
      };
    },
    staleTime: 3_000,
    refetchOnWindowFocus: true,
    refetchInterval: (query) => queryRefetchInterval(5_000, query, { enabled: !hasSelectedRef.current }),
  });

  const clients = usersQuery.data?.clients ?? EMPTY_CLIENTS;
  const defaults: HysteriaClientDefaults | null = usersQuery.data?.defaults || null;
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
    setPage(0);
  }, [filter, searchQuery, sort, pageSize]);

  useEffect(() => {
    const existing = new Set(clients.map((client) => client.id));
    setSelectedClientIDs((current) => current.filter((id) => existing.has(id)));
  }, [clients]);

  useEffect(() => {
    if (usersQuery.isSuccess) {
      setDismissedQueryError(false);
    }
  }, [usersQuery.dataUpdatedAt, usersQuery.isSuccess]);

  const filteredClients = useMemo(() => {
    const needle = searchQuery.trim().toLowerCase();
    const filtered = clients.filter((client) => {
      if (filter === "online" && client.online_count <= 0) return false;
      if (filter === "enabled" && !client.enabled) return false;
      if (filter === "disabled" && client.enabled) return false;
      if (!needle) return true;
      const haystack = [asText(client.username), asText(client.username_normalized), asText(client.note), asText(client.id)].join(" ").toLowerCase();
      return haystack.includes(needle);
    });

    const sorted = [...filtered];
    const dir = sort.dir === "asc" ? 1 : -1;
    sorted.sort((a, b) => {
      if (sort.field === "username") {
        return dir * asText(a.username).localeCompare(asText(b.username), undefined, { sensitivity: "base" });
      }
      if (sort.field === "traffic") {
        return dir * ((a.last_tx_bytes + a.last_rx_bytes) - (b.last_tx_bytes + b.last_rx_bytes));
      }
      const ta = new Date(a.last_seen_at || a.updated_at).getTime();
      const tb = new Date(b.last_seen_at || b.updated_at).getTime();
      return dir * (ta - tb);
    });

    return sorted;
  }, [clients, filter, searchQuery, sort]);

  const selectedSet = useMemo(() => new Set(selectedClientIDs), [selectedClientIDs]);
  const filteredIDs = useMemo(() => filteredClients.map((client) => client.id), [filteredClients]);

  const selectedFilteredCount = useMemo(
    () => filteredIDs.reduce((sum, id) => sum + (selectedSet.has(id) ? 1 : 0), 0),
    [filteredIDs, selectedSet],
  );

  const allFilteredSelected = filteredIDs.length > 0 && selectedFilteredCount === filteredIDs.length;

  const pageCount = Math.max(1, Math.ceil(filteredClients.length / pageSize));
  const pagedClients = useMemo(() => {
    const start = page * pageSize;
    return filteredClients.slice(start, start + pageSize);
  }, [filteredClients, page, pageSize]);

  const pageStart = filteredClients.length === 0 ? 0 : page * pageSize + 1;
  const pageEnd = filteredClients.length === 0 ? 0 : Math.min(filteredClients.length, (page + 1) * pageSize);

  useEffect(() => {
    const maxPage = Math.max(0, Math.ceil(filteredClients.length / pageSize) - 1);
    if (page > maxPage) {
      setPage(maxPage);
    }
  }, [filteredClients.length, page, pageSize]);

  async function refreshUsers() {
    await queryClient.invalidateQueries({ queryKey: ["users", "page-data"] });
  }

  function exportCSV() {
    const header = "username,enabled,status,traffic_bytes,download_bps,upload_bps,last_seen,note";
    const rows = filteredClients.map((client) => {
      const status = resolveStatus(client);
      const traffic = client.last_tx_bytes + client.last_rx_bytes;
      return `${escapeCSV(client.username)},${client.enabled},${status},${traffic},${client.download_bps || 0},${client.upload_bps || 0},${escapeCSV(client.last_seen_at || client.updated_at)},${escapeCSV(client.note || "")}`;
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

  const openCreate = useCallback(() => {
    setFormMode("create");
    setEditingClient(null);
    setFormError("");
    setFormOpen(true);
  }, []);

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
      await refreshUsers();
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
      await refreshUsers();
    } catch (err) {
      setActionError(err instanceof APIError ? err.message : "Failed to delete user");
    }
  }

  async function deleteSelectedClients() {
    if (!selectedClientIDs.length) {
      return;
    }
    const targetIDs = [...selectedClientIDs];
    setActionError("");
    const toastId = toast.notify(`Deleting 0/${targetIDs.length}...`, "info");

    try {
      const failedIDs: string[] = [];
      let deletedCount = 0;
      let processed = 0;
      let firstError = "";

      const results = await Promise.all(
        targetIDs.map(async (id) => {
          try {
            await deleteClient(id);
            return { id, ok: true as const };
          } catch (err) {
            return { id, ok: false as const, err };
          } finally {
            processed += 1;
            toast.update(toastId, `Deleting ${processed}/${targetIDs.length}...`);
          }
        }),
      );

      results.forEach((result) => {
        if (result.ok) {
          deletedCount += 1;
          return;
        }
        failedIDs.push(result.id);
        if (!firstError) {
          firstError = result.err instanceof APIError ? result.err.message : "Failed to delete selected users";
        }
      });

      if (deletedCount > 0) {
        toast.update(toastId, deletedCount === 1 ? "1 user deleted" : `${deletedCount} users deleted`, "success");
      }

      if (failedIDs.length > 0) {
        setSelectedClientIDs(failedIDs);
        setActionError(firstError || "Failed to delete selected users");
      } else {
        setSelectedClientIDs([]);
      }
    } finally {
      await refreshUsers();
    }
  }

  async function toggleEnabled(client: HysteriaClient) {
    try {
      await setClientEnabled(client.id, !client.enabled);
      await refreshUsers();
    } catch (err) {
      setActionError(err instanceof APIError ? err.message : "Failed to change state");
    }
  }

  async function bulkSetEnabled(enabled: boolean) {
    if (!selectedClientIDs.length) return;
    const targetIDs = [...selectedClientIDs];
    const results = await Promise.allSettled(targetIDs.map((id) => setClientEnabled(id, enabled)));
    const failCount = results.filter((result) => result.status === "rejected").length;
    if (failCount > 0) {
      toast.notify(`Failed to update ${failCount} users`, "error");
    } else {
      toast.notify(`${targetIDs.length} users ${enabled ? "enabled" : "disabled"}`);
    }
    await refreshUsers();
  }

  async function openArtifacts(client: HysteriaClient) {
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
  }

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.notify("Copied");
    } catch {
      setActionError("Clipboard failed");
    }
  }

  function toggleClientSelection(clientID: string, checked: boolean) {
    setSelectedClientIDs((current) => {
      if (checked) {
        if (current.includes(clientID)) return current;
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
      if (selectedClientIDs.length) {
        event.preventDefault();
        setSelectedClientIDs([]);
        return;
      }
      if (searchInput) {
        event.preventDefault();
        setSearchInput("");
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [artifactOpen, formOpen, openCreate, searchInput, selectedClientIDs.length]);

  const retryUsers = useCallback(() => {
    setDismissedQueryError(false);
    void usersQuery.refetch();
  }, [usersQuery]);

  return (
    <div className={cn("space-y-6", selectedClientIDs.length ? "pb-40 sm:pb-24" : "pb-20 sm:pb-12")}>
      <PageHeader
        title="Users"
        actions={
          <>
            <Tooltip content="Add user">
              <Button variant="primary" onClick={openCreate} className="header-btn w-full rounded-2xl px-5 sm:w-auto">
                <Plus size={17} strokeWidth={1.8} />
                Add user
              </Button>
            </Tooltip>
            <Tooltip content={filteredClients.length ? "Export" : "No users"}>
              <span className="inline-flex w-full sm:w-auto">
                <Button
                  onClick={exportCSV}
                  disabled={!filteredClients.length}
                  className={cn("header-btn w-full rounded-2xl px-5 sm:w-auto", !filteredClients.length && "pointer-events-none")}
                >
                  <Download size={17} strokeWidth={1.8} />
                  Export
                </Button>
              </span>
            </Tooltip>
          </>
        }
      />

      <div className="panel-card space-y-3">
        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr),auto,auto] lg:items-center">
          <div className="relative">
            <Search size={16} strokeWidth={1.7} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-tertiary" />
            {searchInput !== searchQuery && (
              <Loader2 size={13} strokeWidth={2} className="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 animate-spin text-txt-muted" />
            )}
            <Input
              ref={searchInputRef}
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="Search users"
              className="h-11 rounded-xl pl-11"
            />
          </div>

          <div className="inline-flex items-center rounded-xl bg-surface-3/30 p-1">
            {(["all", "online", "enabled", "disabled"] as ClientFilter[]).map((item) => (
              <button
                key={item}
                type="button"
                onClick={() => setFilter(item)}
                className={cn(
                  "rounded-lg px-3 py-1.5 text-[12px] font-semibold capitalize transition-colors",
                  filter === item ? "bg-surface-4 text-txt-primary" : "text-txt-secondary hover:text-txt",
                )}
              >
                {item}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-2">
            <div className="w-[180px]">
              <Select value={sortLabel(sort)} onValueChange={(value) => {
                const next = parseSortLabel(value);
                if (next) setSort(next);
              }}>
                <SelectTrigger className="h-11 rounded-xl">
                  <div className="inline-flex items-center gap-2 text-[13px]">
                    <SlidersHorizontal size={14} strokeWidth={1.8} />
                    <SelectValue />
                  </div>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="last_seen_desc">Last seen</SelectItem>
                  <SelectItem value="traffic_desc">Traffic</SelectItem>
                  <SelectItem value="username_asc">Username A-Z</SelectItem>
                  <SelectItem value="username_desc">Username Z-A</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="w-[110px]">
              <Select value={String(pageSize)} onValueChange={(value) => {
                const parsed = Number.parseInt(value, 10);
                if (Number.isFinite(parsed) && parsed > 0) setPageSize(parsed);
              }}>
                <SelectTrigger className="h-11 rounded-xl">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PAGE_SIZE_OPTIONS.map((value) => (
                    <SelectItem key={value} value={String(value)}>{value}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-between gap-3 text-[12px] text-txt-secondary">
          <div className="inline-flex items-center gap-2">
            <span>{filteredClients.length} users</span>
            <button
              type="button"
              onClick={() => toggleSelectFiltered(!allFilteredSelected)}
              className="rounded-lg px-2 py-1 transition-colors hover:bg-surface-3/55"
            >
              {allFilteredSelected ? "Clear all" : "Select all"}
            </button>
          </div>
          <span>{pageStart}-{pageEnd}</span>
        </div>
      </div>

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
        <div className="rounded-xl bg-status-warning/8 px-5 py-3 text-[13px] text-status-warning">
          Showing first 500 users.
        </div>
      )}

      {loading ? (
        <UsersSkeleton />
      ) : pagedClients.length ? (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {pagedClients.map((client) => (
            <UserCard
              key={client.id}
              client={client}
              selected={selectedSet.has(client.id)}
              onSelect={toggleClientSelection}
              onOpenArtifacts={openArtifacts}
              onOpenEdit={openEdit}
              onDelete={(clientID) => void removeClient(clientID)}
              onToggleEnabled={(item) => void toggleEnabled(item)}
            />
          ))}
        </div>
      ) : (
        <StateBlock
          tone="empty"
          title={clients.length ? "No matching users" : "No users"}
          actionLabel={clients.length ? undefined : "Add user"}
          onAction={clients.length ? undefined : openCreate}
          minHeightClassName="min-h-[240px]"
        />
      )}

      {!loading && filteredClients.length > 0 && (
        <div className="flex items-center justify-between rounded-xl bg-surface-3/24 px-3 py-2.5">
          <p className="text-[12px] text-txt-secondary">Page {Math.min(page + 1, pageCount)} of {pageCount}</p>
          <div className="flex items-center gap-2">
            <Button size="sm" disabled={page <= 0} onClick={() => setPage((value) => Math.max(0, value - 1))}>Prev</Button>
            <Button size="sm" disabled={page + 1 >= pageCount} onClick={() => setPage((value) => Math.min(pageCount - 1, value + 1))}>Next</Button>
          </div>
        </div>
      )}

      <div
        className={cn(
          "fixed bottom-4 left-1/2 z-40 w-[min(calc(100vw-16px),760px)] -translate-x-1/2 transition-all duration-200",
          selectedClientIDs.length ? "translate-y-0 opacity-100" : "pointer-events-none translate-y-3 opacity-0",
        )}
      >
        <div className="flex flex-wrap items-center gap-2 rounded-2xl bg-surface-2/95 px-3 py-2.5 shadow-[0_20px_46px_-12px_var(--dialog-shadow)] backdrop-blur-xl sm:px-4">
          <span className="mr-1 inline-flex h-7 min-w-[28px] items-center justify-center rounded-lg bg-accent/15 px-2 text-[13px] font-bold tabular-nums text-accent">
            {selectedClientIDs.length}
          </span>
          <span className="mr-2 text-[13px] font-medium text-txt-secondary">selected</span>

          <button
            type="button"
            onClick={() => setSelectedClientIDs([])}
            className="ml-auto inline-flex items-center justify-center rounded-lg p-1.5 text-txt-muted transition-colors hover:bg-surface-3 hover:text-txt"
          >
            <X size={14} strokeWidth={1.8} />
          </button>

          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
            <button
              type="button"
              onClick={() => void bulkSetEnabled(true)}
              className="inline-flex flex-1 items-center justify-center gap-1.5 rounded-xl px-3 py-2 text-[12px] font-semibold text-status-success transition-colors hover:bg-status-success/10 sm:flex-none"
            >
              <Power size={14} strokeWidth={1.8} />
              Enable
            </button>
            <button
              type="button"
              onClick={() => void bulkSetEnabled(false)}
              className="inline-flex flex-1 items-center justify-center gap-1.5 rounded-xl px-3 py-2 text-[12px] font-semibold text-status-warning transition-colors hover:bg-status-warning/10 sm:flex-none"
            >
              <PowerOff size={14} strokeWidth={1.8} />
              Disable
            </button>
            <ConfirmPopover
              title="Delete selected users"
              description={selectedDeleteDescription(selectedClientIDs, clients)}
              confirmText="Delete"
              onConfirm={() => void deleteSelectedClients()}
            >
              <button
                type="button"
                className="inline-flex flex-1 items-center justify-center gap-1.5 rounded-xl px-3 py-2 text-[12px] font-semibold text-status-danger transition-colors hover:bg-status-danger/10 sm:flex-none"
              >
                <Trash2 size={14} strokeWidth={1.8} />
                Delete
              </button>
            </ConfirmPopover>
          </div>
        </div>
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
