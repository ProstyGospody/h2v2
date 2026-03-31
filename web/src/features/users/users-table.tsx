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
  sort: SortState | null;
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

function SkeletonRow() {
  return (
    <tr className="border-t border-border/30">
      <td className="w-10 px-5 py-3.5"><div className="h-4 w-4 animate-pulse rounded bg-surface-3/60" /></td>
      <td className="hidden w-14 px-5 py-3.5 md:table-cell"><div className="h-4 w-full max-w-[24px] animate-pulse rounded bg-surface-3/60" /></td>
      <td className="px-5 py-3.5">
        <div className="flex items-center gap-2">
          <div className="h-9 w-9 shrink-0 animate-pulse rounded-xl bg-surface-3/60" />
          <div className="min-w-0 flex-1 space-y-1.5">
            <div className="h-3.5 w-full max-w-[120px] animate-pulse rounded bg-surface-3/60" />
            <div className="h-3 w-full max-w-[80px] animate-pulse rounded bg-surface-3/50" />
          </div>
        </div>
      </td>
      <td className="hidden px-5 py-3.5 lg:table-cell"><div className="h-5 w-full max-w-[40px] animate-pulse rounded-badge bg-surface-3/60" /></td>
      <td className="px-5 py-3.5"><div className="h-5 w-full max-w-[80px] animate-pulse rounded bg-surface-3/60" /></td>
      <td className="hidden px-5 py-3.5 lg:table-cell">
        <div className="space-y-1.5">
          <div className="h-2 w-full animate-pulse rounded-full bg-surface-3/60" />
          <div className="h-3 w-full max-w-[48px] animate-pulse rounded bg-surface-3/50" />
        </div>
      </td>
      <td className="hidden px-5 py-3.5 text-right lg:table-cell"><div className="ml-auto h-8 w-full max-w-[96px] animate-pulse rounded bg-surface-3/60" /></td>
      <td className="hidden px-5 py-3.5 text-right md:table-cell"><div className="ml-auto h-4 w-full max-w-[96px] animate-pulse rounded bg-surface-3/60" /></td>
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
    <div className="h-2 w-full overflow-hidden rounded-full bg-surface-3/40">
      <div
        className={cn(
          "h-full rounded-full transition-[width] duration-300",
          isHigh
            ? "bg-gradient-to-r from-status-warning to-status-danger"
            : "bg-gradient-to-r from-accent to-accent-light",
        )}
        style={{ width: `${ratio}%` }}
      />
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
    <TableContainer className="hidden sm:block">
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
              <TableHead className="w-[76px] text-center"><span className="sr-only">QR Code</span></TableHead>
              <TableHead className="w-[76px] text-center"><span className="sr-only">Edit</span></TableHead>
              <TableHead className="w-[84px] text-center"><span className="sr-only">Delete</span></TableHead>
            </TableRow>
          </TableHeader>
          <tbody>
            {Array.from({ length: SKELETON_ROWS }, (_, i) => <SkeletonRow key={i} />)}
          </tbody>
        </Table>
      ) : (
        <>
          <div className="max-h-[calc(100dvh-23rem)] overflow-auto">
            <Table aria-rowcount={filteredClients.length + 1}>
              <TableHeader>
                <TableRow className="border-t-0 hover:bg-transparent">
                  <TableHead className="w-10">
                    <Checkbox
                      checked={allFilteredSelected ? true : someFilteredSelected ? "indeterminate" : false}
                      onCheckedChange={(value) => onToggleSelectFiltered(value === true)}
                      aria-label="select filtered users"
                    />
                  </TableHead>
                  <TableHead className="hidden w-14 md:table-cell">#</TableHead>
                  <TableHead aria-sort={sortAria("username", sort)}>
                    <button type="button" onClick={() => onToggleSort("username")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                      USERS <SortIcon field="username" sort={sort} />
                    </button>
                  </TableHead>
                  <TableHead className="hidden lg:table-cell">PROTOCOL</TableHead>
                  <TableHead>STATUS</TableHead>
                  <TableHead className="hidden lg:table-cell" aria-sort={sortAria("traffic", sort)}>
                    <button type="button" onClick={() => onToggleSort("traffic")} className="inline-flex items-center gap-1.5 hover:text-txt-primary">
                      TRAFFIC <SortIcon field="traffic" sort={sort} />
                    </button>
                  </TableHead>
                  <TableHead className="hidden text-right lg:table-cell">
                    NETWORK
                  </TableHead>
                  <TableHead className="hidden text-right md:table-cell" aria-sort={sortAria("last_seen", sort)}>
                    <button type="button" onClick={() => onToggleSort("last_seen")} className="ml-auto inline-flex items-center gap-1.5 hover:text-txt-primary">
                      LAST SEEN <SortIcon field="last_seen" sort={sort} />
                    </button>
                  </TableHead>
                  <TableHead className="w-[76px] text-center"><span className="sr-only">QR Code</span></TableHead>
                  <TableHead className="w-[76px] text-center"><span className="sr-only">Edit</span></TableHead>
                  <TableHead className="w-[84px] text-center"><span className="sr-only">Delete</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pagedClients.length ? (
                  <>
                    {pagedClients.map((client, index) => {
                      const traffic = client.last_tx_bytes + client.last_rx_bytes;
                      const ratio = maxTraffic > 0 ? Math.min(100, (traffic / maxTraffic) * 100) : 0;
                      const ratioWidth = traffic > 0 ? Math.max(ratio, 4) : 0;
                      const status = resolveStatus(client);
                      const statusOnline = status === "online";
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
                          <TableCell className="hidden md:table-cell">{page * rowsPerPage + index + 1}</TableCell>
                          <TableCell>
                            <div className="flex min-w-0 items-center gap-2">
                              <button
                                type="button"
                                onClick={() => onOpenArtifacts(client)}
                                className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-accent/15 to-accent-secondary/10 text-[13px] font-bold text-txt-primary"
                              >
                                {initials(client.username)}
                              </button>
                              <div className="min-w-0">
                                <button
                                  type="button"
                                  onClick={() => onOpenArtifacts(client)}
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
                                    "h-2 w-2 rounded-full",
                                    statusOnline && "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]",
                                    !statusOnline && status !== "disabled" && "bg-status-warning",
                                    status === "disabled" && "bg-txt-muted",
                                  )}
                                />
                                <span className="w-[62px] text-[11px] text-txt-secondary">{status}</span>
                              </span>
                              <Toggle className="shrink-0" checked={client.enabled} onCheckedChange={() => void onToggleEnabled(client)} />
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
                            <Tooltip content="QR">
                              <button
                                type="button"
                                onClick={() => onOpenArtifacts(client)}
                                aria-label={`show qr for ${client.username}`}
                                className="inline-flex h-10 w-10 items-center justify-center rounded-xl text-txt-tertiary transition-colors hover:bg-surface-3 hover:text-txt"
                              >
                                <QrCode size={18} strokeWidth={1.7} />
                              </button>
                            </Tooltip>
                          </TableCell>
                          <TableCell className="text-center">
                            <Tooltip content="Edit">
                              <button
                                type="button"
                                onClick={() => onOpenEdit(client)}
                                aria-label={`edit ${client.username}`}
                                className="inline-flex h-10 w-10 items-center justify-center rounded-xl text-txt-tertiary transition-colors hover:bg-surface-3 hover:text-txt"
                              >
                                <Pencil size={18} strokeWidth={1.7} />
                              </button>
                            </Tooltip>
                          </TableCell>
                          <TableCell className="text-center">
                            <ConfirmPopover
                              title="Delete user"
                              description={`Remove ${client.username}?`}
                              confirmText="Delete"
                              onConfirm={() => onRemoveClient(client.id)}
                            >
                              <button
                                type="button"
                                aria-label={`delete ${client.username}`}
                                className="inline-flex h-10 w-10 items-center justify-center rounded-xl text-txt-tertiary transition-colors hover:bg-status-danger/10 hover:text-status-danger"
                              >
                                <Trash2 size={18} strokeWidth={1.7} />
                              </button>
                            </ConfirmPopover>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </>
                ) : (
                  <TableRow>
                    <TableCell colSpan={11}>{clients.length ? "No users match the current filters." : "No users yet."}</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          <div className="flex flex-col gap-3 border-t border-border/50 px-4 py-3.5 sm:flex-row sm:items-center sm:justify-between sm:px-5">
            <div className="space-y-1">
              <p className="text-[13px] text-txt-secondary">
                Page {Math.min(page + 1, pageCount)} of {pageCount}
              </p>
              <p className="text-[12px] text-txt-muted">
                Showing {pageStart}-{pageEnd} of {filteredClients.length} users
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" disabled={page <= 0} onClick={() => onPageChange(Math.max(0, page - 1))}>
                Prev
              </Button>
              <Button
                size="sm"
                disabled={page + 1 >= pageCount}
                onClick={() => onPageChange(Math.min(pageCount - 1, page + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        </>
      )}
    </TableContainer>
  );
}
