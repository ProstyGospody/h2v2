import { Code, Gauge, Globe, Lock, Shield, SlidersHorizontal } from "lucide-react";
import { useEffect, useMemo, type ReactNode } from "react";

import { Hy2Settings } from "@/domain/settings/types";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Toggle } from "@/src/components/ui";

function SectionTitle({ icon, title }: { icon: ReactNode; title: string }) {
  return (
    <div className="flex items-center gap-3">
      <div className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-surface-3/55 text-txt-secondary">
        {icon}
      </div>
      <h3 className="text-[15px] font-semibold text-txt-primary">{title}</h3>
    </div>
  );
}

function SectionCard({
  title,
  icon,
  children,
}: {
  title: string;
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="panel-card space-y-4">
      <SectionTitle icon={icon} title={title} />
      {children}
    </section>
  );
}

function SelectField({
  label,
  value,
  onValueChange,
  options,
}: {
  label: string;
  value: string;
  onValueChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <div>
      <label className="mb-2 block text-[13px] font-medium text-txt-secondary">{label}</label>
      <Select value={value} onValueChange={onValueChange}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function ToggleField({
  label,
  checked,
  onCheckedChange,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: (next: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between rounded-xl bg-[var(--control-bg)] px-4 py-3 shadow-[inset_0_0_0_1px_var(--control-border)]">
      <span className="text-[14px] font-medium text-txt-primary">{label}</span>
      <Toggle checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  );
}

function escapeHtml(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function highlightYaml(yaml: string): string {
  return yaml
    .split("\n")
    .map((line) => {
      if (/^\s*#/.test(line)) {
        return `<span class="text-txt-muted">${escapeHtml(line)}</span>`;
      }
      const matched = line.replace(
        /^(\s*)([\w./-]+)(:)(.*)/,
        (_match, indent, key, colon, rest) => {
          const restTrim = rest.trim();
          let value = escapeHtml(rest);
          if (/^\s*(true|false)$/i.test(rest)) {
            value = ` <span class="text-status-warning">${escapeHtml(restTrim)}</span>`;
          } else if (/^\s*\d+(\.\d+)?$/.test(rest)) {
            value = ` <span class="text-status-info">${escapeHtml(restTrim)}</span>`;
          } else if (restTrim.length > 0) {
            value = ` <span class="text-status-success">${escapeHtml(restTrim)}</span>`;
          }
          return `${escapeHtml(indent)}<span class="text-accent">${escapeHtml(key)}</span><span class="text-txt-muted">${escapeHtml(colon)}</span>${value}`;
        },
      );
      return matched === line ? escapeHtml(line) : matched;
    })
    .join("\n");
}

function YamlPreview({ value }: { value: string }) {
  return (
    <pre
      className="max-h-[58vh] overflow-auto rounded-xl bg-[var(--control-bg)] px-4 py-3 font-mono text-[12px] leading-6 text-txt-secondary shadow-[inset_0_0_0_1px_var(--control-border)]"
      dangerouslySetInnerHTML={{ __html: highlightYaml(value) }}
    />
  );
}

export function ServerSettingsForm({
  draft,
  rawYaml,
  onDraftChange,
}: {
  draft: Hy2Settings;
  rawYaml: string;
  onDraftChange: (next: Hy2Settings) => void;
}) {
  const tlsMode = draft.tlsMode === "tls" ? "tls" : "acme";
  const obfsType = draft.obfs?.type === "salamander" ? "salamander" : "none";
  const masqueradeType = draft.masquerade?.type || "none";
  const acmeDomains = (draft.acme?.domains || []).join(", ");

  useEffect(() => {
    if (obfsType !== "none" && masqueradeType !== "none") {
      onDraftChange({ ...draft, masquerade: undefined });
    }
  }, [draft, masqueradeType, obfsType, onDraftChange]);

  const snapshotItems = useMemo(
    () => [
      { label: "Listen", value: draft.listen || "-" },
      { label: "TLS", value: tlsMode.toUpperCase() },
      { label: "Masking", value: obfsType !== "none" ? "OBFS" : masqueradeType !== "none" ? masqueradeType : "None" },
      { label: "QUIC", value: draft.quicEnabled ? "On" : "Off" },
    ],
    [draft.listen, draft.quicEnabled, masqueradeType, obfsType, tlsMode],
  );

  return (
    <div className="grid gap-4 xl:grid-cols-12">
      <div className="space-y-4 xl:col-span-8">
        <SectionCard title="General" icon={<Globe size={17} strokeWidth={1.7} />}>
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Listen"
              value={draft.listen}
              onChange={(event) => onDraftChange({ ...draft, listen: event.target.value.replace(/^:/, "") })}
            />
            <SelectField
              label="TLS Mode"
              value={tlsMode}
              onValueChange={(value) => onDraftChange({ ...draft, tlsMode: value, tlsEnabled: true })}
              options={[
                { value: "acme", label: "ACME" },
                { value: "tls", label: "Manual" },
              ]}
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <ToggleField
              label="Speed Test"
              checked={Boolean(draft.speedTest)}
              onCheckedChange={(value) => onDraftChange({ ...draft, speedTest: value })}
            />
            <ToggleField
              label="Ignore Client Bandwidth"
              checked={Boolean(draft.ignoreClientBandwidth)}
              onCheckedChange={(value) => onDraftChange({ ...draft, ignoreClientBandwidth: value })}
            />
          </div>
        </SectionCard>

        <SectionCard title="TLS" icon={<Lock size={17} strokeWidth={1.7} />}>
          {tlsMode === "acme" ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <Input
                label="ACME Domains"
                value={acmeDomains}
                onChange={(event) =>
                  onDraftChange({
                    ...draft,
                    acme: {
                      domains: event.target.value
                        .split(",")
                        .map((item) => item.trim())
                        .filter(Boolean),
                      email: draft.acme?.email || "",
                    },
                  })
                }
                placeholder="example.com, api.example.com"
              />
              <Input
                label="ACME Email"
                value={draft.acme?.email || ""}
                onChange={(event) =>
                  onDraftChange({
                    ...draft,
                    acme: { domains: draft.acme?.domains || [], email: event.target.value },
                  })
                }
              />
            </div>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2">
              <Input
                label="Cert Path"
                value={draft.tls?.cert || ""}
                onChange={(event) =>
                  onDraftChange({
                    ...draft,
                    tls: { cert: event.target.value, key: draft.tls?.key || "" },
                  })
                }
              />
              <Input
                label="Key Path"
                value={draft.tls?.key || ""}
                onChange={(event) =>
                  onDraftChange({
                    ...draft,
                    tls: { cert: draft.tls?.cert || "", key: event.target.value },
                  })
                }
              />
            </div>
          )}
          <ToggleField
            label="TLS Insecure"
            checked={Boolean(draft.clientTLSInsecure)}
            onCheckedChange={(value) => onDraftChange({ ...draft, clientTLSInsecure: value })}
          />
        </SectionCard>

        <SectionCard title="Network" icon={<Shield size={17} strokeWidth={1.7} />}>
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Bandwidth Up"
              value={draft.bandwidth?.up || ""}
              onChange={(event) =>
                onDraftChange({
                  ...draft,
                  bandwidth: { up: event.target.value, down: draft.bandwidth?.down || "" },
                })
              }
              placeholder="100 mbps"
            />
            <Input
              label="Bandwidth Down"
              value={draft.bandwidth?.down || ""}
              onChange={(event) =>
                onDraftChange({
                  ...draft,
                  bandwidth: { up: draft.bandwidth?.up || "", down: event.target.value },
                })
              }
              placeholder="200 mbps"
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <ToggleField
              label="Disable UDP"
              checked={Boolean(draft.disableUDP)}
              onCheckedChange={(value) => onDraftChange({ ...draft, disableUDP: value })}
            />
            <Input
              label="UDP Idle Timeout"
              value={draft.udpIdleTimeout || ""}
              onChange={(event) => onDraftChange({ ...draft, udpIdleTimeout: event.target.value })}
              placeholder="90s"
            />
          </div>
        </SectionCard>

        <SectionCard title="Masking" icon={<Lock size={17} strokeWidth={1.7} />}>
          <div className="grid gap-3 sm:grid-cols-2">
            <SelectField
              label="OBFS"
              value={obfsType}
              onValueChange={(value) => {
                if (value === "salamander") {
                  onDraftChange({
                    ...draft,
                    obfs: {
                      type: "salamander",
                      salamander: { password: draft.obfs?.salamander?.password || "" },
                    },
                    masquerade: undefined,
                  });
                  return;
                }
                onDraftChange({ ...draft, obfs: undefined });
              }}
              options={[
                { value: "none", label: "Disabled" },
                { value: "salamander", label: "Salamander" },
              ]}
            />
            <SelectField
              label="Masquerade"
              value={masqueradeType}
              onValueChange={(value) => {
                if (value === "none") {
                  onDraftChange({ ...draft, masquerade: undefined });
                  return;
                }
                onDraftChange({
                  ...draft,
                  obfs: undefined,
                  masquerade: { ...(draft.masquerade || {}), type: value },
                });
              }}
              options={[
                { value: "none", label: "Disabled" },
                { value: "proxy", label: "Proxy" },
                { value: "file", label: "File" },
                { value: "string", label: "String" },
              ]}
            />
          </div>

          {obfsType === "salamander" && (
            <Input
              label="OBFS Password"
              value={draft.obfs?.salamander?.password || ""}
              onChange={(event) =>
                onDraftChange({
                  ...draft,
                  obfs: { type: "salamander", salamander: { password: event.target.value } },
                })
              }
            />
          )}

          {masqueradeType === "proxy" && (
            <div className="space-y-3">
              <Input
                label="Proxy URL"
                value={draft.masquerade?.proxy?.url || ""}
                onChange={(event) =>
                  onDraftChange({
                    ...draft,
                    masquerade: {
                      type: "proxy",
                      proxy: {
                        url: event.target.value,
                        rewriteHost: draft.masquerade?.proxy?.rewriteHost || false,
                        insecure: draft.masquerade?.proxy?.insecure || false,
                      },
                    },
                  })
                }
              />
              <div className="grid gap-3 sm:grid-cols-2">
                <ToggleField
                  label="Rewrite Host"
                  checked={Boolean(draft.masquerade?.proxy?.rewriteHost)}
                  onCheckedChange={(value) =>
                    onDraftChange({
                      ...draft,
                      masquerade: {
                        type: "proxy",
                        proxy: {
                          url: draft.masquerade?.proxy?.url || "",
                          rewriteHost: value,
                          insecure: draft.masquerade?.proxy?.insecure || false,
                        },
                      },
                    })
                  }
                />
                <ToggleField
                  label="Proxy Insecure"
                  checked={Boolean(draft.masquerade?.proxy?.insecure)}
                  onCheckedChange={(value) =>
                    onDraftChange({
                      ...draft,
                      masquerade: {
                        type: "proxy",
                        proxy: {
                          url: draft.masquerade?.proxy?.url || "",
                          rewriteHost: draft.masquerade?.proxy?.rewriteHost || false,
                          insecure: value,
                        },
                      },
                    })
                  }
                />
              </div>
            </div>
          )}

          {masqueradeType === "file" && (
            <Input
              label="File Dir"
              value={draft.masquerade?.file?.dir || ""}
              onChange={(event) =>
                onDraftChange({
                  ...draft,
                  masquerade: { type: "file", file: { dir: event.target.value } },
                })
              }
            />
          )}

          {masqueradeType === "string" && (
            <div className="grid gap-3 sm:grid-cols-12">
              <div className="sm:col-span-9">
                <label className="mb-2 block text-[13px] font-medium text-txt-secondary">String Content</label>
                <textarea
                  value={draft.masquerade?.string?.content || ""}
                  onChange={(event) =>
                    onDraftChange({
                      ...draft,
                      masquerade: {
                        type: "string",
                        string: {
                          content: event.target.value,
                          statusCode: draft.masquerade?.string?.statusCode || 200,
                        },
                      },
                    })
                  }
                  rows={4}
                  className="w-full rounded-xl bg-[var(--control-bg)] px-4 py-3 text-[14px] text-txt-primary shadow-[inset_0_0_0_1px_var(--control-border)] outline-none transition-colors focus:bg-[var(--control-bg-hover)] focus:shadow-[inset_0_0_0_1px_var(--accent),0_0_0_3px_var(--accent-soft)]"
                />
              </div>
              <div className="sm:col-span-3">
                <Input
                  label="Status"
                  type="number"
                  value={String(draft.masquerade?.string?.statusCode ?? 200)}
                  onChange={(event) => {
                    const parsed = Number.parseInt(event.target.value, 10);
                    onDraftChange({
                      ...draft,
                      masquerade: {
                        type: "string",
                        string: {
                          content: draft.masquerade?.string?.content || "",
                          statusCode: Number.isFinite(parsed) ? parsed : 200,
                        },
                      },
                    });
                  }}
                  min={100}
                  max={599}
                />
              </div>
            </div>
          )}
        </SectionCard>

        <SectionCard title="Performance" icon={<SlidersHorizontal size={17} strokeWidth={1.7} />}>
          <ToggleField
            label="Custom QUIC"
            checked={draft.quicEnabled}
            onCheckedChange={(value) => onDraftChange({ ...draft, quicEnabled: value })}
          />
          {draft.quicEnabled && (
            <div className="grid gap-3 sm:grid-cols-2">
              <Input
                label="QUIC Max Idle"
                value={draft.quic?.maxIdleTimeout || ""}
                onChange={(event) =>
                  onDraftChange({
                    ...draft,
                    quic: { ...(draft.quic || {}), maxIdleTimeout: event.target.value },
                  })
                }
                placeholder="30s"
              />
              <ToggleField
                label="Disable Path MTU"
                checked={Boolean(draft.quic?.disablePathMTUDiscovery)}
                onCheckedChange={(value) =>
                  onDraftChange({
                    ...draft,
                    quic: { ...(draft.quic || {}), disablePathMTUDiscovery: value },
                  })
                }
              />
            </div>
          )}
        </SectionCard>
      </div>

      <aside className="space-y-4 xl:sticky xl:top-6 xl:col-span-4 xl:self-start">
        <section className="panel-card-compact space-y-3">
          <SectionTitle icon={<Gauge size={16} strokeWidth={1.7} />} title="Snapshot" />
          <div className="grid gap-2">
            {snapshotItems.map((item) => (
              <div key={item.label} className="flex items-center justify-between rounded-lg bg-surface-3/35 px-3 py-2 text-[13px]">
                <span className="text-txt-secondary">{item.label}</span>
                <span className="font-medium text-txt-primary">{item.value || "-"}</span>
              </div>
            ))}
          </div>
        </section>

        <section className="panel-card-compact space-y-3">
          <SectionTitle icon={<Code size={16} strokeWidth={1.7} />} title="YAML" />
          <YamlPreview value={rawYaml} />
        </section>
      </aside>
    </div>
  );
}
