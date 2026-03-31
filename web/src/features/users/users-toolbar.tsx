import { Download, Loader2, Plus, Power, PowerOff, Search, Trash2, X } from "lucide-react";
import { type RefObject } from "react";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { PageHeader } from "@/components/ui/page-header";
import {
  Button,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Tooltip,
  cn,
} from "@/src/components/ui";

import { type ClientFilter, type SortState } from "./users-utils";

type UsersToolbarProps = {
  searchInput: string;
  searchQuery: string;
  filter: ClientFilter;
  filteredClientsCount: number;
  rowsPerPage: number;
  rowsPerPageOptions: number[];
  sort: SortState;
  searchInputRef: RefObject<HTMLInputElement | null>;
  hasUsersToExport: boolean;
  onCreate: () => void;
  onExportCSV: () => void;
  onSearchInputChange: (value: string) => void;
  onFilterChange: (value: ClientFilter) => void;
  onRowsPerPageChange: (value: string) => void;
  onSortChange: (value: string) => void;
  selectedCount: number;
  selectedDeleteDescription: string;
  onClearSelection: () => void;
  onEnableSelected: () => void;
  onDisableSelected: () => void;
  onDeleteSelected: () => void;
};

const SORT_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "last_seen_desc", label: "Seen" },
  { value: "traffic_desc", label: "Traffic" },
  { value: "username_asc", label: "Name A-Z" },
  { value: "username_desc", label: "Name Z-A" },
];

function sortValue(sort: SortState): string {
  return `${sort.field}_${sort.dir}`;
}

export function UsersToolbar({
  searchInput,
  searchQuery,
  filter,
  filteredClientsCount,
  rowsPerPage,
  rowsPerPageOptions,
  sort,
  searchInputRef,
  hasUsersToExport,
  onCreate,
  onExportCSV,
  onSearchInputChange,
  onFilterChange,
  onRowsPerPageChange,
  onSortChange,
  selectedCount,
  selectedDeleteDescription,
  onClearSelection,
  onEnableSelected,
  onDisableSelected,
  onDeleteSelected,
}: UsersToolbarProps) {
  return (
    <>
      <PageHeader
        title="Users"
        actions={
          <>
            <Button variant="primary" onClick={onCreate} className="header-btn w-full rounded-2xl px-5 sm:w-auto">
              <Plus size={17} strokeWidth={1.8} />
              Add user
            </Button>

            <Tooltip content={hasUsersToExport ? "Export" : "No users"}>
              <span className="inline-flex w-full sm:w-auto">
                <Button
                  onClick={onExportCSV}
                  disabled={!hasUsersToExport}
                  className={cn("header-btn w-full rounded-2xl px-5 sm:w-auto", !hasUsersToExport && "pointer-events-none")}
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
                  onChange={(event) => onSearchInputChange(event.target.value)}
                  placeholder="Search users"
                  className="header-btn rounded-2xl border-border/80 bg-surface-2/70 pl-11 shadow-[inset_0_1px_0_var(--shell-highlight)]"
                />
              </div>
            </Tooltip>

            <div className="flex w-full items-center gap-1 rounded-2xl bg-surface-2/70 p-1 shadow-[inset_0_1px_0_var(--shell-highlight)] sm:w-auto">
              {(["all", "online", "enabled", "disabled"] as ClientFilter[]).map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => onFilterChange(item)}
                  className={cn(
                    "flex-1 rounded-xl px-3 py-2 text-center text-[13px] font-semibold capitalize transition-colors sm:flex-none sm:px-4",
                    filter === item ? "bg-surface-4 text-txt-primary" : "text-txt-secondary hover:text-txt-primary",
                  )}
                >
                  {item}
                </button>
              ))}
            </div>

            <div className="w-full sm:w-[170px]">
              <Select value={sortValue(sort)} onValueChange={onSortChange}>
                <SelectTrigger className="header-btn rounded-2xl">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SORT_OPTIONS.map((item) => (
                    <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="w-full sm:w-[100px]">
              <Select value={String(rowsPerPage)} onValueChange={onRowsPerPageChange}>
                <SelectTrigger className="header-btn rounded-2xl">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {rowsPerPageOptions.map((value) => (
                    <SelectItem key={value} value={String(value)}>{value}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {selectedCount > 0 ? (
              <div className="flex w-full items-center gap-1 rounded-2xl bg-surface-2/70 p-1 shadow-[inset_0_1px_0_var(--shell-highlight)] sm:w-auto">
                <span className="inline-flex h-9 min-w-[36px] items-center justify-center rounded-xl bg-accent/15 px-2 text-[13px] font-bold tabular-nums text-accent">
                  {selectedCount}
                </span>
                <span className="px-2 text-[13px] font-medium text-txt-secondary">selected</span>

                <button
                  type="button"
                  onClick={onClearSelection}
                  className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-txt-muted transition-colors hover:bg-surface-3/55 hover:text-txt"
                >
                  <X size={14} strokeWidth={1.9} />
                </button>

                <button
                  type="button"
                  onClick={onEnableSelected}
                  className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-[13px] font-semibold text-status-success transition-colors hover:bg-status-success/10"
                >
                  <Power size={14} strokeWidth={1.8} />
                  Enable
                </button>

                <button
                  type="button"
                  onClick={onDisableSelected}
                  className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-[13px] font-semibold text-status-warning transition-colors hover:bg-status-warning/10"
                >
                  <PowerOff size={14} strokeWidth={1.8} />
                  Disable
                </button>

                <ConfirmPopover
                  title="Delete selected users"
                  description={selectedDeleteDescription}
                  confirmText="Delete"
                  onConfirm={() => void onDeleteSelected()}
                >
                  <button
                    type="button"
                    className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-[13px] font-semibold text-status-danger transition-colors hover:bg-status-danger/10"
                  >
                    <Trash2 size={14} strokeWidth={1.8} />
                    Delete
                  </button>
                </ConfirmPopover>
              </div>
            ) : null}
          </>
        }
      />

      <div className="text-[13px] text-txt-secondary">{filteredClientsCount} users</div>
    </>
  );
}
