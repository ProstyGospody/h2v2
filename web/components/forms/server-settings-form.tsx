import { Code, Globe, Shield, SlidersHorizontal, Wrench } from "lucide-react";
import { ReactNode, useEffect } from "react";

import { Hy2Settings } from "@/domain/settings/types";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Toggle, cn } from "@/src/components/ui";

function SectionTitle({ icon, title, description }: { icon: ReactNode; title: string; description?: string }) {
  return (
    <div className="flex items-center gap-3">
      <div className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-surface-3/60">
        <span className="text-txt-secondary">{icon}</span>
      </div>
      <div>
        <h3 className="text-[15px] font-bold text-txt-primary">{title}</h3>
        {description && <p className="text-[13px] text-txt-muted">{description}</p>}
      </div>
    </div>
  );
}

function LabeledToggle({ label, checked, onCheckedChange }: { label: string; checked: boolean; onCheckedChange: (v: boolean) => void }) {
  return (
    <div className="flex min-h-[52px] items-center justify-between rounded-xl border border-[var(--control-border)] bg-[var(--control-bg)] px-4 py-3 transition-colors hover:border-[var(--control-border-strong)] hover:bg-[var(--control-bg-hover)]">
      <p className="text-[14px] font-medium text-txt-primary">{label}</p>
      <Toggle checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  );
}

