import { Copy, Link2, QrCode } from "lucide-react";

import { HysteriaClient, HysteriaUserPayload } from "@/domain/clients/types";
import { qrURL } from "@/domain/clients/services";
import { Button, Dialog } from "@/src/components/ui";

export function ClientArtifactsDialog({
  open, client, payload, loading, onClose, onCopy,
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
    <Dialog open={open} onOpenChange={(n) => { if (!n) onClose(); }} title={currentClient?.username || "Client"} contentClassName="max-w-[780px]"
      footer={<Button onClick={onClose} variant="ghost">Close</Button>}>
      {loading ? (
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="flex flex-col items-center gap-3">
            <div className="h-7 w-7 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
            <p className="text-[14px] text-txt-secondary">Loading connection artifacts...</p>
          </div>
        </div>
      ) : artifacts && currentClient ? (
        <div className="grid gap-6 sm:grid-cols-2">
          <div className="space-y-4">
            <div className="flex items-center justify-center gap-2 text-[13px] font-semibold text-txt-secondary">
              <QrCode size={16} strokeWidth={1.6} />Configuration QR
            </div>
            <div className="flex justify-center">
              <img alt="Configuration QR" src={shareQRSrc} className="h-[220px] w-[220px] rounded-xl border border-border bg-white p-2 shadow-sm" />
            </div>
            <Button variant="ghost" className="w-full justify-center" onClick={() => onCopy(shareURI)} disabled={!shareURI}>
              <Copy size={16} strokeWidth={1.6} />Copy Config Link
            </Button>
          </div>
          <div className="space-y-4">
            <div className="flex items-center justify-center gap-2 text-[13px] font-semibold text-txt-secondary">
              <Link2 size={16} strokeWidth={1.6} />Subscription QR
            </div>
            <div className="flex justify-center">
              <img alt="Subscription QR" src={subscriptionQRSrc} className="h-[220px] w-[220px] rounded-xl border border-border bg-white p-2 shadow-sm" />
            </div>
            <Button variant="ghost" className="w-full justify-center" onClick={() => onCopy(subscriptionURL)} disabled={!subscriptionURL}>
              <Copy size={16} strokeWidth={1.6} />Copy Subscription URL
            </Button>
          </div>
        </div>
      ) : (
        <div className="flex min-h-[180px] flex-col items-center justify-center gap-4 rounded-xl border border-status-warning/15 bg-status-warning/5 p-8">
          <QrCode size={28} strokeWidth={1.4} className="text-status-warning/60" />
          <p className="text-[14px] text-status-warning">No active artifacts for this client.</p>
        </div>
      )}
    </Dialog>
  );
}
