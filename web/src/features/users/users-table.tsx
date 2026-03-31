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

const SKELETON_ROWS = 8;

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
      <div className="max-h-[calc(100dvh-23rem)] overflow-auto">
        <Table className="min-w-[980px]" aria-rowcount={filteredClients.length + 1}>
          <TableHeader className="sticky top-0 z-10 bg-surface-2/96 backdrop-blur">
            <TableRow className="border-t-0 hover:bg-transparent">
              <TableHead className="w-10">
                <Checkbox
                  checked={allFilteredSelected ? true : someFilteredSelected ? "indeterminate" : false}
                  onCheckedChange={(value) => onToggleSelectFiltered(value === true)}
                  aria-label="select filtered users"
                />
              </TableHead>
              <TableHead aria-sort={sortAria("username", sort)}>
                <button type="button" onClick={() => onToggleSort("username")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                  User <SortIcon field="username" sort={sort} />
                </button>
              </TableHead>
              <TableHead>Status</TableHead>
              <TableHead aria-sort={sortAria("traffic", sort)}>
                <button type="button" onClick={() => onToggleSort("traffic")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                  Traffic <SortIcon field="traffic" sort={sort} />
                </button>
              </TableHead>
              <TableHead>Rate</TableHead>
              <TableHead aria-sort={sortAria("last_seen", sort)}>
                <button type="button" onClick={() => onToggleSort("last_seen")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                  Seen <SortIcon field="last_seen" sort={sort} />
                </button>
              </TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>

          {loading ? (
            <tbody>
              {Array.from({ length: SKELETON_ROWS }, (_, index) => <SkeletonRow key={index} />)}
            </tbody>
          ) : (
            <TableBody>
              {pagedClients.length ? (
                pagedClients.map((client, index) => {
                  const status = resolveStatus(client);
                  const traffic = client.last_tx_bytes + client.last_rx_bytes;
                  const downBps = Math.max(0, client.download_bps || 0);
                  const upBps = Math.max(0, client.upload_bps || 0);

                  return (
                    <TableRow key={client.id} aria-rowindex={page * rowsPerPage + index + 2}>
                      <TableCell>
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
                        <div className="flex min-w-[162px] items-center justify-between gap-3">
                          <Badge variant={statusBadgeVariant(status)} className="px-2 py-0.5 text-[10px]">{status}</Badge>
                          <Toggle checked={client.enabled} onCheckedChange={() => void onToggleEnabled(client)} className="shrink-0" />
                        </div>
                      </TableCell>

                      <TableCell>
                        <div className="min-w-[150px] max-w-[220px]">
                          <TrafficMeter value={traffic} maxValue={maxTraffic} />
                        </div>
                      </TableCell>

                      <TableCell>
                        <div className="space-y-1 text-[12px] font-medium tabular-nums text-txt-secondary">
                          <p className="inline-flex items-center gap-1.5 whitespace-nowrap">
                            <ArrowDownToLine size={12} strokeWidth={1.8} className="text-status-success" />
                            {formatRate(downBps)}
                          </p>
                          <p className="inline-flex items-center gap-1.5 whitespace-nowrap">
                            <ArrowUpFromLine size={12} strokeWidth={1.8} className="text-status-warning" />
                            {formatRate(upBps)}
                          </p>
                        </div>
                      </TableCell>

                      <TableCell className="whitespace-nowrap text-[12px] text-txt-secondary">
                        {formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}
                      </TableCell>

                      <TableCell>
                        <div className="flex items-center justify-end gap-1">
                          <Tooltip content="QR">
                            <button
                              type="button"
                              onClick={() => onOpenArtifacts(client)}
                              aria-label={`show qr for ${client.username}`}
                              className="inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/55 hover:text-txt"
                            >
                              <QrCode size={16} strokeWidth={1.8} />
                            </button>
                          </Tooltip>

                          <Tooltip content="Edit">
                            <button
                              type="button"
                              onClick={() => onOpenEdit(client)}
                              aria-label={`edit ${client.username}`}
                              className="inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-surface-3/55 hover:text-txt"
                            >
                              <Pencil size={16} strokeWidth={1.8} />
                            </button>
                          </Tooltip>

                          <ConfirmPopover
                            title="Delete user"
                            description={`Remove ${client.username}?`}
                            confirmText="Delete"
                            onConfirm={() => onRemoveClient(client.id)}
                          >
                            <button
                              type="button"
                              aria-label={`delete ${client.username}`}
                              className="inline-flex h-9 w-9 items-center justify-center rounded-lg text-txt-tertiary transition-colors hover:bg-status-danger/10 hover:text-status-danger"
                            >
                              <Trash2 size={16} strokeWidth={1.8} />
                            </button>
                          </ConfirmPopover>
                        </div>
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
          )}
        </Table>
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
