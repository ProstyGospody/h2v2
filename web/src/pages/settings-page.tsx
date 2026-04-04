import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle2,
  Code2,
  Loader2,
  Network,
  RefreshCw,
  Save,
  Server,
  ShieldCheck,
} from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";

import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { listInbounds, listServers, previewServerConfig, updateInbound, updateServer } from "@/domain/inbounds/services";
import type { Inbound, Server as ServerType } from "@/domain/inbounds/types";
import { getAPIErrorMessage } from "@/services/api";
import {
  Button,
  Input,
  SectionCard,
  ToggleField,
  cn,
} from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";

// ---------------------------------------------------------------------------
// Server settings form
// ---------------------------------------------------------------------------

type ServerFormState = {
  public_host: string;
  subscription_base_url: string;
  singbox_binary_path: string;
  singbox_config_path: string;
  singbox_service_name: string;
};

function fromServer(s: ServerType): ServerFormState {
  return {
    public_host: s.public_host,
    subscription_base_url: s.subscription_base_url ?? "",
    singbox_binary_path: s.singbox_binary_path ?? "",
    singbox_config_path: s.singbox_config_path ?? "",
    singbox_service_name: s.singbox_service_name ?? "",
  };
}

function ServerSection({
  server,
  onSaved,
}: {
  server: ServerType;
  onSaved: () => void;
}) {
  const [form, setForm] = useState<ServerFormState>(() => fromServer(server));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  useEffect(() => {
    setForm(fromServer(server));
  }, [server]);

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
    <SectionCard title="Server" icon={<Server size={18} strokeWidth={1.8} />}>
      <form className="space-y-4" onSubmit={submit}>
        {error && <ErrorBanner message={error} />}
        <Input
          label="Public Host"
          placeholder="example.com"
          value={form.public_host}
          onChange={(e) => set("public_host", e.target.value)}
        />
        <Input
          label="Subscription Base URL"
          placeholder="https://example.com (optional, defaults to panel URL)"
          value={form.subscription_base_url}
          onChange={(e) => set("subscription_base_url", e.target.value)}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="sing-box Binary Path"
            placeholder="/usr/local/bin/sing-box"
            value={form.singbox_binary_path}
            onChange={(e) => set("singbox_binary_path", e.target.value)}
          />
          <Input
            label="sing-box Config Path"
            placeholder="/etc/h2v2/sing-box/config.json"
            value={form.singbox_config_path}
            onChange={(e) => set("singbox_config_path", e.target.value)}
          />
        </div>
        <Input
          label="sing-box Service Name"
          placeholder="sing-box"
          value={form.singbox_service_name}
          onChange={(e) => set("singbox_service_name", e.target.value)}
        />
        <div className="flex justify-end">
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? (
              <><Loader2 size={15} className="animate-spin" /> Saving…</>
            ) : (
              <><Save size={15} /> Save</>
            )}
          </Button>
        </div>
      </form>
    </SectionCard>
  );
}

// ---------------------------------------------------------------------------
// VLESS inbound settings form
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

