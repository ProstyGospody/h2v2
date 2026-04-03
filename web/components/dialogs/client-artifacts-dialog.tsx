import { Copy } from "lucide-react";

import { Client, UserPayload } from "@/domain/clients/types";
import { Button, Dialog } from "@/src/components/ui";

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
  const accessURL = payload?.artifacts?.access_url || "";
  const subscriptionURL = payload?.artifacts?.subscription_url || "";
  const accessQRSource = payload?.artifacts?.access_qr_url || "";
  const subscriptionQRSource = payload?.artifacts?.subscription_qr_url || "";

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => { if (!next) onClose(); }}
      title={currentClient?.username || "User"}
      contentClassName="max-w-[860px]"
      footer={<Button onClick={onClose}>Close</Button>}
    >
      {loading ? (
        <div className="flex min-h-[260px] items-center justify-center text-[14px] text-txt-secondary">Loading</div>
      ) : (
        <div className="grid gap-5 sm:grid-cols-2">
          <div className="space-y-2">
            <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">URL</label>
            <div className="w-fit rounded-lg bg-surface-2/65 p-2">
              {accessQRSource ? (
                <img
                  src={accessQRSource}
                  alt="URL"
                  width={320}
                  height={320}
                  className="h-[220px] w-[220px] rounded-md object-contain sm:h-[320px] sm:w-[320px]"
                />
              ) : (
                <div className="h-[220px] w-[220px] rounded-md bg-surface-3 sm:h-[320px] sm:w-[320px]" />
              )}
            </div>
            <Button className="w-full sm:w-auto" onClick={() => onCopy(accessURL)} disabled={!accessURL}>
              <Copy size={16} strokeWidth={1.6} />Copy
            </Button>
          </div>

          <div className="space-y-2">
            <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">Subscription</label>
            <div className="w-fit rounded-lg bg-surface-2/65 p-2">
              {subscriptionQRSource ? (
                <img
                  src={subscriptionQRSource}
                  alt="Subscription"
                  width={320}
                  height={320}
                  className="h-[220px] w-[220px] rounded-md object-contain sm:h-[320px] sm:w-[320px]"
                />
              ) : (
                <div className="h-[220px] w-[220px] rounded-md bg-surface-3 sm:h-[320px] sm:w-[320px]" />
              )}
            </div>
            <Button className="w-full sm:w-auto" onClick={() => onCopy(subscriptionURL)} disabled={!subscriptionURL}>
              <Copy size={16} strokeWidth={1.6} />Copy
            </Button>
          </div>
        </div>
      )}
    </Dialog>
  );
}
