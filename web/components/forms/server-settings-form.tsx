import { Download, Gauge, Globe, Lock, Shield, SlidersHorizontal, Upload } from "lucide-react";
import { useEffect, useMemo } from "react";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { type Hy2Settings } from "@/domain/settings/types";
import { buildSnapshotItems } from "@/src/features/settings/server-settings-utils";
import { Button, Input, SectionCard, SectionTitle, SelectField, ToggleField } from "@/src/components/ui";

export function ServerSettingsForm({
  draft,
  onDraftChange,
  snapshotStorage,
}: {
  draft: Hy2Settings;
  onDraftChange: (next: Hy2Settings) => void;
  snapshotStorage?: {
    busy: boolean;
    restoreFileName: string;
    onBackup: () => void;
    onSelectRestore: () => void;
    onRestore: () => void | Promise<void>;
  };
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
    () => buildSnapshotItems(draft, tlsMode, obfsType, masqueradeType),
    [draft, tlsMode, obfsType, masqueradeType],
  );

  return (
    <div className="grid gap-4">
      <div className="min-w-0 space-y-4 xl:grid xl:grid-cols-2 xl:gap-4 xl:space-y-0">
        <SectionCard title="General" icon={<Globe size={17} strokeWidth={1.7} />}>
          <div className="grid gap-3 md:grid-cols-2">
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
          <div className="grid gap-3 md:grid-cols-2">
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
            <div className="grid gap-3 md:grid-cols-2">
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
            <div className="grid gap-3 md:grid-cols-2">
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
          <div className="grid gap-3 md:grid-cols-2">
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
            <Input
              label="UDP Idle Timeout"
              value={draft.udpIdleTimeout || ""}
              onChange={(event) => onDraftChange({ ...draft, udpIdleTimeout: event.target.value })}
              placeholder="90s"
            />
          </div>
          <ToggleField
            label="Disable UDP"
            checked={Boolean(draft.disableUDP)}
            onCheckedChange={(value) => onDraftChange({ ...draft, disableUDP: value })}
          />
        </SectionCard>

        <SectionCard title="Masking" icon={<Lock size={17} strokeWidth={1.7} />}>
          <div className="grid gap-3 md:grid-cols-2">
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
              <div className="grid gap-3 md:grid-cols-2">
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
            <div className="grid gap-3 md:grid-cols-12">
              <div className="md:col-span-9">
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
              <div className="md:col-span-3">
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

        <SectionCard title="Performance" icon={<SlidersHorizontal size={17} strokeWidth={1.7} />} className="xl:col-span-2">
          <ToggleField
            label="Custom QUIC"
            checked={draft.quicEnabled}
            onCheckedChange={(value) => onDraftChange({ ...draft, quicEnabled: value })}
          />
          {draft.quicEnabled && (
            <div className="grid gap-3 md:grid-cols-2">
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

      <aside className="min-w-0 w-full xl:w-[560px] xl:justify-self-start">
        <section className="panel-card-compact space-y-3">
          <SectionTitle icon={<Gauge size={16} strokeWidth={1.7} />} title="Snapshot" />
          <div className="grid gap-2">
            {snapshotItems.map((item) => (
              <div key={item.label} className="flex items-center gap-2 rounded-lg bg-surface-3/35 px-3 py-2 text-[13px]">
                <span className="w-[88px] shrink-0 text-txt-secondary">{item.label}</span>
                <span className="min-w-0 truncate font-medium text-txt-primary">{item.value || "-"}</span>
              </div>
            ))}
          </div>
          {snapshotStorage ? (
            <div className="space-y-2 pt-1">
              <Button size="sm" onClick={snapshotStorage.onBackup} disabled={snapshotStorage.busy} className="h-9 w-full justify-center">
                <Download size={14} strokeWidth={1.7} />
                Backup
              </Button>
              {snapshotStorage.restoreFileName ? (
                <>
                  <div className="break-all rounded-lg bg-surface-3/35 px-3 py-2 text-[12px] text-txt-secondary">{snapshotStorage.restoreFileName}</div>
                  <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                    <Button size="sm" onClick={snapshotStorage.onSelectRestore} disabled={snapshotStorage.busy} className="h-9 w-full justify-center">
                      <Upload size={14} strokeWidth={1.7} />
                      Select DB
                    </Button>
                    <ConfirmPopover
                      title="Restore database"
                      description={`Restore ${snapshotStorage.restoreFileName}?`}
                      confirmText="Restore"
                      onConfirm={snapshotStorage.onRestore}
                    >
                      <Button size="sm" variant="danger" disabled={snapshotStorage.busy} className="h-9 w-full justify-center">
                        <Upload size={14} strokeWidth={1.7} />
                        Restore
                      </Button>
                    </ConfirmPopover>
                  </div>
                </>
              ) : (
                <Button size="sm" variant="danger" onClick={snapshotStorage.onSelectRestore} disabled={snapshotStorage.busy} className="h-9 w-full justify-center">
                  <Upload size={14} strokeWidth={1.7} />
                  Restore
                </Button>
              )}
            </div>
          ) : null}
        </section>
      </aside>
    </div>
  );
}
