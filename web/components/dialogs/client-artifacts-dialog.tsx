import { Copy, Link2, QrCode } from "lucide-react";

import { HysteriaClient, HysteriaUserPayload } from "@/domain/clients/types";
import { qrURL } from "@/domain/clients/services";
import { Button, Dialog } from "@/src/components/ui";

export function ClientArtifactsDialog({
  open,
  client,
  payload,
  loading,
  onClose,
  onCopy,
}: {
  open: boolean;
  client: HysteriaClient | null;
  payload: HysteriaUserPayload | null;
  loading: boolean;
  onClose: () => void;
  onCopy: (value: string) => void;
}) {
  const artifacts = payload?.artifacts || null;
  const currentClient = payload?.user || client;
  const shareURI = artifacts?.uri_hy2 || artifacts?.uri || "";
  const subscriptionURL = artifacts?.subscription_url || "";
  const shareQRSrc = currentClient ? `${qrURL(currentClient.id, 360, "access")}&v=${encodeURIComponent(shareURI)}` : "";
  const subscriptionQRSrc = currentClient ? `${qrURL(currentClient.id, 360, "subscription")}&v=${encodeURIComponent(subscriptionURL)}` : "";

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
      title={currentClient?.username || "Client"}
      contentClassName="max-w-[760px]"
      footer={
        <Button onClick={onClose} variant="ghost">
          Close
        </Button>
      }
    >
      {loading ? (
        <div className="flex min-h-[280px] items-center justify-center">
          <div className="flex flex-col items-center gap-3">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
            <p className="text-[12px] text-txt-secondary">Loading connection artifacts...</p>
          </div>
        </div>
      ) : artifacts && currentClient ? (
        <div className="grid gap-5 sm:grid-cols-2">
          <div className="space-y-3">
            <div className="flex items-center justify-center gap-1.5 text-[11px] font-medium text-txt-secondary">
              <QrCode size={13} strokeWidth={1.4} />
              Configuration QR
            </div>
            <div className="flex justify-center">
              <img
                alt="Configuration QR"
                src={shareQRSrc}
                className="h-[200px] w-[200px] rounded-[10px] border border-border bg-white p-1.5 shadow-sm"
              />
            </div>
            <Button variant="ghost" className="w-full justify-center" onClick={() => onCopy(shareURI)} disabled={!shareURI}>
              <Copy size={14} strokeWidth={1.4} />
              Copy Config Link
            </Button>
          </div>

          <div className="space-y-3">
            <div className="flex items-center justify-center gap-1.5 text-[11px] font-medium text-txt-secondary">
              <Link2 size={13} strokeWidth={1.4} />
              Subscription QR
            </div>
            <div className="flex justify-center">
              <img
                alt="Subscription QR"
                src={subscriptionQRSrc}
                className="h-[200px] w-[200px] rounded-[10px] border border-border bg-white p-1.5 shadow-sm"
              />
            </div>
            <Button variant="ghost" className="w-full justify-center" onClick={() => onCopy(subscriptionURL)} disabled={!subscriptionURL}>
              <Copy size={14} strokeWidth={1.4} />
              Copy Subscription URL
            </Button>
          </div>
        </div>
      ) : (
        <div className="flex min-h-[160px] flex-col items-center justify-center gap-3 rounded-[10px] border border-status-warning/15 bg-status-warning/5 p-6">
          <QrCode size={24} strokeWidth={1.2} className="text-status-warning/60" />
          <p className="text-[12px] text-status-warning">No active artifacts for this client.</p>
        </div>
      )}
    </Dialog>
  );
}
