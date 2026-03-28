import { Copy, Loader2 } from "lucide-react";

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
        <div className="flex min-h-[220px] items-center justify-center">
          <div className="flex flex-col items-center gap-2">
            <Loader2 size={20} strokeWidth={1.4} className="animate-spin text-accent-light" />
            <p className="text-[12px] text-txt-secondary">Loading connection artifacts...</p>
          </div>
        </div>
      ) : artifacts && currentClient ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <p className="text-center text-[11px] text-txt-secondary">Configuration QR</p>
            <img alt="Configuration QR" src={shareQRSrc} className="mx-auto h-[220px] w-[220px] rounded-btn border border-border bg-white p-1" />
            <Button variant="ghost" className="w-full justify-center" onClick={() => onCopy(shareURI)} disabled={!shareURI}>
              <Copy size={16} strokeWidth={1.4} />
              Copy Config Link
            </Button>
          </div>

          <div className="space-y-2">
            <p className="text-center text-[11px] text-txt-secondary">Subscription QR</p>
            <img alt="Subscription QR" src={subscriptionQRSrc} className="mx-auto h-[220px] w-[220px] rounded-btn border border-border bg-white p-1" />
            <Button variant="ghost" className="w-full justify-center" onClick={() => onCopy(subscriptionURL)} disabled={!subscriptionURL}>
              <Copy size={16} strokeWidth={1.4} />
              Copy Subscription URL
            </Button>
          </div>
        </div>
      ) : (
        <div className="rounded-btn border border-status-warning/20 bg-status-warning/10 px-3 py-2 text-[12px] text-status-warning">
          No active artifacts for this client.
        </div>
      )}
    </Dialog>
  );
}
