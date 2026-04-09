import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Copy,
  FileCode2,
  Loader2,
  RefreshCw,
  RotateCcw,
  Save,
  Server as ServerIcon,
  Shield,
  Zap,
} from "lucide-react";
import {
  type FormEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { PolicyManager, type PolicyDescriptor, type PolicyEntity, type PolicyField } from "@/components/settings/policy-manager";
import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import {
  applyServerConfig,
  generateRealityKeypair,
  getServerDraftState,
  listInbounds,
  listServers,
  previewServerConfig,
  renderServerConfig,
  updateInbound,
  updateServer,
  validateServerConfig,
} from "@/domain/inbounds/services";
import type { Inbound, Server as ServerType } from "@/domain/inbounds/types";
import {
  deleteClientProfile,
  deleteDNSProfile,
  deleteHY2MasqueradeProfile,
  deleteLogProfile,
  deleteMultiplexProfile,
  deleteOutbound,
  deleteRealityProfile,
  deleteRouteRule,
  deleteTLSProfile,
  deleteTransportProfile,
  HY2_MODE_LABELS,
  listClientProfiles,
  listDNSProfiles,
  listHY2MasqueradeProfiles,
  listLogProfiles,
  listMultiplexProfiles,
  listOutbounds,
  listRealityProfiles,
  listRouteRules,
  listTLSProfiles,
  listTransportProfiles,
  upsertClientProfile,
  upsertDNSProfile,
  upsertHY2MasqueradeProfile,
  upsertLogProfile,
  upsertMultiplexProfile,
  upsertOutbound,
  upsertRealityProfile,
  upsertRouteRule,
  upsertTLSProfile,
  upsertTransportProfile,
  VLESS_MODE_LABELS,
} from "@/domain/policy/services";
import { getAPIErrorMessage } from "@/services/api";
import type {
  ClientProfile,
  DNSProfile,
  DraftRevisionState,
  HY2MasqueradeProfile,
  LogProfile,
  MultiplexProfile,
  Outbound,
  RealityProfile,
  RouteRule,
  TLSProfile,
  TransportProfile,
} from "@/types/common";
import { Badge, Button, Input, SelectField, Toggle, cn } from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";

function useDirtyForm<T>(initial: T) {
  const [form, setForm] = useState<T>(initial);
  const snapshot = useRef<T>(initial);
  useEffect(() => {
    snapshot.current = initial;
    setForm(initial);
  }, [initial]);
  const dirty = useMemo(
    () => JSON.stringify(form) !== JSON.stringify(snapshot.current),
    [form],
  );
  const reset = useCallback(() => setForm(snapshot.current), []);
  function set<K extends keyof T>(key: K, value: T[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }
  return { form, set, dirty, reset };
}

function useFormSubmit(save: () => Promise<unknown>, onSaved: () => void) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  async function submit(event?: FormEvent) {
    event?.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await save();
      toast.notify("Saved", "success");
      onSaved();
    } catch (err) {
      setError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setBusy(false);
    }
  }

  return { busy, error, submit };
}

function FieldGroup({
  title,
  children,
}: {
  title?: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-4">
      {title ? (
        <h3 className="text-[13px] font-semibold uppercase tracking-wider text-txt-muted">
          {title}
        </h3>
      ) : null}
      {children}
    </div>
  );
}

function SectionDivider() {
  return <div className="h-px w-full bg-border/40" />;
}

function InlineToggle({
  label,
  checked,
  onCheckedChange,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: (value: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-4 rounded-lg px-1 py-2">
      <span className="text-[16px] font-medium text-txt-primary">{label}</span>
      <Toggle checked={checked} onCheckedChange={onCheckedChange} />
    </label>
  );
}

function CopyField({
  label,
  value,
  onChange,
  onGenerate,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  onGenerate?: () => void | Promise<void>;
}) {
  const [copied, setCopied] = useState(false);
  const [generating, setGenerating] = useState(false);

  async function copy() {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Ignore clipboard failures.
    }
  }

  async function generate() {
    if (!onGenerate) return;
    setGenerating(true);
    try {
      await onGenerate();
    } finally {
      setGenerating(false);
    }
  }

  return (
    <div>
      <label className="mb-2 block text-[15px] font-medium text-txt-secondary">{label}</label>
      <div className="flex gap-2">
        <input
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="w-full rounded-lg border-0 bg-[var(--control-bg)] px-4 py-2.5 font-mono text-[14px] text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors placeholder:text-txt-tertiary focus:bg-[var(--control-bg-hover)] focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]"
        />
        {onGenerate ? (
          <button
            type="button"
            onClick={() => void generate()}
            disabled={generating}
            className="inline-grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-surface-3/40 text-txt-muted transition-colors hover:bg-surface-3/70 hover:text-txt-primary disabled:opacity-40"
            aria-label="Generate"
          >
            <RefreshCw size={14} className={generating ? "animate-spin" : ""} />
          </button>
        ) : null}
        <button
          type="button"
          onClick={() => void copy()}
          disabled={!value}
          className="inline-grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-surface-3/40 text-txt-muted transition-colors hover:bg-surface-3/70 hover:text-txt-primary disabled:opacity-40"
          aria-label="Copy"
        >
          {copied ? <Check size={14} className="text-status-success" /> : <Copy size={14} />}
        </button>
      </div>
    </div>
  );
}

function SaveBar({
  dirty,
  busy,
  onSave,
  onReset,
}: {
  dirty: boolean;
  busy: boolean;
  onSave: () => void;
  onReset: () => void;
}) {
  if (!dirty) return null;
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-6 z-40 flex justify-center px-4">
      <div className="pointer-events-auto flex items-center gap-2 rounded-xl bg-surface-2/95 px-3 py-2 shadow-[0_18px_42px_-16px_var(--dialog-shadow)] backdrop-blur-xl ring-1 ring-border/40">
        <div className="flex items-center gap-2 px-2">
          <span className="h-1.5 w-1.5 rounded-full bg-status-warning animate-pulse" />
          <span className="text-[15px] font-medium text-txt-secondary">Unsaved changes</span>
        </div>
        <div className="mx-1 h-4 w-px bg-border/40" />
        <Button size="sm" onClick={onReset} disabled={busy}>
          <RotateCcw size={13} /> Reset
        </Button>
        <Button size="sm" variant="primary" onClick={onSave} disabled={busy}>
          {busy ? <Loader2 size={13} className="animate-spin" /> : <Save size={13} />}
          Save
        </Button>
      </div>
    </div>
  );
}

function SectionHeader({
  icon,
  title,
}: {
  icon: ReactNode;
  title: string;
}) {
  return (
    <div className="mb-6 flex items-center gap-3 border-b border-border/40 pb-5">
      <div className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-surface-3/55 text-txt-secondary">
        {icon}
      </div>
      <h2 className="text-[17px] font-semibold text-txt-primary">{title}</h2>
    </div>
  );
}

