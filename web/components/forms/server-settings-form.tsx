import { Code, Network, Shield, SlidersHorizontal, Wrench } from "lucide-react";
import { ReactNode, useEffect } from "react";

import { Hy2Settings } from "@/domain/settings/types";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Toggle } from "@/src/components/ui";

function SectionTitle({ icon, title, description }: { icon: ReactNode; title: string; description?: string }) {
  return (
    <div className="flex items-center gap-2.5">
      <div className="grid h-8 w-8 shrink-0 place-items-center rounded-[8px] bg-accent/10">
        <span className="text-accent-light">{icon}</span>
      </div>
      <div>
        <h3 className="text-[13px] font-semibold text-white">{title}</h3>
        {description ? <p className="text-[11px] text-txt-muted">{description}</p> : null}
      </div>
    </div>
  );
}

function LabeledToggle({
  label,
  checked,
  onCheckedChange,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: (value: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between rounded-[10px] border border-border/60 bg-surface-1/50 px-3.5 py-2.5 transition-colors hover:border-border">
      <p className="text-[12px] text-txt-secondary">{label}</p>
      <Toggle checked={checked} onCheckedChange={onCheckedChange} />
    </div>
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
  onValueChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
}) {
  return (
    <div>
      <label className="mb-1.5 block text-[11px] font-medium text-txt-secondary">{label}</label>
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

export function ServerSettingsForm({
  draft,
  rawYaml,
  onDraftChange,
}: {
  draft: Hy2Settings;
  rawYaml: string;
  onDraftChange: (next: Hy2Settings) => void;
}) {
  const acmeDomains = (draft.acme?.domains || []).join(", ");
  const tlsMode = draft.tlsMode === "tls" ? "tls" : "acme";
  const obfsType = draft.obfs?.type === "salamander" ? "salamander" : "none";
  const masqueradeType = draft.masquerade?.type || "none";

  useEffect(() => {
    if (obfsType !== "none" && masqueradeType !== "none") {
      onDraftChange({ ...draft, masquerade: undefined });
    }
  }, [draft, masqueradeType, obfsType, onDraftChange]);

  return (
    <div className="space-y-4">
      <section className="space-y-5 rounded-[14px] border border-border/80 bg-surface-2 p-5">
        <SectionTitle icon={<Network size={15} strokeWidth={1.4} />} title="Connection Profile" description="Listener and encryption settings" />

        <div className="grid gap-3 md:grid-cols-4">
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
              { value: "tls", label: "Manual TLS" },
            ]}
          />

          <SelectField
            label="OBFS"
            value={obfsType}
            onValueChange={(value) => {
              if (value === "salamander") {
                onDraftChange({
                  ...draft,
                  obfs: { type: "salamander", salamander: { password: draft.obfs?.salamander?.password || "" } },
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
              onDraftChange({ ...draft, obfs: undefined, masquerade: { ...(draft.masquerade || {}), type: value } });
            }}
            options={[
              { value: "none", label: "Disabled" },
              { value: "proxy", label: "Proxy" },
              { value: "file", label: "File" },
              { value: "string", label: "String" },
            ]}
          />
        </div>

        {tlsMode === "acme" ? (
          <div className="grid gap-3 md:grid-cols-12">
            <div className="md:col-span-8">
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
            </div>
            <div className="md:col-span-4">
              <Input
                label="ACME Email"
                value={draft.acme?.email || ""}
                onChange={(event) => onDraftChange({ ...draft, acme: { domains: draft.acme?.domains || [], email: event.target.value } })}
              />
            </div>
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              label="TLS Cert Path"
              value={draft.tls?.cert || ""}
              onChange={(event) => onDraftChange({ ...draft, tls: { cert: event.target.value, key: draft.tls?.key || "" } })}
            />
            <Input
              label="TLS Key Path"
              value={draft.tls?.key || ""}
              onChange={(event) => onDraftChange({ ...draft, tls: { cert: draft.tls?.cert || "", key: event.target.value } })}
            />
          </div>
        )}

        {obfsType === "salamander" ? (
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
        ) : null}
      </section>

      <section className="space-y-5 rounded-[14px] border border-border/80 bg-surface-2 p-5">
        <SectionTitle icon={<Wrench size={15} strokeWidth={1.4} />} title="Runtime Defaults" description="Bandwidth, transport and protocol options" />

        <div className="grid gap-3 md:grid-cols-3">
          <LabeledToggle
            label="TLS Insecure"
            checked={Boolean(draft.clientTLSInsecure)}
            onCheckedChange={(value) => onDraftChange({ ...draft, clientTLSInsecure: value })}
          />
          <LabeledToggle
            label="Ignore Client Bandwidth"
            checked={Boolean(draft.ignoreClientBandwidth)}
            onCheckedChange={(value) => onDraftChange({ ...draft, ignoreClientBandwidth: value })}
          />
          <LabeledToggle
            label="Disable UDP"
            checked={Boolean(draft.disableUDP)}
            onCheckedChange={(value) => onDraftChange({ ...draft, disableUDP: value })}
          />
          <LabeledToggle
            label="Speed Test"
            checked={Boolean(draft.speedTest)}
            onCheckedChange={(value) => onDraftChange({ ...draft, speedTest: value })}
          />
          <Input
            label="Bandwidth Up"
            value={draft.bandwidth?.up || ""}
            onChange={(event) =>
              onDraftChange({
                ...draft,
                bandwidth: {
                  up: event.target.value,
                  down: draft.bandwidth?.down || "",
                },
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
                bandwidth: {
                  up: draft.bandwidth?.up || "",
                  down: event.target.value,
                },
              })
            }
            placeholder="200 mbps"
          />
        </div>

        <Input
          label="UDP Idle Timeout"
          value={draft.udpIdleTimeout || ""}
          onChange={(event) => onDraftChange({ ...draft, udpIdleTimeout: event.target.value })}
          placeholder="90s"
        />
      </section>

      {masqueradeType !== "none" ? (
        <section className="space-y-5 rounded-[14px] border border-border/80 bg-surface-2 p-5">
          <SectionTitle icon={<Shield size={15} strokeWidth={1.4} />} title="Masquerade Details" description="Traffic camouflage configuration" />

          {masqueradeType === "proxy" ? (
            <div className="space-y-3">
              <Input
                label="Masquerade Proxy URL"
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
              <div className="grid gap-3 md:grid-cols-2">
                <LabeledToggle
                  label="Rewrite Host Header"
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
                <LabeledToggle
                  label="Allow Insecure TLS"
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
          ) : null}

          {masqueradeType === "file" ? (
            <Input
              label="Masquerade File Dir"
              value={draft.masquerade?.file?.dir || ""}
              onChange={(event) =>
                onDraftChange({
                  ...draft,
                  masquerade: { type: "file", file: { dir: event.target.value } },
                })
              }
            />
          ) : null}

          {masqueradeType === "string" ? (
            <div className="grid gap-3 md:grid-cols-12">
              <div className="md:col-span-9">
                <label className="mb-1.5 block text-[11px] font-medium text-txt-secondary">Masquerade String Content</label>
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
                  rows={3}
                  className="w-full rounded-[8px] border border-border bg-surface-1 px-3 py-2 text-[12px] text-txt outline-none transition-all focus:border-accent/40 focus:shadow-[0_0_0_3px_rgba(99,102,241,0.06)]"
                />
              </div>
              <div className="md:col-span-3">
                <Input
                  label="Status Code"
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
          ) : null}
        </section>
      ) : null}

      <section className="space-y-5 rounded-[14px] border border-border/80 bg-surface-2 p-5">
        <SectionTitle icon={<SlidersHorizontal size={15} strokeWidth={1.4} />} title="QUIC Tuning" description="Advanced transport parameters" />
        <LabeledToggle
          label="Enable Custom QUIC"
          checked={draft.quicEnabled}
          onCheckedChange={(value) => onDraftChange({ ...draft, quicEnabled: value })}
        />
        {draft.quicEnabled ? (
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              label="QUIC Max Idle"
              value={draft.quic?.maxIdleTimeout || ""}
              onChange={(event) =>
                onDraftChange({
                  ...draft,
                  quic: {
                    ...(draft.quic || {}),
                    maxIdleTimeout: event.target.value,
                  },
                })
              }
              placeholder="30s"
            />
            <LabeledToggle
              label="Disable Path MTU Discovery"
              checked={Boolean(draft.quic?.disablePathMTUDiscovery)}
              onCheckedChange={(value) =>
                onDraftChange({
                  ...draft,
                  quic: {
                    ...(draft.quic || {}),
                    disablePathMTUDiscovery: value,
                  },
                })
              }
            />
          </div>
        ) : null}
      </section>

      <section className="space-y-4 rounded-[14px] border border-border/80 bg-surface-2 p-5">
        <SectionTitle icon={<Code size={15} strokeWidth={1.4} />} title="Generated YAML" description="Preview of the configuration file" />
        <textarea
          readOnly
          value={rawYaml}
          rows={16}
          className="w-full rounded-[10px] border border-border/60 bg-surface-0/80 px-4 py-3 font-mono text-[11px] leading-5 text-accent-light/80 outline-none"
        />
        <p className="text-[11px] text-txt-muted">Read-only preview of generated configuration</p>
      </section>
    </div>
  );
}
