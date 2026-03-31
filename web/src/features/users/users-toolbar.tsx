import { Download, Loader2, Plus, Search } from "lucide-react";
import { type RefObject } from "react";

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
          </>
        }
      />

      <div className="text-[13px] text-txt-secondary">{filteredClientsCount} users</div>
    </>
  );
}
