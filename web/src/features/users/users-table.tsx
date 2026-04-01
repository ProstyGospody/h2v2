import { ArrowDownToLine, ArrowUpDown, ArrowUpFromLine, ChevronDown, ChevronUp, Pencil, QrCode, Trash2 } from "lucide-react";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { type HysteriaClient } from "@/domain/clients/types";
import {
  Badge,
  Button,
  Checkbox,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  Tooltip,
  Toggle,
  cn,
} from "@/src/components/ui";
import { formatBytes, formatDateTime, formatRate } from "@/utils/format";

import { initials, resolveStatus, sortAria, type SortField, type SortState } from "./users-utils";

type UsersTableProps = {
  loading: boolean;
  clients: HysteriaClient[];
  filteredClients: HysteriaClient[];
  pagedClients: HysteriaClient[];
  page: number;
  rowsPerPage: number;
  pageCount: number;
  pageStart: number;
  pageEnd: number;
  sort: SortState;
  allFilteredSelected: boolean;
  someFilteredSelected: boolean;
  selectedSet: Set<string>;
  maxTraffic: number;
  onToggleSort: (field: SortField) => void;
  onToggleSelectFiltered: (checked: boolean) => void;
  onToggleClientSelection: (clientID: string, checked: boolean) => void;
  onOpenArtifacts: (client: HysteriaClient) => void;
  onOpenEdit: (client: HysteriaClient) => void;
  onRemoveClient: (clientID: string) => void;
  onToggleEnabled: (client: HysteriaClient) => void;
  onPageChange: (next: number) => void;
};

type UserActionsProps = {
  client: HysteriaClient;
  onOpenArtifacts: (client: HysteriaClient) => void;
  onOpenEdit: (client: HysteriaClient) => void;
  onRemoveClient: (clientID: string) => void;
  compact?: boolean;
};

const SKELETON_ROWS = 8;
const STICKY_HEAD_CLASS = "sticky top-0 z-10 bg-surface-2/96";
const SORT_OPTIONS: Array<{ field: SortField; label: string }> = [
  { field: "username", label: "User" },
  { field: "traffic", label: "Traffic" },
  { field: "last_seen", label: "Seen" },
];

function statusBadgeVariant(status: ReturnType<typeof resolveStatus>): "default" | "success" | "warning" {
  if (status === "online") return "success";
  if (status === "offline") return "warning";
  return "default";
}