type Tab = "hy2" | "policies" | "preview" | "profiles" | "server" | "vless";
type ProfileTab =
  | "client"
  | "dns"
  | "hy2-masquerade"
  | "log"
  | "multiplex"
  | "reality"
  | "tls"
  | "transport";
type PolicyTab = "outbound" | "route-rule";

type TabDef = {
  key: Tab;
  label: string;
  icon: ReactNode;
};

function TabsNav({
  active,
  onChange,
  tabs,
}: {
  active: string;
  onChange: (value: string) => void;
  tabs: { key: string; label: string; icon: ReactNode }[];
}) {
  return (
    <nav className="flex items-center gap-1.5 overflow-x-auto">
      {tabs.map((tab) => {
        const selected = active === tab.key;
        return (
          <button
            key={tab.key}
            type="button"
            onClick={() => onChange(tab.key)}
            className={cn(
              "group relative flex h-11 shrink-0 items-center gap-2.5 rounded-2xl px-4 transition-[background-color,color,box-shadow] duration-200",
              selected
                ? "bg-surface-3/70 text-txt-primary shadow-[inset_0_1px_0_var(--shell-highlight),0_2px_8px_var(--shell-shadow)]"
                : "text-txt-secondary hover:bg-surface-3/45 hover:text-txt-primary",
            )}
          >
            <span
              className={cn(
                "shrink-0 transition-colors duration-200",
                selected ? "text-accent-secondary" : "text-txt-tertiary group-hover:text-txt-primary",
              )}
            >
              {tab.icon}
            </span>
            <span className="whitespace-nowrap text-[16px] font-semibold">{tab.label}</span>
          </button>
        );
      })}
    </nav>
  );
}

