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

function rowKey(item: AuditLogItem): string {
  return `${item.id}:${item.created_at}:${item.action}:${item.entity_type}:${item.entity_id || ""}`;
}

function PayloadCell({ value }: { value: string }) {
  const [expanded, setExpanded] = useState(false);
  const text = value || "{}";
  const isLong = text.length > 150;

  return (
    <div className="max-w-[360px] space-y-1.5">
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

export function AuditTable({ loading, items, hasSourceItems }: AuditTableProps) {
  const emptyMessage = useMemo(() => (hasSourceItems ? "No records" : "No audit records"), [hasSourceItems]);

  return (
    <TableContainer>
      <div className="max-h-[calc(100dvh-21rem)] overflow-auto">
        <Table className="min-w-[940px]" aria-rowcount={items.length + 1} aria-busy={loading}>
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
                    <Badge variant={actionVariant(item.action)} className="px-2 py-0.5 text-[10px]">{item.action}</Badge>
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-[12px] font-medium text-txt-primary">{item.admin_email || "system"}</TableCell>
                  <TableCell className="whitespace-nowrap text-[12px] text-txt-secondary">{entityLabel(item)}</TableCell>
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
    </TableContainer>
  );
}
