import {
  Check,
  Copy,
  HardDriveDownload,
  KeyRound,
  Loader2,
  MoreHorizontal,
  Power,
  PowerOff,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { qrURL, subscriptionQRURL } from "@/domain/clients/services";
import type { Client, ClientAccess, ClientArtifacts } from "@/domain/clients/types";
import type { Inbound } from "@/domain/inbounds/types";
import type { ClientProfile } from "@/types/common";
import { formatBytes, formatDateTime } from "@/utils/format";
import {
  Badge,
  Button,
  Checkbox,
  Drawer,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  cn,
} from "@/src/components/ui";

type Panel = { label: string; qrSrc: string; value: string };

type UserActionKind = "delete" | "disable" | "edit" | "enable" | "regenerate" | "rotate-token";
type AccessActionKind =
  | "delete"
  | "disable"
  | "edit"
  | "enable"
  | "regenerate"
  | "rotate-credentials";

function protocolBadgeVariant(protocol: ClientAccess["protocol"]) {
  return protocol === "vless" ? "protocol-vless" : "protocol-hy2";
}

function accessStatusVariant(access: ClientAccess) {
  if (!access.enabled) return "default";
  if (access.credential_status === "expired") return "danger";
  return "success";
}

function accessExpires(access: ClientAccess) {
  return access.expire_at_override ? formatDateTime(access.expire_at_override, { includeSeconds: false }) : "Inherited";
}

function accessLimit(access: ClientAccess) {
  if (access.traffic_limit_bytes_override == null) return "Inherited";
  if (access.traffic_limit_bytes_override <= 0) return "Unlimited";
  return formatBytes(access.traffic_limit_bytes_override);
}

function ArtifactStat({
  label,
  value,
  accent,
}: {
  label: string;
  value: string;
  accent?: "danger" | "success" | "warning";
}) {
  return (
    <div
      className={cn(
        "rounded-2xl bg-surface-1/50 px-4 py-3 shadow-[inset_0_0_0_1px_var(--control-border)]",
        accent === "success" && "shadow-[inset_0_0_0_1px_rgba(96,150,118,0.28)]",
        accent === "warning" && "shadow-[inset_0_0_0_1px_rgba(188,154,86,0.3)]",
        accent === "danger" && "shadow-[inset_0_0_0_1px_rgba(185,120,130,0.36)]",
      )}
    >
      <div className="text-[13px] font-semibold uppercase tracking-wide text-txt-muted">{label}</div>
      <div className="mt-2 text-[16px] font-semibold text-txt-primary">{value}</div>
    </div>
  );
}

export function ClientArtifactsDialog({
  open,
  client,
  artifacts,
  loading,
  refreshing,
  inbounds,
  clientProfiles,
  onClose,
  onRefreshArtifacts,
  onOpenUserAction,
  onOpenAccessAction,
}: {
  open: boolean;
  client: Client | null;
  artifacts: ClientArtifacts | null;
  loading: boolean;
  refreshing?: boolean;
  inbounds?: Inbound[];
  clientProfiles?: ClientProfile[];
  onClose: () => void;
  onRefreshArtifacts?: (client: Client) => void;
  onOpenUserAction?: (ids: string[], kind?: UserActionKind) => void;
  onOpenAccessAction?: (ids: string[], kind?: AccessActionKind) => void;
}) {
  const [copied, setCopied] = useState<string | null>(null);
  const [selectedAccess, setSelectedAccess] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (!open) return;
    setSelectedAccess(new Set());
  }, [client?.id, open]);

  const inboundByID = useMemo(
    () => new Map((inbounds ?? []).map((item) => [item.id, item])),
    [inbounds],
  );
  const profileByID = useMemo(
    () => new Map((clientProfiles ?? []).map((item) => [item.id, item])),
    [clientProfiles],
  );

  const copy = useCallback(async (value: string, key: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      window.setTimeout(() => setCopied(null), 1500);
    } catch {
      // Ignore clipboard failures.
    }
  }, []);

  const panels = useMemo(() => {
    const items: Panel[] = [];
    if (!client || !artifacts) return items;
    if (artifacts.vless_uris.length > 0) {
      items.push({
        label: "VLESS",
        qrSrc: qrURL(client.id, artifacts.vless_uris[0], 240),
        value: artifacts.vless_uris[0],
      });
    }
    if (artifacts.hy2_uris.length > 0) {
      items.push({
        label: "Hysteria2",
        qrSrc: qrURL(client.id, artifacts.hy2_uris[0], 240),
        value: artifacts.hy2_uris[0],
      });
    }
    if (artifacts.subscription_import_url) {
      items.push({
        label: "Sing-box",
        qrSrc: subscriptionQRURL(client.id, 240),
        value: artifacts.subscription_import_url,
      });
    }
    if (artifacts.subscription_clash_url) {
      items.push({
        label: "Clash",
        qrSrc: qrURL(client.id, artifacts.subscription_clash_url, 240),
        value: artifacts.subscription_clash_url,
      });
    }
    if (artifacts.subscription_base64_url) {
      items.push({
        label: "v2ray",
        qrSrc: qrURL(client.id, artifacts.subscription_base64_url, 240),
        value: artifacts.subscription_base64_url,
      });
    }
    return items;
  }, [artifacts, client]);

  if (!client) return null;

  const allAccessSelected =
    client.access.length > 0 && client.access.every((item) => selectedAccess.has(item.id));
  const someAccessSelected =
    client.access.some((item) => selectedAccess.has(item.id)) && !allAccessSelected;
  const selectedAccessIDs = Array.from(selectedAccess);
  const needsRefresh = artifacts?.artifacts_need_refresh ?? client.artifacts_need_refresh ?? false;
  const lastRendered = artifacts?.last_artifact_rendered_at ?? client.last_artifact_rendered_at ?? null;
  const refreshReason =
    artifacts?.last_artifact_refresh_reason ?? client.last_artifact_refresh_reason ?? null;
  const tokenPrefix = artifacts?.primary_token_prefix ?? "";

  function toggleAccess(id: string, checked: boolean) {
    setSelectedAccess((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  function toggleAllAccess(checked: boolean) {
    setSelectedAccess(() => {
      if (!checked) return new Set();
      return new Set(client.access.map((item) => item.id));
    });
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(value) => {
        if (!value) onClose();
      }}
      title={client.username}
      description="User"
      width="xl"
      footer={
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            {onOpenUserAction ? (
              <>
                <Button size="sm" onClick={() => onOpenUserAction([client.id], client.enabled ? "disable" : "enable")}>
                  {client.enabled ? <PowerOff size={13} /> : <Power size={13} />}
                  {client.enabled ? "Disable" : "Enable"}
                </Button>
                <Button size="sm" onClick={() => onOpenUserAction([client.id], "rotate-token")}>
                  <KeyRound size={13} /> Rotate token
                </Button>
                <Button size="sm" onClick={() => onOpenUserAction([client.id], "regenerate")}>
                  <HardDriveDownload size={13} /> Regenerate
                </Button>
              </>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            {onRefreshArtifacts ? (
              <Button size="sm" onClick={() => onRefreshArtifacts(client)} disabled={refreshing}>
                {refreshing ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
                Refresh
              </Button>
            ) : null}
            <Button onClick={onClose}>Close</Button>
          </div>
        </div>
      }
    >
      {loading ? (
        <div className="flex min-h-[320px] items-center justify-center gap-2 text-[15px] text-txt-secondary">
          <Loader2 size={16} className="animate-spin" /> Loading
        </div>
      ) : (
        <div className="space-y-8">
          <section className="space-y-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h3 className="text-[15px] font-semibold uppercase tracking-wide text-txt-muted">User</h3>
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <Badge variant={client.enabled ? "success" : "default"}>
                    {client.enabled ? "Enabled" : "Disabled"}
                  </Badge>
                  {client.has_subscription ? <Badge variant="default">Subscription</Badge> : null}
                  {needsRefresh ? <Badge variant="warning">Needs refresh</Badge> : null}
                </div>
              </div>
              {onOpenUserAction ? (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      aria-label="User actions"
                      className="inline-grid h-9 w-9 place-items-center rounded-lg text-txt-muted transition-colors hover:bg-surface-3/60 hover:text-txt-primary"
                    >
                      <MoreHorizontal size={16} />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent>
                    <DropdownMenuItem onSelect={() => onOpenUserAction([client.id], "edit")}>Edit</DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => onOpenUserAction([client.id], client.enabled ? "disable" : "enable")}>
                      {client.enabled ? "Disable" : "Enable"}
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => onOpenUserAction([client.id], "rotate-token")}>
                      Rotate token
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => onOpenUserAction([client.id], "regenerate")}>
                      Regenerate artifacts
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem danger onSelect={() => onOpenUserAction([client.id], "delete")}>
                      Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : null}
            </div>

            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <ArtifactStat label="Traffic" value={client.traffic_limit_bytes > 0 ? formatBytes(client.traffic_limit_bytes) : "Unlimited"} />
              <ArtifactStat
                label="Expires"
                value={client.expire_at ? formatDateTime(client.expire_at, { includeSeconds: false }) : "Never"}
              />
              <ArtifactStat
                label="Token"
                value={tokenPrefix ? `${tokenPrefix}...` : client.has_subscription ? "Issued" : "Not issued"}
                accent={tokenPrefix ? "success" : undefined}
              />
              <ArtifactStat
                label="Last render"
                value={lastRendered ? formatDateTime(lastRendered, { includeSeconds: false }) : "Never"}
                accent={needsRefresh ? "warning" : "success"}
              />
            </div>

            {refreshReason ? (
              <div className="rounded-2xl bg-surface-1/50 px-4 py-3 text-[14px] font-medium text-txt-secondary shadow-[inset_0_0_0_1px_var(--control-border)]">
                {refreshReason}
              </div>
            ) : null}
          </section>

          <section className="space-y-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h3 className="text-[15px] font-semibold uppercase tracking-wide text-txt-muted">Access</h3>
                <p className="mt-2 text-[15px] text-txt-secondary">{client.access.length}</p>
              </div>
              {selectedAccessIDs.length > 0 && onOpenAccessAction ? (
                <div className="flex flex-wrap items-center gap-2">
                  <Button size="sm" onClick={() => onOpenAccessAction(selectedAccessIDs, "enable")}>
                    <Power size={13} /> Enable
                  </Button>
                  <Button size="sm" onClick={() => onOpenAccessAction(selectedAccessIDs, "disable")}>
                    <PowerOff size={13} /> Disable
                  </Button>
                  <Button size="sm" onClick={() => onOpenAccessAction(selectedAccessIDs, "edit")}>
                    Patch
                  </Button>
                  <Button size="sm" variant="danger" onClick={() => onOpenAccessAction(selectedAccessIDs, "delete")}>
                    <Trash2 size={13} /> Remove
                  </Button>
                </div>
              ) : null}
            </div>

            {client.access.length === 0 ? (
              <div className="rounded-2xl bg-surface-1/50 px-4 py-10 text-center text-[15px] text-txt-muted shadow-[inset_0_0_0_1px_var(--control-border)]">
                No access
              </div>
            ) : (
              <div className="overflow-hidden rounded-2xl bg-surface-1/35 shadow-[inset_0_0_0_1px_var(--control-border)]">
                <div className="overflow-x-auto">
                  <div className="min-w-[920px]">
                    <div className="flex items-center gap-3 px-4 py-3 text-[13px] font-semibold uppercase tracking-wide text-txt-muted">
                      <div className="w-5">
                        <Checkbox
                          checked={allAccessSelected ? true : someAccessSelected ? "indeterminate" : false}
                          onCheckedChange={(value) => toggleAllAccess(Boolean(value))}
                          aria-label="Select access"
                        />
                      </div>
                      <div className="min-w-[180px] flex-1">Access</div>
                      <div className="w-[150px]">Inbound</div>
                      <div className="w-[150px]">Profile</div>
                      <div className="w-[140px]">Expires</div>
                      <div className="w-[120px]">Limit</div>
                      <div className="w-[160px]">Last seen</div>
                      <div className="w-[120px]">IP</div>
                      <div className="w-8" />
                    </div>
                    <div>
                      {client.access.map((access) => {
                        const inbound = inboundByID.get(access.inbound_id);
                        const profile = access.client_profile_id
                          ? profileByID.get(access.client_profile_id)
                          : undefined;
                        const isSelected = selectedAccess.has(access.id);
                        const statusLabel = access.enabled ? access.credential_status || "active" : "disabled";
                        return (
                          <div
                            key={access.id}
                            className={cn(
                              "group flex items-center gap-3 border-t border-border/30 px-4 py-3 transition-colors",
                              isSelected ? "bg-accent/6" : "hover:bg-surface-2/35",
                            )}
                          >
                            <div className="w-5">
                              <Checkbox
                                checked={isSelected}
                                onCheckedChange={(value) => toggleAccess(access.id, Boolean(value))}
                                aria-label="Select access row"
                              />
                            </div>
                            <div className="min-w-[180px] flex-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <Badge variant={protocolBadgeVariant(access.protocol)}>
                                  {access.protocol === "vless" ? "VLESS" : "HY2"}
                                </Badge>
                                <Badge variant={accessStatusVariant(access)}>{statusLabel}</Badge>
                                {client.has_subscription ? <Badge variant="default">Subscription</Badge> : null}
                              </div>
                              <div className="mt-2 flex flex-wrap items-center gap-2 text-[14px] text-txt-secondary">
                                <span>{access.display_name || access.description || access.id.slice(0, 8)}</span>
                                <span className="text-txt-muted">/</span>
                                <span>{profile?.name || "Default"}</span>
                              </div>
                            </div>
                            <div className="w-[150px] text-[14px] text-txt-secondary">
                              {inbound?.name || access.inbound_id}
                            </div>
                            <div className="w-[150px] text-[14px] text-txt-secondary">
                              {profile?.name || "Inherited"}
                            </div>
                            <div className="w-[140px] text-[14px] text-txt-secondary">
                              {accessExpires(access)}
                            </div>
                            <div className="w-[120px] text-[14px] text-txt-secondary">
                              {accessLimit(access)}
                            </div>
                            <div className="w-[160px] text-[14px] text-txt-secondary">
                              {access.last_seen_at
                                ? formatDateTime(access.last_seen_at, { includeSeconds: false })
                                : "Never"}
                            </div>
                            <div className="w-[120px] truncate text-[14px] text-txt-secondary">
                              {access.last_client_ip || "-"}
                            </div>
                            <div className="w-8">
                              {onOpenAccessAction ? (
                                <DropdownMenu>
                                  <DropdownMenuTrigger asChild>
                                    <button
                                      type="button"
                                      aria-label="Access actions"
                                      className="inline-grid h-8 w-8 place-items-center rounded-lg text-txt-muted opacity-0 transition-opacity hover:bg-surface-3/60 hover:text-txt-primary group-hover:opacity-100 data-[state=open]:opacity-100"
                                    >
                                      <MoreHorizontal size={16} />
                                    </button>
                                  </DropdownMenuTrigger>
                                  <DropdownMenuContent>
                                    <DropdownMenuItem onSelect={() => onOpenAccessAction([access.id], "edit")}>
                                      Patch
                                    </DropdownMenuItem>
                                    <DropdownMenuItem
                                      onSelect={() =>
                                        onOpenAccessAction([access.id], access.enabled ? "disable" : "enable")
                                      }
                                    >
                                      {access.enabled ? "Disable" : "Enable"}
                                    </DropdownMenuItem>
                                    <DropdownMenuItem
                                      onSelect={() => onOpenAccessAction([access.id], "rotate-credentials")}
                                    >
                                      Rotate credentials
                                    </DropdownMenuItem>
                                    <DropdownMenuItem onSelect={() => onOpenAccessAction([access.id], "regenerate")}>
                                      Regenerate artifacts
                                    </DropdownMenuItem>
                                    <DropdownMenuSeparator />
                                    <DropdownMenuItem danger onSelect={() => onOpenAccessAction([access.id], "delete")}>
                                      Remove access
                                    </DropdownMenuItem>
                                  </DropdownMenuContent>
                                </DropdownMenu>
                              ) : null}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </section>

          <section className="space-y-4">
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-[15px] font-semibold uppercase tracking-wide text-txt-muted">Artifacts</h3>
              {artifacts?.all_uris.length ? (
                <span className="text-[14px] font-medium text-txt-secondary">{artifacts.all_uris.length}</span>
              ) : null}
            </div>

            {panels.length === 0 ? (
              <div className="rounded-2xl bg-surface-1/50 px-4 py-10 text-center text-[15px] text-txt-muted shadow-[inset_0_0_0_1px_var(--control-border)]">
                No artifacts
              </div>
            ) : (
              <div className="space-y-6">
                {panels.map((panel) => (
                  <section key={panel.label}>
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <h4 className="text-[15px] font-semibold uppercase tracking-wide text-txt-muted">
                        {panel.label}
                      </h4>
                      <button
                        type="button"
                        onClick={() => copy(panel.value, panel.label)}
                        className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-[14px] font-medium text-txt-secondary transition-colors hover:bg-surface-3/50 hover:text-txt-primary"
                      >
                        {copied === panel.label ? <Check size={12} /> : <Copy size={12} />}
                        {copied === panel.label ? "Copied" : "Copy"}
                      </button>
                    </div>
                    <div className="flex items-start gap-4">
                      {panel.qrSrc ? (
                        <img
                          src={panel.qrSrc}
                          alt=""
                          className="h-[148px] w-[148px] shrink-0 rounded-xl bg-white p-1.5"
                        />
                      ) : null}
                      <textarea
                        readOnly
                        value={panel.value}
                        onFocus={(event) => event.currentTarget.select()}
                        rows={6}
                        className="h-[148px] w-full resize-none overflow-auto rounded-xl bg-surface-1/55 p-3 font-mono text-[13px] leading-relaxed text-txt-secondary outline-none shadow-[inset_0_0_0_1px_var(--control-border)]"
                      />
                    </div>
                  </section>
                ))}

                {artifacts && artifacts.all_uris.length > panels.length ? (
                  <details className="group">
                    <summary className="cursor-pointer list-none text-[14px] font-semibold uppercase tracking-wide text-txt-muted hover:text-txt-secondary">
                      All URIs ({artifacts.all_uris.length})
                    </summary>
                    <div className="mt-3 space-y-1">
                      {artifacts.all_uris.map((uri, index) => (
                        <div
                          key={`${uri}-${index}`}
                          className="group/row flex items-center gap-2 rounded-xl px-3 py-2 hover:bg-surface-2/40"
                        >
                          <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-txt-secondary">
                            {uri}
                          </code>
                          <button
                            type="button"
                            className="shrink-0 rounded p-1 opacity-0 transition-opacity hover:bg-surface-3/60 group-hover/row:opacity-100"
                            onClick={() => copy(uri, `uri-${index}`)}
                            aria-label="Copy URI"
                          >
                            {copied === `uri-${index}` ? (
                              <Check size={12} />
                            ) : (
                              <Copy size={12} className="text-txt-muted" />
                            )}
                          </button>
                        </div>
                      ))}
                    </div>
                  </details>
                ) : null}
              </div>
            )}
          </section>
        </div>
      )}
    </Drawer>
  );
}
