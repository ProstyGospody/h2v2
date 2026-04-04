import { Check, Copy, Loader2 } from "lucide-react";
import { useCallback, useState } from "react";

import type { Client, ClientArtifacts } from "@/domain/clients/types";
import { qrURL, subscriptionQRURL } from "@/domain/clients/services";
import { Button, Drawer } from "@/src/components/ui";

type Panel = { label: string; qrSrc: string; copyValue: string };

export function ClientArtifactsDialog({
  open,
  client,
  artifacts,
  loading,
  onClose,
}: {
  open: boolean;
  client: Client | null;
  artifacts: ClientArtifacts | null;
  loading: boolean;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState<string | null>(null);

  const copy = useCallback(async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(label);
      setTimeout(() => setCopied(null), 2000);
    } catch { /* ignore */ }
  }, []);

  if (!client) return null;

  const panels: Panel[] = [];
  if (artifacts) {
    if (artifacts.vless_uris.length > 0) {
      panels.push({
        label: "VLESS",
        qrSrc: qrURL(client.id, artifacts.vless_uris[0], 280),
        copyValue: artifacts.vless_uris[0],
      });
    }
    if (artifacts.hy2_uris.length > 0) {
      panels.push({
        label: "Hysteria2",
        qrSrc: qrURL(client.id, artifacts.hy2_uris[0], 280),
        copyValue: artifacts.hy2_uris[0],
      });
    }
    if (artifacts.subscription_import_url) {
      panels.push({
        label: "Sing-box subscription",
        qrSrc: subscriptionQRURL(client.id, 280),
        copyValue: artifacts.subscription_import_url,
      });
    }
    if (artifacts.subscription_clash_url) {
      panels.push({
        label: "Clash subscription",
        qrSrc: qrURL(client.id, artifacts.subscription_clash_url, 280),
        copyValue: artifacts.subscription_clash_url,
      });
    }
    if (artifacts.subscription_base64_url) {
      panels.push({
        label: "Shadowrocket / v2ray",
        qrSrc: qrURL(client.id, artifacts.subscription_base64_url, 280),
        copyValue: artifacts.subscription_base64_url,
      });
    }
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(v) => { if (!v) onClose(); }}
      title={client.username}
      description="Connection artifacts, QR codes, and subscription links."
      width="xl"
      footer={
        <div className="flex justify-end">
          <Button onClick={onClose}>Close</Button>
        </div>
      }
    >
      {loading ? (
        <div className="flex min-h-[280px] items-center justify-center gap-2 text-[14px] text-txt-secondary">
          <Loader2 size={18} className="animate-spin" /> Loading…
        </div>
      ) : panels.length === 0 ? (
        <div className="flex min-h-[200px] items-center justify-center text-[14px] text-txt-secondary">
          No access configured for this user.
        </div>
      ) : (
        <div className="space-y-5">
          {panels.map((p) => (
            <section
              key={p.label}
              className="rounded-2xl border border-border/40 bg-surface-1/40 p-4"
            >
              <div className="mb-3 flex items-center justify-between gap-2">
                <h3 className="text-[13px] font-semibold text-txt-primary">{p.label}</h3>
                <Button
                  size="sm"
                  onClick={() => copy(p.copyValue, p.label)}
                  disabled={!p.copyValue}
                >
                  {copied === p.label ? <Check size={13} /> : <Copy size={13} />}
                  {copied === p.label ? "Copied" : "Copy"}
                </Button>
              </div>
              <div className="flex items-start gap-4">
                <div className="shrink-0 rounded-xl bg-surface-2/65 p-2">
                  {p.qrSrc ? (
                    <img
                      src={p.qrSrc}
                      alt={p.label}
                      className="h-[168px] w-[168px] rounded-md object-contain"
                    />
                  ) : (
                    <div className="h-[168px] w-[168px] rounded-md bg-surface-3" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <textarea
                    readOnly
                    value={p.copyValue}
                    onFocus={(e) => e.currentTarget.select()}
                    rows={6}
                    className="h-[168px] w-full resize-none overflow-auto rounded-lg border border-border/50 bg-surface-1 p-3 font-mono text-[11px] leading-relaxed text-txt-secondary outline-none focus:border-accent/60"
                  />
                </div>
              </div>
            </section>
          ))}
        </div>
      )}

      {artifacts && artifacts.all_uris.length > 0 && (
        <details className="mt-5 rounded-2xl border border-border/40 bg-surface-1/40 p-4">
          <summary className="cursor-pointer text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">
            All URIs ({artifacts.all_uris.length})
          </summary>
          <div className="mt-3 space-y-1 border-t border-border/30 pt-3">
            {artifacts.all_uris.map((uri, i) => (
              <div key={i} className="group flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-surface-2/40">
                <code className="min-w-0 flex-1 truncate font-mono text-[11px] text-txt-primary">{uri}</code>
                <button
                  type="button"
                  className="shrink-0 rounded p-1 opacity-0 transition-opacity hover:bg-surface-3 group-hover:opacity-100"
                  onClick={() => copy(uri, `uri-${i}`)}
                  aria-label={`Copy URI ${i + 1}`}
                >
                  {copied === `uri-${i}` ? <Check size={12} /> : <Copy size={12} className="text-txt-secondary" />}
                </button>
              </div>
            ))}
          </div>
        </details>
      )}
    </Drawer>
  );
}