function fromVLESSInbound(ib: Inbound): VLESSFormState {
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

function VLESSSection({
  inbound,
  onSaved,
}: {
  inbound: Inbound;
  onSaved: () => void;
}) {
  const [form, setForm] = useState<VLESSFormState>(() => fromVLESSInbound(inbound));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  useEffect(() => {
    setForm(fromVLESSInbound(inbound));
  }, [inbound]);

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
          reality_handshake_server_port: parseInt(form.reality_handshake_server_port, 10) || 443,
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
    <SectionCard title="VLESS Inbound" icon={<ShieldCheck size={18} strokeWidth={1.8} />}>
      <form className="space-y-4" onSubmit={submit}>
        {error && <ErrorBanner message={error} />}

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="Listen Port"
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

        <ToggleField
          label="Enabled"
          checked={form.enabled}
          onCheckedChange={(v) => set("enabled", v)}
        />

        <ToggleField
          label="Reality"
          checked={form.reality_enabled}
          onCheckedChange={(v) => set("reality_enabled", v)}
        />

        {form.reality_enabled && (
          <div className="space-y-3 rounded-xl border border-border/30 bg-surface-1/40 p-4">
            <p className="text-[12px] font-semibold uppercase tracking-wide text-txt-muted">Reality settings</p>
            <Input
              label="Handshake Server"
              placeholder="www.cloudflare.com"
              value={form.reality_handshake_server}
              onChange={(e) => set("reality_handshake_server", e.target.value)}
            />
            <div className="grid gap-3 sm:grid-cols-2">
              <Input
                label="Handshake Port"
                type="number"
                value={form.reality_handshake_server_port}
                onChange={(e) => set("reality_handshake_server_port", e.target.value)}
              />
              <Input
                label="Short ID"
                placeholder="auto-generated if empty"
                value={form.reality_short_id}
                onChange={(e) => set("reality_short_id", e.target.value)}
              />
            </div>
            <Input
              label="Private Key"
              placeholder="auto-generated if empty"
              value={form.reality_private_key}
              onChange={(e) => set("reality_private_key", e.target.value)}
            />
            <Input
              label="Public Key"
              placeholder="derived from private key if empty"
              value={form.reality_public_key}
              onChange={(e) => set("reality_public_key", e.target.value)}
            />
          </div>
        )}

        {!form.reality_enabled && (
          <Input
            label="TLS SNI"
            placeholder="example.com"
            value={form.tls_server_name}
            onChange={(e) => set("tls_server_name", e.target.value)}
          />
        )}

        <div className="grid gap-3 sm:grid-cols-3">
          <div className="space-y-1">
            <label className="text-[13px] font-medium text-txt-secondary">Transport</label>
            <select
              className={cn(
                "w-full rounded-xl border border-border/60 bg-surface-1 px-3 py-2.5 text-[14px] text-txt-primary",
                "focus:outline-none focus:ring-2 focus:ring-accent/40",
              )}
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

        <div className="flex justify-end">
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? (
              <><Loader2 size={15} className="animate-spin" /> Saving…</>
            ) : (
              <><Save size={15} /> Save</>
            )}
          </Button>
        </div>
      </form>
    </SectionCard>
  );
}

// ---------------------------------------------------------------------------
// HY2 inbound settings form
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

function fromHY2Inbound(ib: Inbound): HY2FormState {
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

function HY2Section({
  inbound,
  onSaved,
}: {
  inbound: Inbound;
  onSaved: () => void;
}) {
  const [form, setForm] = useState<HY2FormState>(() => fromHY2Inbound(inbound));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  useEffect(() => {
    setForm(fromHY2Inbound(inbound));
  }, [inbound]);

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
    <SectionCard title="Hysteria2 Inbound" icon={<Network size={18} strokeWidth={1.8} />}>
      <form className="space-y-4" onSubmit={submit}>
        {error && <ErrorBanner message={error} />}

        <Input
          label="Listen Port"
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

        <div className="space-y-3 rounded-xl border border-border/30 bg-surface-1/40 p-4">
          <p className="text-[12px] font-semibold uppercase tracking-wide text-txt-muted">TLS</p>
          <Input
            label="SNI / Domain"
            placeholder="example.com"
            value={form.tls_server_name}
            onChange={(e) => set("tls_server_name", e.target.value)}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Certificate Path"
              placeholder="/etc/h2v2/hysteria/server.crt"
              value={form.tls_certificate_path}
              onChange={(e) => set("tls_certificate_path", e.target.value)}
            />
            <Input
              label="Key Path"
              placeholder="/etc/h2v2/hysteria/server.key"
              value={form.tls_key_path}
              onChange={(e) => set("tls_key_path", e.target.value)}
            />
          </div>
          <ToggleField
            label="Allow Insecure (self-signed cert)"
            checked={form.allow_insecure}
            onCheckedChange={(v) => set("allow_insecure", v)}
          />
        </div>

        <ToggleField
          label="Ignore Client Bandwidth"
          checked={form.ignore_client_bandwidth}
          onCheckedChange={(v) => set("ignore_client_bandwidth", v)}
        />

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="Upload Limit (Mbps)"
            type="number"
            min="0"
            placeholder="Unlimited"
            value={form.up_mbps}
            onChange={(e) => set("up_mbps", e.target.value)}
          />
          <Input
            label="Download Limit (Mbps)"
            type="number"
            min="0"
            placeholder="Unlimited"
            value={form.down_mbps}
            onChange={(e) => set("down_mbps", e.target.value)}
          />
        </div>

        <div className="space-y-3 rounded-xl border border-border/30 bg-surface-1/40 p-4">
          <p className="text-[12px] font-semibold uppercase tracking-wide text-txt-muted">Obfuscation (optional)</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Obfs Type"
              placeholder="salamander"
              value={form.obfs_type}
              onChange={(e) => set("obfs_type", e.target.value)}
            />
            <Input
              label="Obfs Password"
              placeholder="leave empty to disable"
              value={form.obfs_password}
              onChange={(e) => set("obfs_password", e.target.value)}
            />
          </div>
        </div>

        <div className="flex justify-end">
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? (
              <><Loader2 size={15} className="animate-spin" /> Saving…</>
            ) : (
              <><Save size={15} /> Save</>
            )}
          </Button>
        </div>
      </form>
    </SectionCard>
  );
}

