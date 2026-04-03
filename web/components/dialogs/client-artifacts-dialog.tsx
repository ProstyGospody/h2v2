import { Copy } from "lucide-react";

import { Client, UserPayload } from "@/domain/clients/types";
import { Button, Dialog } from "@/src/components/ui";

type QRPanel = {
  label: string;
  qrSrc: string;
  copyValue: string;
};

export function ClientArtifactsDialog({
  open, client, payload, loading, onClose, onCopy,
}: {
  open: boolean;
  client: Client | null;
  payload: UserPayload | null;
  loading: boolean;
  onClose: () => void;
  onCopy: (value: string) => void;
}) {
  const currentClient = payload?.user || client;
  const a = payload?.artifacts;

  const panels: QRPanel[] = [];
  if (a?.vless_uris && a.vless_uris.length > 0) {
    panels.push({ label: "VLESS", qrSrc: a.vless_qr_url, copyValue: a.vless_uris[0] });
  }
  if (a?.hy2_uris && a.hy2_uris.length > 0) {
    panels.push({ label: "HY2", qrSrc: a.hy2_qr_url, copyValue: a.hy2_uris[0] });
  }
  if (a?.subscription_url) {
    panels.push({ label: "Subscription", qrSrc: a.subscription_qr_url, copyValue: a.subscription_url });
  }

  const colClass = panels.length >= 3 ? "sm:grid-cols-3" : panels.length === 2 ? "sm:grid-cols-2" : "sm:grid-cols-1";
  const qrClass = panels.length >= 3
    ? "h-[180px] w-[180px] sm:h-[220px] sm:w-[220px]"
    : "h-[220px] w-[220px] sm:h-[280px] sm:w-[280px]";

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => { if (!next) onClose(); }}
      title={currentClient?.username || "User"}
      contentClassName="max-w-[900px]"
      footer={<Button onClick={onClose}>Close</Button>}
    >
      {loading ? (
        <div className="flex min-h-[260px] items-center justify-center text-[14px] text-txt-secondary">Loading</div>
      ) : (
        <div className={`grid gap-5 ${colClass}`}>
          {panels.map((panel) => (
            <div key={panel.label} className="space-y-2">
              <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">{panel.label}</label>
              <div className="w-fit rounded-lg bg-surface-2/65 p-2">
                {panel.qrSrc ? (
                  <img
                    src={panel.qrSrc}
                    alt={panel.label}
                    className={`${qrClass} rounded-md object-contain`}
                  />
                ) : (
                  <div className={`${qrClass} rounded-md bg-surface-3`} />
                )}
              </div>
              <Button className="w-full sm:w-auto" onClick={() => onCopy(panel.copyValue)} disabled={!panel.copyValue}>
                <Copy size={16} strokeWidth={1.6} />Copy
              </Button>
            </div>
          ))}
        </div>
      )}
    </Dialog>
  );
}
