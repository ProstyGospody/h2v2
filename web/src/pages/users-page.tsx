import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRightLeft,
  HardDriveDownload,
  KeyRound,
  Loader2,
  MoreHorizontal,
  Pencil,
  Plus,
  Power,
  PowerOff,
  QrCode,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";

import { ClientArtifactsDialog } from "@/components/dialogs/client-artifacts-dialog";
import { ClientFormDialog } from "@/components/forms/client-form-dialog";
import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { expireToISO, trafficLimitToBytes } from "@/domain/clients/adapters";
import {
  applyAccessBulkPatch,
  applyClientsBulkPatch,
  createClient,
  getClientArtifacts,
  listClients,
  previewAccessBulkPatch,
  previewClientsBulkPatch,
  refreshClientArtifacts,
  updateClient,
} from "@/domain/clients/services";
import type {
  BulkAccessPatch,
  BulkMutationResult,
  BulkUserPatch,
  Client,
  ClientArtifacts,
  ClientFormValues,
} from "@/domain/clients/types";
import {
  applyServerConfig,
  getServerDraftState,
  listInbounds,
  listServers,
  validateServerConfig,
} from "@/domain/inbounds/services";
import type { Inbound, Server as ServerType } from "@/domain/inbounds/types";
import { listClientProfiles } from "@/domain/policy/services";
import { getAPIErrorMessage } from "@/services/api";
import type { ChangeImpact, ClientProfile, DraftRevisionState } from "@/types/common";
import {
  Badge,
  Button,
  Checkbox,
  Dialog,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Input,
  SelectField,
  Toggle,
  cn,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { queryRefetchInterval } from "@/src/queries/polling";
import { formatBytes, formatDateTime } from "@/utils/format";

const PAGE_SIZE = 30;
const EXPIRE_SOON_DAYS = 7;
const SECONDS_PER_DAY = 86_400;
const EMPTY_CLIENTS: Client[] = [];
const EMPTY_INBOUNDS: Inbound[] = [];
const EMPTY_SERVERS: ServerType[] = [];
const EMPTY_CLIENT_PROFILES: ClientProfile[] = [];

type StatusFilter = "active" | "all" | "disabled";
type ExpirationFilter = "active" | "all" | "expired";
type BinaryFilter = "all" | "no" | "yes";
type MutationScope = "access" | "users";
type DialogAction =
  | ""
  | "change-inbound"
  | "client-profile"
  | "delete"
  | "disable"
  | "enable"
  | "extend"
  | "regenerate"
  | "rotate"
  | "traffic-limit";
type UserActionPreset = "delete" | "disable" | "edit" | "enable" | "regenerate" | "rotate-token";
type AccessActionPreset =
  | "delete"
  | "disable"
  | "edit"
  | "enable"
  | "regenerate"
  | "rotate-credentials";
type MutationDialogState =
  | { ids: string[]; preset?: AccessActionPreset; scope: "access" }
  | { ids: string[]; preset?: UserActionPreset; scope: "users" }
  | null;

const totalTraffic = (client: Client) => client.traffic_used_up_bytes + client.traffic_used_down_bytes;
const trafficPercent = (client: Client) =>
  client.traffic_limit_bytes <= 0 ? 0 : Math.min(100, (totalTraffic(client) / client.traffic_limit_bytes) * 100);

type ExpireState = "expired" | "none" | "ok" | "soon";

function expireState(expireAt: string | null): ExpireState {
  if (!expireAt) return "none";
  const diff = new Date(expireAt).getTime() - Date.now();
  if (diff <= 0) return "expired";
  if (diff < EXPIRE_SOON_DAYS * SECONDS_PER_DAY * 1000) return "soon";
  return "ok";
}

function clientHasTrafficLimit(client: Client) {
  if (client.traffic_limit_bytes > 0) return true;
  return client.access.some((item) => (item.traffic_limit_bytes_override ?? 0) > 0);
}

function userPresetToAction(preset?: UserActionPreset): DialogAction {
  switch (preset) {
    case "enable":
      return "enable";
    case "disable":
      return "disable";
    case "rotate-token":
      return "rotate";
    case "regenerate":
      return "regenerate";
    case "delete":
      return "delete";
    default:
      return "";
  }
}

function accessPresetToAction(preset?: AccessActionPreset): DialogAction {
  switch (preset) {
    case "enable":
      return "enable";
    case "disable":
      return "disable";
    case "rotate-credentials":
      return "rotate";
    case "regenerate":
      return "regenerate";
    case "delete":
      return "delete";
    default:
      return "";
  }
}

function mergeDraftStates(next: DraftRevisionState[], prev: DraftRevisionState[]) {
  const merged = new Map(prev.map((item) => [item.server_id, item]));
  for (const item of next) {
    merged.set(item.server_id, item);
  }
  return Array.from(merged.values()).filter(
    (item) => item.pending_changes || !!item.apply_error || !!item.check_error,
  );
}

function StatusDot({ active }: { active: boolean }) {
  return (
    <span
      className={cn(
        "inline-block h-1.5 w-1.5 rounded-full",
        active ? "bg-status-success" : "bg-txt-muted/60",
      )}
    />
  );
}

function TrafficBar({ client }: { client: Client }) {
  const used = totalTraffic(client);
  const pct = trafficPercent(client);
  const limited = client.traffic_limit_bytes > 0;
  const danger = pct >= 90;
  const warn = pct >= 70;

  if (!limited) {
    return <span className="text-[15px] text-txt-secondary tabular-nums">{formatBytes(used)}</span>;
  }
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <div className="flex items-baseline gap-1 text-[14px] tabular-nums">
        <span
          className={cn(
            "font-medium",
            danger ? "text-status-danger" : warn ? "text-status-warning" : "text-txt-primary",
          )}
        >
          {formatBytes(used)}
        </span>
        <span className="text-txt-muted">/ {formatBytes(client.traffic_limit_bytes)}</span>
      </div>
      <div className="h-1 w-full overflow-hidden rounded-full bg-surface-3/60">
        <div
          className={cn(
            "h-full rounded-full transition-all duration-500",
            danger ? "bg-status-danger" : warn ? "bg-status-warning" : "bg-accent",
          )}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function ExpireLabel({ expireAt }: { expireAt: string | null }) {
  const state = expireState(expireAt);
  if (state === "none") return <span className="text-[15px] text-txt-muted">-</span>;
  const text = formatDateTime(expireAt, { includeSeconds: false });
  return (
    <span
      className={cn(
        "text-[15px] tabular-nums",
        state === "expired" && "text-status-danger",
        state === "soon" && "text-status-warning",
        state === "ok" && "text-txt-secondary",
      )}
    >
      {text}
    </span>
  );
}

function StatusSegment({
  value,
  counts,
  onChange,
}: {
  value: StatusFilter;
  counts: Record<StatusFilter, number>;
  onChange: (value: StatusFilter) => void;
}) {
  const options: { key: StatusFilter; label: string }[] = [
    { key: "all", label: "All" },
    { key: "active", label: "Active" },
    { key: "disabled", label: "Disabled" },
  ];
  return (
    <div role="radiogroup" className="flex items-center gap-0.5">
      {options.map((option) => (
        <button
          key={option.key}
          role="radio"
          aria-checked={value === option.key}
          type="button"
          onClick={() => onChange(option.key)}
          className={cn(
            "rounded-md px-2.5 py-1.5 text-[14px] font-medium transition-colors",
            value === option.key
              ? "bg-surface-3/70 text-txt-primary"
              : "text-txt-secondary hover:text-txt-primary",
          )}
        >
          {option.label}
          <span className="ml-1.5 text-txt-muted tabular-nums">{counts[option.key]}</span>
        </button>
      ))}
    </div>
  );
}

function ImpactSummary({ impact }: { impact: ChangeImpact }) {
  return (
    <div className="space-y-3 rounded-2xl bg-surface-1/50 px-4 py-4 shadow-[inset_0_0_0_1px_var(--control-border)]">
      <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Impact</div>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <div>
          <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Users</div>
          <div className="mt-1 text-[16px] font-semibold text-txt-primary">{impact.affected_users}</div>
        </div>
        <div>
          <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Access</div>
          <div className="mt-1 text-[16px] font-semibold text-txt-primary">{impact.affected_access}</div>
        </div>
        <div>
          <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Inbounds</div>
          <div className="mt-1 text-[16px] font-semibold text-txt-primary">{impact.affected_inbounds}</div>
        </div>
        <div>
          <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Subscriptions</div>
          <div className="mt-1 text-[16px] font-semibold text-txt-primary">{impact.affected_subscriptions}</div>
        </div>
        <div>
          <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Artifacts</div>
          <div className="mt-1 text-[16px] font-semibold text-txt-primary">{impact.affected_artifacts}</div>
        </div>
        <div>
          <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">Runtime</div>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <Badge variant={impact.requires_runtime_apply ? "warning" : "default"}>
              {impact.requires_runtime_apply ? "Draft update" : "No runtime"}
            </Badge>
            {impact.requires_artifact_refresh ? (
              <Badge variant="warning">Artifacts refresh</Badge>
            ) : (
              <Badge variant="default">Artifacts stable</Badge>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function DraftStatesPanel({
  applyBusy,
  drafts,
  onApply,
  onValidate,
  serverByID,
  validateBusy,
}: {
  applyBusy: Record<string, boolean>;
  drafts: DraftRevisionState[];
  onApply: (draft: DraftRevisionState) => void;
  onValidate: (draft: DraftRevisionState) => void;
  serverByID: Map<string, ServerType>;
  validateBusy: Record<string, boolean>;
}) {
  if (drafts.length === 0) return null;

  return (
    <div className="rounded-2xl bg-surface-2 px-5 py-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-[17px] font-semibold text-txt-primary">Pending draft</h2>
          <p className="mt-1 text-[15px] text-txt-secondary">Preview, validate, apply</p>
        </div>
        <Link
          to="/settings"
          className="inline-flex items-center gap-2 rounded-xl bg-[var(--control-bg)] px-4 py-2 text-[15px] font-semibold text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] transition-colors hover:bg-[var(--control-bg-hover)]"
        >
          Open Settings
        </Link>
      </div>
      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        {drafts.map((draft) => {
          const server = serverByID.get(draft.server_id);
          const statusVariant = draft.check_error
            ? "danger"
            : draft.check_ok
              ? "success"
              : "warning";
          const statusLabel = draft.check_error ? "Check failed" : draft.check_ok ? "Valid" : "Pending";
          return (
            <div
              key={draft.server_id}
              className="rounded-2xl bg-surface-1/45 px-4 py-4 shadow-[inset_0_0_0_1px_var(--control-border)]"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-[16px] font-semibold text-txt-primary">
                    {server?.name || draft.server_id}
                  </div>
                  <div className="mt-1 text-[14px] text-txt-secondary">
                    Applied #{draft.current_revision_no ?? "-"} / Draft #{draft.draft_revision_no ?? "-"}
                  </div>
                </div>
                <Badge variant={statusVariant}>{statusLabel}</Badge>
              </div>
              {draft.check_error ? (
                <div className="mt-3 rounded-xl bg-status-danger/10 px-3 py-2 text-[14px] text-status-danger">
                  {draft.check_error}
                </div>
              ) : null}
              {draft.apply_error ? (
                <div className="mt-3 rounded-xl bg-status-danger/10 px-3 py-2 text-[14px] text-status-danger">
                  {draft.apply_error}
                </div>
              ) : null}
              <div className="mt-4 flex flex-wrap items-center gap-2">
                <Button
                  size="sm"
                  onClick={() => onValidate(draft)}
                  disabled={!draft.draft_revision_id || validateBusy[draft.server_id]}
                >
                  {validateBusy[draft.server_id] ? (
                    <Loader2 size={13} className="animate-spin" />
                  ) : (
                    "Validate"
                  )}
                </Button>
                <Button
                  size="sm"
                  variant="primary"
                  onClick={() => onApply(draft)}
                  disabled={!draft.draft_revision_id || !draft.check_ok || applyBusy[draft.server_id]}
                >
                  {applyBusy[draft.server_id] ? <Loader2 size={13} className="animate-spin" /> : "Apply"}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function MutationDialog({
  clientProfiles,
  inbounds,
  onApplied,
  onClose,
  open,
  state,
}: {
  clientProfiles: ClientProfile[];
  inbounds: Inbound[];
  onApplied: (result: BulkMutationResult, scope: MutationScope, ids: string[]) => void;
  onClose: () => void;
  open: boolean;
  state: MutationDialogState;
}) {
  const toast = useToast();
  const scope = state?.scope ?? "users";
  const ids = state?.ids ?? [];
  const presetAction =
    scope === "users"
      ? userPresetToAction(state?.scope === "users" ? state.preset : undefined)
      : accessPresetToAction(state?.scope === "access" ? state.preset : undefined);

  const [action, setAction] = useState<DialogAction>("");
  const [extendDays, setExtendDays] = useState("30");
  const [trafficLimitGB, setTrafficLimitGB] = useState("");
  const [clientProfileID, setClientProfileID] = useState("");
  const [inboundID, setInboundID] = useState("");
  const [deleteMode, setDeleteMode] = useState<"hard" | "soft">("soft");
  const [preview, setPreview] = useState<ChangeImpact | null>(null);
  const [previewKey, setPreviewKey] = useState("");
  const [previewBusy, setPreviewBusy] = useState(false);
  const [applyBusy, setApplyBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setAction(presetAction);
    setExtendDays("30");
    setTrafficLimitGB("");
    setClientProfileID("");
    setInboundID("");
    setDeleteMode("soft");
    setPreview(null);
    setPreviewKey("");
    setPreviewBusy(false);
    setApplyBusy(false);
    setError("");
  }, [open, presetAction]);

  const currentPatch = useMemo(() => {
    if (!state || ids.length === 0 || action === "") return null;
    if (scope === "users") {
      const patch: BulkUserPatch = { ids };
      switch (action) {
        case "enable":
          patch.enabled = true;
          break;
        case "disable":
          patch.enabled = false;
          break;
        case "extend": {
          const days = parseInt(extendDays, 10);
          if (!Number.isFinite(days) || days <= 0) return null;
          patch.extend_seconds = days * SECONDS_PER_DAY;
          break;
        }
        case "client-profile":
          if (!clientProfileID) return null;
          patch.client_profile_id = clientProfileID;
          break;
        case "change-inbound":
          if (!inboundID) return null;
          patch.inbound_id = inboundID;
          break;
        case "traffic-limit":
          if (!trafficLimitGB.trim()) return null;
          patch.traffic_limit_bytes = trafficLimitToBytes(trafficLimitGB);
          break;
        case "rotate":
          patch.rotate_tokens = true;
          break;
        case "regenerate":
          patch.regenerate_artifacts = true;
          break;
        case "delete":
          patch.delete_mode = deleteMode;
          break;
      }
      return patch;
    }

    const patch: BulkAccessPatch = { ids };
    switch (action) {
      case "enable":
        patch.enabled = true;
        break;
      case "disable":
        patch.enabled = false;
        break;
      case "extend": {
        const days = parseInt(extendDays, 10);
        if (!Number.isFinite(days) || days <= 0) return null;
        patch.extend_seconds = days * SECONDS_PER_DAY;
        break;
      }
      case "client-profile":
        if (!clientProfileID) return null;
        patch.client_profile_id = clientProfileID;
        break;
      case "change-inbound":
        if (!inboundID) return null;
        patch.inbound_id = inboundID;
        break;
      case "traffic-limit":
        if (!trafficLimitGB.trim()) return null;
        patch.traffic_limit_bytes = trafficLimitToBytes(trafficLimitGB);
        break;
      case "rotate":
        patch.rotate_credentials = true;
        break;
      case "regenerate":
        patch.regenerate_artifacts = true;
        break;
      case "delete":
        patch.delete_mode = deleteMode;
        break;
    }
    return patch;
  }, [action, clientProfileID, deleteMode, extendDays, ids, inboundID, scope, state, trafficLimitGB]);

  const currentKey = currentPatch ? JSON.stringify(currentPatch) : "";
  const canPreview = !!currentPatch && !previewBusy && !applyBusy;
  const canApply = !!currentPatch && previewKey === currentKey && !previewBusy && !applyBusy;

  const actionOptions: { label: string; value: string }[] = [
    { label: "Action", value: "__none__" },
    { label: "Enable", value: "enable" },
    { label: "Disable", value: "disable" },
    { label: "Extend expiration", value: "extend" },
    { label: "Assign client profile", value: "client-profile" },
    { label: "Change inbound", value: "change-inbound" },
    { label: "Update traffic limit", value: "traffic-limit" },
    {
      label: scope === "users" ? "Rotate token" : "Rotate credentials",
      value: "rotate",
    },
    { label: "Regenerate artifacts", value: "regenerate" },
    { label: scope === "users" ? "Delete user" : "Remove access", value: "delete" },
  ];

  async function handlePreview() {
    if (!currentPatch) return;
    setPreviewBusy(true);
    setError("");
    try {
      const next =
        scope === "users"
          ? await previewClientsBulkPatch(currentPatch as BulkUserPatch)
          : await previewAccessBulkPatch(currentPatch as BulkAccessPatch);
      setPreview(next);
      setPreviewKey(JSON.stringify(currentPatch));
    } catch (err) {
      setError(getAPIErrorMessage(err, "Preview failed"));
    } finally {
      setPreviewBusy(false);
    }
  }

  async function handleApply() {
    if (!currentPatch) return;
    setApplyBusy(true);
    setError("");
    try {
      const result =
        scope === "users"
          ? await applyClientsBulkPatch(currentPatch as BulkUserPatch)
          : await applyAccessBulkPatch(currentPatch as BulkAccessPatch);
      onApplied(result, scope, ids);
      onClose();
    } catch (err) {
      const message = getAPIErrorMessage(err, "Apply failed");
      setError(message);
      toast.notify(message, "error");
    } finally {
      setApplyBusy(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!value && !previewBusy && !applyBusy) onClose();
      }}
      title={scope === "users" ? "Bulk users" : "Bulk access"}
      contentClassName="max-w-[720px]"
      footer={
        <>
          <Button onClick={onClose} disabled={previewBusy || applyBusy}>
            Cancel
          </Button>
          <Button onClick={() => void handlePreview()} disabled={!canPreview}>
            {previewBusy ? <Loader2 size={14} className="animate-spin" /> : "Preview"}
          </Button>
          <Button variant="primary" onClick={() => void handleApply()} disabled={!canApply}>
            {applyBusy ? <Loader2 size={14} className="animate-spin" /> : "Apply"}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <div className="rounded-2xl bg-surface-1/50 px-4 py-3 text-[15px] font-medium text-txt-secondary shadow-[inset_0_0_0_1px_var(--control-border)]">
          {ids.length} selected
        </div>

        {error ? (
          <div className="rounded-2xl bg-status-danger/10 px-4 py-3 text-[15px] text-status-danger">
            {error}
          </div>
        ) : null}

        <SelectField
          label="Action"
          value={action || "__none__"}
          onValueChange={(value) => setAction(value === "__none__" ? "" : (value as DialogAction))}
          options={actionOptions}
        />

        {action === "extend" ? (
          <Input
            label="Days"
            type="number"
            min="1"
            step="1"
            value={extendDays}
            onChange={(event) => setExtendDays(event.target.value)}
          />
        ) : null}

        {action === "traffic-limit" ? (
          <Input
            label="Limit (GB)"
            type="number"
            min="0"
            step="0.1"
            value={trafficLimitGB}
            onChange={(event) => setTrafficLimitGB(event.target.value)}
          />
        ) : null}

        {action === "client-profile" ? (
          <SelectField
            label="Client profile"
            value={clientProfileID || "__none__"}
            onValueChange={(value) => setClientProfileID(value === "__none__" ? "" : value)}
            options={[
              { label: "Select profile", value: "__none__" },
              ...clientProfiles.map((item) => ({ label: item.name, value: item.id })),
            ]}
          />
        ) : null}

        {action === "change-inbound" ? (
          <SelectField
            label="Inbound"
            value={inboundID || "__none__"}
            onValueChange={(value) => setInboundID(value === "__none__" ? "" : value)}
            options={[
              { label: "Select inbound", value: "__none__" },
              ...inbounds.map((item) => ({ label: item.name, value: item.id })),
            ]}
          />
        ) : null}

        {action === "delete" ? (
          <SelectField
            label="Mode"
            value={deleteMode}
            onValueChange={(value) => setDeleteMode(value === "hard" ? "hard" : "soft")}
            options={[
              { label: scope === "users" ? "Soft delete" : "Disable access", value: "soft" },
              { label: scope === "users" ? "Hard delete" : "Remove access", value: "hard" },
            ]}
          />
        ) : null}

        {preview ? <ImpactSummary impact={preview} /> : null}
      </div>
    </Dialog>
  );
}

export default function UsersPage() {
  const qc = useQueryClient();
  const toast = useToast();
  const searchRef = useRef<HTMLInputElement>(null);

  const usersQuery = useQuery({
    queryKey: ["clients"],
    queryFn: listClients,
    refetchInterval: (query) => queryRefetchInterval(10_000, query),
  });
  const inboundsQuery = useQuery({
    queryKey: ["settings", "inbounds"],
    queryFn: () => listInbounds(),
  });
  const serversQuery = useQuery({
    queryKey: ["settings", "servers"],
    queryFn: listServers,
  });
  const clientProfilesQuery = useQuery({
    queryKey: ["settings", "client-profiles"],
    queryFn: () => listClientProfiles(),
  });

  const clients = usersQuery.data ?? EMPTY_CLIENTS;
  const inbounds = inboundsQuery.data ?? EMPTY_INBOUNDS;
  const servers = serversQuery.data ?? EMPTY_SERVERS;
  const clientProfiles = clientProfilesQuery.data ?? EMPTY_CLIENT_PROFILES;

  const inboundByID = useMemo(() => new Map(inbounds.map((item) => [item.id, item])), [inbounds]);
  const serverByID = useMemo(() => new Map(servers.map((item) => [item.id, item])), [servers]);

  const counts = useMemo<Record<StatusFilter, number>>(
    () => ({
      all: clients.length,
      active: clients.filter((client) => client.enabled).length,
      disabled: clients.filter((client) => !client.enabled).length,
    }),
    [clients],
  );

  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [protocolFilter, setProtocolFilter] = useState("all");
  const [inboundFilter, setInboundFilter] = useState("all");
  const [serverFilter, setServerFilter] = useState("all");
  const [expirationFilter, setExpirationFilter] = useState<ExpirationFilter>("all");
  const [subscriptionFilter, setSubscriptionFilter] = useState<BinaryFilter>("all");
  const [trafficFilter, setTrafficFilter] = useState<BinaryFilter>("all");
  const [clientProfileFilter, setClientProfileFilter] = useState("all");
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [drafts, setDrafts] = useState<DraftRevisionState[]>([]);
  const [validateBusy, setValidateBusy] = useState<Record<string, boolean>>({});
  const [applyBusy, setApplyBusy] = useState<Record<string, boolean>>({});
  const [mutationDialog, setMutationDialog] = useState<MutationDialogState>(null);

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState("");
  const [editingClient, setEditingClient] = useState<Client | null>(null);

  const [detailsOpen, setDetailsOpen] = useState(false);
  const [detailsClient, setDetailsClient] = useState<Client | null>(null);
  const [artifactsData, setArtifactsData] = useState<ClientArtifacts | null>(null);
  const [artifactsLoading, setArtifactsLoading] = useState(false);
  const [artifactsRefreshing, setArtifactsRefreshing] = useState(false);

  const refreshAllDrafts = useCallback(async () => {
    if (servers.length === 0) {
      setDrafts((prev) => (prev.length === 0 ? prev : []));
      return;
    }
    try {
      const states = await Promise.all(servers.map((server) => getServerDraftState(server.id)));
      setDrafts(states.filter((item) => item.pending_changes || !!item.check_error || !!item.apply_error));
    } catch {
      // Ignore draft polling failures here; action handlers surface explicit errors.
    }
  }, [servers]);

  useEffect(() => {
    if (!detailsClient) return;
    const next = clients.find((item) => item.id === detailsClient.id) ?? null;
    if (next) {
      setDetailsClient(next);
      return;
    }
    if (!usersQuery.isLoading) {
      setDetailsClient(null);
      setDetailsOpen(false);
    }
  }, [clients, detailsClient, usersQuery.isLoading]);

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return clients
      .filter((client) => {
        if (status === "active" && !client.enabled) return false;
        if (status === "disabled" && client.enabled) return false;
        if (protocolFilter !== "all" && !client.access.some((item) => item.protocol === protocolFilter)) {
          return false;
        }
        if (inboundFilter !== "all" && !client.access.some((item) => item.inbound_id === inboundFilter)) {
          return false;
        }
        if (
          serverFilter !== "all" &&
          !client.access.some((item) => inboundByID.get(item.inbound_id)?.server_id === serverFilter)
        ) {
          return false;
        }
        if (expirationFilter === "expired" && expireState(client.expire_at) !== "expired") return false;
        if (
          expirationFilter === "active" &&
          client.expire_at &&
          expireState(client.expire_at) === "expired"
        ) {
          return false;
        }
        if (subscriptionFilter === "yes" && !client.has_subscription) return false;
        if (subscriptionFilter === "no" && client.has_subscription) return false;
        if (trafficFilter === "yes" && !clientHasTrafficLimit(client)) return false;
        if (trafficFilter === "no" && clientHasTrafficLimit(client)) return false;
        if (
          clientProfileFilter !== "all" &&
          !client.access.some((item) => item.client_profile_id === clientProfileFilter)
        ) {
          return false;
        }
        if (query) {
          const haystack = [
            client.username,
            ...client.protocols,
            ...client.access.map((item) => item.display_name || ""),
            ...client.access.map((item) => item.description || ""),
            ...client.access.map((item) => item.last_client_ip || ""),
            ...client.access.map((item) => inboundByID.get(item.inbound_id)?.name || item.inbound_id),
          ]
            .join(" ")
            .toLowerCase();
          if (!haystack.includes(query)) return false;
        }
        return true;
      })
      .sort((left, right) => left.username.localeCompare(right.username));
  }, [
    clientProfileFilter,
    clients,
    expirationFilter,
    inboundByID,
    inboundFilter,
    protocolFilter,
    search,
    serverFilter,
    status,
    subscriptionFilter,
    trafficFilter,
  ]);

  useEffect(() => {
    setPage(0);
  }, [
    search,
    status,
    protocolFilter,
    inboundFilter,
    serverFilter,
    expirationFilter,
    subscriptionFilter,
    trafficFilter,
    clientProfileFilter,
  ]);

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const visible = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  useEffect(() => {
    setPage((value) => Math.min(value, Math.max(0, pageCount - 1)));
  }, [pageCount]);

  const allOnPageSelected = visible.length > 0 && visible.every((client) => selected.has(client.id));
  const someOnPageSelected =
    visible.some((client) => selected.has(client.id)) && !allOnPageSelected;

  const selectedIDs = useMemo(() => Array.from(selected), [selected]);

  function toggleAllOnPage(checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      for (const client of visible) {
        if (checked) next.add(client.id);
        else next.delete(client.id);
      }
      return next;
    });
  }

  function toggleOne(id: string, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  function clearSelection() {
    setSelected(new Set());
  }

  const openCreate = useCallback(() => {
    setEditingClient(null);
    setFormMode("create");
    setFormError("");
    setFormOpen(true);
  }, []);

  const openEdit = useCallback((client: Client) => {
    setEditingClient(client);
    setFormMode("edit");
    setFormError("");
    setFormOpen(true);
  }, []);

  const openUserMutation = useCallback((ids: string[], preset?: UserActionPreset) => {
    setMutationDialog({ scope: "users", ids, preset });
  }, []);

  const openAccessMutation = useCallback((ids: string[], preset?: AccessActionPreset) => {
    setMutationDialog({ scope: "access", ids, preset });
  }, []);

  const loadArtifacts = useCallback(
    async (client: Client, mode: "load" | "refresh") => {
      if (mode === "load") {
        setArtifactsLoading(true);
        setArtifactsData(null);
      } else {
        setArtifactsRefreshing(true);
      }
      try {
        const data =
          mode === "load"
            ? await getClientArtifacts(client.id)
            : await refreshClientArtifacts(client.id);
        setArtifactsData(data);
      } catch (err) {
        toast.notify(getAPIErrorMessage(err, "Failed to load"), "error");
      } finally {
        if (mode === "load") setArtifactsLoading(false);
        else setArtifactsRefreshing(false);
      }
    },
    [toast],
  );

  const openDetails = useCallback(
    async (client: Client) => {
      setDetailsClient(client);
      setDetailsOpen(true);
      await loadArtifacts(client, "load");
    },
    [loadArtifacts],
  );

  const handleRefreshArtifacts = useCallback(async () => {
    if (!detailsClient) return;
    await loadArtifacts(detailsClient, "refresh");
    await qc.invalidateQueries({ queryKey: ["clients"] });
  }, [detailsClient, loadArtifacts, qc]);

  async function submitForm(values: ClientFormValues) {
    setFormBusy(true);
    setFormError("");
    try {
      const payload = {
        username: values.username.trim(),
        traffic_limit_bytes: trafficLimitToBytes(values.traffic_limit_gb),
        expire_at: expireToISO(values.expire_at),
      };
      if (formMode === "create") {
        await createClient(payload);
        toast.notify(`Created ${payload.username}`);
      } else if (editingClient) {
        await updateClient(editingClient.id, payload);
        toast.notify(`Updated ${payload.username}`);
      }
      await qc.invalidateQueries({ queryKey: ["clients"] });
      await refreshAllDrafts();
      setFormOpen(false);
      setEditingClient(null);
    } catch (err) {
      setFormError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setFormBusy(false);
    }
  }

  async function handleMutationApplied(
    result: BulkMutationResult,
    scope: MutationScope,
    ids: string[],
  ) {
    const parts: string[] = [];
    if (result.updated > 0) parts.push(`${result.updated} updated`);
    if (result.deleted > 0) parts.push(`${result.deleted} deleted`);
    if (result.rotated > 0) parts.push(`${result.rotated} rotated`);
    if (result.regenerated > 0) parts.push(`${result.regenerated} regenerated`);
    toast.notify(parts.join(", ") || "Done");

    await qc.invalidateQueries({ queryKey: ["clients"] });
    if (scope === "users") {
      clearSelection();
    }

    if (result.drafts && result.drafts.length > 0) {
      setDrafts((prev) => mergeDraftStates(result.drafts ?? [], prev));
    } else {
      await refreshAllDrafts();
    }

    if (
      detailsClient &&
      (scope === "users"
        ? ids.includes(detailsClient.id)
        : detailsClient.access.some((item) => ids.includes(item.id)))
    ) {
      await loadArtifacts(detailsClient, "load");
    }
  }

  async function handleValidateDraft(draft: DraftRevisionState) {
    if (!draft.draft_revision_id) return;
    setValidateBusy((prev) => ({ ...prev, [draft.server_id]: true }));
    try {
      await validateServerConfig(draft.server_id, draft.draft_revision_id);
      const next = await getServerDraftState(draft.server_id);
      setDrafts((prev) => mergeDraftStates([next], prev));
      toast.notify("Validated");
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Validate failed"), "error");
    } finally {
      setValidateBusy((prev) => ({ ...prev, [draft.server_id]: false }));
    }
  }

  async function handleApplyDraft(draft: DraftRevisionState) {
    if (!draft.draft_revision_id) return;
    setApplyBusy((prev) => ({ ...prev, [draft.server_id]: true }));
    try {
      await applyServerConfig(draft.server_id, draft.draft_revision_id);
      const next = await getServerDraftState(draft.server_id);
      setDrafts((prev) => mergeDraftStates([next], prev));
      toast.notify("Applied");
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Apply failed"), "error");
    } finally {
      setApplyBusy((prev) => ({ ...prev, [draft.server_id]: false }));
    }
  }

  useEffect(() => {
    void refreshAllDrafts();
  }, [refreshAllDrafts]);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      if (formOpen || detailsOpen || mutationDialog) return;
      if (selected.size > 0) {
        clearSelection();
        return;
      }
      if (search) setSearch("");
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [detailsOpen, formOpen, mutationDialog, search, selected.size]);

  const isLoading = usersQuery.isLoading;
  const isError = usersQuery.isError;

  const inboundOptions = useMemo(
    () => [{ label: "All inbounds", value: "all" }, ...inbounds.map((item) => ({ label: item.name, value: item.id }))],
    [inbounds],
  );
  const serverOptions = useMemo(
    () => [{ label: "All servers", value: "all" }, ...servers.map((item) => ({ label: item.name, value: item.id }))],
    [servers],
  );
  const clientProfileOptions = useMemo(
    () => [
      { label: "All profiles", value: "all" },
      ...clientProfiles.map((item) => ({ label: item.name, value: item.id })),
    ],
    [clientProfiles],
  );

  return (
    <div className="space-y-5 pb-24">
      <PageHeader
        title="Users"
        actions={
          <Button variant="primary" onClick={openCreate}>
            <Plus size={16} strokeWidth={2} /> New user
          </Button>
        }
      />

      <DraftStatesPanel
        drafts={drafts}
        serverByID={serverByID}
        onValidate={(draft) => void handleValidateDraft(draft)}
        onApply={(draft) => void handleApplyDraft(draft)}
        validateBusy={validateBusy}
        applyBusy={applyBusy}
      />

      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative min-w-[240px] flex-1 sm:max-w-[360px]">
            <Search
              size={14}
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-txt-muted"
            />
            <input
              ref={searchRef}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search"
              className="w-full rounded-lg bg-surface-2/50 py-2 pl-9 pr-8 text-[15px] font-medium text-txt-primary outline-none transition-colors placeholder:text-txt-tertiary focus:bg-surface-2/80"
            />
            {search ? (
              <button
                type="button"
                onClick={() => setSearch("")}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-txt-muted hover:text-txt-primary"
                aria-label="Clear"
              >
                <X size={13} />
              </button>
            ) : null}
          </div>
          <StatusSegment value={status} counts={counts} onChange={setStatus} />
        </div>

        <div className="grid gap-3 lg:grid-cols-3 xl:grid-cols-6">
          <SelectField
            label="Protocol"
            value={protocolFilter}
            onValueChange={setProtocolFilter}
            options={[
              { label: "All protocols", value: "all" },
              { label: "VLESS", value: "vless" },
              { label: "HY2", value: "hy2" },
            ]}
          />
          <SelectField
            label="Inbound"
            value={inboundFilter}
            onValueChange={setInboundFilter}
            options={inboundOptions}
          />
          <SelectField
            label="Server"
            value={serverFilter}
            onValueChange={setServerFilter}
            options={serverOptions}
          />
          <SelectField
            label="Expiration"
            value={expirationFilter}
            onValueChange={(value) => setExpirationFilter(value as ExpirationFilter)}
            options={[
              { label: "All", value: "all" },
              { label: "Active", value: "active" },
              { label: "Expired", value: "expired" },
            ]}
          />
          <SelectField
            label="Subscription"
            value={subscriptionFilter}
            onValueChange={(value) => setSubscriptionFilter(value as BinaryFilter)}
            options={[
              { label: "All", value: "all" },
              { label: "Yes", value: "yes" },
              { label: "No", value: "no" },
            ]}
          />
          <SelectField
            label="Traffic limit"
            value={trafficFilter}
            onValueChange={(value) => setTrafficFilter(value as BinaryFilter)}
            options={[
              { label: "All", value: "all" },
              { label: "Limited", value: "yes" },
              { label: "Unlimited", value: "no" },
            ]}
          />
          <SelectField
            label="Client profile"
            value={clientProfileFilter}
            onValueChange={setClientProfileFilter}
            options={clientProfileOptions}
          />
        </div>
      </div>

      {isError ? (
        <ErrorBanner
          message={getAPIErrorMessage(usersQuery.error, "Failed to load")}
          actionLabel="Retry"
          onAction={() => usersQuery.refetch()}
        />
      ) : null}

      <div className="overflow-hidden rounded-2xl bg-surface-2">
        <div className="overflow-x-auto">
          <div className="min-w-[1080px]">
            <div className="flex items-center gap-4 px-5 py-3 text-[13px] font-semibold uppercase tracking-wider text-txt-muted">
              <div className="w-5">
                <Checkbox
                  checked={allOnPageSelected ? true : someOnPageSelected ? "indeterminate" : false}
                  onCheckedChange={(value) => toggleAllOnPage(Boolean(value))}
                  aria-label="Select page"
                />
              </div>
              <div className="min-w-[220px] flex-1">User</div>
              <div className="w-[220px]">Access</div>
              <div className="w-[90px]">Status</div>
              <div className="w-[160px]">Traffic</div>
              <div className="w-[140px]">Expires</div>
              <div className="w-[160px]">Subscription</div>
              <div className="w-8" />
            </div>

            {isLoading ? (
              <div className="space-y-px">
                {Array.from({ length: 6 }).map((_, index) => (
                  <div key={index} className="flex items-center gap-4 px-5 py-4">
                    <div className="h-4 w-4 rounded bg-surface-3/60" />
                    <div className="h-3.5 flex-1 max-w-[160px] animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-32 animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-16 animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-24 animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-20 animate-pulse rounded bg-surface-3/60" />
                    <div className="h-3 w-24 animate-pulse rounded bg-surface-3/60" />
                    <div className="w-8" />
                  </div>
                ))}
              </div>
            ) : visible.length === 0 ? (
              <div className="flex flex-col items-center gap-3 px-5 py-16 text-center text-txt-muted">
                <p className="text-[16px]">
                  {search || status !== "all" ? "Nothing matches." : "No users yet."}
                </p>
                {!search && status === "all" ? (
                  <Button variant="primary" size="sm" onClick={openCreate}>
                    <Plus size={14} /> Create user
                  </Button>
                ) : null}
              </div>
            ) : (
              <div>
                {visible.map((client) => {
                  const accessProfiles = Array.from(
                    new Set(
                      client.access
                        .map((item) => item.client_profile_id)
                        .filter((value): value is string => !!value)
                        .map((id) => clientProfiles.find((profile) => profile.id === id)?.name || id),
                    ),
                  );
                  const accessInbounds = Array.from(
                    new Set(
                      client.access
                        .map((item) => inboundByID.get(item.inbound_id)?.name || item.inbound_id)
                        .filter(Boolean),
                    ),
                  );
                  const isSelected = selected.has(client.id);
                  const subscriptionLabel = client.has_subscription
                    ? client.artifacts_need_refresh
                      ? "Refresh"
                      : "Ready"
                    : "None";
                  const subscriptionVariant = client.has_subscription
                    ? client.artifacts_need_refresh
                      ? "warning"
                      : "success"
                    : "default";

                  return (
                    <div
                      key={client.id}
                      className={cn(
                        "group flex items-center gap-4 px-5 py-3.5 transition-colors",
                        isSelected ? "bg-accent/6" : "hover:bg-surface-3/25",
                      )}
                    >
                      <div className="w-5">
                        <Checkbox
                          checked={isSelected}
                          onCheckedChange={(value) => toggleOne(client.id, Boolean(value))}
                          aria-label={`Select ${client.username}`}
                        />
                      </div>

                      <div className="min-w-[220px] flex-1">
                        <div className="flex items-center gap-3">
                          <button
                            type="button"
                            onClick={() => void openDetails(client)}
                            className="truncate text-left text-[16px] font-semibold text-txt-primary hover:text-accent-light"
                          >
                            {client.username}
                          </button>
                          <div className="flex shrink-0 gap-1">
                            {client.protocols.includes("vless") ? (
                              <Badge variant="protocol-vless">VLESS</Badge>
                            ) : null}
                            {client.protocols.includes("hy2") ? (
                              <Badge variant="protocol-hy2">HY2</Badge>
                            ) : null}
                            {client.artifacts_need_refresh ? <Badge variant="warning">Refresh</Badge> : null}
                          </div>
                        </div>
                        <div className="mt-2 flex flex-wrap items-center gap-2 text-[14px] text-txt-secondary">
                          <span>{client.access.length} access</span>
                          {accessProfiles.slice(0, 1).map((label) => (
                            <span key={label} className="truncate rounded-md bg-surface-3/50 px-2 py-0.5">
                              {label}
                            </span>
                          ))}
                        </div>
                      </div>

                      <div className="w-[220px]">
                        <div className="flex flex-wrap gap-1">
                          {accessInbounds.slice(0, 2).map((label) => (
                            <span key={label} className="rounded-md bg-surface-3/50 px-2 py-1 text-[13px] text-txt-secondary">
                              {label}
                            </span>
                          ))}
                          {accessInbounds.length > 2 ? (
                            <span className="rounded-md bg-surface-3/50 px-2 py-1 text-[13px] text-txt-secondary">
                              +{accessInbounds.length - 2}
                            </span>
                          ) : null}
                        </div>
                      </div>

                      <div className="flex w-[90px] items-center gap-2">
                        <Toggle
                          checked={client.enabled}
                          onCheckedChange={() =>
                            openUserMutation([client.id], client.enabled ? "disable" : "enable")
                          }
                        />
                        <StatusDot active={client.enabled} />
                      </div>

                      <div className="w-[160px]">
                        <TrafficBar client={client} />
                      </div>

                      <div className="w-[140px]">
                        <ExpireLabel expireAt={client.expire_at} />
                      </div>

                      <div className="w-[160px]">
                        <div className="flex flex-col gap-1">
                          <Badge variant={subscriptionVariant}>{subscriptionLabel}</Badge>
                          <span className="text-[13px] text-txt-muted">
                            {client.last_artifact_rendered_at
                              ? formatDateTime(client.last_artifact_rendered_at, { includeSeconds: false })
                              : "No render"}
                          </span>
                        </div>
                      </div>

                      <div className="w-8">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <button
                              type="button"
                              aria-label="Actions"
                              className="inline-grid h-8 w-8 place-items-center rounded-lg text-txt-muted opacity-0 transition-opacity hover:bg-surface-3/60 hover:text-txt-primary group-hover:opacity-100 data-[state=open]:opacity-100"
                            >
                              <MoreHorizontal size={16} />
                            </button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent>
                            <DropdownMenuItem
                              icon={<QrCode size={14} />}
                              onSelect={() => void openDetails(client)}
                            >
                              Details
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              icon={<Pencil size={14} />}
                              onSelect={() => openEdit(client)}
                            >
                              Edit
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              icon={client.enabled ? <PowerOff size={14} /> : <Power size={14} />}
                              onSelect={() =>
                                openUserMutation([client.id], client.enabled ? "disable" : "enable")
                              }
                            >
                              {client.enabled ? "Disable" : "Enable"}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              icon={<KeyRound size={14} />}
                              onSelect={() => openUserMutation([client.id], "rotate-token")}
                            >
                              Rotate token
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              icon={<HardDriveDownload size={14} />}
                              onSelect={() => openUserMutation([client.id], "regenerate")}
                            >
                              Regenerate artifacts
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              danger
                              icon={<Trash2 size={14} />}
                              onSelect={() => openUserMutation([client.id], "delete")}
                            >
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>

        {pageCount > 1 && !isLoading ? (
          <div className="flex items-center justify-between px-5 py-3 text-[14px] text-txt-secondary">
            <span className="tabular-nums">
              {page * PAGE_SIZE + 1}-{Math.min((page + 1) * PAGE_SIZE, filtered.length)} of {filtered.length}
            </span>
            <div className="flex items-center gap-1">
              <Button size="sm" disabled={page === 0} onClick={() => setPage((value) => Math.max(0, value - 1))}>
                Prev
              </Button>
              <span className="px-2 tabular-nums">
                {page + 1} / {pageCount}
              </span>
              <Button
                size="sm"
                disabled={page + 1 >= pageCount}
                onClick={() => setPage((value) => Math.min(pageCount - 1, value + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        ) : null}
      </div>

      {selected.size > 0 ? (
        <div className="pointer-events-none fixed inset-x-0 bottom-6 z-40 flex justify-center px-4">
          <div className="pointer-events-auto flex items-center gap-1 rounded-xl bg-surface-2/95 px-3 py-2 shadow-[0_18px_42px_-16px_var(--dialog-shadow)] backdrop-blur-xl">
            <span className="px-2 text-[15px] font-semibold text-txt-primary tabular-nums">
              {selected.size}
            </span>
            <div className="mx-1 h-4 w-px bg-border/40" />
            <Button size="sm" onClick={() => openUserMutation(selectedIDs, "enable")}>
              <Power size={13} /> Enable
            </Button>
            <Button size="sm" onClick={() => openUserMutation(selectedIDs, "disable")}>
              <PowerOff size={13} /> Disable
            </Button>
            <Button size="sm" onClick={() => openUserMutation(selectedIDs, "edit")}>
              <ArrowRightLeft size={13} /> Patch
            </Button>
            <Button size="sm" variant="danger" onClick={() => openUserMutation(selectedIDs, "delete")}>
              <Trash2 size={13} /> Delete
            </Button>
            <div className="mx-1 h-4 w-px bg-border/40" />
            <button
              type="button"
              onClick={clearSelection}
              className="rounded-md p-1.5 text-txt-muted hover:bg-surface-3/60 hover:text-txt-primary"
              aria-label="Clear selection"
            >
              <X size={14} />
            </button>
          </div>
        </div>
      ) : null}

      <MutationDialog
        open={!!mutationDialog}
        state={mutationDialog}
        inbounds={inbounds}
        clientProfiles={clientProfiles}
        onClose={() => setMutationDialog(null)}
        onApplied={(result, scope, ids) => void handleMutationApplied(result, scope, ids)}
      />

      <ClientFormDialog
        open={formOpen}
        mode={formMode}
        busy={formBusy}
        client={editingClient}
        error={formError}
        onClose={() => {
          if (formBusy) return;
          setFormOpen(false);
          setEditingClient(null);
        }}
        onSubmit={submitForm}
      />

      <ClientArtifactsDialog
        open={detailsOpen}
        client={detailsClient}
        artifacts={artifactsData}
        loading={artifactsLoading}
        refreshing={artifactsRefreshing}
        inbounds={inbounds}
        clientProfiles={clientProfiles}
        onClose={() => setDetailsOpen(false)}
        onRefreshArtifacts={() => void handleRefreshArtifacts()}
        onOpenUserAction={(ids, preset) => {
          if (preset === "edit" && ids.length === 1) {
            const client = clients.find((item) => item.id === ids[0]);
            if (client) {
              openEdit(client);
              return;
            }
          }
          openUserMutation(ids, preset);
        }}
        onOpenAccessAction={(ids, preset) => openAccessMutation(ids, preset)}
      />
    </div>
  );
}