// ---------------------------------------------------------------------------
// Server config preview
// ---------------------------------------------------------------------------

function ConfigPreviewSection({ server }: { server: ServerType }) {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ config_json: string; check_warning?: string } | null>(null);
  const [error, setError] = useState<string | null>(null);

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

  return (
    <SectionCard title="Config Preview" icon={<Code2 size={18} strokeWidth={1.8} />}>
      <div className="space-y-4">
        <p className="text-[13px] text-txt-secondary">
          Preview the sing-box JSON config that would be applied to the server. The config is built from current inbound settings and active users.
        </p>
        <Button type="button" variant="secondary" disabled={loading} onClick={() => void loadPreview()}>
          {loading ? (
            <><Loader2 size={15} className="animate-spin" /> Generating…</>
          ) : (
            <><RefreshCw size={15} /> {result ? "Refresh" : "Generate Preview"}</>
          )}
        </Button>

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
            <pre className="max-h-[520px] overflow-auto rounded-xl border border-border/40 bg-surface-0 p-4 text-[12px] leading-relaxed text-txt-secondary">
              {prettyJson}
            </pre>
          </div>
        )}
      </div>
    </SectionCard>
  );
}

// ---------------------------------------------------------------------------
// Main settings page
// ---------------------------------------------------------------------------

export default function SettingsPage() {
  const qc = useQueryClient();

  const serversQ = useQuery({
    queryKey: ["settings", "servers"],
    queryFn: listServers,
  });

  const inboundsQ = useQuery({
    queryKey: ["settings", "inbounds"],
    queryFn: () => listInbounds(),
  });

  const server = serversQ.data?.[0] ?? null;
  const vlessInbound = inboundsQ.data?.find((ib) => ib.protocol === "vless") ?? null;
  const hy2Inbound = inboundsQ.data?.find((ib) => ib.protocol === "hysteria2") ?? null;

  const loading = serversQ.isLoading || inboundsQ.isLoading;
  const error = serversQ.error || inboundsQ.error;

  function invalidate() {
    void qc.invalidateQueries({ queryKey: ["settings"] });
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <PageHeader
        title="Settings"
        subtitle="Configure server, inbound protocols, and subscription options."
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
        <div className="space-y-6">
          {server ? (
            <ServerSection server={server} onSaved={invalidate} />
          ) : (
            <div className="rounded-xl border border-border/40 bg-surface-1/50 p-6 text-center text-[14px] text-txt-secondary">
              No server configured. Create a user first via the Users page — the server will be auto-provisioned.
            </div>
          )}

          {vlessInbound && (
            <VLESSSection inbound={vlessInbound} onSaved={invalidate} />
          )}

          {hy2Inbound && (
            <HY2Section inbound={hy2Inbound} onSaved={invalidate} />
          )}

          {!vlessInbound && !hy2Inbound && !loading && (
            <div className="rounded-xl border border-border/40 bg-surface-1/50 p-6 text-center text-[14px] text-txt-secondary">
              No inbounds found. They will appear here after provisioning a user.
            </div>
          )}

          {server && <ConfigPreviewSection server={server} />}
        </div>
      )}
    </div>
  );
}