function SelectField({ label, value, onValueChange, options }: { label: string; value: string; onValueChange: (v: string) => void; options: { value: string; label: string }[] }) {
  return (
    <div>
      <label className="mb-2 block text-[13px] font-medium text-txt-secondary">{label}</label>
      <Select value={value} onValueChange={onValueChange}>
        <SelectTrigger><SelectValue /></SelectTrigger>
        <SelectContent>{options.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
      </Select>
    </div>
  );
}

export function ServerSettingsForm({ draft, rawYaml, onDraftChange }: { draft: Hy2Settings; rawYaml: string; onDraftChange: (n: Hy2Settings) => void }) {
  const acmeDomains = (draft.acme?.domains || []).join(", ");
  const tlsMode = draft.tlsMode === "tls" ? "tls" : "acme";
  const obfsType = draft.obfs?.type === "salamander" ? "salamander" : "none";
  const masqueradeType = draft.masquerade?.type || "none";

  useEffect(() => { if (obfsType !== "none" && masqueradeType !== "none") onDraftChange({ ...draft, masquerade: undefined }); }, [draft, masqueradeType, obfsType, onDraftChange]);

  return (
    <div className="grid gap-5 xl:grid-cols-12">
      <section className="space-y-5 rounded-2xl bg-surface-2 p-6 xl:col-span-7">
        <SectionTitle icon={<Globe size={18} strokeWidth={1.6} />} title="Connection Profile" description="Listener and encryption settings" />
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Input label="Listen" value={draft.listen} onChange={(e) => onDraftChange({ ...draft, listen: e.target.value.replace(/^:/, "") })} />
          <SelectField label="TLS Mode" value={tlsMode} onValueChange={(v) => onDraftChange({ ...draft, tlsMode: v, tlsEnabled: true })} options={[{ value: "acme", label: "ACME" }, { value: "tls", label: "Manual TLS" }]} />
          <SelectField label="OBFS" value={obfsType} onValueChange={(v) => { if (v === "salamander") { onDraftChange({ ...draft, obfs: { type: "salamander", salamander: { password: draft.obfs?.salamander?.password || "" } }, masquerade: undefined }); return; } onDraftChange({ ...draft, obfs: undefined }); }} options={[{ value: "none", label: "Disabled" }, { value: "salamander", label: "Salamander" }]} />
          <SelectField label="Masquerade" value={masqueradeType} onValueChange={(v) => { if (v === "none") { onDraftChange({ ...draft, masquerade: undefined }); return; } onDraftChange({ ...draft, obfs: undefined, masquerade: { ...(draft.masquerade || {}), type: v } }); }} options={[{ value: "none", label: "Disabled" }, { value: "proxy", label: "Proxy" }, { value: "file", label: "File" }, { value: "string", label: "String" }]} />
        </div>
        {tlsMode === "acme" ? (
          <div className="grid gap-4 md:grid-cols-12">
            <div className="md:col-span-8"><Input label="ACME Domains" value={acmeDomains} onChange={(e) => onDraftChange({ ...draft, acme: { domains: e.target.value.split(",").map((s) => s.trim()).filter(Boolean), email: draft.acme?.email || "" } })} placeholder="example.com, api.example.com" /></div>
            <div className="md:col-span-4"><Input label="ACME Email" value={draft.acme?.email || ""} onChange={(e) => onDraftChange({ ...draft, acme: { domains: draft.acme?.domains || [], email: e.target.value } })} /></div>
          </div>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            <Input label="TLS Cert Path" value={draft.tls?.cert || ""} onChange={(e) => onDraftChange({ ...draft, tls: { cert: e.target.value, key: draft.tls?.key || "" } })} />
            <Input label="TLS Key Path" value={draft.tls?.key || ""} onChange={(e) => onDraftChange({ ...draft, tls: { cert: draft.tls?.cert || "", key: e.target.value } })} />
          </div>
        )}
        {obfsType === "salamander" && <Input label="OBFS Password" value={draft.obfs?.salamander?.password || ""} onChange={(e) => onDraftChange({ ...draft, obfs: { type: "salamander", salamander: { password: e.target.value } } })} />}
      </section>

      <section className="space-y-5 rounded-2xl bg-surface-2 p-6 xl:col-span-5">
        <SectionTitle icon={<Wrench size={18} strokeWidth={1.6} />} title="Runtime Defaults" description="Bandwidth, transport and protocol options" />
        <div className="grid gap-4 md:grid-cols-2">
          <LabeledToggle label="TLS Insecure" checked={Boolean(draft.clientTLSInsecure)} onCheckedChange={(v) => onDraftChange({ ...draft, clientTLSInsecure: v })} />
          <LabeledToggle label="Ignore Client Bandwidth" checked={Boolean(draft.ignoreClientBandwidth)} onCheckedChange={(v) => onDraftChange({ ...draft, ignoreClientBandwidth: v })} />
          <LabeledToggle label="Disable UDP" checked={Boolean(draft.disableUDP)} onCheckedChange={(v) => onDraftChange({ ...draft, disableUDP: v })} />
          <LabeledToggle label="Speed Test" checked={Boolean(draft.speedTest)} onCheckedChange={(v) => onDraftChange({ ...draft, speedTest: v })} />
          <Input label="Bandwidth Up" value={draft.bandwidth?.up || ""} onChange={(e) => onDraftChange({ ...draft, bandwidth: { up: e.target.value, down: draft.bandwidth?.down || "" } })} placeholder="100 mbps" />
          <Input label="Bandwidth Down" value={draft.bandwidth?.down || ""} onChange={(e) => onDraftChange({ ...draft, bandwidth: { up: draft.bandwidth?.up || "", down: e.target.value } })} placeholder="200 mbps" />
        </div>
        <Input label="UDP Idle Timeout" value={draft.udpIdleTimeout || ""} onChange={(e) => onDraftChange({ ...draft, udpIdleTimeout: e.target.value })} placeholder="90s" />
      </section>

      {masqueradeType !== "none" && (
        <section className="space-y-5 rounded-2xl bg-surface-2 p-6 xl:col-span-7">
          <SectionTitle icon={<Shield size={18} strokeWidth={1.6} />} title="Masquerade Details" description="Traffic camouflage configuration" />
          {masqueradeType === "proxy" && (
            <div className="space-y-4">
              <Input label="Masquerade Proxy URL" value={draft.masquerade?.proxy?.url || ""} onChange={(e) => onDraftChange({ ...draft, masquerade: { type: "proxy", proxy: { url: e.target.value, rewriteHost: draft.masquerade?.proxy?.rewriteHost || false, insecure: draft.masquerade?.proxy?.insecure || false } } })} />
              <div className="grid gap-4 md:grid-cols-2">
                <LabeledToggle label="Rewrite Host Header" checked={Boolean(draft.masquerade?.proxy?.rewriteHost)} onCheckedChange={(v) => onDraftChange({ ...draft, masquerade: { type: "proxy", proxy: { url: draft.masquerade?.proxy?.url || "", rewriteHost: v, insecure: draft.masquerade?.proxy?.insecure || false } } })} />
                <LabeledToggle label="Allow Insecure TLS" checked={Boolean(draft.masquerade?.proxy?.insecure)} onCheckedChange={(v) => onDraftChange({ ...draft, masquerade: { type: "proxy", proxy: { url: draft.masquerade?.proxy?.url || "", rewriteHost: draft.masquerade?.proxy?.rewriteHost || false, insecure: v } } })} />
              </div>
            </div>
          )}
          {masqueradeType === "file" && <Input label="Masquerade File Dir" value={draft.masquerade?.file?.dir || ""} onChange={(e) => onDraftChange({ ...draft, masquerade: { type: "file", file: { dir: e.target.value } } })} />}
          {masqueradeType === "string" && (
            <div className="grid gap-4 md:grid-cols-12">
              <div className="md:col-span-9">
                <label className="mb-2 block text-[13px] font-medium text-txt-secondary">Masquerade String Content</label>
                <textarea value={draft.masquerade?.string?.content || ""} onChange={(e) => onDraftChange({ ...draft, masquerade: { type: "string", string: { content: e.target.value, statusCode: draft.masquerade?.string?.statusCode || 200 } } })} rows={3}
                  className="w-full rounded-lg border border-[var(--control-border)] bg-[var(--control-bg)] px-4 py-2.5 text-[14px] font-medium text-txt-primary outline-none transition-colors focus:border-accent-secondary/50 focus:bg-[var(--control-bg-hover)] focus:shadow-[0_0_0_3px_var(--accent-soft)]" />
              </div>
              <div className="md:col-span-3">
                <Input label="Status Code" type="number" value={String(draft.masquerade?.string?.statusCode ?? 200)} onChange={(e) => { const p = Number.parseInt(e.target.value, 10); onDraftChange({ ...draft, masquerade: { type: "string", string: { content: draft.masquerade?.string?.content || "", statusCode: Number.isFinite(p) ? p : 200 } } }); }} min={100} max={599} />
              </div>
            </div>
          )}
        </section>
      )}

      <section className={cn("space-y-5 rounded-2xl bg-surface-2 p-6", masqueradeType !== "none" ? "xl:col-span-5" : "xl:col-span-12")}>
        <SectionTitle icon={<SlidersHorizontal size={18} strokeWidth={1.6} />} title="QUIC Tuning" description="Advanced transport parameters" />
        <LabeledToggle label="Enable Custom QUIC" checked={draft.quicEnabled} onCheckedChange={(v) => onDraftChange({ ...draft, quicEnabled: v })} />
        {draft.quicEnabled && (
          <div className="grid gap-4 md:grid-cols-2">
            <Input label="QUIC Max Idle" value={draft.quic?.maxIdleTimeout || ""} onChange={(e) => onDraftChange({ ...draft, quic: { ...(draft.quic || {}), maxIdleTimeout: e.target.value } })} placeholder="30s" />
            <div>
              <label className="mb-2 block text-[13px] font-medium text-txt-secondary">Disable Path MTU Discovery</label>
              <div className="flex h-10 items-center justify-end rounded-lg border border-[var(--control-border)] bg-[var(--control-bg)] px-3 transition-colors hover:border-[var(--control-border-strong)] hover:bg-[var(--control-bg-hover)]">
                <Toggle checked={Boolean(draft.quic?.disablePathMTUDiscovery)} onCheckedChange={(v) => onDraftChange({ ...draft, quic: { ...(draft.quic || {}), disablePathMTUDiscovery: v } })} />
              </div>
            </div>
          </div>
        )}
      </section>

      <section className="space-y-5 rounded-2xl bg-surface-2 p-6 xl:col-span-12">
        <SectionTitle icon={<Code size={18} strokeWidth={1.6} />} title="Generated YAML" description="Preview of the configuration file" />
        <textarea readOnly value={rawYaml} rows={16}
          className="w-full rounded-xl border border-[var(--control-border)] bg-[var(--control-bg)] px-5 py-4 font-mono text-[13px] leading-6 text-txt outline-none" />
        <p className="text-[13px] text-txt-muted">Read-only preview of generated configuration</p>
      </section>
    </div>
  );
}
