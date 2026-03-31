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

import { type ClientFilter } from "./users-utils";

type UsersToolbarProps = {
  searchInput: string;
  searchQuery: string;
  filter: ClientFilter;
  filteredClientsCount: number;
  rowsPerPage: number;
  rowsPerPageOptions: number[];
  searchInputRef: RefObject<HTMLInputElement | null>;
  hasUsersToExport: boolean;
  onCreate: () => void;
  onExportCSV: () => void;
  onSearchInputChange: (value: string) => void;
  onFilterChange: (value: ClientFilter) => void;
  onRowsPerPageChange: (value: string) => void;
};

export function UsersToolbar({
  searchInput,
  searchQuery,
  filter,
  filteredClientsCount,
  rowsPerPage,
  rowsPerPageOptions,
  searchInputRef,
  hasUsersToExport,
  onCreate,
  onExportCSV,
  onSearchInputChange,
  onFilterChange,
  onRowsPerPageChange,
}: UsersToolbarProps) {
  return (
    <>
      <PageHeader
        title="Users"
        actions={
          <>
            <Tooltip content="Add user">
              <Button variant="primary" onClick={onCreate} className="header-btn w-full rounded-2xl px-5 sm:w-auto">
                <Plus size={18} strokeWidth={1.6} />
                Add user
              </Button>
            </Tooltip>

            <Tooltip content={!hasUsersToExport ? "No users" : "Export"}>
              <span className="inline-flex w-full sm:w-auto">
                <Button
                  onClick={onExportCSV}
                  disabled={!hasUsersToExport}
                  className={cn("header-btn w-full rounded-2xl px-5 sm:w-auto", !hasUsersToExport && "pointer-events-none")}
                >
                  <Download size={18} strokeWidth={1.6} />
                  Export CSV
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
          </>
        }
      />

      <div className="flex items-center justify-between gap-3 text-[13px] text-txt-secondary">
        <span>{filteredClientsCount} users</span>
        <div className="hidden items-center gap-2 sm:flex">
          <span>Rows:</span>
          <Select value={String(rowsPerPage)} onValueChange={onRowsPerPageChange}>
            <SelectTrigger className="h-9 min-w-[84px] rounded-lg px-3 text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {rowsPerPageOptions.map((value) => (
                <SelectItem key={value} value={String(value)}>
                  {value}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </>
  );
}
