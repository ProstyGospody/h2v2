import { Check, Copy, Loader2 } from "lucide-react";
import { useCallback, useState } from "react";

import type { Client, ClientArtifacts } from "@/domain/clients/types";
import { qrURL, subscriptionQRURL } from "@/domain/clients/services";
import { Button, Drawer } from "@/src/components/ui";

type Panel = { label: string; qrSrc: string; value: string };

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

  const copy = useCallback(async (value: string, key: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      setTimeout(() => setCopied(null), 1500);
    } catch { /* ignore */ }
  }, []);

  if (!client) return null;

  const panels: Panel[] = [];
  if (artifacts) {
    if (artifacts.vless_uris.length > 0) {
      panels.push({
        label: "VLESS",
        qrSrc: qrURL(client.id, artifacts.vless_uris[0], 240),
        value: artifacts.vless_uris[0],
      });
    }
    if (artifacts.hy2_uris.length > 0) {
      panels.push({
        label: "Hysteria2",
        qrSrc: qrURL(client.id, artifacts.hy2_uris[0], 240),
        value: artifacts.hy2_uris[0],
      });
    }
    if (artifacts.subscription_import_url) {
      panels.push({
        label: "Sing-box",
        qrSrc: subscriptionQRURL(client.id, 240),
        value: artifacts.subscription_import_url,
      });
    }
    if (artifacts.subscription_clash_url) {
      panels.push({
        label: "Clash",
        qrSrc: qrURL(client.id, artifacts.subscription_clash_url, 240),
        value: artifacts.subscription_clash_url,
      });
    }
    if (artifacts.subscription_base64_url) {
      panels.push({
        label: "v2ray",
        qrSrc: qrURL(client.id, artifacts.subscription_base64_url, 240),
        value: artifacts.subscription_base64_url,
      });
    }
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(v) => { if (!v) onClose(); }}
      title={client.username}
      width="xl"
      footer={
        <div className="flex justify-end">
          <Button onClick={onClose}>Close</Button>
        </div>
      }
    >
      {loading ? (
        <div className="flex min-h-[240px] items-center justify-center gap-2 text-[13px] text-txt-secondary">
          <Loader2 size={16} className="animate-spin" /> Loading
        </div>
      ) : panels.length === 0 ? (
        <div className="flex min-h-[180px] items-center justify-center text-[13px] text-txt-muted">
          No access configured.
        </div>
      ) : (
        <div className="space-y-8">
          {panels.map((p) => (
            <section key={p.label}>
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-[13px] font-semibold uppercase tracking-wider text-txt-muted">
                  {p.label}
                </h3>
                <button
                  type="button"
                  onClick={() => copy(p.value, p.label)}
                  className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-[12px] font-medium text-txt-secondary transition-colors hover:bg-surface-3/50 hover:text-txt-primary"
                >
                  {copied === p.label ? <Check size={12} /> : <Copy size={12} />}
                  {copied === p.label ? "Copied" : "Copy"}
                </button>
              </div>
              <div className="flex items-start gap-4">
                {p.qrSrc ? (
                  <img
                    src={p.qrSrc}
                    alt=""
                    className="h-[148px] w-[148px] shrink-0 rounded-lg bg-white p-1.5"
                  />
                ) : null}
                <textarea
                  readOnly
                  value={p.value}
                  onFocus={(e) => e.currentTarget.select()}
                  rows={6}
                  className="h-[148px] w-full resize-none overflow-auto rounded-lg bg-surface-1/60 p-3 font-mono text-[11px] leading-relaxed text-txt-secondary outline-none"
                />
              </div>
            </section>
          ))}

          {artifacts && artifacts.all_uris.length > panels.length ? (
            <details className="group">
              <summary className="cursor-pointer list-none text-[12px] font-semibold uppercase tracking-wider text-txt-muted hover:text-txt-secondary">
                All URIs ({artifacts.all_uris.length})
              </summary>
              <div className="mt-3 space-y-1">
                {artifacts.all_uris.map((uri, i) => (
                  <div
                    key={i}
                    className="group/row flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-surface-2/40"
                  >
                    <code className="min-w-0 flex-1 truncate font-mono text-[11px] text-txt-secondary">
                      {uri}
                    </code>
                    <button
                      type="button"
                      className="shrink-0 rounded p-1 opacity-0 transition-opacity hover:bg-surface-3/60 group-hover/row:opacity-100"
                      onClick={() => copy(uri, `uri-${i}`)}
                      aria-label="Copy"
                    >
                      {copied === `uri-${i}` ? (
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
    </Drawer>
  );
}