function RevisionCard({
  applyBusy,
  draft,
  onApply,
  onRender,
  onValidate,
  renderBusy,
  validateBusy,
}: {
  applyBusy: boolean;
  draft: DraftRevisionState | null;
  onApply: () => void;
  onRender: () => void;
  onValidate: () => void;
  renderBusy: boolean;
  validateBusy: boolean;
}) {
  return (
    <div className="rounded-2xl bg-surface-2 px-5 py-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-[17px] font-semibold text-txt-primary">Revision</h2>
          <p className="mt-1 text-[15px] text-txt-secondary">
            Current #{draft?.current_revision_no ?? "-"} / Draft #{draft?.draft_revision_no ?? "-"}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button onClick={onRender} disabled={renderBusy}>
            {renderBusy ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
            Render
          </Button>
          <Button onClick={onValidate} disabled={!draft?.draft_revision_id || validateBusy}>
            {validateBusy ? <Loader2 size={14} className="animate-spin" /> : "Validate"}
          </Button>
          <Button
            variant="primary"
            onClick={onApply}
            disabled={!draft?.draft_revision_id || !draft?.check_ok || applyBusy}
          >
            {applyBusy ? <Loader2 size={14} className="animate-spin" /> : "Apply"}
          </Button>
        </div>
      </div>
      <div className="mt-4 flex flex-wrap gap-2">
        <Badge variant={draft?.pending_changes ? "warning" : "default"}>
          {draft?.pending_changes ? "Pending changes" : "No pending changes"}
        </Badge>
        <Badge variant={draft?.check_error ? "danger" : draft?.check_ok ? "success" : "default"}>
          {draft?.check_error ? "Check failed" : draft?.check_ok ? "Valid" : "Not checked"}
        </Badge>
        {draft?.apply_status ? (
          <Badge variant={draft.apply_status === "succeeded" ? "success" : "danger"}>
            {draft.apply_status}
          </Badge>
        ) : null}
      </div>
      {draft?.check_error ? (
        <div className="mt-4 rounded-2xl bg-status-danger/10 px-4 py-3 text-[15px] text-status-danger">
          {draft.check_error}
        </div>
      ) : null}
      {draft?.apply_error ? (
        <div className="mt-4 rounded-2xl bg-status-danger/10 px-4 py-3 text-[15px] text-status-danger">
          {draft.apply_error}
        </div>
      ) : null}
    </div>
  );
}

type ServerFormState = {
  public_host: string;
  singbox_binary_path: string;
  singbox_config_path: string;
  singbox_service_name: string;
  subscription_base_url: string;
};

function serverToForm(server: ServerType): ServerFormState {
  return {
    public_host: server.public_host,
    subscription_base_url: server.subscription_base_url ?? "",
    singbox_binary_path: server.singbox_binary_path ?? "",
    singbox_config_path: server.singbox_config_path ?? "",
    singbox_service_name: server.singbox_service_name ?? "",
  };
}

function ServerForm({ onSaved, server }: { onSaved: () => void; server: ServerType }) {
  const initial = useMemo(() => serverToForm(server), [server]);
  const { form, set, dirty, reset } = useDirtyForm(initial);
  const { busy, error, submit } = useFormSubmit(
    () =>
      updateServer(server.id, {
        public_host: form.public_host.trim() || undefined,
        subscription_base_url: form.subscription_base_url.trim() || undefined,
        singbox_binary_path: form.singbox_binary_path.trim() || undefined,
        singbox_config_path: form.singbox_config_path.trim() || undefined,
        singbox_service_name: form.singbox_service_name.trim() || undefined,
      }),
    onSaved,
  );

  return (
    <>
      <SectionHeader icon={<ServerIcon size={18} strokeWidth={1.8} />} title="Server" />
      <form className="space-y-6" onSubmit={submit}>
        {error ? <ErrorBanner message={error} /> : null}

        <FieldGroup title="Endpoints">
          <Input
            label="Public host"
            placeholder="example.com"
            value={form.public_host}
            onChange={(event) => set("public_host", event.target.value)}
          />
          <Input
            label="Subscription base URL"
            placeholder="https://example.com"
            value={form.subscription_base_url}
            onChange={(event) => set("subscription_base_url", event.target.value)}
          />
        </FieldGroup>

        <SectionDivider />

        <FieldGroup title="sing-box">
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Binary path"
              value={form.singbox_binary_path}
              onChange={(event) => set("singbox_binary_path", event.target.value)}
            />
            <Input
              label="Config path"
              value={form.singbox_config_path}
              onChange={(event) => set("singbox_config_path", event.target.value)}
            />
          </div>
          <Input
            label="Service name"
            value={form.singbox_service_name}
            onChange={(event) => set("singbox_service_name", event.target.value)}
          />
        </FieldGroup>
      </form>
      <SaveBar dirty={dirty} busy={busy} onSave={() => void submit()} onReset={reset} />
    </>
  );
}

type VLESSFormState = {
  enabled: boolean;
  flow: string;
  listen_port: string;
  multiplex_profile_id: string;
  reality_enabled: boolean;
  reality_handshake_server: string;
  reality_handshake_server_port: string;
  reality_private_key: string;
  reality_profile_id: string;
  reality_public_key: string;
  reality_short_id: string;
  tls_profile_id: string;
  tls_server_name: string;
  transport_host: string;
  transport_path: string;
  transport_profile_id: string;
  transport_type: string;
};

function vlessToForm(inbound: Inbound): VLESSFormState {
  const item = inbound.vless;
  return {
    listen_port: String(inbound.listen_port),
    enabled: inbound.enabled,
    reality_enabled: item?.reality_enabled ?? false,
    reality_public_key: item?.reality_public_key ?? "",
    reality_private_key: item?.reality_private_key ?? "",
    reality_short_id: item?.reality_short_id ?? "",
    reality_handshake_server: item?.reality_handshake_server ?? "www.cloudflare.com",
    reality_handshake_server_port: String(item?.reality_handshake_server_port ?? 443),
    tls_server_name: item?.tls_server_name ?? "",
    flow: item?.flow ?? "xtls-rprx-vision",
    transport_type: item?.transport_type ?? "tcp",
    transport_host: item?.transport_host ?? "",
    transport_path: item?.transport_path ?? "",
    tls_profile_id: item?.tls_profile_id ?? "",
    reality_profile_id: item?.reality_profile_id ?? "",
    transport_profile_id: item?.transport_profile_id ?? "",
    multiplex_profile_id: item?.multiplex_profile_id ?? "",
  };
}

type HY2FormState = {
  allow_insecure: boolean;
  bandwidth_profile_mode: string;
  down_mbps: string;
  enabled: boolean;
  hop_interval: string;
  ignore_client_bandwidth: boolean;
  listen_port: string;
  masquerade_profile_id: string;
  network: string;
  obfs_password: string;
  obfs_type: string;
  server_ports: string;
  tls_certificate_path: string;
  tls_key_path: string;
  tls_profile_id: string;
  tls_server_name: string;
  up_mbps: string;
};

function hy2ToForm(inbound: Inbound): HY2FormState {
  const item = inbound.hysteria2;
  return {
    listen_port: String(inbound.listen_port),
    enabled: inbound.enabled,
    tls_server_name: item?.tls_server_name ?? "",
    tls_certificate_path: item?.tls_certificate_path ?? "",
    tls_key_path: item?.tls_key_path ?? "",
    allow_insecure: item?.allow_insecure ?? false,
    ignore_client_bandwidth: item?.ignore_client_bandwidth ?? true,
    up_mbps: item?.up_mbps != null ? String(item.up_mbps) : "",
    down_mbps: item?.down_mbps != null ? String(item.down_mbps) : "",
    obfs_type: item?.obfs_type ?? "",
    obfs_password: item?.obfs_password ?? "",
    tls_profile_id: item?.tls_profile_id ?? "",
    masquerade_profile_id: item?.masquerade_profile_id ?? "",
    server_ports: item?.server_ports ?? "",
    hop_interval: item?.hop_interval != null ? String(item.hop_interval) : "",
    network: item?.network ?? "",
    bandwidth_profile_mode: item?.bandwidth_profile_mode ?? "",
  };
}

function VLESSForm({
  inbound,
  multiplexProfiles,
  onSaved,
  realityProfiles,
  tlsProfiles,
  transportProfiles,
}: {
  inbound: Inbound;
  multiplexProfiles: MultiplexProfile[];
  onSaved: () => void;
  realityProfiles: RealityProfile[];
  tlsProfiles: TLSProfile[];
  transportProfiles: TransportProfile[];
}) {
  const initial = useMemo(() => vlessToForm(inbound), [inbound]);
  const { form, set, dirty, reset } = useDirtyForm(initial);
  const { busy, error, submit } = useFormSubmit(() => {
    const port = parseInt(form.listen_port, 10);
    return updateInbound(inbound.id, {
      listen_port: Number.isFinite(port) ? port : inbound.listen_port,
      enabled: form.enabled,
      vless: {
        tls_enabled: true,
        reality_enabled: form.reality_enabled,
        reality_public_key: form.reality_public_key.trim(),
        reality_private_key: form.reality_private_key.trim(),
        reality_short_id: form.reality_short_id.trim(),
        reality_handshake_server: form.reality_handshake_server.trim() || "www.cloudflare.com",
        reality_handshake_server_port: parseInt(form.reality_handshake_server_port, 10) || 443,
        tls_server_name: form.tls_server_name.trim(),
        flow: form.flow.trim(),
        transport_type: form.transport_type,
        transport_host: form.transport_host.trim(),
        transport_path: form.transport_path.trim(),
        tls_profile_id: form.tls_profile_id || undefined,
        reality_profile_id: form.reality_profile_id || undefined,
        transport_profile_id: form.transport_profile_id || undefined,
        multiplex_profile_id: form.multiplex_profile_id || undefined,
      },
    });
  }, onSaved);

  return (
    <>
      <SectionHeader icon={<Shield size={18} strokeWidth={1.8} />} title="VLESS" />
      <form className="space-y-6" onSubmit={submit}>
        {error ? <ErrorBanner message={error} /> : null}

        <FieldGroup title="Inbound">
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Port"
              type="number"
              min="1"
              max="65535"
              value={form.listen_port}
              onChange={(event) => set("listen_port", event.target.value)}
            />
            <SelectField
              label="Flow"
              value={form.flow || "__none__"}
              onValueChange={(value) => set("flow", value === "__none__" ? "" : value)}
              options={[
                { value: "__none__", label: "None" },
                { value: "xtls-rprx-vision", label: "xtls-rprx-vision" },
              ]}
            />
          </div>
          <InlineToggle label="Enabled" checked={form.enabled} onCheckedChange={(value) => set("enabled", value)} />
          <InlineToggle label="Reality" checked={form.reality_enabled} onCheckedChange={(value) => set("reality_enabled", value)} />
        </FieldGroup>

        <SectionDivider />

        <FieldGroup title="Profiles">
          <div className="grid gap-4 sm:grid-cols-2">
            <SelectField
              label="TLS profile"
              value={form.tls_profile_id || "__none__"}
              onValueChange={(value) => set("tls_profile_id", value === "__none__" ? "" : value)}
              options={[{ value: "__none__", label: "None" }, ...tlsProfiles.map((item) => ({ value: item.id, label: item.name }))]}
            />
            <SelectField
              label="Reality profile"
              value={form.reality_profile_id || "__none__"}
              onValueChange={(value) => set("reality_profile_id", value === "__none__" ? "" : value)}
              options={[{ value: "__none__", label: "None" }, ...realityProfiles.map((item) => ({ value: item.id, label: item.name }))]}
            />
            <SelectField
              label="Transport profile"
              value={form.transport_profile_id || "__none__"}
              onValueChange={(value) => set("transport_profile_id", value === "__none__" ? "" : value)}
              options={[{ value: "__none__", label: "None" }, ...transportProfiles.map((item) => ({ value: item.id, label: item.name }))]}
            />
            <SelectField
              label="Multiplex profile"
              value={form.multiplex_profile_id || "__none__"}
              onValueChange={(value) => set("multiplex_profile_id", value === "__none__" ? "" : value)}
              options={[{ value: "__none__", label: "None" }, ...multiplexProfiles.map((item) => ({ value: item.id, label: item.name }))]}
            />
          </div>
        </FieldGroup>

        <SectionDivider />

        {form.reality_enabled ? (
          <FieldGroup title="Reality">
            <Input
              label="Handshake server"
              value={form.reality_handshake_server}
              onChange={(event) => set("reality_handshake_server", event.target.value)}
            />
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label="Handshake port"
                type="number"
                value={form.reality_handshake_server_port}
                onChange={(event) => set("reality_handshake_server_port", event.target.value)}
              />
              <CopyField
                label="Short ID"
                value={form.reality_short_id}
                onChange={(value) => set("reality_short_id", value)}
                onGenerate={() => {
                  const bytes = new Uint8Array(8);
                  crypto.getRandomValues(bytes);
                  set("reality_short_id", Array.from(bytes).map((item) => item.toString(16).padStart(2, "0")).join(""));
                }}
              />
            </div>
            <CopyField
              label="Private key"
              value={form.reality_private_key}
              onChange={(value) => set("reality_private_key", value)}
              onGenerate={async () => {
                const keypair = await generateRealityKeypair();
                set("reality_private_key", keypair.private_key);
                set("reality_public_key", keypair.public_key);
              }}
            />
            <CopyField
              label="Public key"
              value={form.reality_public_key}
              onChange={(value) => set("reality_public_key", value)}
            />
          </FieldGroup>
        ) : (
          <FieldGroup title="TLS">
            <Input
              label="SNI"
              value={form.tls_server_name}
              onChange={(event) => set("tls_server_name", event.target.value)}
            />
          </FieldGroup>
        )}

        <SectionDivider />

        <FieldGroup title="Transport">
          <div className="grid gap-4 sm:grid-cols-3">
            <SelectField
              label="Type"
              value={form.transport_type}
              onValueChange={(value) => set("transport_type", value)}
              options={[
                { value: "tcp", label: "TCP" },
                { value: "ws", label: "WebSocket" },
                { value: "grpc", label: "gRPC" },
                { value: "xhttp", label: "XHTTP" },
              ]}
            />
            {form.transport_type !== "tcp" ? (
              <>
                <Input
                  label="Host"
                  value={form.transport_host}
                  onChange={(event) => set("transport_host", event.target.value)}
                />
                <Input
                  label="Path"
                  value={form.transport_path}
                  onChange={(event) => set("transport_path", event.target.value)}
                />
              </>
            ) : null}
          </div>
        </FieldGroup>
      </form>
      <SaveBar dirty={dirty} busy={busy} onSave={() => void submit()} onReset={reset} />
    </>
  );
}

function HY2Form({
  inbound,
  masqueradeProfiles,
  onSaved,
  tlsProfiles,
}: {
  inbound: Inbound;
  masqueradeProfiles: HY2MasqueradeProfile[];
  onSaved: () => void;
  tlsProfiles: TLSProfile[];
}) {
  const initial = useMemo(() => hy2ToForm(inbound), [inbound]);
  const { form, set, dirty, reset } = useDirtyForm(initial);
  const { busy, error, submit } = useFormSubmit(() => {
    const port = parseInt(form.listen_port, 10);
    const upMbps = form.up_mbps.trim() ? parseInt(form.up_mbps, 10) : null;
    const downMbps = form.down_mbps.trim() ? parseInt(form.down_mbps, 10) : null;
    const hopInterval = form.hop_interval.trim() ? parseInt(form.hop_interval, 10) : null;
    return updateInbound(inbound.id, {
      listen_port: Number.isFinite(port) ? port : inbound.listen_port,
      enabled: form.enabled,
      hysteria2: {
        tls_enabled: true,
        tls_server_name: form.tls_server_name.trim(),
        tls_certificate_path: form.tls_certificate_path.trim(),
        tls_key_path: form.tls_key_path.trim(),
        allow_insecure: form.allow_insecure,
        ignore_client_bandwidth: form.ignore_client_bandwidth,
        up_mbps: Number.isFinite(upMbps as number) ? upMbps : null,
        down_mbps: Number.isFinite(downMbps as number) ? downMbps : null,
        obfs_type: form.obfs_type.trim(),
        obfs_password: form.obfs_password.trim(),
        tls_profile_id: form.tls_profile_id || undefined,
        masquerade_profile_id: form.masquerade_profile_id || undefined,
        server_ports: form.server_ports.trim() || undefined,
        hop_interval: Number.isFinite(hopInterval as number) ? hopInterval ?? undefined : undefined,
        network: form.network.trim() || undefined,
        bandwidth_profile_mode: form.bandwidth_profile_mode.trim() || undefined,
      },
    });
  }, onSaved);

  return (
    <>
      <SectionHeader icon={<Zap size={18} strokeWidth={1.8} />} title="Hysteria2" />
      <form className="space-y-6" onSubmit={submit}>
        {error ? <ErrorBanner message={error} /> : null}

        <FieldGroup title="Inbound">
          <Input
            label="Port"
            type="number"
            min="1"
            max="65535"
            value={form.listen_port}
            onChange={(event) => set("listen_port", event.target.value)}
          />
          <InlineToggle label="Enabled" checked={form.enabled} onCheckedChange={(value) => set("enabled", value)} />
        </FieldGroup>

        <SectionDivider />

        <FieldGroup title="Profiles">
          <div className="grid gap-4 sm:grid-cols-2">
            <SelectField
              label="TLS profile"
              value={form.tls_profile_id || "__none__"}
              onValueChange={(value) => set("tls_profile_id", value === "__none__" ? "" : value)}
              options={[{ value: "__none__", label: "None" }, ...tlsProfiles.map((item) => ({ value: item.id, label: item.name }))]}
            />
            <SelectField
              label="Masquerade profile"
              value={form.masquerade_profile_id || "__none__"}
              onValueChange={(value) => set("masquerade_profile_id", value === "__none__" ? "" : value)}
              options={[{ value: "__none__", label: "None" }, ...masqueradeProfiles.map((item) => ({ value: item.id, label: item.name }))]}
            />
          </div>
        </FieldGroup>

        <SectionDivider />

        <FieldGroup title="TLS">
          <Input
            label="SNI"
            value={form.tls_server_name}
            onChange={(event) => set("tls_server_name", event.target.value)}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Certificate path"
              value={form.tls_certificate_path}
              onChange={(event) => set("tls_certificate_path", event.target.value)}
            />
            <Input
              label="Key path"
              value={form.tls_key_path}
              onChange={(event) => set("tls_key_path", event.target.value)}
            />
          </div>
          <InlineToggle label="Allow insecure" checked={form.allow_insecure} onCheckedChange={(value) => set("allow_insecure", value)} />
        </FieldGroup>

        <SectionDivider />

        <FieldGroup title="Bandwidth">
          <InlineToggle label="Ignore client bandwidth" checked={form.ignore_client_bandwidth} onCheckedChange={(value) => set("ignore_client_bandwidth", value)} />
          <div className="grid gap-4 sm:grid-cols-2">
            <Input label="Upload (Mbps)" type="number" value={form.up_mbps} onChange={(event) => set("up_mbps", event.target.value)} />
            <Input label="Download (Mbps)" type="number" value={form.down_mbps} onChange={(event) => set("down_mbps", event.target.value)} />
          </div>
          <div className="grid gap-4 sm:grid-cols-3">
            <Input label="Server ports" value={form.server_ports} onChange={(event) => set("server_ports", event.target.value)} />
            <Input label="Hop interval" type="number" value={form.hop_interval} onChange={(event) => set("hop_interval", event.target.value)} />
            <Input label="Network" value={form.network} onChange={(event) => set("network", event.target.value)} />
          </div>
          <Input
            label="Bandwidth mode"
            value={form.bandwidth_profile_mode}
            onChange={(event) => set("bandwidth_profile_mode", event.target.value)}
          />
        </FieldGroup>

        <SectionDivider />

        <FieldGroup title="Obfuscation">
          <div className="grid gap-4 sm:grid-cols-2">
            <SelectField
              label="Type"
              value={form.obfs_type || "__none__"}
              onValueChange={(value) => {
                const next = value === "__none__" ? "" : value;
                set("obfs_type", next);
                if (next === "salamander" && !form.obfs_password) {
                  set("obfs_password", crypto.randomUUID().replace(/-/g, ""));
                }
              }}
              options={[
                { value: "__none__", label: "None" },
                { value: "salamander", label: "salamander" },
              ]}
            />
            <CopyField
              label="Password"
              value={form.obfs_password}
              onChange={(value) => set("obfs_password", value)}
              onGenerate={() => set("obfs_password", crypto.randomUUID().replace(/-/g, ""))}
            />
          </div>
        </FieldGroup>
      </form>
      <SaveBar dirty={dirty} busy={busy} onSave={() => void submit()} onReset={reset} />
    </>
  );
}

function ConfigPreview({ server }: { server: ServerType }) {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ config_json: string; check_warning?: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function load() {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const data = await previewServerConfig(server.id);
      setResult(data);
    } catch (err) {
      setError(getAPIErrorMessage(err, "Failed to load"));
    } finally {
      setLoading(false);
    }
  }

  let pretty = "";
  if (result?.config_json) {
    try {
      pretty = JSON.stringify(JSON.parse(result.config_json), null, 2);
    } catch {
      pretty = result.config_json;
    }
  }

  async function copy() {
    if (!pretty) return;
    try {
      await navigator.clipboard.writeText(pretty);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Ignore clipboard failures.
    }
  }

  return (
    <>
      <SectionHeader icon={<FileCode2 size={18} strokeWidth={1.8} />} title="Config" />
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Button type="button" disabled={loading} onClick={() => void load()}>
            {loading ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
            {result ? "Refresh" : "Generate"}
          </Button>
          {result ? (
            <Button type="button" onClick={() => void copy()} disabled={!pretty}>
              {copied ? <Check size={14} /> : <Copy size={14} />}
              {copied ? "Copied" : "Copy"}
            </Button>
          ) : null}
        </div>
        {error ? <ErrorBanner message={error} /> : null}
        {result ? (
          <>
            {result.check_warning ? (
              <div className="flex items-start gap-2 rounded-lg bg-status-warning/10 px-3 py-2.5 text-[15px] text-status-warning">
                <AlertTriangle size={15} className="mt-0.5 shrink-0" />
                <span className="break-all">{result.check_warning}</span>
              </div>
            ) : (
              <div className="flex items-center gap-2 rounded-lg bg-status-success/10 px-3 py-2.5 text-[15px] text-status-success">
                <CheckCircle2 size={15} className="shrink-0" />
                <span>Valid</span>
              </div>
            )}
            <pre className="max-h-[560px] overflow-auto rounded-xl bg-surface-0 p-4 font-mono text-[14px] leading-relaxed text-txt-secondary ring-1 ring-border/30">
              {pretty}
            </pre>
          </>
        ) : null}
      </div>
    </>
  );
}

function dotIcon(colorClass: string) {
  return <span className={cn("inline-block h-2 w-2 rounded-full", colorClass)} />;
}

function cloneEntity(item: PolicyEntity, key: string) {
  const next = { ...item, id: "", created_at: undefined, updated_at: undefined };
  if (typeof next[key] === "string" && next[key]) {
    next[key] = `${String(next[key])} Copy`;
  }
  return next;
}

function csvField(key: string, label: string): PolicyField {
  return { key, kind: "csv", label };
}

function portsField(key: string, label: string): PolicyField {
  return {
    key,
    kind: "csv",
    label,
    parse: (value) =>
      value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean)
        .map((item) => Number(item))
        .filter((item) => Number.isFinite(item)),
    format: (value) => (Array.isArray(value) ? value.join(", ") : ""),
  };
}

