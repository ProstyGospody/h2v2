import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Copy,
  Loader2,
  RefreshCw,
  RotateCcw,
  Save,
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

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import {
  listInbounds,
  listServers,
  previewServerConfig,
  updateInbound,
  updateServer,
} from "@/domain/inbounds/services";
import type { Inbound, Server as ServerType } from "@/domain/inbounds/types";
import { getAPIErrorMessage } from "@/services/api";
import { Button, Input, Toggle, cn } from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";

// ---------------------------------------------------------------------------
// Shared primitives
// ---------------------------------------------------------------------------

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
  return { form, setForm, dirty, reset };
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
        <h3 className="text-[11px] font-semibold uppercase tracking-wider text-txt-muted">
          {title}
        </h3>
      ) : null}
      {children}
    </div>
  );
}

function InlineToggle({
  label,
  checked,
  onCheckedChange,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: (v: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-4 rounded-lg px-1 py-2">
      <span className="text-[14px] font-medium text-txt-primary">{label}</span>
      <Toggle checked={checked} onCheckedChange={onCheckedChange} />
    </label>
  );
}

function SelectField({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <div>
      <label className="mb-2 block text-[13px] font-medium text-txt-secondary">{label}</label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border-0 bg-[var(--control-bg)] px-4 py-2.5 text-[14px] font-medium text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  );
}

function CopyField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch { /* ignore */ }
  }
  return (
    <div>
      <label className="mb-2 block text-[13px] font-medium text-txt-secondary">{label}</label>
      <div className="flex gap-2">
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full rounded-lg border-0 bg-[var(--control-bg)] px-4 py-2.5 font-mono text-[12px] text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors placeholder:text-txt-tertiary focus:bg-[var(--control-bg-hover)] focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]"
        />
        <button
          type="button"
          onClick={copy}
          disabled={!value}
          className="inline-grid h-10 w-10 shrink-0 place-items-center rounded-lg text-txt-muted transition-colors hover:bg-surface-3/60 hover:text-txt-primary disabled:opacity-40"
          aria-label="Copy"
        >
          {copied ? <Check size={14} /> : <Copy size={14} />}
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
      <div className="pointer-events-auto flex items-center gap-2 rounded-xl bg-surface-2/95 px-3 py-2 shadow-[0_18px_42px_-16px_var(--dialog-shadow)] backdrop-blur-xl">
        <span className="px-2 text-[13px] font-medium text-txt-secondary">Unsaved changes</span>
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

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------

type Tab = "server" | "vless" | "hy2" | "preview";

