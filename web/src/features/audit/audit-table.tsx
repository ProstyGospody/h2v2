import { ChevronDown, ChevronUp } from "lucide-react";
import { useMemo, useState } from "react";

import {
  Badge,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  cn,
} from "@/src/components/ui";
import { type AuditLogItem } from "@/types/common";
import { formatDateTime } from "@/utils/format";

import { actionVariant, entityLabel } from "./audit-utils";

type AuditTableProps = {
  loading: boolean;
  items: AuditLogItem[];
  hasSourceItems: boolean;
};

const SKELETON_ROWS = 9;
const STICKY_HEAD_CLASS = "sticky top-0 z-10 bg-surface-2/96";
const AUDIT_TABLE_MIN_WIDTH = "1120px";

function rowKey(item: AuditLogItem): string {
  return `${item.id}:${item.created_at}:${item.action}:${item.entity_type}:${item.entity_id || ""}`;
}

function PayloadCell({ value, compact = false }: { value: string; compact?: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const text = value || "{}";
  const isLong = text.length > 150;

  return (
    <div className={cn("space-y-1.5", compact ? "max-w-none" : "max-w-[320px]")}>
      <pre
        className={cn(
          "m-0 whitespace-pre-wrap break-words rounded-lg bg-surface-0/45 p-2.5 font-mono text-[12px] leading-5 text-txt-secondary",
          !expanded && isLong && "max-h-[82px] overflow-hidden",
        )}
      >
        {text}
      </pre>
      {isLong ? (
        <button
          type="button"
          onClick={() => setExpanded((current) => !current)}
          className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium text-txt-secondary transition-colors hover:bg-surface-3/45 hover:text-txt"
        >
          {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          {expanded ? "Less" : "More"}
        </button>
      ) : null}
    </div>
  );
}

function SkeletonRow() {
  return (
    <tr className="border-t border-border/30">
      <td className="px-4 py-3"><div className="h-6 w-[132px] animate-pulse rounded bg-surface-3/55" /></td>
      <td className="px-4 py-3"><div className="h-6 w-[82px] animate-pulse rounded bg-surface-3/55" /></td>
      <td className="px-4 py-3"><div className="h-6 w-[136px] animate-pulse rounded bg-surface-3/55" /></td>
      <td className="px-4 py-3"><div className="h-6 w-[150px] animate-pulse rounded bg-surface-3/55" /></td>
      <td className="px-4 py-3"><div className="h-14 w-[310px] animate-pulse rounded-lg bg-surface-3/55" /></td>
    </tr>
  );
}

function MobileSkeletonCard() {
  return (
    <div className="space-y-2.5 rounded-xl bg-surface-0/45 p-3.5 animate-pulse">
      <div className="h-3.5 w-32 rounded bg-surface-3/55" />
      <div className="h-3.5 w-24 rounded bg-surface-3/55" />
      <div className="h-14 rounded-lg bg-surface-3/50" />
    </div>
  );
}

function AuditCard({ item }: { item: AuditLogItem }) {
  return (
    <article className="space-y-2.5 rounded-xl bg-surface-0/45 p-3.5">
      <div className="grid gap-2 sm:grid-cols-2">
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Time</p>
          <p className="mt-1 text-[12px] text-txt-secondary">{formatDateTime(item.created_at)}</p>
        </div>
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Action</p>
          <div className="mt-1">
            <Badge variant={actionVariant(item.action)} className="px-2 py-0.5 text-[10px]">{item.action}</Badge>
          </div>
        </div>
      </div>

      <div className="grid gap-2 sm:grid-cols-2">
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Actor</p>
          <p className="mt-1 text-[12px] text-txt-primary">{item.admin_email || "system"}</p>
        </div>
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wide text-txt-muted">Entity</p>
          <p className="mt-1 text-[12px] text-txt-secondary">{entityLabel(item)}</p>
        </div>
      </div>

      <div>
        <p className="mb-1 text-[11px] font-medium uppercase tracking-wide text-txt-muted">Payload</p>
        <PayloadCell value={item.payload_json} compact />
      </div>
    </article>
  );
}

export function AuditTable({ loading, items, hasSourceItems }: AuditTableProps) {
  const emptyMessage = useMemo(() => (hasSourceItems ? "No records" : "No audit records"), [hasSourceItems]);

  return (
    <TableContainer>
      <div className="hidden max-h-[calc(100dvh-21rem)] overflow-x-auto overflow-y-scroll xl:block">
        <Table className="table-fixed" style={{ minWidth: AUDIT_TABLE_MIN_WIDTH }} aria-rowcount={items.length + 1} aria-busy={loading}>
          <colgroup>
            <col style={{ width: "180px" }} />
            <col style={{ width: "120px" }} />
            <col style={{ width: "220px" }} />
            <col style={{ width: "200px" }} />
            <col style={{ width: "400px" }} />
          </colgroup>
          <TableHeader className="bg-surface-2/96">
            <TableRow className="border-t-0 hover:bg-transparent">
              <TableHead className={STICKY_HEAD_CLASS}>Time</TableHead>
              <TableHead className={STICKY_HEAD_CLASS}>Action</TableHead>
              <TableHead className={STICKY_HEAD_CLASS}>Actor</TableHead>
              <TableHead className={STICKY_HEAD_CLASS}>Entity</TableHead>
              <TableHead className={STICKY_HEAD_CLASS}>Payload</TableHead>
            </TableRow>
          </TableHeader>

          <TableBody>
            {loading ? (
              Array.from({ length: SKELETON_ROWS }, (_, index) => <SkeletonRow key={index} />)
            ) : items.length ? (
              items.map((item) => (
                <TableRow key={rowKey(item)}>
                  <TableCell className="whitespace-nowrap text-[12px] text-txt-secondary">{formatDateTime(item.created_at)}</TableCell>
                  <TableCell>
                    <Badge variant={actionVariant(item.action)} className="min-w-[72px] justify-center px-2 py-0.5 text-[10px]">{item.action}</Badge>
                  </TableCell>
                  <TableCell className="text-[12px] font-medium text-txt-primary">
                    <span className="block truncate whitespace-nowrap">{item.admin_email || "system"}</span>
                  </TableCell>
                  <TableCell className="text-[12px] text-txt-secondary">
                    <span className="block truncate whitespace-nowrap">{entityLabel(item)}</span>
                  </TableCell>
                  <TableCell>
                    <PayloadCell value={item.payload_json} />
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={5}>{emptyMessage}</TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className="space-y-3 p-3 xl:hidden">
        {loading ? (
          Array.from({ length: SKELETON_ROWS }, (_, index) => <MobileSkeletonCard key={index} />)
        ) : items.length ? (
          items.map((item) => <AuditCard key={rowKey(item)} item={item} />)
        ) : (
          <div className="rounded-xl bg-surface-0/45 px-4 py-6 text-[13px] text-txt-secondary">
            {emptyMessage}
          </div>
        )}
      </div>
    </TableContainer>
  );
}
