import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Code2,
  Copy,
  Loader2,
  Network,
  RefreshCw,
  RotateCcw,
  Save,
  Server,
  ShieldCheck,
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
import { Button, Input, ToggleField, Tooltip, cn } from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";

// ---------------------------------------------------------------------------
// Shared section shell + scrollspy nav
// ---------------------------------------------------------------------------

type SectionKey = "server" | "vless" | "hy2" | "preview";

type SectionMeta = {
  key: SectionKey;
  label: string;
  icon: ReactNode;
};

const SECTIONS: SectionMeta[] = [
  { key: "server", label: "Server", icon: <Server size={15} strokeWidth={1.9} /> },
  { key: "vless", label: "VLESS Reality", icon: <ShieldCheck size={15} strokeWidth={1.9} /> },
  { key: "hy2", label: "Hysteria2", icon: <Network size={15} strokeWidth={1.9} /> },
  { key: "preview", label: "Config preview", icon: <Code2 size={15} strokeWidth={1.9} /> },
];

function useScrollSpy(keys: SectionKey[]) {
  const [active, setActive] = useState<SectionKey>(keys[0]);
  useEffect(() => {
    const els = keys
      .map((k) => document.getElementById(`settings-${k}`))
      .filter((el): el is HTMLElement => el != null);
    if (els.length === 0) return;
    const obs = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => (a.target as HTMLElement).offsetTop - (b.target as HTMLElement).offsetTop);
        if (visible[0]) {
          const id = (visible[0].target as HTMLElement).id.replace("settings-", "") as SectionKey;
          setActive(id);
        }
      },
      { rootMargin: "-35% 0px -55% 0px", threshold: [0, 0.1, 0.5, 1] },
    );
    els.forEach((el) => obs.observe(el));
    return () => obs.disconnect();
  }, [keys]);
  return active;
}

function SectionShell({
  id,
  title,
  description,
  icon,
  dirty,
  onReset,
  children,
  footer,
}: {
  id: SectionKey;
  title: string;
  description?: string;
  icon: ReactNode;
  dirty?: boolean;
  onReset?: () => void;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <section
      id={`settings-${id}`}
      className="scroll-mt-24 rounded-2xl border border-border/40 bg-surface-2/30 p-6"
    >
      <div className="mb-5 flex items-start justify-between gap-4 border-b border-border/30 pb-5">
        <div className="flex items-start gap-3">
          <div className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-surface-3/55 text-txt-secondary">
            {icon}
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="text-[16px] font-semibold text-txt-primary">{title}</h2>
              {dirty ? (
                <Tooltip content="Unsaved changes">
                  <span className="inline-flex h-2 w-2 rounded-full bg-status-warning" />
                </Tooltip>
              ) : null}
            </div>
            {description ? (
              <p className="mt-1 text-[13px] text-txt-secondary">{description}</p>
            ) : null}
          </div>
        </div>
        {dirty && onReset ? (
          <button
            type="button"
            onClick={onReset}
            className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[12px] font-medium text-txt-muted transition-colors hover:bg-surface-3/60 hover:text-txt-primary"
          >
            <RotateCcw size={13} /> Reset
          </button>
        ) : null}
      </div>
      {children}
      {footer ? <div className="mt-5 flex justify-end border-t border-border/30 pt-4">{footer}</div> : null}
    </section>
  );
}

function SaveButton({ busy, disabled }: { busy: boolean; disabled?: boolean }) {
  return (
    <Button type="submit" variant="primary" disabled={busy || disabled}>
      {busy ? (
        <>
          <Loader2 size={15} className="animate-spin" /> Saving…
        </>
      ) : (
        <>
          <Save size={15} /> Save changes
        </>
      )}
    </Button>
  );
}

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