function TabsBar({
  active,
  onChange,
  tabs,
}: {
  active: Tab;
  onChange: (t: Tab) => void;
  tabs: { key: Tab; label: string }[];
}) {
  return (
    <div className="flex items-center gap-1 overflow-x-auto">
      {tabs.map((t) => (
        <button
          key={t.key}
          type="button"
          onClick={() => onChange(t.key)}
          className={cn(
            "shrink-0 rounded-lg px-3.5 py-2 text-[13px] font-semibold transition-colors",
            active === t.key
              ? "bg-surface-3/70 text-txt-primary"
              : "text-txt-secondary hover:text-txt-primary",
          )}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Server form
// ---------------------------------------------------------------------------

type ServerFormState = {
  public_host: string;
  subscription_base_url: string;
  singbox_binary_path: string;
  singbox_config_path: string;
  singbox_service_name: string;
};

function serverToForm(s: ServerType): ServerFormState {
  return {
    public_host: s.public_host,
    subscription_base_url: s.subscription_base_url ?? "",
    singbox_binary_path: s.singbox_binary_path ?? "",
    singbox_config_path: s.singbox_config_path ?? "",
    singbox_service_name: s.singbox_service_name ?? "",
  };
}

function ServerForm({ server, onSaved }: { server: ServerType; onSaved: () => void }) {
  const initial = useMemo(() => serverToForm(server), [server]);
  const { form, setForm, dirty, reset } = useDirtyForm(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  function set<K extends keyof ServerFormState>(k: K, v: ServerFormState[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function submit(e?: FormEvent) {
    e?.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await updateServer(server.id, {
        public_host: form.public_host.trim() || undefined,
        subscription_base_url: form.subscription_base_url.trim() || undefined,
        singbox_binary_path: form.singbox_binary_path.trim() || undefined,
        singbox_config_path: form.singbox_config_path.trim() || undefined,
        singbox_service_name: form.singbox_service_name.trim() || undefined,
      });
      toast.notify("Saved", "success");
      onSaved();
    } catch (err) {
      setError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <form className="space-y-8" onSubmit={submit}>
        {error ? <ErrorBanner message={error} /> : null}

        <FieldGroup title="Endpoints">
          <Input
            label="Public host"
            placeholder="example.com"
            value={form.public_host}
            onChange={(e) => set("public_host", e.target.value)}
          />
          <Input
            label="Subscription base URL"
            placeholder="https://example.com"
            value={form.subscription_base_url}
            onChange={(e) => set("subscription_base_url", e.target.value)}
          />
        </FieldGroup>

        <FieldGroup title="sing-box">
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Binary path"
              placeholder="/usr/local/bin/sing-box"
              value={form.singbox_binary_path}
              onChange={(e) => set("singbox_binary_path", e.target.value)}
            />
            <Input
              label="Config path"
              placeholder="/etc/h2v2/sing-box/config.json"
              value={form.singbox_config_path}
              onChange={(e) => set("singbox_config_path", e.target.value)}
            />
          </div>
          <Input
            label="Service name"
            placeholder="sing-box"
            value={form.singbox_service_name}
            onChange={(e) => set("singbox_service_name", e.target.value)}
          />
        </FieldGroup>
      </form>
      <SaveBar dirty={dirty} busy={busy} onSave={() => void submit()} onReset={reset} />
    </>
  );
}

// ---------------------------------------------------------------------------
// VLESS form
// ---------------------------------------------------------------------------

type VLESSFormState = {
  listen_port: string;
  enabled: boolean;
  reality_enabled: boolean;
  reality_public_key: string;
  reality_private_key: string;
  reality_short_id: string;
  reality_handshake_server: string;
  reality_handshake_server_port: string;
  tls_server_name: string;
  flow: string;
  transport_type: string;
  transport_host: string;
  transport_path: string;
};

function vlessToForm(ib: Inbound): VLESSFormState {
  const v = ib.vless;
  return {
    listen_port: String(ib.listen_port),
    enabled: ib.enabled,
    reality_enabled: v?.reality_enabled ?? false,
    reality_public_key: v?.reality_public_key ?? "",
    reality_private_key: v?.reality_private_key ?? "",
    reality_short_id: v?.reality_short_id ?? "",
    reality_handshake_server: v?.reality_handshake_server ?? "www.cloudflare.com",
    reality_handshake_server_port: String(v?.reality_handshake_server_port ?? 443),
    tls_server_name: v?.tls_server_name ?? "",
    flow: v?.flow ?? "xtls-rprx-vision",
    transport_type: v?.transport_type ?? "tcp",
    transport_host: v?.transport_host ?? "",
    transport_path: v?.transport_path ?? "",
  };
}

function VLESSForm({ inbound, onSaved }: { inbound: Inbound; onSaved: () => void }) {
  const initial = useMemo(() => vlessToForm(inbound), [inbound]);
  const { form, setForm, dirty, reset } = useDirtyForm(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  function set<K extends keyof VLESSFormState>(k: K, v: VLESSFormState[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function submit(e?: FormEvent) {
    e?.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const port = parseInt(form.listen_port, 10);
      await updateInbound(inbound.id, {
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
        },
      });
      toast.notify("Saved", "success");
      onSaved();
    } catch (err) {
      setError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <form className="space-y-8" onSubmit={submit}>
        {error ? <ErrorBanner message={error} /> : null}

        <FieldGroup title="Inbound">
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Port"
              type="number"
              min="1"
              max="65535"
              value={form.listen_port}
              onChange={(e) => set("listen_port", e.target.value)}
            />
            <Input
              label="Flow"
              value={form.flow}
              onChange={(e) => set("flow", e.target.value)}
            />
          </div>
          <InlineToggle
            label="Enabled"
            checked={form.enabled}
            onCheckedChange={(v) => set("enabled", v)}
          />
          <InlineToggle
            label="Reality"
            checked={form.reality_enabled}
            onCheckedChange={(v) => set("reality_enabled", v)}
          />
        </FieldGroup>

        {form.reality_enabled ? (
          <FieldGroup title="Reality">
            <Input
              label="Handshake server"
              value={form.reality_handshake_server}
              onChange={(e) => set("reality_handshake_server", e.target.value)}
            />
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label="Handshake port"
                type="number"
                value={form.reality_handshake_server_port}
                onChange={(e) => set("reality_handshake_server_port", e.target.value)}
              />
              <CopyField
                label="Short ID"
                value={form.reality_short_id}
                onChange={(v) => set("reality_short_id", v)}
              />
            </div>
            <CopyField
              label="Private key"
              value={form.reality_private_key}
              onChange={(v) => set("reality_private_key", v)}
            />
            <CopyField
              label="Public key"
              value={form.reality_public_key}
              onChange={(v) => set("reality_public_key", v)}
            />
          </FieldGroup>
        ) : (
          <FieldGroup title="TLS">
            <Input
              label="SNI"
              value={form.tls_server_name}
              onChange={(e) => set("tls_server_name", e.target.value)}
            />
          </FieldGroup>
        )}

        <FieldGroup title="Transport">
          <div className="grid gap-4 sm:grid-cols-3">
            <SelectField
              label="Type"
              value={form.transport_type}
              onChange={(v) => set("transport_type", v)}
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
                  onChange={(e) => set("transport_host", e.target.value)}
                />
                <Input
                  label="Path"
                  placeholder="/"
                  value={form.transport_path}
                  onChange={(e) => set("transport_path", e.target.value)}
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

// ---------------------------------------------------------------------------
// HY2 form
// ---------------------------------------------------------------------------

type HY2FormState = {
  listen_port: string;
  enabled: boolean;
  tls_server_name: string;
  tls_certificate_path: string;
  tls_key_path: string;
  allow_insecure: boolean;
  ignore_client_bandwidth: boolean;
  up_mbps: string;
  down_mbps: string;
  obfs_type: string;
  obfs_password: string;
};

function hy2ToForm(ib: Inbound): HY2FormState {
  const h = ib.hysteria2;
  return {
    listen_port: String(ib.listen_port),
    enabled: ib.enabled,
    tls_server_name: h?.tls_server_name ?? "",
    tls_certificate_path: h?.tls_certificate_path ?? "",
    tls_key_path: h?.tls_key_path ?? "",
    allow_insecure: h?.allow_insecure ?? false,
    ignore_client_bandwidth: h?.ignore_client_bandwidth ?? true,
    up_mbps: h?.up_mbps != null ? String(h.up_mbps) : "",
    down_mbps: h?.down_mbps != null ? String(h.down_mbps) : "",
    obfs_type: h?.obfs_type ?? "",
    obfs_password: h?.obfs_password ?? "",
  };
}

function HY2Form({ inbound, onSaved }: { inbound: Inbound; onSaved: () => void }) {
  const initial = useMemo(() => hy2ToForm(inbound), [inbound]);
  const { form, setForm, dirty, reset } = useDirtyForm(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  function set<K extends keyof HY2FormState>(k: K, v: HY2FormState[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function submit(e?: FormEvent) {
    e?.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const port = parseInt(form.listen_port, 10);
      const upMbps = form.up_mbps.trim() ? parseInt(form.up_mbps, 10) : null;
      const downMbps = form.down_mbps.trim() ? parseInt(form.down_mbps, 10) : null;
      await updateInbound(inbound.id, {
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
        },
      });
      toast.notify("Saved", "success");
      onSaved();
    } catch (err) {
      setError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <form className="space-y-8" onSubmit={submit}>
        {error ? <ErrorBanner message={error} /> : null}

        <FieldGroup title="Inbound">
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Port"
              type="number"
              min="1"
              max="65535"
              value={form.listen_port}
              onChange={(e) => set("listen_port", e.target.value)}
            />
            <div className="self-end">
              <InlineToggle
                label="Enabled"
                checked={form.enabled}
                onCheckedChange={(v) => set("enabled", v)}
              />
            </div>
          </div>
        </FieldGroup>

        <FieldGroup title="TLS">
          <Input
            label="SNI"
            value={form.tls_server_name}
            onChange={(e) => set("tls_server_name", e.target.value)}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Certificate path"
              value={form.tls_certificate_path}
              onChange={(e) => set("tls_certificate_path", e.target.value)}
            />
            <Input
              label="Key path"
              value={form.tls_key_path}
              onChange={(e) => set("tls_key_path", e.target.value)}
            />
          </div>
          <InlineToggle
            label="Allow insecure"
            checked={form.allow_insecure}
            onCheckedChange={(v) => set("allow_insecure", v)}
          />
        </FieldGroup>

        <FieldGroup title="Bandwidth">
          <InlineToggle
            label="Ignore client bandwidth"
            checked={form.ignore_client_bandwidth}
            onCheckedChange={(v) => set("ignore_client_bandwidth", v)}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Upload (Mbps)"
              type="number"
              min="0"
              placeholder="Unlimited"
              value={form.up_mbps}
              onChange={(e) => set("up_mbps", e.target.value)}
            />
            <Input
              label="Download (Mbps)"
              type="number"
              min="0"
              placeholder="Unlimited"
              value={form.down_mbps}
              onChange={(e) => set("down_mbps", e.target.value)}
            />
          </div>
        </FieldGroup>

        <FieldGroup title="Obfuscation">
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              label="Type"
              placeholder="salamander"
              value={form.obfs_type}
              onChange={(e) => set("obfs_type", e.target.value)}
            />
            <Input
              label="Password"
              value={form.obfs_password}
              onChange={(e) => set("obfs_password", e.target.value)}
            />
          </div>
        </FieldGroup>
      </form>
      <SaveBar dirty={dirty} busy={busy} onSave={() => void submit()} onReset={reset} />
    </>
  );
}

// ---------------------------------------------------------------------------
// Config preview
// ---------------------------------------------------------------------------

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
    } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Button type="button" disabled={loading} onClick={() => void load()}>
          {loading ? (
            <Loader2 size={14} className="animate-spin" />
          ) : (
            <RefreshCw size={14} />
          )}
          {result ? "Refresh" : "Generate"}
        </Button>
        {result ? (
          <Button type="button" onClick={copy} disabled={!pretty}>
            {copied ? <Check size={14} /> : <Copy size={14} />}
            {copied ? "Copied" : "Copy"}
          </Button>
        ) : null}
      </div>

      {error ? <ErrorBanner message={error} /> : null}

      {result ? (
        <>
          {result.check_warning ? (
            <div className="flex items-start gap-2 rounded-lg bg-status-warning/10 px-3 py-2.5 text-[13px] text-status-warning">
              <AlertTriangle size={15} className="mt-0.5 shrink-0" />
              <span className="break-all">{result.check_warning}</span>
            </div>
          ) : (
            <div className="flex items-center gap-2 rounded-lg bg-status-success/10 px-3 py-2.5 text-[13px] text-status-success">
              <CheckCircle2 size={15} className="shrink-0" />
              <span>Valid</span>
            </div>
          )}
          <pre className="max-h-[560px] overflow-auto rounded-xl bg-surface-0 p-4 font-mono text-[12px] leading-relaxed text-txt-secondary">
            {pretty}
          </pre>
        </>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function SettingsPage() {
  const qc = useQueryClient();

  const serversQ = useQuery({ queryKey: ["settings", "servers"], queryFn: listServers });
  const inboundsQ = useQuery({ queryKey: ["settings", "inbounds"], queryFn: () => listInbounds() });

  const server = serversQ.data?.[0] ?? null;
  const vlessInbound = inboundsQ.data?.find((ib) => ib.protocol === "vless") ?? null;
  const hy2Inbound = inboundsQ.data?.find((ib) => ib.protocol === "hysteria2") ?? null;

  const loading = serversQ.isLoading || inboundsQ.isLoading;
  const error = serversQ.error || inboundsQ.error;

  const invalidate = useCallback(() => {
    void qc.invalidateQueries({ queryKey: ["settings"] });
  }, [qc]);

  const tabs = useMemo(() => {
    const list: { key: Tab; label: string }[] = [];
    if (server) list.push({ key: "server", label: "Server" });
    if (vlessInbound) list.push({ key: "vless", label: "VLESS" });
    if (hy2Inbound) list.push({ key: "hy2", label: "Hysteria2" });
    if (server) list.push({ key: "preview", label: "Config" });
    return list;
  }, [server, vlessInbound, hy2Inbound]);

  const [tab, setTab] = useState<Tab>("server");

  useEffect(() => {
    if (tabs.length > 0 && !tabs.find((t) => t.key === tab)) {
      setTab(tabs[0].key);
    }
  }, [tabs, tab]);

  return (
    <div className="space-y-6 pb-24">
      <PageHeader title="Settings" />

      {loading ? (
        <div className="flex items-center gap-2 py-8 text-[14px] text-txt-secondary">
          <Loader2 size={18} className="animate-spin" /> Loading
        </div>
      ) : error ? (
        <ErrorBanner message="Failed to load settings." />
      ) : tabs.length === 0 ? (
        <div className="py-10 text-center text-[14px] text-txt-muted">Nothing configured.</div>
      ) : (
        <>
          <TabsBar active={tab} onChange={setTab} tabs={tabs} />

          <div className="max-w-[720px]">
            {tab === "server" && server ? <ServerForm server={server} onSaved={invalidate} /> : null}
            {tab === "vless" && vlessInbound ? (
              <VLESSForm inbound={vlessInbound} onSaved={invalidate} />
            ) : null}
            {tab === "hy2" && hy2Inbound ? (
              <HY2Form inbound={hy2Inbound} onSaved={invalidate} />
            ) : null}
            {tab === "preview" && server ? <ConfigPreview server={server} /> : null}
          </div>
        </>
      )}
    </div>
  );
}