function SortIcon({ field, sort }: { field: SortField; sort: SortState }) {
  if (sort.field !== field) return <ArrowUpDown size={12} strokeWidth={1.5} className="text-txt-muted/60" />;
  return sort.dir === "asc"
    ? <ChevronUp size={13} strokeWidth={2} className="text-accent" />
    : <ChevronDown size={13} strokeWidth={2} className="text-accent" />;
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

function MobileSortControls({
  sort,
  onToggleSort,
}: {
  sort: SortState;
  onToggleSort: (field: SortField) => void;
}) {
  return (
    <div className="border-b border-border/40 p-3 xl:hidden">
      <div className="flex flex-wrap items-center gap-2 rounded-xl bg-surface-0/45 p-1.5">
        {SORT_OPTIONS.map((item) => (
          <button
            key={item.field}
            type="button"
            onClick={() => onToggleSort(item.field)}
            className={cn(
              "inline-flex min-w-[84px] flex-1 items-center justify-center gap-1 rounded-lg px-2.5 py-2 text-[12px] font-semibold transition-colors",
              sort.field === item.field ? "bg-surface-4 text-txt-primary" : "text-txt-secondary hover:text-txt-primary",
            )}
          >
            {item.label}
            <SortIcon field={item.field} sort={sort} />
          </button>
        ))}
        <button
          type="button"
          onClick={() => onToggleSort(sort.field)}
          className="inline-flex h-8 min-w-[74px] items-center justify-center rounded-lg bg-surface-3/45 px-2.5 text-[12px] font-semibold text-txt-secondary transition-colors hover:bg-surface-3/70 hover:text-txt-primary"
        >
          {sort.dir === "asc" ? "Asc" : "Desc"}
        </button>
      </div>
    </div>
  );
}

function UserActions({
  client,
  onOpenArtifacts,
  onOpenEdit,
  onRemoveClient,
  compact = false,
}: UserActionsProps) {
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

function UserCard({
  client,
  selected,
  maxTraffic,
  onToggleClientSelection,
  onOpenArtifacts,
  onOpenEdit,
  onRemoveClient,
  onToggleEnabled,
}: {
  client: HysteriaClient;
  selected: boolean;
  maxTraffic: number;
  onToggleClientSelection: (clientID: string, checked: boolean) => void;
  onOpenArtifacts: (client: HysteriaClient) => void;
  onOpenEdit: (client: HysteriaClient) => void;
  onRemoveClient: (clientID: string) => void;
  onToggleEnabled: (client: HysteriaClient) => void;
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
            onCheckedChange={(value) => onToggleClientSelection(client.id, value === true)}
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
          <p className="truncate text-[12px] text-txt-muted">{client.note || "-"}</p>
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

export function UsersTable({
  loading,
  clients,
  filteredClients,
  pagedClients,
  page,
  rowsPerPage,
  pageCount,
  pageStart,
  pageEnd,
  sort,
  allFilteredSelected,
  someFilteredSelected,
  selectedSet,
  maxTraffic,
  onToggleSort,
  onToggleSelectFiltered,
  onToggleClientSelection,
  onOpenArtifacts,
  onOpenEdit,
  onRemoveClient,
  onToggleEnabled,
  onPageChange,
}: UsersTableProps) {
  return (
    <TableContainer>
      <MobileSortControls sort={sort} onToggleSort={onToggleSort} />

      <div className="px-3 pb-1 pt-3 xl:hidden">
        <label className="inline-flex items-center gap-2 text-[12px] font-medium text-txt-secondary">
          <Checkbox
            checked={allFilteredSelected ? true : someFilteredSelected ? "indeterminate" : false}
            onCheckedChange={(value) => onToggleSelectFiltered(value === true)}
            aria-label="select filtered users"
          />
          Select all
        </label>
      </div>

      <div className="hidden max-h-[calc(100dvh-23rem)] overflow-auto xl:block">
        <Table className="min-w-[860px]" aria-rowcount={filteredClients.length + 1} aria-busy={loading}>
          <TableHeader className="bg-surface-2/96">
            <TableRow className="border-t-0 hover:bg-transparent">
              <TableHead className={`${STICKY_HEAD_CLASS} w-10`}>
                <Checkbox
                  checked={allFilteredSelected ? true : someFilteredSelected ? "indeterminate" : false}
                  onCheckedChange={(value) => onToggleSelectFiltered(value === true)}
                  aria-label="select filtered users"
                />
              </TableHead>
              <TableHead className={STICKY_HEAD_CLASS} aria-sort={sortAria("username", sort)}>
                <button type="button" onClick={() => onToggleSort("username")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                  User <SortIcon field="username" sort={sort} />
                </button>
              </TableHead>
              <TableHead className={STICKY_HEAD_CLASS}>Status</TableHead>
              <TableHead className={STICKY_HEAD_CLASS} aria-sort={sortAria("traffic", sort)}>
                <button type="button" onClick={() => onToggleSort("traffic")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                  Traffic <SortIcon field="traffic" sort={sort} />
                </button>
              </TableHead>
              <TableHead className={STICKY_HEAD_CLASS}>Rate</TableHead>
              <TableHead className={STICKY_HEAD_CLASS} aria-sort={sortAria("last_seen", sort)}>
                <button type="button" onClick={() => onToggleSort("last_seen")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                  Seen <SortIcon field="last_seen" sort={sort} />
                </button>
              </TableHead>
              <TableHead className={`${STICKY_HEAD_CLASS} text-right`}>Actions</TableHead>
            </TableRow>
          </TableHeader>

          <TableBody>
            {loading ? (
              Array.from({ length: SKELETON_ROWS }, (_, index) => <SkeletonRow key={index} />)
            ) : pagedClients.length ? (
              pagedClients.map((client, index) => {
                const status = resolveStatus(client);
                const traffic = client.last_tx_bytes + client.last_rx_bytes;
                const downBps = Math.max(0, client.download_bps || 0);
                const upBps = Math.max(0, client.upload_bps || 0);

                return (
                  <TableRow key={client.id} aria-rowindex={page * rowsPerPage + index + 2}>
                    <TableCell className="w-10">
                      <Checkbox
                        checked={selectedSet.has(client.id)}
                        onCheckedChange={(value) => onToggleClientSelection(client.id, value === true)}
                        aria-label={`select ${client.username}`}
                      />
                    </TableCell>

                    <TableCell>
                      <div className="flex min-w-0 items-center gap-2.5">
                        <button
                          type="button"
                          onClick={() => onOpenArtifacts(client)}
                          className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-gradient-to-br from-accent/16 to-accent-secondary/12 text-[13px] font-bold text-txt-primary"
                        >
                          {initials(client.username)}
                        </button>
                        <div className="min-w-0">
                          <button
                            type="button"
                            onClick={() => onOpenArtifacts(client)}
                            className="max-w-full truncate text-left text-[13px] font-semibold text-txt-primary hover:text-txt"
                          >
                            {client.username}
                          </button>
                          <p className="truncate text-[12px] text-txt-muted">{client.note || "-"}</p>
                        </div>
                      </div>
                    </TableCell>

                    <TableCell>
                      <div className="flex items-center justify-between gap-3">
                        <Badge variant={statusBadgeVariant(status)} className="px-2 py-0.5 text-[10px]">{status}</Badge>
                        <Toggle checked={client.enabled} onCheckedChange={() => void onToggleEnabled(client)} className="shrink-0" />
                      </div>
                    </TableCell>

                    <TableCell>
                      <div className="max-w-[200px]">
                        <TrafficMeter value={traffic} maxValue={maxTraffic} />
                      </div>
                    </TableCell>

                    <TableCell>
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
                    </TableCell>

                    <TableCell className="whitespace-nowrap text-[12px] text-txt-secondary">
                      {formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}
                    </TableCell>

                    <TableCell>
                      <UserActions
                        client={client}
                        onOpenArtifacts={onOpenArtifacts}
                        onOpenEdit={onOpenEdit}
                        onRemoveClient={onRemoveClient}
                      />
                    </TableCell>
                  </TableRow>
                );
              })
            ) : (
              <TableRow>
                <TableCell colSpan={7}>{clients.length ? "No matching users" : "No users"}</TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className="space-y-3 p-3 xl:hidden">
        {loading ? (
          Array.from({ length: SKELETON_ROWS }, (_, index) => <MobileSkeletonCard key={index} />)
        ) : pagedClients.length ? (
          pagedClients.map((client) => (
            <UserCard
              key={client.id}
              client={client}
              selected={selectedSet.has(client.id)}
              maxTraffic={maxTraffic}
              onToggleClientSelection={onToggleClientSelection}
              onOpenArtifacts={onOpenArtifacts}
              onOpenEdit={onOpenEdit}
              onRemoveClient={onRemoveClient}
              onToggleEnabled={onToggleEnabled}
            />
          ))
        ) : (
          <div className="rounded-xl bg-surface-0/45 px-4 py-6 text-[13px] text-txt-secondary">
            {clients.length ? "No matching users" : "No users"}
          </div>
        )}
      </div>

      {!loading && filteredClients.length > 0 ? (
        <div className="flex flex-col gap-3 border-t border-border/50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-0.5 text-[12px] text-txt-secondary">
            <p>Page {Math.min(page + 1, pageCount)} of {pageCount}</p>
            <p>{pageStart}-{pageEnd} of {filteredClients.length}</p>
          </div>
          <div className="flex items-center gap-2">
            <Button size="sm" disabled={page <= 0} onClick={() => onPageChange(Math.max(0, page - 1))}>Prev</Button>
            <Button size="sm" disabled={page + 1 >= pageCount} onClick={() => onPageChange(Math.min(pageCount - 1, page + 1))}>Next</Button>
          </div>
        </div>
      ) : null}
    </TableContainer>
  );
}