function CopyField({
  label,
  value,
  placeholder,
  onChange,
  mono,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (v: string) => void;
  mono?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* ignore */
    }
  }
  return (
    <div>
      <label className="mb-2 block text-[13px] font-medium text-txt-secondary">{label}</label>
      <div className="flex gap-2">
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className={cn(
            "w-full rounded-lg border-0 bg-[var(--control-bg)] px-4 py-2.5 text-[14px] font-medium text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors placeholder:text-txt-tertiary focus:bg-[var(--control-bg-hover)] focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]",
            mono && "font-mono text-[12px]",
          )}
        />
        <button
          type="button"
          onClick={copy}
          disabled={!value}
          className="inline-grid h-10 w-10 shrink-0 place-items-center rounded-lg border border-border/50 bg-surface-2/60 text-txt-muted transition-colors hover:bg-surface-3/60 hover:text-txt-primary disabled:opacity-40"
          aria-label={`Copy ${label}`}
        >
          {copied ? <Check size={14} /> : <Copy size={14} />}
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Server section
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

function ServerSection({ server, onSaved }: { server: ServerType; onSaved: () => void }) {
  const initial = useMemo(() => serverToForm(server), [server]);
  const { form, setForm, dirty, reset } = useDirtyForm(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  function set<K extends keyof ServerFormState>(k: K, v: ServerFormState[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
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
      toast.notify("Server settings saved", "success");
      onSaved();
    } catch (err) {
      setError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <SectionShell
      id="server"
      title="Server"
      description="Public endpoints and sing-box process configuration."
      icon={<Server size={18} strokeWidth={1.8} />}
      dirty={dirty}
      onReset={reset}
      footer={<SaveButton busy={busy} disabled={!dirty} />}
    >
      <form className="space-y-4" onSubmit={submit}>
        {error && <ErrorBanner message={error} />}
        <Input
          label="Public host"
          placeholder="example.com"
          value={form.public_host}
          onChange={(e) => set("public_host", e.target.value)}
        />
        <Input
          label="Subscription base URL"
          placeholder="https://example.com (optional, defaults to panel URL)"
          value={form.subscription_base_url}
          onChange={(e) => set("subscription_base_url", e.target.value)}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="sing-box binary path"
            placeholder="/usr/local/bin/sing-box"
            value={form.singbox_binary_path}
            onChange={(e) => set("singbox_binary_path", e.target.value)}
          />
          <Input
            label="sing-box config path"
            placeholder="/etc/h2v2/sing-box/config.json"
            value={form.singbox_config_path}
            onChange={(e) => set("singbox_config_path", e.target.value)}
          />
        </div>
        <Input
          label="sing-box service name"
          placeholder="sing-box"
          value={form.singbox_service_name}
          onChange={(e) => set("singbox_service_name", e.target.value)}
        />
      </form>
    </SectionShell>
  );
}

// ---------------------------------------------------------------------------
// VLESS section
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

function VLESSSection({ inbound, onSaved }: { inbound: Inbound; onSaved: () => void }) {
  const initial = useMemo(() => vlessToForm(inbound), [inbound]);
  const { form, setForm, dirty, reset } = useDirtyForm(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  function set<K extends keyof VLESSFormState>(k: K, v: VLESSFormState[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
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
          reality_handshake_server_port:
            parseInt(form.reality_handshake_server_port, 10) || 443,
          tls_server_name: form.tls_server_name.trim(),
          flow: form.flow.trim(),
          transport_type: form.transport_type,
          transport_host: form.transport_host.trim(),
          transport_path: form.transport_path.trim(),
        },
      });
      toast.notify("VLESS settings saved", "success");
      onSaved();
    } catch (err) {
      setError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <SectionShell
      id="vless"
      title="VLESS Reality"
      description="Reality handshake spoofs a real TLS target — leave keys blank to auto-generate."
      icon={<ShieldCheck size={18} strokeWidth={1.8} />}
      dirty={dirty}
      onReset={reset}
      footer={<SaveButton busy={busy} disabled={!dirty} />}
    >
      <form className="space-y-4" onSubmit={submit}>
        {error && <ErrorBanner message={error} />}

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="Listen port"
            type="number"
            min="1"
            max="65535"
            value={form.listen_port}
            onChange={(e) => set("listen_port", e.target.value)}
          />
          <Input
            label="Flow"
            placeholder="xtls-rprx-vision"
            value={form.flow}
            onChange={(e) => set("flow", e.target.value)}
          />
        </div>

        <ToggleField label="Enabled" checked={form.enabled} onCheckedChange={(v) => set("enabled", v)} />
        <ToggleField
          label="Reality"
          checked={form.reality_enabled}
          onCheckedChange={(v) => set("reality_enabled", v)}
        />

        {form.reality_enabled ? (
          <div className="space-y-3 rounded-xl border border-border/30 bg-surface-1/40 p-4">
            <p className="text-[12px] font-semibold uppercase tracking-wide text-txt-muted">
              Reality handshake
            </p>
            <Input
              label="Handshake server"
              placeholder="www.cloudflare.com"
              value={form.reality_handshake_server}
              onChange={(e) => set("reality_handshake_server", e.target.value)}
            />
            <div className="grid gap-3 sm:grid-cols-2">
              <Input
                label="Handshake port"
                type="number"
                value={form.reality_handshake_server_port}
                onChange={(e) => set("reality_handshake_server_port", e.target.value)}
              />
              <CopyField
                label="Short ID"
                placeholder="auto-generated if empty"
                value={form.reality_short_id}
                onChange={(v) => set("reality_short_id", v)}
                mono
              />
            </div>
            <CopyField
              label="Private key"
              placeholder="auto-generated if empty"
              value={form.reality_private_key}
              onChange={(v) => set("reality_private_key", v)}
              mono
            />
            <CopyField
              label="Public key"
              placeholder="derived from private key if empty"
              value={form.reality_public_key}
              onChange={(v) => set("reality_public_key", v)}
              mono
            />
          </div>
        ) : (
          <Input
            label="TLS SNI"
            placeholder="example.com"
            value={form.tls_server_name}
            onChange={(e) => set("tls_server_name", e.target.value)}
          />
        )}

        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <label className="mb-2 block text-[13px] font-medium text-txt-secondary">Transport</label>
            <select
              className="w-full rounded-lg border-0 bg-[var(--control-bg)] px-3 py-2.5 text-[14px] font-medium text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]"
              value={form.transport_type}
              onChange={(e) => set("transport_type", e.target.value)}
            >
              <option value="tcp">TCP</option>
              <option value="ws">WebSocket</option>
              <option value="grpc">gRPC</option>
              <option value="xhttp">XHTTP (SplitHTTP)</option>
            </select>
          </div>
          {form.transport_type !== "tcp" && (
            <>
              <Input
                label="Host"
                placeholder="optional"
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
          )}
        </div>
      </form>
    </SectionShell>
  );
}

// ---------------------------------------------------------------------------
// HY2 section
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

function HY2Section({ inbound, onSaved }: { inbound: Inbound; onSaved: () => void }) {
  const initial = useMemo(() => hy2ToForm(inbound), [inbound]);
  const { form, setForm, dirty, reset } = useDirtyForm(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  function set<K extends keyof HY2FormState>(k: K, v: HY2FormState[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
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
      toast.notify("Hysteria2 settings saved", "success");
      onSaved();
    } catch (err) {
      setError(getAPIErrorMessage(err, "Operation failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <SectionShell
      id="hy2"
      title="Hysteria2"
      description="Low-latency QUIC-based transport with optional obfuscation."
      icon={<Network size={18} strokeWidth={1.8} />}
      dirty={dirty}
      onReset={reset}
      footer={<SaveButton busy={busy} disabled={!dirty} />}
    >
      <form className="space-y-4" onSubmit={submit}>
        {error && <ErrorBanner message={error} />}

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="Listen port"
            type="number"
            min="1"
            max="65535"
            value={form.listen_port}
            onChange={(e) => set("listen_port", e.target.value)}
          />
          <ToggleField
            label="Enabled"
            checked={form.enabled}
            onCheckedChange={(v) => set("enabled", v)}
          />
        </div>

        <div className="space-y-3 rounded-xl border border-border/30 bg-surface-1/40 p-4">
          <p className="text-[12px] font-semibold uppercase tracking-wide text-txt-muted">TLS</p>
          <Input
            label="SNI / domain"
            placeholder="example.com"
            value={form.tls_server_name}
            onChange={(e) => set("tls_server_name", e.target.value)}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Certificate path"
              placeholder="/etc/h2v2/hysteria/server.crt"
              value={form.tls_certificate_path}
              onChange={(e) => set("tls_certificate_path", e.target.value)}
            />
            <Input
              label="Key path"
              placeholder="/etc/h2v2/hysteria/server.key"
              value={form.tls_key_path}
              onChange={(e) => set("tls_key_path", e.target.value)}
            />
          </div>
          <ToggleField
            label="Allow insecure (self-signed cert)"
            checked={form.allow_insecure}
            onCheckedChange={(v) => set("allow_insecure", v)}
          />
        </div>

        <div className="space-y-3 rounded-xl border border-border/30 bg-surface-1/40 p-4">
          <p className="text-[12px] font-semibold uppercase tracking-wide text-txt-muted">Bandwidth</p>
          <ToggleField
            label="Ignore client-advertised bandwidth"
            checked={form.ignore_client_bandwidth}
            onCheckedChange={(v) => set("ignore_client_bandwidth", v)}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Upload limit (Mbps)"
              type="number"
              min="0"
              placeholder="Unlimited"
              value={form.up_mbps}
              onChange={(e) => set("up_mbps", e.target.value)}
            />
            <Input
              label="Download limit (Mbps)"
              type="number"
              min="0"
              placeholder="Unlimited"
              value={form.down_mbps}
              onChange={(e) => set("down_mbps", e.target.value)}
            />
          </div>
        </div>

        <div className="space-y-3 rounded-xl border border-border/30 bg-surface-1/40 p-4">
          <p className="text-[12px] font-semibold uppercase tracking-wide text-txt-muted">
            Obfuscation (optional)
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Obfs type"
              placeholder="salamander"
              value={form.obfs_type}
              onChange={(e) => set("obfs_type", e.target.value)}
            />
            <Input
              label="Obfs password"
              placeholder="leave empty to disable"
              value={form.obfs_password}
              onChange={(e) => set("obfs_password", e.target.value)}
            />
          </div>
        </div>
      </form>
    </SectionShell>
  );
}

// ---------------------------------------------------------------------------
// Config preview section
// ---------------------------------------------------------------------------

function ConfigPreviewSection({ server }: { server: ServerType }) {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ config_json: string; check_warning?: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function loadPreview() {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const data = await previewServerConfig(server.id);
      setResult(data);
    } catch (err) {
      setError(getAPIErrorMessage(err, "Failed to load preview"));
    } finally {
      setLoading(false);
    }
  }

  let prettyJson = "";
  if (result?.config_json) {
    try {
      prettyJson = JSON.stringify(JSON.parse(result.config_json), null, 2);
    } catch {
      prettyJson = result.config_json;
    }
  }

  async function copy() {
    if (!prettyJson) return;
    try {
      await navigator.clipboard.writeText(prettyJson);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* ignore */
    }
  }

  return (
    <SectionShell
      id="preview"
      title="Config preview"
      description="The sing-box JSON that would be rendered from current inbound settings."
      icon={<Code2 size={18} strokeWidth={1.8} />}
    >
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Button type="button" disabled={loading} onClick={() => void loadPreview()}>
            {loading ? (
              <>
                <Loader2 size={15} className="animate-spin" /> Generating…
              </>
            ) : (
              <>
                <RefreshCw size={15} /> {result ? "Refresh" : "Generate preview"}
              </>
            )}
          </Button>
          {result ? (
            <Button type="button" onClick={copy} disabled={!prettyJson}>
              {copied ? <Check size={14} /> : <Copy size={14} />}
              {copied ? "Copied" : "Copy JSON"}
            </Button>
          ) : null}
        </div>

        {error && <ErrorBanner message={error} />}

        {result && (
          <div className="space-y-3">
            {result.check_warning ? (
              <div className="flex items-start gap-2 rounded-xl border border-status-warning/30 bg-status-warning/8 px-3 py-2.5 text-[13px] text-status-warning">
                <AlertTriangle size={15} className="mt-0.5 shrink-0" />
                <span className="break-all">{result.check_warning}</span>
              </div>
            ) : (
              <div className="flex items-center gap-2 rounded-xl border border-status-success/30 bg-status-success/8 px-3 py-2.5 text-[13px] text-status-success">
                <CheckCircle2 size={15} className="shrink-0" />
                <span>sing-box validation passed</span>
              </div>
            )}
            <pre className="max-h-[520px] overflow-auto rounded-xl border border-border/40 bg-surface-0 p-4 font-mono text-[12px] leading-relaxed text-txt-secondary">
              {prettyJson}
            </pre>
          </div>
        )}
      </div>
    </SectionShell>
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

  const activeKeys = useMemo<SectionKey[]>(() => {
    const keys: SectionKey[] = [];
    if (server) keys.push("server");
    if (vlessInbound) keys.push("vless");
    if (hy2Inbound) keys.push("hy2");
    if (server) keys.push("preview");
    return keys;
  }, [server, vlessInbound, hy2Inbound]);

  const active = useScrollSpy(activeKeys);

  function jump(key: SectionKey) {
    const el = document.getElementById(`settings-${key}`);
    if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  return (
    <div className="space-y-5">
      <PageHeader
        title="Settings"
        subtitle="Configure server endpoints, inbound protocols, and review the rendered sing-box config."
      />

      {loading && (
        <div className="flex items-center gap-2 py-8 text-[14px] text-txt-secondary">
          <Loader2 size={18} className="animate-spin" /> Loading settings…
        </div>
      )}

      {!loading && error && (
        <ErrorBanner message="Failed to load settings. Make sure the server is configured." />
      )}

      {!loading && !error && (
        <div className="grid gap-6 lg:grid-cols-[220px_minmax(0,1fr)]">
          {/* Sidebar */}
          <aside className="lg:sticky lg:top-20 lg:self-start">
            <nav className="flex flex-row gap-1 overflow-x-auto rounded-2xl border border-border/40 bg-surface-2/30 p-2 lg:flex-col lg:overflow-visible">
              {SECTIONS.filter((s) => activeKeys.includes(s.key)).map((s) => (
                <button
                  key={s.key}
                  type="button"
                  onClick={() => jump(s.key)}
                  className={cn(
                    "flex shrink-0 items-center gap-2.5 rounded-lg px-3 py-2 text-left text-[13px] font-medium transition-colors",
                    active === s.key
                      ? "bg-surface-3/70 text-txt-primary"
                      : "text-txt-secondary hover:bg-surface-3/40 hover:text-txt-primary",
                  )}
                >
                  <span
                    className={cn(
                      "grid h-6 w-6 place-items-center rounded-md",
                      active === s.key
                        ? "bg-accent/12 text-accent-light"
                        : "bg-surface-3/50 text-txt-muted",
                    )}
                  >
                    {s.icon}
                  </span>
                  <span className="truncate">{s.label}</span>
                </button>
              ))}
            </nav>
          </aside>

          {/* Content */}
          <div className="space-y-6">
            {server ? (
              <ServerSection server={server} onSaved={invalidate} />
            ) : (
              <div className="rounded-2xl border border-border/40 bg-surface-1/50 p-6 text-center text-[14px] text-txt-secondary">
                No server configured. Run <code>panel-api bootstrap-inbounds</code> or create a user first.
              </div>
            )}

            {vlessInbound && <VLESSSection inbound={vlessInbound} onSaved={invalidate} />}
            {hy2Inbound && <HY2Section inbound={hy2Inbound} onSaved={invalidate} />}

            {!vlessInbound && !hy2Inbound && (
              <div className="rounded-2xl border border-border/40 bg-surface-1/50 p-6 text-center text-[14px] text-txt-secondary">
                No inbounds found. They will appear here after the installer provisions them.
              </div>
            )}

            {server && <ConfigPreviewSection server={server} />}
          </div>
        </div>
      )}
    </div>
  );
}
