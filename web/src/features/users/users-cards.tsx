import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { ArrowDownToLine, ArrowUpFromLine, MoreVertical, Pencil, QrCode, Trash2 } from "lucide-react";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { type HysteriaClient } from "@/domain/clients/types";
import { Button, Checkbox, StateBlock, Toggle, cn } from "@/src/components/ui";
import { formatBytes, formatDateTime, formatRate } from "@/utils/format";

import { initials, resolveStatus } from "./users-utils";

type UsersCardsProps = {
  loading: boolean;
  clients: HysteriaClient[];
  pagedClients: HysteriaClient[];
  page: number;
  pageCount: number;
  pageStart: number;
  pageEnd: number;
  filteredClientsCount: number;
  selectedSet: Set<string>;
  maxTraffic: number;
  onCreate: () => void;
  onToggleClientSelection: (clientID: string, checked: boolean) => void;
  onOpenArtifacts: (client: HysteriaClient) => void;
  onOpenEdit: (client: HysteriaClient) => void;
  onRemoveClient: (clientID: string) => void;
  onToggleEnabled: (client: HysteriaClient) => void;
  onPageChange: (next: number) => void;
};

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

function MobileActions({
  client,
  onArtifacts,
  onEdit,
  onDelete,
}: {
  client: HysteriaClient;
  onArtifacts: (client: HysteriaClient) => void;
  onEdit: (client: HysteriaClient) => void;
  onDelete: (client: HysteriaClient) => void;
}) {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          aria-label={`actions for ${client.username}`}
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
            onSelect={() => onArtifacts(client)}
            className="flex cursor-pointer items-center gap-2 rounded-[7px] px-2.5 py-2 text-[12px] text-txt outline-none transition-colors hover:bg-surface-3/60"
          >
            <QrCode size={15} strokeWidth={1.4} />
            QR
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

export function UsersCards({
  loading,
  clients,
  pagedClients,
  page,
  pageCount,
  pageStart,
  pageEnd,
  filteredClientsCount,
  selectedSet,
  maxTraffic,
  onCreate,
  onToggleClientSelection,
  onOpenArtifacts,
  onOpenEdit,
  onRemoveClient,
  onToggleEnabled,
  onPageChange,
}: UsersCardsProps) {
  return (
    <div className="space-y-3 sm:hidden">
      {loading ? (
        <StateBlock tone="loading" title="Loading users" minHeightClassName="min-h-[220px]" />
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
            <div key={client.id} className="card-hover panel-card-compact">
              <div className="flex items-center gap-3">
                <Checkbox
                  checked={selectedSet.has(client.id)}
                  onCheckedChange={(value) => onToggleClientSelection(client.id, value === true)}
                  aria-label={`select ${client.username}`}
                />
                <button
                  type="button"
                  onClick={() => onOpenArtifacts(client)}
                  className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-accent/15 to-accent-secondary/10 text-[14px] font-bold text-txt-primary"
                >
                  {initials(client.username)}
                </button>
                <div className="min-w-0 flex-1">
                  <button
                    type="button"
                    onClick={() => onOpenArtifacts(client)}
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
                        "h-2 w-2 rounded-full",
                        statusOnline && "bg-status-success shadow-[0_0_8px_var(--status-success-soft)]",
                        !statusOnline && status !== "disabled" && "bg-status-warning",
                        status === "disabled" && "bg-txt-muted",
                      )}
                    />
                    {status}
                  </span>
                  <MobileActions
                    client={client}
                    onArtifacts={onOpenArtifacts}
                    onEdit={onOpenEdit}
                    onDelete={(item) => onRemoveClient(item.id)}
                  />
                </div>
              </div>

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

              <div className="mt-2.5 flex items-center justify-between border-t border-border/20 pt-2.5">
                <Toggle className="shrink-0" checked={client.enabled} onCheckedChange={() => void onToggleEnabled(client)} />
                <span className="text-[11px] text-txt-muted">
                  {formatDateTime(client.last_seen_at || client.updated_at, { includeSeconds: false })}
                </span>
              </div>
            </div>
          );
        })
      ) : (
        <StateBlock
          tone="empty"
          title={clients.length ? "No matching users" : "No users"}
          actionLabel={clients.length ? undefined : "Add user"}
          onAction={clients.length ? undefined : onCreate}
          minHeightClassName="min-h-[180px]"
        />
      )}

      {!loading && pagedClients.length > 0 && (
        <div className="flex items-center justify-between pt-2">
          <div className="space-y-0.5">
            <p className="text-[13px] text-txt-secondary">
              {Math.min(page + 1, pageCount)}/{pageCount}
            </p>
            <p className="text-[11px] text-txt-muted">
              {pageStart}-{pageEnd} of {filteredClientsCount} users
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button size="sm" disabled={page <= 0} onClick={() => onPageChange(Math.max(0, page - 1))}>Prev</Button>
            <Button size="sm" disabled={page + 1 >= pageCount} onClick={() => onPageChange(Math.min(pageCount - 1, page + 1))}>Next</Button>
          </div>
        </div>
      )}
    </div>
  );
}
