import { Check, Copy, Loader2 } from "lucide-react";
import { useCallback, useState } from "react";

import type { Client, ClientArtifacts } from "@/domain/clients/types";
import { qrURL, subscriptionQRURL } from "@/domain/clients/services";
import { Button, Dialog } from "@/src/components/ui";

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
        label: "HY2",
        qrSrc: qrURL(client.id, artifacts.hy2_uris[0], 280),
        copyValue: artifacts.hy2_uris[0],
      });
    }
    if (artifacts.subscription_import_url) {
      panels.push({
        label: "Sing-box",
        qrSrc: subscriptionQRURL(client.id, 280),
        copyValue: artifacts.subscription_import_url,
      });
    }
    if (artifacts.subscription_clash_url) {
      panels.push({
        label: "Clash",
        qrSrc: qrURL(client.id, artifacts.subscription_clash_url, 280),
        copyValue: artifacts.subscription_clash_url,
      });
    }
    if (artifacts.subscription_base64_url) {
      panels.push({
        label: "Shadowrocket",
        qrSrc: qrURL(client.id, artifacts.subscription_base64_url, 280),
        copyValue: artifacts.subscription_base64_url,
      });
    }
  }

  const cols =
    panels.length >= 3
      ? "sm:grid-cols-3"
      : panels.length === 2
        ? "sm:grid-cols-2"
        : "sm:grid-cols-1";

  const qrSize =
    panels.length >= 3
      ? "h-[180px] w-[180px] sm:h-[220px] sm:w-[220px]"
      : "h-[220px] w-[220px] sm:h-[280px] sm:w-[280px]";

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => { if (!v) onClose(); }}
      title={client.username}
      contentClassName="max-w-[920px]"
      footer={<Button onClick={onClose}>Close</Button>}
    >
      {loading ? (
        <div className="flex min-h-[280px] items-center justify-center gap-2 text-[14px] text-txt-secondary">
          <Loader2 size={18} className="animate-spin" /> Loading...
        </div>
      ) : panels.length === 0 ? (
        <div className="flex min-h-[200px] items-center justify-center text-[14px] text-txt-secondary">
          No access configured for this user.
        </div>
      ) : (
        <div className={`grid gap-6 ${cols}`}>
          {panels.map((p) => (
            <div key={p.label} className="space-y-2">
              <span className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">
                {p.label}
              </span>
              <div className="w-fit rounded-lg bg-surface-2/65 p-2">
                {p.qrSrc ? (
                  <img src={p.qrSrc} alt={p.label} className={`${qrSize} rounded-md object-contain`} />
                ) : (
                  <div className={`${qrSize} rounded-md bg-surface-3`} />
                )}
              </div>
              <div className="flex gap-2">
                <input
                  readOnly
                  value={p.copyValue}
                  className="min-w-0 flex-1 truncate rounded-lg border border-border/60 bg-surface-1 px-3 py-1.5 text-[12px] text-txt-secondary"
                  onFocus={(e) => e.target.select()}
                />
                <Button
                  className="shrink-0"
                  onClick={() => copy(p.copyValue, p.label)}
                  disabled={!p.copyValue}
                >
                  {copied === p.label ? <Check size={14} /> : <Copy size={14} />}
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {artifacts && artifacts.all_uris.length > 0 && (
        <div className="mt-6 space-y-2 border-t border-border/40 pt-4">
          <span className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">All URIs</span>
          <div className="max-h-[160px] overflow-auto rounded-lg border border-border/40 bg-surface-1 p-3">
            {artifacts.all_uris.map((uri, i) => (
              <div key={i} className="group flex items-center gap-2 py-1">
                <code className="min-w-0 flex-1 truncate text-[11px] text-txt-primary">{uri}</code>
                <button
                  type="button"
                  className="shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                  onClick={() => copy(uri, `uri-${i}`)}
                >
                  {copied === `uri-${i}` ? <Check size={12} /> : <Copy size={12} className="text-txt-secondary" />}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </Dialog>
  );
}