export default function SettingsPage() {
  const qc = useQueryClient();
  const toast = useToast();

  const serversQ = useQuery({ queryKey: ["settings", "servers"], queryFn: listServers });
  const server = serversQ.data?.[0] ?? null;
  const serverID = server?.id ?? "";
  const scopedEnabled = !!serverID;
  const inboundsQ = useQuery({
    queryKey: ["settings", "inbounds", serverID],
    queryFn: () => listInbounds(serverID),
    enabled: scopedEnabled,
  });
  const clientProfilesQ = useQuery({
    queryKey: ["settings", "client-profiles", serverID],
    queryFn: () => listClientProfiles(serverID),
    enabled: scopedEnabled,
  });
  const dnsProfilesQ = useQuery({
    queryKey: ["settings", "dns-profiles", serverID],
    queryFn: () => listDNSProfiles(serverID),
    enabled: scopedEnabled,
  });
  const routeRulesQ = useQuery({
    queryKey: ["settings", "route-rules", serverID],
    queryFn: () => listRouteRules(serverID),
    enabled: scopedEnabled,
  });
  const realityProfilesQ = useQuery({
    queryKey: ["settings", "reality-profiles", serverID],
    queryFn: () => listRealityProfiles(serverID),
    enabled: scopedEnabled,
  });
  const transportProfilesQ = useQuery({
    queryKey: ["settings", "transport-profiles", serverID],
    queryFn: () => listTransportProfiles(serverID),
    enabled: scopedEnabled,
  });
  const multiplexProfilesQ = useQuery({
    queryKey: ["settings", "multiplex-profiles", serverID],
    queryFn: () => listMultiplexProfiles(serverID),
    enabled: scopedEnabled,
  });
  const hy2MasqueradeProfilesQ = useQuery({
    queryKey: ["settings", "hy2-masquerade-profiles", serverID],
    queryFn: () => listHY2MasqueradeProfiles(serverID),
    enabled: scopedEnabled,
  });
  const logProfilesQ = useQuery({
    queryKey: ["settings", "log-profiles", serverID],
    queryFn: () => listLogProfiles(serverID),
    enabled: scopedEnabled,
  });
  const tlsProfilesQ = useQuery({
    queryKey: ["settings", "tls-profiles", serverID],
    queryFn: () => listTLSProfiles(serverID),
    enabled: scopedEnabled,
  });
  const outboundsQ = useQuery({
    queryKey: ["settings", "outbounds", serverID],
    queryFn: () => listOutbounds(serverID),
    enabled: scopedEnabled,
  });
  const draftQ = useQuery({
    queryKey: ["settings", "draft-state", server?.id],
    queryFn: () => getServerDraftState(server!.id),
    enabled: !!server?.id,
  });

  const vlessInbound = inboundsQ.data?.find((item) => item.protocol === "vless") ?? null;
  const hy2Inbound = inboundsQ.data?.find((item) => item.protocol === "hysteria2") ?? null;

  const loading = serversQ.isLoading || inboundsQ.isLoading;
  const error = serversQ.error || inboundsQ.error;

  const [tab, setTab] = useState<Tab>("server");
  const [profileTab, setProfileTab] = useState<ProfileTab>("client");
  const [policyTab, setPolicyTab] = useState<PolicyTab>("route-rule");
  const [renderBusy, setRenderBusy] = useState(false);
  const [validateBusy, setValidateBusy] = useState(false);
  const [applyBusy, setApplyBusy] = useState(false);

  const invalidateAll = useCallback(async () => {
    await qc.invalidateQueries({ queryKey: ["settings"] });
    await qc.invalidateQueries({ queryKey: ["clients"] });
    await draftQ.refetch();
  }, [draftQ, qc]);

  const handleConfigSaved = useCallback(async () => {
    await invalidateAll();
  }, [invalidateAll]);

  const handleRuntimePolicyChanged = useCallback(async () => {
    await qc.invalidateQueries({ queryKey: ["settings"] });
    await qc.invalidateQueries({ queryKey: ["clients"] });
    if (server?.id) {
      await renderServerConfig(server.id);
      await draftQ.refetch();
    }
  }, [draftQ, qc, server?.id]);

  const handleArtifactPolicyChanged = useCallback(async () => {
    await qc.invalidateQueries({ queryKey: ["settings"] });
    await qc.invalidateQueries({ queryKey: ["clients"] });
  }, [qc]);

  async function handleRenderDraft() {
    if (!server) return;
    setRenderBusy(true);
    try {
      await renderServerConfig(server.id);
      await draftQ.refetch();
      toast.notify("Rendered");
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Render failed"), "error");
    } finally {
      setRenderBusy(false);
    }
  }

  async function handleValidateDraft() {
    if (!server || !draftQ.data?.draft_revision_id) return;
    setValidateBusy(true);
    try {
      await validateServerConfig(server.id, draftQ.data.draft_revision_id);
      await draftQ.refetch();
      toast.notify("Validated");
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Validate failed"), "error");
    } finally {
      setValidateBusy(false);
    }
  }

  async function handleApplyDraft() {
    if (!server || !draftQ.data?.draft_revision_id) return;
    setApplyBusy(true);
    try {
      await applyServerConfig(server.id, draftQ.data.draft_revision_id);
      await draftQ.refetch();
      toast.notify("Applied");
    } catch (err) {
      toast.notify(getAPIErrorMessage(err, "Apply failed"), "error");
    } finally {
      setApplyBusy(false);
    }
  }

  const tabs = useMemo<TabDef[]>(() => {
    const items: TabDef[] = [];
    if (server) items.push({ key: "server", label: "Server", icon: <ServerIcon size={18} strokeWidth={1.8} /> });
    if (vlessInbound) items.push({ key: "vless", label: "VLESS", icon: <Shield size={18} strokeWidth={1.8} /> });
    if (hy2Inbound) items.push({ key: "hy2", label: "Hysteria2", icon: <Zap size={18} strokeWidth={1.8} /> });
    if (server) items.push({ key: "profiles", label: "Profiles", icon: dotIcon("bg-accent") });
    if (server) items.push({ key: "policies", label: "Policies", icon: dotIcon("bg-accent-secondary") });
    if (server) items.push({ key: "preview", label: "Config", icon: <FileCode2 size={18} strokeWidth={1.8} /> });
    return items;
  }, [hy2Inbound, server, vlessInbound]);

  useEffect(() => {
    if (tabs.length > 0 && !tabs.find((item) => item.key === tab)) {
      setTab(tabs[0].key);
    }
  }, [tab, tabs]);

  const profileTabs = useMemo(
    () => [
      { key: "client", label: "Client", icon: dotIcon("bg-accent") },
      { key: "dns", label: "DNS", icon: dotIcon("bg-accent") },
      { key: "reality", label: "Reality", icon: dotIcon("bg-accent") },
      { key: "transport", label: "Transport", icon: dotIcon("bg-accent") },
      { key: "multiplex", label: "Multiplex", icon: dotIcon("bg-accent") },
      { key: "hy2-masquerade", label: "HY2 Mask", icon: dotIcon("bg-accent") },
      { key: "log", label: "Log", icon: dotIcon("bg-accent") },
      { key: "tls", label: "TLS", icon: dotIcon("bg-accent") },
    ],
    [],
  );

  const policyTabs = useMemo(
    () => [
      { key: "route-rule", label: "Route rules", icon: dotIcon("bg-accent-secondary") },
      { key: "outbound", label: "Outbounds", icon: dotIcon("bg-accent-secondary") },
    ],
    [],
  );

  const profileDescriptors = useMemo<Record<ProfileTab, PolicyDescriptor>>(
    () => ({
      client: {
        label: "Client Profiles",
        noun: "client profile",
        kind: "client-profile",
        items: (clientProfilesQ.data ?? []) as PolicyEntity[],
        loading: clientProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleArtifactPolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", protocol: "vless", mode: "standard", description: "", settings_json: "", enabled: true }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => `${String(item.protocol || "")} / ${String(item.mode || "")}`,
        save: (body, id) => upsertClientProfile(body as Partial<ClientProfile>, id),
        remove: deleteClientProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "protocol", kind: "select", label: "Protocol", options: [{ value: "vless", label: "VLESS" }, { value: "hysteria2", label: "HY2" }] },
          { key: "mode", kind: "select", label: "Mode", options: [
            ...Object.entries(VLESS_MODE_LABELS).map(([value, label]) => ({ value, label })),
            ...Object.entries(HY2_MODE_LABELS).map(([value, label]) => ({ value, label })),
          ] },
          { key: "description", kind: "text", label: "Description" },
          { key: "settings_json", kind: "json", label: "Settings JSON", rows: 8 },
          { key: "enabled", kind: "toggle", label: "Enabled" },
        ],
      },
      dns: {
        label: "DNS Profiles",
        noun: "DNS profile",
        kind: "dns-profile",
        items: (dnsProfilesQ.data ?? []) as PolicyEntity[],
        loading: dnsProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", enabled: true, strategy: "", disable_cache: false, final_server: "", servers_json: "", rules_json: "", fakeip_enabled: false }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => String(item.strategy || "default"),
        save: (body, id) => upsertDNSProfile(body as Partial<DNSProfile>, id),
        remove: deleteDNSProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "strategy", kind: "text", label: "Strategy" },
          { key: "final_server", kind: "text", label: "Final server" },
          { key: "servers_json", kind: "json", label: "Servers JSON", rows: 6 },
          { key: "rules_json", kind: "json", label: "Rules JSON", rows: 6 },
          { key: "enabled", kind: "toggle", label: "Enabled" },
          { key: "disable_cache", kind: "toggle", label: "Disable cache" },
          { key: "fakeip_enabled", kind: "toggle", label: "FakeIP" },
        ],
      },
      reality: {
        label: "Reality Profiles",
        noun: "reality profile",
        kind: "reality-profile",
        items: (realityProfilesQ.data ?? []) as PolicyEntity[],
        loading: realityProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", enabled: true, server_name: "", handshake_server: "www.cloudflare.com", handshake_server_port: 443, public_key: "", short_ids: [], short_id_rotation_mode: "", key_rotation_mode: "" }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => `${String(item.handshake_server || "")}:${String(item.handshake_server_port || "")}`,
        save: (body, id) => upsertRealityProfile(body as Partial<RealityProfile>, id),
        remove: deleteRealityProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "server_name", kind: "text", label: "Server name" },
          { key: "handshake_server", kind: "text", label: "Handshake server" },
          { key: "handshake_server_port", kind: "number", label: "Handshake port" },
          { key: "public_key", kind: "text", label: "Public key" },
          csvField("short_ids", "Short IDs"),
          { key: "short_id_rotation_mode", kind: "text", label: "Short ID rotation" },
          { key: "key_rotation_mode", kind: "text", label: "Key rotation" },
          { key: "enabled", kind: "toggle", label: "Enabled" },
        ],
      },
      transport: {
        label: "Transport Profiles",
        noun: "transport profile",
        kind: "transport-profile",
        items: (transportProfilesQ.data ?? []) as PolicyEntity[],
        loading: transportProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", enabled: true, type: "tcp", host: "", path: "", service_name: "", headers_json: "", idle_timeout: null, ping_timeout: null }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => String(item.type || ""),
        save: (body, id) => upsertTransportProfile(body as Partial<TransportProfile>, id),
        remove: deleteTransportProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "type", kind: "text", label: "Type" },
          { key: "host", kind: "text", label: "Host" },
          { key: "path", kind: "text", label: "Path" },
          { key: "service_name", kind: "text", label: "Service name" },
          { key: "headers_json", kind: "json", label: "Headers JSON", rows: 6 },
          { key: "idle_timeout", kind: "number", label: "Idle timeout" },
          { key: "ping_timeout", kind: "number", label: "Ping timeout" },
          { key: "enabled", kind: "toggle", label: "Enabled" },
        ],
      },
      multiplex: {
        label: "Multiplex Profiles",
        noun: "multiplex profile",
        kind: "multiplex-profile",
        items: (multiplexProfilesQ.data ?? []) as PolicyEntity[],
        loading: multiplexProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", enabled: true, protocol: "", max_connections: null, min_streams: null, max_streams: null, padding: false, brutal: false }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => String(item.protocol || ""),
        save: (body, id) => upsertMultiplexProfile(body as Partial<MultiplexProfile>, id),
        remove: deleteMultiplexProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "protocol", kind: "text", label: "Protocol" },
          { key: "max_connections", kind: "number", label: "Max connections" },
          { key: "min_streams", kind: "number", label: "Min streams" },
          { key: "max_streams", kind: "number", label: "Max streams" },
          { key: "enabled", kind: "toggle", label: "Enabled" },
          { key: "padding", kind: "toggle", label: "Padding" },
          { key: "brutal", kind: "toggle", label: "Brutal" },
        ],
      },
      "hy2-masquerade": {
        label: "HY2 Masquerade Profiles",
        noun: "masquerade profile",
        kind: "hy2-masquerade-profile",
        items: (hy2MasqueradeProfilesQ.data ?? []) as PolicyEntity[],
        loading: hy2MasqueradeProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", enabled: true, type: "off", url: "", rewrite_host: false, directory: "", status_code: null, headers_json: "", content: "" }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => String(item.type || ""),
        save: (body, id) => upsertHY2MasqueradeProfile(body as Partial<HY2MasqueradeProfile>, id),
        remove: deleteHY2MasqueradeProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "type", kind: "select", label: "Type", options: [{ value: "off", label: "Off" }, { value: "string", label: "String" }, { value: "file", label: "File" }, { value: "proxy", label: "Proxy" }] },
          { key: "url", kind: "text", label: "URL" },
          { key: "directory", kind: "text", label: "Directory" },
          { key: "status_code", kind: "number", label: "Status code" },
          { key: "headers_json", kind: "json", label: "Headers JSON", rows: 6 },
          { key: "content", kind: "textarea", label: "Content", rows: 6 },
          { key: "enabled", kind: "toggle", label: "Enabled" },
          { key: "rewrite_host", kind: "toggle", label: "Rewrite host" },
        ],
      },
      log: {
        label: "Log Profiles",
        noun: "log profile",
        kind: "log-profile",
        items: (logProfilesQ.data ?? []) as PolicyEntity[],
        loading: logProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", enabled: true, level: "info", output: "", timestamp: true, access_log_enabled: false, debug_mode: false }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => String(item.level || ""),
        save: (body, id) => upsertLogProfile(body as Partial<LogProfile>, id),
        remove: deleteLogProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "level", kind: "select", label: "Level", options: [{ value: "debug", label: "debug" }, { value: "info", label: "info" }, { value: "warn", label: "warn" }, { value: "error", label: "error" }] },
          { key: "output", kind: "text", label: "Output" },
          { key: "enabled", kind: "toggle", label: "Enabled" },
          { key: "timestamp", kind: "toggle", label: "Timestamp" },
          { key: "access_log_enabled", kind: "toggle", label: "Access log" },
          { key: "debug_mode", kind: "toggle", label: "Debug mode" },
        ],
      },
      tls: {
        label: "TLS Profiles",
        noun: "TLS profile",
        kind: "tls-profile",
        items: (tlsProfilesQ.data ?? []) as PolicyEntity[],
        loading: tlsProfilesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, name: "", enabled: true, server_name: "", alpn: [], certificate_path: "", key_path: "", allow_insecure: false }),
        clone: (item) => cloneEntity(item, "name"),
        title: (item) => String(item.name || "Untitled"),
        describe: (item) => String(item.server_name || ""),
        save: (body, id) => upsertTLSProfile(body as Partial<TLSProfile>, id),
        remove: deleteTLSProfile,
        fields: [
          { key: "name", kind: "text", label: "Name" },
          { key: "server_name", kind: "text", label: "Server name" },
          csvField("alpn", "ALPN"),
          { key: "certificate_path", kind: "text", label: "Certificate path" },
          { key: "key_path", kind: "text", label: "Key path" },
          { key: "enabled", kind: "toggle", label: "Enabled" },
          { key: "allow_insecure", kind: "toggle", label: "Allow insecure" },
        ],
      },
    }),
    [clientProfilesQ.data, clientProfilesQ.isLoading, dnsProfilesQ.data, dnsProfilesQ.isLoading, handleArtifactPolicyChanged, handleRuntimePolicyChanged, hy2MasqueradeProfilesQ.data, hy2MasqueradeProfilesQ.isLoading, logProfilesQ.data, logProfilesQ.isLoading, multiplexProfilesQ.data, multiplexProfilesQ.isLoading, realityProfilesQ.data, realityProfilesQ.isLoading, server?.id, tlsProfilesQ.data, tlsProfilesQ.isLoading, transportProfilesQ.data, transportProfilesQ.isLoading],
  );

  const policyDescriptors = useMemo<Record<PolicyTab, PolicyDescriptor>>(
    () => ({
      "route-rule": {
        label: "Route Rules",
        noun: "route rule",
        kind: "route-rule",
        items: (routeRulesQ.data ?? []) as PolicyEntity[],
        loading: routeRulesQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, enabled: true, priority: 100, inbound_tags: [], protocols: [], domains: [], domain_suffixes: [], domain_keywords: [], ip_cidrs: [], ports: [], network: "", geoip_codes: [], geosite_codes: [], outbound_tag: "", action: "", invert: false }),
        clone: (item) => ({ ...item, id: "", created_at: undefined, updated_at: undefined }),
        title: (item) => `Rule ${String(item.priority ?? "")}`,
        describe: (item) => `${String(item.outbound_tag || "")} / ${String(item.network || "")}`,
        save: (body, id) => upsertRouteRule(body as Partial<RouteRule>, id),
        remove: deleteRouteRule,
        fields: [
          { key: "priority", kind: "number", label: "Priority" },
          { key: "outbound_tag", kind: "text", label: "Outbound tag" },
          { key: "action", kind: "text", label: "Action" },
          { key: "network", kind: "text", label: "Network" },
          csvField("inbound_tags", "Inbound tags"),
          csvField("protocols", "Protocols"),
          csvField("domains", "Domains"),
          csvField("domain_suffixes", "Domain suffixes"),
          csvField("domain_keywords", "Domain keywords"),
          csvField("ip_cidrs", "IP CIDRs"),
          portsField("ports", "Ports"),
          csvField("geoip_codes", "GeoIP"),
          csvField("geosite_codes", "GeoSite"),
          { key: "enabled", kind: "toggle", label: "Enabled" },
          { key: "invert", kind: "toggle", label: "Invert" },
        ],
      },
      outbound: {
        label: "Outbounds",
        noun: "outbound",
        kind: "outbound",
        items: (outboundsQ.data ?? []) as PolicyEntity[],
        loading: outboundsQ.isLoading,
        serverID: server?.id ?? "",
        onChanged: handleRuntimePolicyChanged,
        createEmpty: (serverID) => ({ server_id: serverID, tag: "", type: "direct", enabled: true, priority: 100, settings_json: "", healthcheck_enabled: false }),
        clone: (item) => cloneEntity(item, "tag"),
        title: (item) => String(item.tag || "Untitled"),
        describe: (item) => `${String(item.type || "")} / ${String(item.priority ?? "")}`,
        save: (body, id) => upsertOutbound(body as Partial<Outbound>, id),
        remove: deleteOutbound,
        fields: [
          { key: "tag", kind: "text", label: "Tag" },
          { key: "type", kind: "text", label: "Type" },
          { key: "priority", kind: "number", label: "Priority" },
          { key: "settings_json", kind: "json", label: "Settings JSON", rows: 8 },
          { key: "enabled", kind: "toggle", label: "Enabled" },
          { key: "healthcheck_enabled", kind: "toggle", label: "Healthcheck" },
        ],
      },
    }),
    [handleRuntimePolicyChanged, outboundsQ.data, outboundsQ.isLoading, routeRulesQ.data, routeRulesQ.isLoading, server?.id],
  );

  const currentProfileDescriptor = profileDescriptors[profileTab];
  const currentPolicyDescriptor = policyDescriptors[policyTab];

  return (
    <div className="space-y-6 pb-24">
      <PageHeader title="Settings" />

      {server ? (
        <RevisionCard
          draft={draftQ.data ?? null}
          renderBusy={renderBusy}
          validateBusy={validateBusy}
          applyBusy={applyBusy}
          onRender={() => void handleRenderDraft()}
          onValidate={() => void handleValidateDraft()}
          onApply={() => void handleApplyDraft()}
        />
      ) : null}

      {loading ? (
        <div className="flex items-center gap-2 py-8 text-[16px] text-txt-secondary">
          <Loader2 size={18} className="animate-spin" /> Loading
        </div>
      ) : error ? (
        <ErrorBanner message="Failed to load settings." />
      ) : tabs.length === 0 ? (
        <div className="rounded-2xl bg-surface-2 py-12 text-center text-[16px] text-txt-muted">
          Nothing configured.
        </div>
      ) : (
        <div className="space-y-5">
          <TabsNav active={tab} onChange={(value) => setTab(value as Tab)} tabs={tabs} />

          <div className="panel-card space-y-5">
            {tab === "server" && server ? <ServerForm server={server} onSaved={() => void handleConfigSaved()} /> : null}
            {tab === "vless" && vlessInbound ? (
              <VLESSForm
                inbound={vlessInbound}
                onSaved={() => void handleConfigSaved()}
                tlsProfiles={tlsProfilesQ.data ?? []}
                realityProfiles={realityProfilesQ.data ?? []}
                transportProfiles={transportProfilesQ.data ?? []}
                multiplexProfiles={multiplexProfilesQ.data ?? []}
              />
            ) : null}
            {tab === "hy2" && hy2Inbound ? (
              <HY2Form
                inbound={hy2Inbound}
                onSaved={() => void handleConfigSaved()}
                tlsProfiles={tlsProfilesQ.data ?? []}
                masqueradeProfiles={hy2MasqueradeProfilesQ.data ?? []}
              />
            ) : null}
            {tab === "profiles" ? (
              <div className="space-y-5">
                <TabsNav active={profileTab} onChange={(value) => setProfileTab(value as ProfileTab)} tabs={profileTabs} />
                <PolicyManager descriptor={currentProfileDescriptor} />
              </div>
            ) : null}
            {tab === "policies" ? (
              <div className="space-y-5">
                <TabsNav active={policyTab} onChange={(value) => setPolicyTab(value as PolicyTab)} tabs={policyTabs} />
                <PolicyManager descriptor={currentPolicyDescriptor} />
              </div>
            ) : null}
            {tab === "preview" && server ? <ConfigPreview server={server} /> : null}
          </div>
        </div>
      )}
    </div>
  );
}
