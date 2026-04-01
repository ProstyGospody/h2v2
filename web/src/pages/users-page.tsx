import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ClientArtifactsDialog } from "@/components/dialogs/client-artifacts-dialog";
import { ErrorBanner } from "@/components/ui/error-banner";
import { ClientFormDialog } from "@/components/forms/client-form-dialog";
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
import { useToast } from "@/src/components/ui/Toast";
import { queryRefetchInterval } from "@/src/queries/polling";

import { UsersTable } from "@/src/features/users/users-table";
import { UsersToolbar } from "@/src/features/users/users-toolbar";
import {
  asText,
  escapeCSV,
  resolveStatus,
  selectedDeleteDescription,
  type ClientFilter,
  type SortField,
  type SortState,
} from "@/src/features/users/users-utils";

const SEARCH_DEBOUNCE_MS = 250;
const EMPTY_CLIENTS: HysteriaClient[] = [];

function defaultSortDir(field: SortField): "asc" | "desc" {
  return field === "username" ? "asc" : "desc";
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tagName = target.tagName.toLowerCase();
  return tagName === "input" || tagName === "textarea" || tagName === "select";
}

export default function UsersPage() {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [filter, setFilter] = useState<ClientFilter>("all");
  const [page, setPage] = useState(0);
  const rowsPerPage = 25;
  const [selectedClientIDs, setSelectedClientIDs] = useState<string[]>([]);
  const [sort, setSort] = useState<SortState>({ field: "last_seen", dir: "desc" });
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
  const hasSelectedRef = useRef(selectedClientIDs.length > 0);
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
  }, [searchQuery, filter, sort]);

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
      switch (sort.field) {
        case "username":
          return dir * asText(a.username).localeCompare(asText(b.username), undefined, { sensitivity: "base" });
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
    return sorted;
  }, [clients, filter, searchQuery, sort]);

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

  const pageCount = Math.max(1, Math.ceil(filteredClients.length / rowsPerPage));
  const hasSelectedClients = selectedClientIDs.length > 0;
  const pageStart = filteredClients.length === 0 ? 0 : page * rowsPerPage + 1;
  const pageEnd = filteredClients.length === 0 ? 0 : Math.min(filteredClients.length, (page + 1) * rowsPerPage);

  async function refreshUsers() {
    await queryClient.invalidateQueries({ queryKey: ["users", "page-data"] });
  }

  function toggleSort(field: SortField) {
    setSort((prev) => {
      if (prev.field === field) {
        return { field, dir: prev.dir === "asc" ? "desc" : "asc" };
      }
      return { field, dir: defaultSortDir(field) };
    });
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
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `users-${new Date().toISOString().slice(0, 10)}.csv`;
    anchor.click();
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
      let firstError = "";
      let deletedCount = 0;
      let processedCount = 0;

      const results = await Promise.all(
        targetIDs.map(async (id) => {
          try {
            await deleteClient(id);
            return { id, ok: true as const };
          } catch (err) {
            return { id, ok: false as const, err };
          } finally {
            processedCount += 1;
            toast.update(toastId, `Deleting ${processedCount}/${targetIDs.length}...`);
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
        toast.update(
          toastId,
          deletedCount > 0 ? `Deleted ${deletedCount} of ${targetIDs.length}` : "Failed to delete users",
          deletedCount > 0 ? "info" : "error",
        );
        setSelectedClientIDs(failedIDs);
        setActionError(firstError || `Deleted ${deletedCount} of ${targetIDs.length} users`);
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
      await refreshUsers();
    } else {
      toast.notify(`${targetIDs.length} users ${enabled ? "enabled" : "disabled"}`);
      await refreshUsers();
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
      setActionError("Clipboard write failed");
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
        setSelectedClientIDs([]);
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

  const retryUsers = useCallback(() => {
    setDismissedQueryError(false);
    void usersQuery.refetch();
  }, [usersQuery]);

  return (
    <div className="space-y-6 pb-20 sm:pb-12">
      <UsersToolbar
        searchInput={searchInput}
        searchQuery={searchQuery}
        filter={filter}
        filteredClientsCount={filteredClients.length}
        searchInputRef={searchInputRef}
        hasUsersToExport={filteredClients.length > 0}
        onCreate={openCreate}
        onExportCSV={exportCSV}
        onSearchInputChange={setSearchInput}
        onFilterChange={setFilter}
        selectedCount={selectedClientIDs.length}
        selectedDeleteDescription={selectedDeleteDescription(selectedClientIDs, clients)}
        onClearSelection={() => setSelectedClientIDs([])}
        onEnableSelected={() => void bulkSetEnabled(true)}
        onDisableSelected={() => void bulkSetEnabled(false)}
        onDeleteSelected={() => void deleteSelectedClients()}
      />

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

      <UsersTable
        loading={loading}
        clients={clients}
        filteredClients={filteredClients}
        pagedClients={pagedClients}
        page={page}
        rowsPerPage={rowsPerPage}
        pageCount={pageCount}
        pageStart={pageStart}
        pageEnd={pageEnd}
        sort={sort}
        allFilteredSelected={allFilteredSelected}
        someFilteredSelected={someFilteredSelected}
        selectedSet={selectedSet}
        maxTraffic={maxTraffic}
        onToggleSort={toggleSort}
        onToggleSelectFiltered={toggleSelectFiltered}
        onToggleClientSelection={toggleClientSelection}
        onOpenArtifacts={openArtifacts}
        onOpenEdit={openEdit}
        onRemoveClient={(clientID) => void removeClient(clientID)}
        onToggleEnabled={(client) => void toggleEnabled(client)}
        onPageChange={setPage}
      />

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
